package agent

import "licode/internal/ai"

// StepInput 是 agent/pre-step waterfall 的输入：LLM 调用前，插件可改写
// System/Messages/Tools（注入上下文、审计、按模式过滤工具等）。
type StepInput struct {
	Iteration int
	System    string
	Messages  []ai.Message
	Tools     []ai.Tool
}

// StepOutput 是 agent/post-step waterfall 的输入：LLM 响应后、写入会话前，
// 插件可审计或改写助手消息（如敏感内容拦截）。
type StepOutput struct {
	Iteration int
	Message   ai.Message
}
