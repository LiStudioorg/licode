/**
 * 对话视图模型。
 *
 * 与协议层的区别：protocol.ts 描述「线上格式」（Go 的 json tag），
 * 这里描述「渲染形状」。映射只发生在 stores/session.ts 一处，
 * 组件永远不接触 ServerEvent。
 */

export type ToolState = 'running' | 'success' | 'error'

/** 工具调用块：tool_start 建 running，tool_done 按同名 LIFO 回填 */
export interface ToolBlock {
  kind: 'tool'
  id: number
  name: string
  /** 原始 JSON 字符串（流式期间可能不完整） */
  args: string
  out: string
  state: ToolState
  /** 收到 tool_start 的时刻，用于算耗时 */
  startedAt: number
  durationMs?: number
}

export interface TextBlock {
  kind: 'text'
  id: number
  text: string
}

export type Block = TextBlock | ToolBlock

export interface MessageAttachment {
  type: string
  mime_type: string
  filename: string
  /** 图片的 data: URL（仅本地预览；不落库） */
  url?: string
}

/** 一轮对话：一条用户输入，或助手的一次完整回复（含交错的工具调用） */
export interface Turn {
  id: number
  role: 'user' | 'assistant'
  blocks: Block[]
  attachments?: MessageAttachment[]
}

export interface AskInfo {
  askId: string
  toolName: string
  toolArgs: string
}

export type ApprovalDecision = 'deny' | 'allow' | 'always'

/** 风险等级由工具名推断：只影响视觉强调，不改变可用动作 */
export type RiskLevel = 'low' | 'medium' | 'high'
