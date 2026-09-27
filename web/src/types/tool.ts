/**
 * 工具调用与审批的数据契约。
 *
 * 这里的形状刻意对齐 Go 后端的 WebSocket 事件（tool_call / tool_result / approval_request），
 * 接入真实后端时只需把 WS 帧映射成这些类型，组件层不用改。
 */

export type ToolState = 'running' | 'success' | 'error' | 'denied'

export interface ToolCall {
  id: string
  /** 工具名，例如 read_file / run_shell */
  name: string
  /** 折叠态下显示的一行摘要，例如文件路径或命令 */
  summary: string
  state: ToolState
  /** 已流式到达的输出片段（拼接即完整输出） */
  output: string
  /** 错误信息，仅在 state === 'error' 时有值 */
  error?: string
  /** 耗时（毫秒），完成时才有值 */
  durationMs?: number
  /** 输出是否被截断 */
  truncated?: boolean
}

export type ApprovalDecision = 'deny' | 'allow' | 'always'

export interface ApprovalRequest {
  id: string
  /** 被审批的工具调用 */
  callId: string
  toolName: string
  /** 需要展示给用户的具体内容，例如将要写入的文件路径或 shell 命令 */
  detail: string
  /** 风险等级影响视觉强调，不影响可用动作 */
  risk: 'low' | 'medium' | 'high'
  /** 请求发起时间，用于提示等待时长 */
  requestedAt: string
}

/** 审批结果回执，供后续 WS 上报 */
export interface ApprovalResult {
  requestId: string
  decision: ApprovalDecision
}
