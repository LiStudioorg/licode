package plugins

import (
	"context"
	"testing"

	"licode/cordis"
	"licode/internal/agent"
)

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func testRuntime(t *testing.T) *cordis.Runtime {
	t.Helper()
	r := cordis.NewRuntime(cordis.WithOutput(discard{}))
	// 测试专用 echo 工具。
	if err := r.Load(cordis.Plugin{Name: "echo-tools", Apply: func(ctx cordis.Context) error {
		return ctx.RegisterTool(cordis.Tool{
			Name: "Echo",
			Execute: func(context.Context, map[string]any) (string, error) {
				return "echoed", nil
			},
		})
	}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Load(BuiltinPermissions()); err != nil {
		t.Fatal(err)
	}
	return r
}

func execEcho(r *cordis.Runtime, h agent.RunHooks) (string, error) {
	reg, _ := r.Get(cordis.ServiceTools)
	tr := reg.(*cordis.ToolRegistry)
	// 始终注入钩子（与 Agent.Run 行为一致）：新契约下"未注入"会被
	// 门禁拒绝，"注入但配置为空"才等价于默认允许。
	ctx := agent.WithRunHooks(context.Background(), h)
	return tr.Execute(ctx, "Echo", map[string]any{"a": 1})
}

func TestPermissionDeny(t *testing.T) {
	r := testRuntime(t)
	out, err := execEcho(r, agent.RunHooks{Permissions: map[string]string{"Echo": "deny"}})
	if err != nil {
		t.Fatal(err)
	}
	if out != "已拒绝执行 Echo（权限配置为禁止）" {
		t.Fatalf("out = %q", out)
	}
}

func TestPermissionAskApproved(t *testing.T) {
	r := testRuntime(t)
	called := 0
	out, err := execEcho(r, agent.RunHooks{
		Permissions: map[string]string{"Echo": "ask"},
		Ask: func(context.Context, string, string) (bool, error) {
			called++
			return true, nil
		},
	})
	if err != nil || out != "echoed" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if called != 1 {
		t.Fatalf("ask called %d", called)
	}
}

func TestPermissionAskRejected(t *testing.T) {
	r := testRuntime(t)
	out, _ := execEcho(r, agent.RunHooks{
		Permissions: map[string]string{"Echo": "ask"},
		Ask: func(context.Context, string, string) (bool, error) {
			return false, nil
		},
	})
	if out != "用户拒绝执行工具 Echo" {
		t.Fatalf("out = %q", out)
	}
}

func TestPermissionAskNoChannel(t *testing.T) {
	r := testRuntime(t)
	out, _ := execEcho(r, agent.RunHooks{Permissions: map[string]string{"Echo": "ask"}})
	if out != "已拒绝执行 Echo（需人工确认，但当前无确认通道）" {
		t.Fatalf("out = %q", out)
	}
}

func TestPermissionDefaultAllow(t *testing.T) {
	r := testRuntime(t)
	out, err := execEcho(r, agent.RunHooks{})
	if err != nil || out != "echoed" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestPlanModeReadOnlyFilter(t *testing.T) {
	r := testRuntime(t)
	// Read 属只读集：plan 模式放行。
	tr, _ := r.Get(cordis.ServiceTools)
	reg := tr.(*cordis.ToolRegistry)
	ctx := agent.WithRunHooks(context.Background(), agent.RunHooks{Mode: agent.ModePlan})
	if _, err := reg.Execute(ctx, "Read", map[string]any{"path": "/nonexistent-xyz"}); err != nil {
		// 文件不存在是工具错误，管道本身应放行到执行。
		t.Logf("Read pipe error (expected tool-level): %v", err)
	}
	// Echo 非只读：plan 模式短路拒绝。
	out, _ := execEcho(r, agent.RunHooks{Mode: agent.ModePlan})
	if out == "echoed" || out == "" {
		t.Fatalf("plan mode should block non-readonly tool, got %q", out)
	}
}

// 内置工具桥接：Read/Write/Shell 进入 Cordis 工具树，插件卸载即消失。
func TestBuiltinToolsBridge(t *testing.T) {
	r := cordis.NewRuntime(cordis.WithOutput(discard{}))
	if err := RegisterAll(r, agent.ShellConfig{}); err != nil {
		t.Fatal(err)
	}
	tr, _ := r.Get(cordis.ServiceTools)
	reg := tr.(*cordis.ToolRegistry)
	for _, name := range append(append([]string{}, fsToolNames...), shellToolNames...) {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("tool %s missing from cordis registry", name)
		}
	}
	if err := r.Unload("builtin-tools-shell"); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Get("Shell"); ok {
		t.Fatal("Shell survived plugin unload")
	}
	if _, ok := reg.Get("Read"); !ok {
		t.Fatal("Read should remain")
	}
}

// TestPermissionBypassWithoutHooks 固化门禁契约：未注入 RunHooks 直接执行
// 管道工具时，非只读工具必须被拒绝（旧实现按 allow 默认放行，构成旁路）。
func TestPermissionBypassWithoutHooks(t *testing.T) {
	r := testRuntime(t)
	tr, _ := r.Get(cordis.ServiceTools)
	reg := tr.(*cordis.ToolRegistry)
	out, _ := reg.Execute(context.Background(), "Echo", map[string]any{"a": 1})
	if out == "echoed" {
		t.Fatal("non-readonly tool must be denied without injected hooks")
	}
}
