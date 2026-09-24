package plugins

import (
	"fmt"

	"licode/cordis"
	"licode/internal/ai"
)

// LLMFactory 按协议类型构造 LLM 客户端（cfg 必须已 Resolve）。
type LLMFactory func(cfg ai.Config) (ai.LLMClient, error)

// llmProviderFactories 是各协议类型与其构造器的映射（拆分为独立插件工厂）。
var llmProviderFactories = []struct {
	protocol string
	plugin   string
	build    LLMFactory
}{
	{"openai", "builtin-llm-openai", ai.NewOpenAI},
	{"claude", "builtin-llm-claude", ai.NewClaude},
	{"ollama", "builtin-llm-ollama", ai.NewOllama},
	{"google", "builtin-llm-gemini", ai.NewGemini},
}

// BuiltinLLMProviders 返回全部 Provider 工厂插件：每个插件把构造器注册为
// llm.factory.<protocol> 服务。卸载某插件即下线该协议。
func BuiltinLLMProviders() []cordis.Plugin {
	out := make([]cordis.Plugin, 0, len(llmProviderFactories))
	for _, f := range llmProviderFactories {
		f := f
		out = append(out, cordis.Plugin{
			Name: f.plugin,
			Apply: func(ctx cordis.Context) error {
				return ctx.Provide(cordis.ServiceLLMFactoryPrefix+f.protocol, LLMFactory(f.build))
			},
		})
	}
	return out
}

// BuiltinLLM 是 LLM 编排器：读取 llm.config 服务，再按协议类型对对应工厂
// 服务做动态 Inject——工厂插件下线时 llm 自动注销、依赖方级联失效，工厂
// 恢复后自动重建；配置服务被替换时整条链重建——即 Provider 级热切换。
func BuiltinLLM() cordis.Plugin {
	return cordis.Plugin{
		Name:   "builtin-llm",
		Inject: []string{cordis.ServiceLLMConfig},
		Apply: func(ctx cordis.Context) error {
			raw, ok := ctx.Get(cordis.ServiceLLMConfig)
			if !ok {
				return fmt.Errorf("missing service %s", cordis.ServiceLLMConfig)
			}
			cfg, ok := raw.(ai.Config)
			if !ok {
				return fmt.Errorf("%s holds %T, want ai.Config", cordis.ServiceLLMConfig, raw)
			}
			if err := cfg.Resolve(); err != nil {
				return err
			}
			factorySvc := cordis.ServiceLLMFactoryPrefix + cfg.Type
			return ctx.Inject([]string{factorySvc}, func(ctx cordis.Context) {
				fraw, ok := ctx.Get(factorySvc)
				if !ok {
					return
				}
				factory, ok := fraw.(LLMFactory)
				if !ok {
					ctx.Logger().Printf("cordis: %s holds %T, want plugins.LLMFactory", factorySvc, fraw)
					return
				}
				client, err := factory(cfg)
				if err != nil {
					ctx.Logger().Printf("cordis: build llm %s: %v", cfg.Type, err)
					return
				}
				if err := ctx.Provide(cordis.ServiceLLM, client); err != nil {
					ctx.Logger().Printf("cordis: provide llm: %v", err)
				}
			})
		},
	}
}
