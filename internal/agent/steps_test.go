package agent

import (
	"context"
	"io"
	"strings"
	"testing"

	"licode/cordis"
)

// 验证 agent/pre-step 改写进入模型请求、agent/post-step 改写进入会话。
func TestStepWaterfalls(t *testing.T) {
	rt := cordis.NewRuntime(cordis.WithOutput(io.Discard))
	err := rt.Load(cordis.Plugin{Name: "audit", Apply: func(ctx cordis.Context) error {
		if err := ctx.On(cordis.EventAgentPreStep, func(_ cordis.Context, in any, next func(any) (any, error)) (any, error) {
			si := in.(StepInput)
			si.System += "\n[AUDITED]"
			return next(si)
		}); err != nil {
			return err
		}
		return ctx.On(cordis.EventAgentPostStep, func(_ cordis.Context, in any, next func(any) (any, error)) (any, error) {
			so := in.(StepOutput)
			so.Message.Content = strings.ToUpper(so.Message.Content)
			return next(so)
		})
	}})
	if err != nil {
		t.Fatal(err)
	}

	mc := &mockClient{model: "m", calls: []func(mockReq) []mockStep{
		func(r mockReq) []mockStep {
			if !strings.Contains(r.system, "[AUDITED]") {
				t.Errorf("pre-step rewrite missing: %q", r.system)
			}
			return []mockStep{{content: "hi"}}
		},
	}}
	a := NewAgent(mc, "BASE")
	a.Cordis = rt
	if err := a.Run(context.Background(), "hello", func(Event) {}); err != nil {
		t.Fatal(err)
	}
	msgs := a.Session.Messages()
	last := msgs[len(msgs)-1]
	if last.Role != "assistant" || last.Content != "HI" {
		t.Fatalf("post-step rewrite missing: %+v", last)
	}
}
