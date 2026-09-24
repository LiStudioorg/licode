// Package cordis 是 licode 的微内核插件运行时，实现 Cordis 范式的三条核心原则：
//
//  1. 可逆副作用：插件通过 Context 注册的一切（服务、事件监听、工具）都自动
//     登记为 Effect，卸载时由框架按 LIFO 顺序自动回滚，插件作者无需手写清理。
//  2. 无特权核心：AI Provider、工具、会话等都是普通插件，核心只负责生命周期、
//     服务总线与事件总线。
//  3. 响应式依赖：插件通过 Inject 声明依赖；服务消失时依赖者自动级联卸载并
//     回到 PENDING，服务恢复后自动重新加载（Provider 热切换的本质）。
//
// 插件示例：
//
//	plugin := cordis.Plugin{
//	    Name:   "builtin-llm-openai",
//	    Inject: []string{"settings"},
//	    Apply: func(ctx cordis.Context) error {
//	        settings, _ := ctx.Get("settings")
//	        ctx.Provide("llm", newOpenAIClient(settings))
//	        return nil
//	    },
//	}
//	r := cordis.NewRuntime()
//	r.Load(plugin)
package cordis

import "errors"

// 运行时错误。
var (
	// ErrPluginNotFound 指定名称的插件未加载。
	ErrPluginNotFound = errors.New("cordis: plugin not found")
	// ErrPluginExists 同名插件已经存在且尚未卸载。
	ErrPluginExists = errors.New("cordis: plugin already loaded")
	// ErrFiberDisposed 向已卸载的 Fiber 注册 Effect 时返回。
	ErrFiberDisposed = errors.New("cordis: fiber already disposed")
	// ErrToolUnknown 调用未注册的工具。
	ErrToolUnknown = errors.New("cordis: unknown tool")
	// ErrNoFiber 在无插件绑定的上下文（如事件回调）中调用注册类 API。
	ErrNoFiber = errors.New("cordis: no plugin fiber bound to this context")
)

// 内置服务名（由 Runtime 引导时提供）。
const (
	// ServiceCordis 指向 Runtime 自身，供插件做运行时自省。
	ServiceCordis = "cordis"
	// ServiceTools 指向 ToolRegistry，注入它的插件可执行/枚举工具。
	ServiceTools = "tools"
)

// 事件总线上约定的 waterfall 事件名。
const (
	// EventAgentPreStep 在每次 LLM 调用前触发；可改写 system prompt、注入上下文。
	EventAgentPreStep = "agent/pre-step"
	// EventAgentPostStep 在 LLM 响应后触发；可审计输出、拦截敏感内容。
	EventAgentPostStep = "agent/post-step"
	// EventToolPreExecute 在工具执行前触发；可做权限检查、参数改写、短路拒绝。
	EventToolPreExecute = "tools/pre-execute"
	// EventToolPostExecute 在工具执行后触发；可脱敏、格式化、缓存结果。
	EventToolPostExecute = "tools/post-execute"
	// EventSessionPreSave 在会话保存前触发；可添加元数据、压缩上下文。
	EventSessionPreSave = "session/pre-save"
)
