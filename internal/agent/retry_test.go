package agent

import (
	"context"
	"strings"
	"testing"

	"licode/internal/ai"
)

// TestRunToolNoEmptyRetryForSideEffectTools 验证修复：副作用工具（如 Shell）
// 成功但零输出时不得自动重试——否则 mv/touch 等命令会被重复执行。
func TestRunToolNoEmptyRetryForSideEffectTools(t *testing.T) {
	calls := 0
	reg := NewRegistry()
	reg.Register(Tool{
		Name:   "Shell",
		Schema: map[string]any{"type": "object"},
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			calls++
			return "", nil // 合法成功：mv 无输出
		},
	})
	a := &Agent{Tools: reg, ToolAutoRetry: true, ToolRetryMax: 2}
	out, err := a.runTool(context.Background(),
		ai.ToolCall{Function: ai.FunctionCall{Name: "Shell", Arguments: `{"command":"mv a b"}`}},
		func(Event) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	_ = out
	if calls != 1 {
		t.Fatalf("side-effect tool with empty output must execute exactly once, got %d", calls)
	}
}

// TestRunToolEmptyRetryForReadOnlyTools 只读工具空结果仍允许自动重试
// （无副作用，重试只是代价问题，保持原特性）。
func TestRunToolEmptyRetryForReadOnlyTools(t *testing.T) {
	calls := 0
	reg := NewRegistry()
	reg.Register(Tool{
		Name:   "Read",
		Schema: map[string]any{"type": "object"},
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			calls++
			if calls < 3 {
				return "", nil
			}
			return "content", nil
		},
	})
	a := &Agent{Tools: reg, ToolAutoRetry: true, ToolRetryMax: 3}
	out, err := a.runTool(context.Background(),
		ai.ToolCall{Function: ai.FunctionCall{Name: "Read", Arguments: `{}`}},
		func(Event) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "content") {
		t.Fatalf("expected retry to reach success, out=%q calls=%d", out, calls)
	}
}

// TestSubAgentToolsNilVsEmpty 固化子代理工具白名单语义：
// nil = 全部默认工具；非 nil 空切片 = 零工具（planner 依赖该语义）。
func TestSubAgentToolsNilVsEmpty(t *testing.T) {
	mc := &mockClient{model: "m"}
	full := SubAgentSpec{Name: "full", Prompt: "x", Tools: nil, Client: mc}.buildAgent()
	if len(full.Tools.Names()) == 0 {
		t.Fatal("nil Tools must materialize the default tool set")
	}
	none := SubAgentSpec{Name: "none", Prompt: "x", Tools: []string{}, Client: mc}.buildAgent()
	if len(none.Tools.Names()) != 0 {
		t.Fatalf("empty Tools must mean zero tools, got %v", none.Tools.Names())
	}
	some := SubAgentSpec{Name: "some", Prompt: "x", Tools: []string{"Read"}, Client: mc}.buildAgent()
	names := some.Tools.Names()
	if len(names) != 1 || names[0] != "Read" {
		t.Fatalf("whitelist filter broken: %v", names)
	}
}
