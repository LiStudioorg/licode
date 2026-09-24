package cordis

import (
	"context"
	"log"
)

// Plugin 描述一个可加载的内置插件。
//
// Apply 在插件依赖（Inject）全部就绪后被调用；Apply 内通过 Context 注册的
// 一切副作用（Provide/On/RegisterTool/Effect/Inject）都由框架自动回滚。
type Plugin struct {
	// Name 插件唯一标识，同时用作 Fiber 的 id。
	Name string
	// Inject 依赖的服务名列表；全部就绪后插件才会被激活。
	Inject []string
	// Apply 插件入口。返回错误时本次加载失败，已注册的副作用会被回滚。
	Apply func(ctx Context) error
}

// Disposer 撤销一个 Effect，使其产生的资源回到 Effect 之前。
type Disposer func()

// Effect 执行一个可逆副作用：fn 应用变更并返回其 Disposer；
// Disposer 会在 Fiber 卸载时由框架按 LIFO 顺序调用。
type Effect func() (Disposer, error)

// PermissionMode 是工具权限模式。
const (
	PermissionAllow = "allow"
	PermissionAsk   = "ask"
	PermissionDeny  = "deny"
)

// Tool 是注册进运行时的可调用工具。
type Tool struct {
	// Name 工具唯一名称。
	Name string
	// Description 给模型看的说明。
	Description string
	// Schema 工具参数的 JSON Schema。
	Schema map[string]any
	// Permission 默认权限模式：allow/ask/deny（空视为 allow）。
	Permission string
	// Execute 执行工具，返回文本输出。ctx 为调用方（Agent 循环）的 Go 上下文。
	Execute func(ctx context.Context, args map[string]any) (string, error)
}

// ToolCall 是 tools/pre-execute waterfall 的输入。
type ToolCall struct {
	Name string
	Args map[string]any
	// Ctx 是发起本次工具调用的 Go 上下文，处理器可从中读取每次运行的
	// 钩子（权限/确认回调等），并响应取消。
	Ctx context.Context
	// Output 非 nil 时，Pre 处理器用它短路工具执行，其值直接作为工具结果。
	Output *string
}

// ToolResult 是工具执行结果，也是 tools/post-execute waterfall 的载体。
type ToolResult struct {
	Name   string
	Output string
	Err    error
}

// Handler 是 waterfall 事件处理器。
//
// 处理器可以：
//   - 修改 input 后调用 next(修改后的 input) 放行；
//   - 直接返回 (result, nil) 短路整条链（不触发后续处理器与核心动作）；
//   - 返回 (nil, err) 中止整条链。
type Handler func(ctx Context, input any, next func(any) (any, error)) (any, error)

// Logger 返回运行时日志器。
type loggerProvider interface {
	Logger() *log.Logger
}
