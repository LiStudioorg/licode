# licode 插件系统方案 —— 基于 dsh/Cordis 范式

## 一、设计原则

licode 要实现的不是普通的"工具扩展"，而是 dsh 式的"一切皆插件"微内核架构。核心设计原则只有三条，直接来自 Cordis 的论文《A Programming Paradigm for Spatiotemporal Composability》：

1. **可逆副作用**：插件加载时注册的任何资源（事件监听、服务、定时器、工具），卸载时由框架自动按 LIFO 顺序回滚，不需要插件作者手写清理逻辑。
2. **无特权核心**：没有"需要打补丁的核心代码"。AI Provider、工具注册表、会话管理、甚至主循环本身，都是普通的 Cordis 插件。
3. **响应式依赖**：插件通过 inject 声明它需要哪些服务。服务消失时，依赖它的插件自动卸载；服务恢复后，自动重新加载。

## 二、架构分层

```
licode 二进制
│
├── cordis/                    ← 新增：Cordis-Go 运行时（微内核）
│   ├── fiber.go               ← Fiber 生命周期状态机
│   ├── effect.go              ← 可逆 Effect 栈
│   ├── injector.go            ← 依赖注入 + 反应式重载
│   ├── events.go              ← 类型化事件总线（含 waterfall）
│   └── context.go             ← Context 接口定义
│
├── plugins/                   ← 新增：内置插件（编译进二进制）
│   ├── builtin-llm-openai/    ← OpenAI Provider 插件
│   ├── builtin-llm-claude/    ← Claude Provider 插件
│   ├── builtin-tools-fs/      ← 文件工具插件（Read/Write/Edit/List/Grep/Glob）
│   ├── builtin-tools-shell/   ← Shell 工具插件
│   ├── builtin-agent-loop/    ← Agent 主循环插件
│   ├── builtin-subagent/      ← 子代理调度插件
│   └── builtin-mcp/           ← MCP 桥接插件
│
├── internal/                  ← 现有代码，逐步改造为插件
│   ├── session/               → 改造为 ctx.session 服务
│   ├── settings/              → 改造为 ctx.settings 服务
│   └── procutil/              → 作为外部插件进程管理的基础
│
└── ~/.licode/plugins/         ← 用户安装的第三方插件（外部进程）
    ├── my-plugin/
    │   ├── plugin.json
    │   └── my-plugin          ← 可执行文件（任意语言）
    └── ...
```

## 三、Cordis-Go 运行时核心机制

### 3.1 Context —— 共享服务总线

Context 是所有插件共享的同一个实例，承担服务注册、事件发布/订阅、副作用绑定的职责：

```go
// cordis/context.go
type Context interface {
    Provide(name string, svc any)
    Get(name string) (any, bool)
    Inject(deps []string, fn func(ctx Context))

    Effect(fn func() (disposer func(), err error)) error

    On(event string, handler Handler)
    Emit(event string, args ...any)
    Waterfall(event string, input any, next func(any) (any, error)) (any, error)

    RegisterTool(tool Tool)

    Logger() *slog.Logger
}
```

### 3.2 Fiber —— 插件生命周期状态机

每个插件加载时创建一个 Fiber，状态机：`PENDING → ACTIVE → DISPOSED`（可回环）。

- `Provide`、`On`、`RegisterTool`、`Inject` 内部都走 `f.Effect`，因此框架自动追踪所有副作用，插件作者无需手写清理代码。
- `Dispose` 逆序执行清理栈，再按 LIFO 卸载依赖本插件 Provide 服务的下游 Fiber。

### 3.3 Injector —— 响应式依赖注入

```go
type Injector struct {
    providers map[string]any
    waiters   map[string][]*Fiber
    dependents map[string][]*Fiber
}
```

- 服务注册：唤醒等待该服务的 Fiber，依次尝试激活（依赖全部就绪才激活）。
- 服务注销：**级联失效**——所有 `inject` 了该服务的 Fiber 自动 Dispose，回到 PENDING 并重新登记等待；服务恢复后自动重新加载。

### 3.4 事件总线 —— 支持 waterfall 决策链

工具执行管道是一条 waterfall 链：`tools/pre-execute → tools/execute → tools/post-execute`。每个监听器收到参数和 `next()` 延续，可选择修改输入、短路或放行。

licode Agent 主循环的 waterfall 节点：

| 事件名 | 触发时机 | 插件可做什么 |
|---|---|---|
| `agent/pre-step` | LLM 调用前 | 修改 system prompt、注入上下文 |
| `agent/post-step` | LLM 响应后 | 审计输出、拦截敏感内容 |
| `tools/pre-execute` | 工具执行前 | 权限检查、参数改写、短路拒绝 |
| `tools/post-execute` | 工具执行后 | 结果脱敏、格式化、缓存 |
| `session/pre-save` | 会话保存前 | 添加元数据、压缩上下文 |

## 四、插件规范

### 4.1 内置插件（编译进二进制）

```go
// plugins/builtin-tools-fs/plugin.go
package fstools

import "licode/cordis"

const Name = "builtin-tools-fs"

var Inject = []string{"permissions"}

func Apply(ctx cordis.Context) error {
    ctx.RegisterTool(cordis.Tool{
        Name:        "Read",
        Description: "读取文件（支持 offset/limit）",
        Permission:  "allow",
        Execute:     readFile,
    })
    return nil
}
```

### 4.2 第三方插件（外部进程）

```
~/.licode/plugins/my-plugin/
├── plugin.json
└── my-plugin           # 可执行文件（任意语言）
```

```json
{
  "name": "git-helper",
  "version": "1.0.0",
  "entry": "./git-helper",
  "description": "Git 操作辅助工具",
  "inject": ["tools", "llm"],
  "provide": ["git_service"],
  "permissions": ["shell:git", "file:read"],
  "auto_start": false
}
```

核心侧的 `MCPPlugin` 适配器把外部进程包装为标准 Plugin 接口，与内置插件共存于同一棵 Cordis 插件树。插件启动后，核心通过 MCP `tools/list` 动态发现其工具，无需在 manifest 中重复定义。

## 五、改造路径

| 现有模块 | 改造后 | 说明 |
|---|---|---|
| ai/ 包中的 Provider | ctx.llm 服务 | 每个 Provider 一个插件，切换 = 卸载旧插件 + 加载新插件 |
| agent/tools.go 内置工具 | ctx.tools 服务 | 文件/Shell/Grep 拆为插件，通过 RegisterTool 注册 |
| Agent 主循环 | builtin-agent-loop 插件 | 监听 agent/pre-step 等事件 |
| session/ | ctx.session 服务 | 基础服务 |
| settings/ | ctx.settings 服务 | 基础服务 |
| Skills 系统 | ctx.skills 服务 | 通过 Context 暴露 |
| MCP 客户端 | builtin-mcp 插件 | 同时作为外部插件的通信层 |

核心启动流程：`main()` 只做三件事——创建 Cordis 运行时、加载内置插件、启动 HTTP 服务。

## 六、分阶段实施

- **阶段一（进行中）**：`cordis/` 包完整实现 Context、Fiber、Effect、Injector、EventBus。验收：卸载/加载插件即可切换 AI Provider，无需重启。
- **阶段二**：`plugins/` 内置插件化，main() 精简为运行时引导。
- **阶段三**：MCPPlugin 适配器接入 `~/.licode/plugins/` 外部进程插件，崩溃隔离。
- **阶段四（按需）**：Bundle/Profile 组装层。

## 七、关键设计决策

- **不用 Go 原生 plugin 包**：要求插件与主程序完全相同的 Go 版本和构建标签，`plugin.Close` 是空实现不支持热卸载。licode 单二进制跨平台分发，限制不可接受。
- **内置 + 外部进程双轨**：内置零开销无隔离（官方可信），外部进程有 IPC 开销但崩溃隔离、任意语言（第三方不可信）。通过 MCPPlugin 适配器统一进同一棵插件树。
- **与旧插件系统的区别**：旧系统并发模型出问题（多会话并发调用同一插件进程的 stdin/stdout）。Cordis 通过 Fiber 生命周期 + waterfall 串行化天然避免：每个工具调用走 waterfall 链，链内串行。
- **与 MCP 的关系**：MCP 是外部插件的通信底层而非替代品。用户手动配置的 MCP Server 保持原逻辑；插件是"自动发现 + manifest + 依赖声明 + 权限模型"的 MCP Server 超集。
