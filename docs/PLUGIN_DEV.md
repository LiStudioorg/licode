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

## 5. 第三方（外部进程）插件协议（阶段三 ✅ 已实现）

```
~/.licode/plugins/<name>/
├── plugin.json     # 清单（见下）
└── <entry>         # 可执行文件或解释器脚本，stdio 上跑 MCP JSON-RPC
```

`plugin.json`（宽松解析，未知字段忽略）：

```json
{
  "name": "git-helper",
  "version": "1.0.0",
  "entry": "./git-helper",
  "args": [],
  "description": "Git 操作辅助工具",
  "inject": ["llm"],
  "provide": ["git_service"],
  "permissions": ["shell:git", "file:read"],
  "auto_start": true
}
```

- **标识以目录名为准**（`name` 仅作显示名）；插件在插件树中的 Fiber 名为
  `ext-<目录名>`，其工具沿用 MCP 命名 `mcp__<目录名>__<tool>`。
- `entry` 含路径分隔符时相对插件目录解析并确保可执行；裸命令名（如 `python3`）
  按 PATH 解析，`args` 传参，子进程工作目录 = 插件目录。
- 工具发现：握手后调用 MCP `tools/list` 动态枚举，无需在清单重复定义。
  若发现 0 工具且无 `provide`，视为无效插件（跳过并告警）。
- `inject` 在核心侧生效（依赖服务不齐则插件保持 PENDING）。
- `permissions` 兼容字符串数组或对象（旧格式），当前仅作声明，
  实际权限由 `builtin-permissions` 依据 `ext-<name>` 工具名统一管理。

最小可用的 Python 插件（stdio MCP，单文件即可）：

```python
#!/usr/bin/env python3
import json, sys

def read_msg():
    line = sys.stdin.buffer.readline()
    if not line: return None
    if line.lstrip().startswith(b"{"): return json.loads(line)
    hdr = {}
    while line not in (b"\r\n", b"\n", b""):
        k, _, v = line.decode().partition(":"); hdr[k.strip().lower()] = v.strip()
        line = sys.stdin.buffer.readline()
    return json.loads(sys.stdin.buffer.read(int(hdr.get("content-length", "0"))))

def send(rid, result):
    b = json.dumps({"jsonrpc": "2.0", "id": rid, "result": result}).encode()
    sys.stdout.buffer.write(b"Content-Length: %d\r\n\r\n" % len(b) + b); sys.stdout.buffer.flush()

while True:
    m = read_msg()
    if m is None: break
    rid, method = m.get("id"), m.get("method")
    if rid is None: continue
    if method == "initialize":
        send(rid, {"protocolVersion": "2024-11-05", "capabilities": {"tools": {}},
                   "serverInfo": {"name": "git-helper", "version": "1.0.0"}})
    elif method == "tools/list":
        send(rid, {"tools": [{"name": "status", "description": "git status",
               "inputSchema": {"type": "object"}}]})
    elif method == "tools/call":
        # 执行真实逻辑，返回 MCP content 数组
        send(rid, {"content": [{"type": "text", "text": "...output..."}]})
    else:
        send(rid, {})
```

### 崩溃隔离

- 子进程握手失败 → 插件 `Apply` 返回错误 → Fiber 回滚，只记录告警，核心与其他插件不受影响。
- 运行期子进程崩溃 → stdio `readLoop` 退出即触发连接关闭，挂起的 `tools/call`
  立即返回错误（不再等 30s 超时），核心无感。
- 卸载（或 `rt.Shutdown()`）→ 框架自动关闭 MCP 连接并 kill 子进程（幂等）。

> 注：`~/.licode/plugins/sample` 是上一代插件系统遗留（工具写在 `contributes.tools`、
> 未实现 MCP `tools/list`，且脚本含语法错误）。新系统以 `tools/list` 为准，
> 该样例会被判定为“0 工具”而跳过。需要示例请复制上面的最小 Python 插件。

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
