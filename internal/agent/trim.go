package agent

import (
	"strings"

	"licode/internal/ai"
)

// 三层裁剪管道（全部确定性，不调用 LLM 做“中间摘要”——那会额外耗一次请求，
// 还会破坏冻结前缀的稳定性）：
//
//	第 1 层（采集时）  spill.go  把超长工具结果溢出落盘，只留有界预览。
//	第 2 层（请求时）  session.MessagesForLLM  纯尾部截断，丢最旧、保最新、清理孤儿 tool 消息。
//	第 3 层（请求时）  本文件  把 C 区易变信息并入“最后一条 user 消息”，不新增消息、不动冻结前缀。

// mergeContextTail 把 C 区易变上下文（日期/用量/模式提示词/记忆/RAG）并入历史里最后一条
// user 消息的副本。之所以“并入”而非“追加新消息”：Anthropic 等协议要求 user/assistant 严格
// 交替，插入独立 user 消息会触发 400；并入既保持角色不变，又不改动冻结的系统前缀。
// 包裹在 <context>…</context> 里，供语义缓存的 lastUser 剥离，避免污染缓存键。
func mergeContextTail(history []ai.Message, ctxBlock string) []ai.Message {
	if strings.TrimSpace(ctxBlock) == "" {
		return history
	}
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role != ai.RoleUser {
			continue
		}
		out := make([]ai.Message, len(history))
		copy(out, history)
		merged := out[i]
		merged.Content = merged.Content + "\n\n<context>\n" + ctxBlock + "\n</context>"
		out[i] = merged
		return out
	}
	return history
}
