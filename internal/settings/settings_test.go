package settings

import "testing"

func TestPromptCacheActive(t *testing.T) {
	claude := &Settings{PromptCache: true, Provider: "claude", Providers: []ProviderConfig{{Provider: "claude", Type: "claude"}}}
	if !claude.PromptCacheActive() {
		t.Fatal("claude + PromptCache should be active")
	}
	anthropicAlias := &Settings{PromptCache: true, Provider: "anthropic", Providers: []ProviderConfig{{Provider: "anthropic"}}}
	if !anthropicAlias.PromptCacheActive() {
		t.Fatal("anthropic provider name should resolve to claude type")
	}
	off := &Settings{PromptCache: false, Provider: "claude", Providers: []ProviderConfig{{Provider: "claude", Type: "claude"}}}
	if off.PromptCacheActive() {
		t.Fatal("PromptCache off must be inactive")
	}
	openai := &Settings{PromptCache: true, Provider: "openai", Providers: []ProviderConfig{{Provider: "openai"}}}
	if openai.PromptCacheActive() {
		t.Fatal("openai has no explicit cache breakpoint; must be inactive")
	}
	gemini := &Settings{PromptCache: true, Provider: "gemini", Providers: []ProviderConfig{{Provider: "gemini"}}}
	if gemini.PromptCacheActive() {
		t.Fatal("gemini has no explicit cache breakpoint; must be inactive")
	}
}
