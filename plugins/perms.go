package plugins

import (
	"encoding/json"

	"licode/cordis"
	"licode/internal/agent"
)

// BuiltinPermissions 把工具的权限模型实现为 tools/pre-execute 监听器：
// deny 短路拒绝、ask 走人工确认、plan 模式只放行只读工具。
// 策略从 RunHooks（Go context）读取，因此核心/Agent 不需要任何权限代码。
func BuiltinPermissions() cordis.Plugin {
	return cordis.Plugin{
		Name: "builtin-permissions",
		Apply: func(ctx cordis.Context) error {
			return ctx.OnPriority(cordis.EventToolPreExecute, -20, checkPermission)
		},
	}
}

func checkPermission(_ cordis.Context, in any, next func(any) (any, error)) (any, error) {
	tc, ok := in.(cordis.ToolCall)
	if !ok || tc.Ctx == nil {
		return next(in)
	}
	h, injected := agent.LookupRunHooks(tc.Ctx)
	if !injected {
		// 未经 Agent.Run 注入钩子直接执行管道工具属于旁路调用：只读工具
		// 无副作用可放行，副作用工具一律拒绝（旧实现按零值默认 allow，
		// 等于任何持有 runtime 引用的代码都能绕过 deny/ask/plan 门禁）。
		if agent.IsReadOnlyTool(tc.Name) {
			return next(tc)
		}
		return shortCircuit(tc, "已拒绝执行 "+tc.Name+"（缺少运行权限上下文，拒绝默认放行）"), nil
	}

	mode := h.Mode
	if mode == "" {
		mode = agent.ModeBuild
	}
	if mode == agent.ModePlan && !agent.IsReadOnlyTool(tc.Name) {
		return shortCircuit(tc, "plan 模式只读：工具 "+tc.Name+" 不可用（需要修改工作区请使用 /build）"), nil
	}

	switch h.PermissionFor(tc.Name) {
	case "deny":
		return shortCircuit(tc, "已拒绝执行 "+tc.Name+"（权限配置为禁止）"), nil
	case "ask":
		if h.Ask == nil {
			return shortCircuit(tc, "已拒绝执行 "+tc.Name+"（需人工确认，但当前无确认通道）"), nil
		}
		approved, err := h.Ask(tc.Ctx, tc.Name, argsJSON(tc.Args))
		if err != nil {
			return nil, err
		}
		if !approved {
			return shortCircuit(tc, "用户拒绝执行工具 "+tc.Name), nil
		}
	}
	return next(tc)
}

func shortCircuit(tc cordis.ToolCall, msg string) cordis.ToolCall {
	tc.Output = &msg
	return tc
}

func argsJSON(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	b, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(b)
}
