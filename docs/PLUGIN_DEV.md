# licode 插件系统开发文档（Cordis-Go 运行时）

> 对应实现：`cordis/` 包。方案背景见 `docs/PLUGIN_SYSTEM.md`。

## 1. 快速上手

```go
r := cordis.NewRuntime(cordis.WithOutput(os.Stderr))
defer r.Shutdown()

// 引导服务（核心/HTTP 层注入既有实现）：
r.Provide("settings", settingsStore)  // 服务名约定：settings / session / llm / skills

// 内置插件加载（阶段二会改为 plugins/ 注册表）：
err := r.Load(cordis.Plugin{
    Name:   "builtin-llm-openai",
    Inject: []string{"settings"},
    Apply: func(ctx cordis.Context) error {
        s, ok := ctx.Get("settings")
        if !ok {
            return errors.New("settings missing")
        }
        return ctx.Provide("llm", newOpenAIClient(s.(*settings.Setting)))
    },
})
```

## 2. 核心概念

### 2.1 Fiber 生命周期

```
Load(Plugin) ──deps 就绪──▶ ACTIVE ──Unload──▶ DISPOSED（永久）
     │                        │
   deps 缺失                  │ 依赖服务被 Remove/替换
     ▼                        ▼
  PENDING ◀──────级联失效（自动回滚副作用）───────┘
     │
 服务恢复（同名服务再次 Provide）→ 自动重新 Apply
```

- `FiberState`：`StatePending` / `StateActive` / `StateDisposed`。
- `Runtime.FiberState(name)` 查询；`Runtime.Status()` 拿全量快照。
- Apply 返回错误或 panic：该插件已注册的副作用**自动回滚**，回到 PENDING，
  `Load` 返回错误；不污染其他插件。

### 2.2 Context API（插件作者唯一需要的接口）

| 方法 | 作用 | 卸载时自动回滚 |
|---|---|---|
| `Provide(name, svc)` | 注册服务 | 是（并级联失效依赖者） |
| `Get(name)` | 读取服务 | — |
| `Inject(deps, fn)` | 依赖就绪后执行 fn；依赖消失回滚 fn 的副作用并重新等待 | 是 |
| `Effect(fn)` | 自定义可逆副作用，返回 Disposer | 是（LIFO） |
| `On(event, h)` / `OnPriority` | 注册 waterfall 监听器 | 是 |
| `Emit(event, args...)` | 广播事件（fire-and-forget） | — |
| `Waterfall(event, input, next)` | 执行决策链 | — |
| `RegisterTool(t)` | 注册工具 | 是 |
| `Logger()` | 运行时日志 | — |

关键规则：**Apply 内的一切注册都必须通过 Context**，禁止裸用 `runtime.Goexit` 前的
goroutine 常驻资源而不注册 Effect——需要后台协程时这样写：

```go
func Apply(ctx cordis.Context) error {
    stop := make(chan struct{})
    go watcher(stop)
    return ctx.Effect(func() (func(), error) {
        return func() { close(stop) }, nil
    })
}
```

### 2.3 响应式依赖（服务消失 → 级联卸载 → 恢复 → 自动重载）

```go
r.Provide("llm", openai)   // consumer(Inject: ["llm"]) 被激活
r.Remove("llm")            // consumer 自动 Dispose：工具/监听器/服务全部回滚，回到 PENDING
r.Provide("llm", claude)   // consumer 自动重新 Apply，看到新 llm 服务
```

同名的服务替换（`Provide` 已存在的名字）等价于 Remove+Provide：旧归属的依赖者
被级联卸载，新值生效后统一唤醒。这就是 Provider 热切换。

级联是递归的：A→B→C 依赖链中 A 的服务消失，B、C 依次卸载、依次等待。

### 2.4 waterfall 事件链

`Handler = func(ctx Context, input any, next func(any) (any, error)) (any, error)`

- 放行并改写：`return next(modified)`
- 短路（不执行核心动作与后续处理器）：`return result, nil`
- 中止全链：`return nil, err`
- 排序：priority 升序，同 priority 按注册顺序（FIFO）。`OnPriority(-10, ...)` 做前置拦截。

约定事件：

| 事件 | input 类型 | next 语义 |
|---|---|---|
| `tools/pre-execute` | `ToolCall` | 执行工具（含 post 链） |
| `tools/post-execute` | `ToolResult` | 返回结果 |
| `agent/pre-step` | （阶段二定义 StepInput） | 发起 LLM 调用 |
| `agent/post-step` | （阶段二定义 StepOutput） | 落库/继续循环 |
| `session/pre-save` | （阶段二定义） | 持久化会话 |

工具管道入口（HTTP/Agent 层调用）：

```go
tr, _ := r.Get(cordis.ServiceTools)
reg := tr.(*cordis.ToolRegistry)
out, err := reg.Execute(ctx, "Read", map[string]any{"path": "main.go"})
```

`Execute` 自动走 `pre → execute → post`；权限、审计、脱敏都是监听器的事，
调用方零感知。

### 2.5 detached context

事件处理器与工具执行器收到的是 *detached* Context：可以 `Get/Emit/Waterfall/Logger`，
但注册类 API（`Provide/On/RegisterTool/Inject/Effect`）返回 `ErrNoFiber`——
没有归属 Fiber 的副作用无法回滚，框架直接拒绝。需要注册请回到插件 Apply。

## 3. 并发与确定性模型

- **生命周期入口串行**：`Load/Unload/Reload/Shutdown/Provide/Remove` 由一把内核锁
  串行化；插件 `Apply` 与 Effect 回滚在单线程语义下按序执行，不存在并发 Apply、
  并发 Dispose，插件作者可放心写直白代码。
- **waterfall 链内串行**：一次工具调用的 pre/post 链在同一 goroutine 串行执行，
  这从结构上避免了旧插件系统"多会话并发写同一插件进程 stdin/stdout"的问题。
- 读取类 API（`Get/Emit/Waterfall/Status`）可在任意 goroutine 调用，内部用
  读写锁保护；事件处理器快照后在锁外调用，插件里阻塞一个 handler 只影响该事件。

## 4. 与现有代码的接线（阶段二迁移指南）

| 现在 | 迁移为 |
|---|---|
| `settings.BuildAgent()` 直接装配 | `builtin-agent-loop` 插件：`Inject: ["settings","session","llm","tools"]`，`Apply` 中注册 `agent` 服务（一个 `Run(req) <-chan Event` 适配器） |
| `agent.Registry.Register(Tool{...})` | `ctx.RegisterTool(cordis.Tool{...})`，工具实现平移，`Run` 即 `Execute` |
| `agent.Run()` 中的 `onEvent(EventToolStart...)` | 拆为 `tools/pre-execute` / `tools/post-execute` 监听器（权限检查=pre 短路，脱敏=post 改写） |
| `ai.LLMClient` 选择逻辑 | `builtin-llm-openai`/`builtin-llm-claude` 插件各自 `Provide("llm", ...)`；HTTP 的 Provider 切换按钮 → `Runtime.Unload(old)` + `Load(new)` |
| `Agent.Ask` 权限确认 | `agent/permissions` 服务 + pre-execute 监听器：`ask` 模式发 WebSocket 确认，拒绝则设 `ToolCall.Output` 短路 |
| `internal/agent/mcp.go` MCP stdio 客户端 | `builtin-mcp` 插件；同时作为阶段三外部插件（stdio JSON-RPC）的通信层 |

迁移顺序建议（每步都能 `go build && go test`）：

1. ✅ `cmd/serve.go` 启动时创建 `cordis.Runtime`（`serverState.cordis/toolReg`），
   `plugins.RegisterAll` 加载内置插件，关停时 `rt.Shutdown()`。
2. ✅ `plugins/builtin-tools-fs` / `builtin-tools-shell`：桥接 `RegisterDefaultTools`
   的实现注册进 Cordis 工具树（含 `~/.licode/tools` 外部命令工具快照）；
   `agent.Registry` 保留为列表/回退门面，`Agent.Pipeline` 非空时执行走
   `ToolRegistry.Execute`（未知工具名自动回退旧 Registry）。
3. ✅ 权限外置：`plugins/builtin-permissions` 在 `tools/pre-execute`（priority -20）
   处理 deny/ask/plan 只读；策略经 `agent.RunHooks`（Go context）透传
   （`Agent.RunWithAttachments` 自动注入 `Permissions/Mode/Ask/AutoAllowPaths`）。
   脱敏暂保留在 Run 循环（`RedactSecrets` 幂等，可平滑迁到 post 监听器）。
4. ✅ `builtin-llm-*` Provider 工厂插件：每个协议（openai/claude/ollama/gemini）
   把构造器注册为 `llm.factory.<type>` 服务；`builtin-llm` 编排器 Inject
   `llm.config` 后**动态 Inject** 对应工厂——配置替换或工厂插件下线都会级联
   注销 `llm` 服务并自动重建（`serve.buildClient`：设置保存/SIGHUP 热重载
   即 Provider 热切换）。`ai.New` 保留为兼容入口。
5. ✅ waterfall 节点打通：`Agent.Cordis` 非空时循环发射 `agent/pre-step`
   （`StepInput`：改写 System/Messages/Tools 后进请求）与 `agent/post-step`
   （`StepOutput`：改写后再入库）；会话管理器新增 `PreSave` 钩子，serve 接线
   `session/pre-save`（payload 为 `*session.Session`，监听器就地加工）。

阶段二全部完成。后续（阶段三）：`~/.licode/plugins/` 外部进程插件经
MCPPlugin 适配器接入同一棵插件树。

## 5. 第三方（外部进程）插件协议（阶段三）

```
~/.licode/plugins/<name>/
├── plugin.json     # name/version/entry/inject/provide/permissions/auto_start
└── <entry>         # 可执行文件，stdio 上跑 MCP JSON-RPC
```

- 核心用 `MCPPlugin` 适配器包装：`Apply` 时拉起进程（经 `internal/procutil`），
  `tools/list` 结果逐个转成 `ctx.RegisterTool`，`tools/call` 路由到子进程；
  进程崩溃 → 适配器 `return err` → Fiber 回滚工具，核心不受影响。
- manifest 的 `inject` 在核心侧生效（服务不齐则插件不启动）。
- 用户手动配置的 MCP Server 保持原逻辑不变；插件 = 有 manifest/依赖/权限的 MCP Server 超集。

## 6. 已验证行为（`cordis/cordis_test.go`）

- Effect LIFO 回滚顺序；
- 级联卸载 + 服务恢复自动重载（工具/监听器随 Fiber 生死自动增删）；
- Provider 热切换（unload openai → load claude → consumer 重载）；
- waterfall 改写/短路/出错中止；
- 工具管道 deny 短路、post 改写、未知工具；
- `ctx.Inject` 子 Fiber 的响应式回滚与重跑；
- Apply 失败/panic 自动回滚；detached context 拒绝注册；
- Shutdown 逆序卸载；卸载后注册立即回滚。

## 7. 已知边界（v1）

- 同一次级联失效中，服务替换后的唤醒发生在同一调用栈内，同步完成；
  插件 Apply 必须尽快返回（长任务放 goroutine + Effect 注册取消）。
- `waterfall` 不支持异步/并发 handler（设计使然，链内串行）。
- 事件 input 目前为 `any`，`agent/*` 的结构体契约在阶段二定稿。
