package agent

import "context"

// compactIfNeeded 保留了 Compaction 开关的调用点，但**不再调用 LLM 生成中间摘要**。
//
// 旧实现会在上下文接近上限时，把较早的对话喂给模型压缩成摘要再 TrimHead，
// 这与 Token 优化的两条硬性约束冲突：
//  1. “禁止中间摘要历史”——摘要文本会插在历史前部，破坏前缀缓存的字节稳定性；
//  2. 额外一次 LLM 调用本身就是纯消耗（输入 = 大段旧历史）。
//
// 取而代之，本函数是确定性的 no-op：真正防溢出由三层裁剪管道负责——
//   - 第 1 层 spill：超长工具结果在采集时即落盘、只留有界预览；
//   - 第 2 层 尾部截断：session.MessagesForLLM 丢最旧、保最新并清理孤儿 tool 消息。
// 两者都不引入非确定性、不额外消耗请求，前缀得以字节冻结。
func (a *Agent) compactIfNeeded(_ context.Context) {
	// 故意留空：见上方说明。保留方法以兼容 Compaction 开关与既有调用点。
}
