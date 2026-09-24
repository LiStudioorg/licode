package plugins

import (
	"testing"

	"licode/cordis"
	"licode/internal/ai"
)

// 验证阶段一验收标准：替换 llm.config 即热切换 Provider，无需重启。
func TestLLMProviderHotSwap(t *testing.T) {
	r := cordis.NewRuntime(cordis.WithOutput(discard{}))
	if err := r.Load(BuiltinLLM()); err != nil {
		t.Fatal(err)
	}
	for _, p := range BuiltinLLMProviders() {
		if err := r.Load(p); err != nil {
			t.Fatal(err)
		}
	}
	// 配置未提供：编排器保持 PENDING，llm 服务不存在。
	if _, ok := r.Get(cordis.ServiceLLM); ok {
		t.Fatal("llm should be absent before config")
	}

	r.Provide(cordis.ServiceLLMConfig, ai.Config{
		Provider: "openai", BaseURL: "https://api.openai.com/v1",
		APIKey: "k", Model: "gpt-4o-mini",
	})
	v, ok := r.Get(cordis.ServiceLLM)
	if !ok {
		t.Fatal("llm missing after openai config")
	}
	if c := v.(ai.LLMClient); c.Model() != "gpt-4o-mini" {
		t.Fatalf("model = %s", c.Model())
	}

	// 切换到 claude：同步级联，llm 服务换成新客户端。
	r.Provide(cordis.ServiceLLMConfig, ai.Config{
		Provider: "claude", BaseURL: "https://api.anthropic.com",
		APIKey: "k", Model: "claude-sonnet-4-20250514",
	})
	v, ok = r.Get(cordis.ServiceLLM)
	if !ok {
		t.Fatal("llm missing after claude config")
	}
	if c := v.(ai.LLMClient); c.Model() != "claude-sonnet-4-20250514" {
		t.Fatalf("model = %s, want claude", c.Model())
	}

	// 下线 claude 工厂插件 → llm 服务注销（编排器的工厂依赖级联失效）。
	if err := r.Unload("builtin-llm-claude"); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get(cordis.ServiceLLM); ok {
		t.Fatal("llm should be gone after factory unload")
	}
	// 工厂插件恢复 → 编排器自动重载。
	if err := r.Load(cordis.Plugin{
		Name: "builtin-llm-claude",
		Apply: func(ctx cordis.Context) error {
			return ctx.Provide(cordis.ServiceLLMFactoryPrefix+"claude", LLMFactory(ai.NewClaude))
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get(cordis.ServiceLLM); !ok {
		t.Fatal("llm should rebuild after factory reload")
	}
}
