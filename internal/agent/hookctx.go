package agent

import "context"

// AskFunc 请求用户确认一次工具调用，返回是否放行。
type AskFunc func(ctx context.Context, toolName, args string) (bool, error)

// RunHooks 把一次 Agent 运行的会话级钩子放进 Go context，随工具执行链透传。
// 插件（builtin-permissions）在 tools/pre-execute 监听器里从这里读取权限配置、
// 运行模式与确认回调——核心因此无需感知任何权限策略。
type RunHooks struct {
	// Permissions 工具名 -> allow/ask/deny；"*" 为默认模式。
	Permissions map[string]string
	// Mode 运行模式（ModeBuild/ModePlan）。
	Mode string
	// Ask 权限 ask 时的人工确认通道；nil 表示无确认通道（ask 按拒绝处理）。
	Ask AskFunc
	// AutoAllowPaths 允许访问工作目录外路径时不再逐次询问。
	AutoAllowPaths bool
}

type runHooksKey struct{}

// WithRunHooks 把本次运行的钩子注入 context。
func WithRunHooks(ctx context.Context, h RunHooks) context.Context {
	return context.WithValue(ctx, runHooksKey{}, h)
}

// GetRunHooks 读取运行钩子；未注入时返回零值（全部按默认策略处理）。
func GetRunHooks(ctx context.Context) RunHooks {
	if h, ok := ctx.Value(runHooksKey{}).(RunHooks); ok {
		return h
	}
	return RunHooks{}
}

// PermissionFor 按 Permissions 解析某工具的模式：显式项 > "*" 默认 > allow。
func (h RunHooks) PermissionFor(tool string) string {
	if len(h.Permissions) == 0 {
		return "allow"
	}
	if m, ok := h.Permissions[tool]; ok {
		return m
	}
	if m, ok := h.Permissions["*"]; ok {
		return m
	}
	return "allow"
}
