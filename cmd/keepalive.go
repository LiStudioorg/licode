package cmd

import (
	"context"
	"log"
	"time"

	"licode/internal/ai"
)

// setWarmPrefix 记录最近一次真实请求的“稳定前缀”，供空闲期 keepalive 预热复用。
// 只存 system/tools/model（都是会话内字节稳定），不存 messages。tools 来自
// Agent.visibleTools()，与真实请求逐字节一致，才能命中同一个 Claude 缓存块。
func (st *serverState) setWarmPrefix(model, system string, tools []ai.Tool) {
	st.warmMu.Lock()
	st.warmModel = model
	st.warmSystem = system
	st.warmTools = tools
	st.warmOK = system != "" || len(tools) > 0
	st.warmMu.Unlock()
}

func (st *serverState) warmPrefix() (ai.ChatRequest, bool) {
	st.warmMu.Lock()
	defer st.warmMu.Unlock()
	if !st.warmOK {
		return ai.ChatRequest{}, false
	}
	return ai.ChatRequest{
		Model:       st.warmModel,
		System:      st.warmSystem,
		Tools:       st.warmTools,
		PromptCache: true,
		MaxTokens:   1,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: "ping"}},
	}, true
}

// startKeepalive 启动空闲期缓存保活（仅 Claude、仅在配置开启时）。绑定到传入的根 ctx，
// 关停即停，不产生游离 goroutine。每 15s 检查一次：
//   - 关闭或非 Claude → 跳过；
//   - 距上次“真实请求”不足 KeepaliveSec → 跳过（真实请求已刷新缓存，无需重复预热）；
//   - 距上次“预热”不足 KeepaliveSec → 跳过（限速）。
//
// 预热请求极小（输出 1 token），输入即稳定前缀：若命中则 0.1× 读取，若过期则重新写入，
// 目的是让用户回到会话时仍能命中缓存而非全价重算。是否值得由用户显式开启。
func startKeepalive(ctx context.Context, st *serverState) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		var lastWarm time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				st.mu.RLock()
				s := st.settings.Snapshot()
				client := st.client
				st.mu.RUnlock()
				if !s.PromptCacheActive() || s.KeepaliveSec <= 0 {
					continue
				}
				interval := time.Duration(s.KeepaliveSec) * time.Second
				if last := time.Unix(0, st.lastRealRun.Load()); !last.IsZero() && time.Since(last) < interval {
					continue // 有真实流量，缓存已被真实请求刷新
				}
				if !lastWarm.IsZero() && time.Since(lastWarm) < interval {
					continue
				}
				req, ok := st.warmPrefix()
				if !ok {
					continue
				}
				lastWarm = time.Now()
				wctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				_, err := client.Chat(wctx, req)
				cancel()
				if err != nil {
					log.Printf("提示词缓存预热失败(忽略): %v", err)
				}
			}
		}
	}()
}
