/**
 * WebSocket 协议类型：与 Go 端 internal/websocket/websocket.go 一一对应。
 *
 * 该文件是前后端协议的唯一契约点。Go 端 ServerEvent / ClientMessage
 * 的 json tag 若变更，只需改这里，其余组件通过类型检查暴露问题。
 */

/* ---------------- 客户端 -> 服务端（ClientMessage） ---------------- */

export const ClientType = {
  Message: 'message',
  Ping: 'ping',
  SettingsGet: 'settings_get',
  SettingsSet: 'settings_set',
  AskReply: 'ask_reply',
  Interrupt: 'interrupt',
  SessionsGet: 'sessions_get',
  SessionNew: 'session_new',
  SessionSwitch: 'session_switch',
  SessionRename: 'session_rename',
  SessionDelete: 'session_delete',
  SessionBranch: 'session_branch',
  SessionHistory: 'session_history',
  SessionReorder: 'session_reorder',
  SessionPin: 'session_pin',
} as const

export interface Attachment {
  type: 'image' | 'file'
  mime_type: string
  /** base64（不含 data: 前缀） */
  data: string
  filename?: string
}

export interface OutgoingMessage {
  type: string
  content?: string
  system?: string
  settings?: unknown
  askId?: string
  askApprove?: boolean
  askAlways?: boolean
  sessionId?: string
  index?: number
  /** session_reorder：期望顺序，未列出的会话由服务端追加到末尾 */
  order?: string[]
  /** session_pin：目标置顶状态 */
  pinned?: boolean
  attachments?: Attachment[]
}

/* ---------------- 服务端 -> 客户端（ServerEvent） ---------------- */

export const ServerType = {
  Delta: 'delta',
  ToolStart: 'tool_start',
  ToolDone: 'tool_done',
  Done: 'done',
  Error: 'error',
  Status: 'status',
  Reasoning: 'reasoning',
  Settings: 'settings',
  Ask: 'ask',
  Sessions: 'sessions',
  Stats: 'stats',
  History: 'history',
} as const

export type ServerTypeValue = (typeof ServerType)[keyof typeof ServerType]

/** 会话列表项，对应 session.Info */
export interface SessionInfo {
  id: string
  title: string
  count: number
  /** 置顶会话由服务端恒定排在前面 */
  pinned?: boolean
}

/** 历史消息，对应 ai.Message */
export interface HistoryMessage {
  role: 'user' | 'assistant' | 'tool' | 'system'
  content: string
  tool_calls?: { id?: string; name?: string; arguments?: string }[]
  tool_call_id?: string
  tool_name?: string
  attachments?: { type: string; mime_type: string; filename?: string; data?: string }[]
}

export interface ServerEvent {
  type: ServerTypeValue
  content?: string
  toolName?: string
  toolArgs?: string
  toolOut?: string
  error?: string
  settings?: unknown
  sessions?: SessionInfo[]
  sessionId?: string
  stats?: unknown
  messages?: HistoryMessage[]
  /** 待确认工具调用的标识，回执时原样带回 */
  askId?: string
}

/* ---------------- 连接状态 ---------------- */

export type ConnectionState = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed'
