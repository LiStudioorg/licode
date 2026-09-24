package agent

import "fmt"

// TokenStats 汇总一次运行（或一次快照）内的 token 账目，用于把“省了多少”显式化。
// Cached = 命中 Provider 前缀缓存的输入 token（OpenAI prompt_tokens_details.cached_tokens /
// Claude cache_read+cache_create / Gemini cachedContentTokenCount）。
type TokenStats struct {
	Requests     int     `json:"requests"`
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	CachedTokens int     `json:"cachedTokens"`
	SysHash      string  `json:"sysHash,omitempty"` // 冻结前缀哈希（稳定性可观测）
}

// HitRatio 返回输入 token 中命中缓存的比例（0~1）。
func (s TokenStats) HitRatio() float64 {
	if s.InputTokens <= 0 {
		return 0
	}
	return float64(s.CachedTokens) / float64(s.InputTokens)
}

// String 是给“状态事件/日志”用的单行摘要。
func (s TokenStats) String() string {
	return fmt.Sprintf("请求 %d · 输入 %d(缓存 %d, %.0f%%) · 输出 %d",
		s.Requests, s.InputTokens, s.CachedTokens, s.HitRatio()*100, s.OutputTokens)
}

// snapshotStats 读取 Agent 当前累计用量快照。requests 由请求计数字段提供。
func (a *Agent) snapshotStats() TokenStats {
	return TokenStats{
		Requests:     a.requests,
		InputTokens:  a.Usage.InputTokens,
		OutputTokens: a.Usage.OutputTokens,
		CachedTokens: a.Usage.CachedTokens,
		SysHash:      a.SysHash,
	}
}
