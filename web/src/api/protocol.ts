/**
 * 前后端协议的唯一契约点。
 *
 * 与 Go 端 internal/websocket/*.go 的 ClientMessage / ServerEvent 及
 * internal/settings.Settings 的 json tag 一一对应；后端 tag 变更只需改这里，
 * 其余代码由类型检查暴露问题。
 *
 * 校验依据（后端源码位置）：
 *   internal/websocket/websocket.go   ClientMessage / ServerEvent / 附件
 *   cmd/serve.go                      事件发送点、buildSessionStats
 *   internal/settings/settings.go     Settings 全量字段与掩码语义
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

export type ClientTypeValue = (typeof ClientType)[keyof typeof ClientType]

/** 附件：base64（不含 data: 前缀）。WS 单帧上限 8 MiB，编码后需 < 8 MiB */
export interface Attachment {
  type: 'image' | 'file'
  mime_type: string
  data: string
  filename?: string
}

export interface OutgoingMessage {
  type: string
  /** message=正文；session_rename=新标题 */
  content?: string
  /** message 的本次系统提示词 */
  system?: string
  /** settings_set 的全量设置对象（后端整体替换，非增量合并） */
  settings?: unknown
  askId?: string
  askApprove?: boolean
  askAlways?: boolean
  sessionId?: string
  /** session_branch 分支点消息序号；-1 表示复制整段对话 */
  index?: number
  /** session_reorder：期望顺序；未列出的会话由服务端追加到末尾 */
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
  /** 置顶会话由服务端恒定排在列表前面 */
  pinned?: boolean
}

/** 工具调用，对应 ai.ToolCall（OpenAI 风格） */
export interface LLMToolCall {
  id?: string
  type?: string
  function?: { name?: string; arguments?: string }
}

/** 历史消息，对应 ai.Message */
export interface HistoryMessage {
  role: 'user' | 'assistant' | 'tool' | 'system'
  content: string
  tool_calls?: LLMToolCall[]
  tool_call_id?: string
  tool_name?: string
  attachments?: { type: string; mime_type: string; filename?: string; data?: string }[]
}

/** stats 事件负载，对应 cmd/serve.go buildSessionStats（snake_case 会话快照） */
export interface SessionStats {
  messages?: number
  context_tokens?: number
  context_max?: number
  context_pct?: number
  provider?: string
  model?: string
  conversation_in?: number
  conversation_out?: number
  usage_cached?: number
  cache_hit_rate?: number
  always_allow?: string[]
}

/**
 * stats 事件的另一种 shape：agent.TokenStats（camelCase，每轮 LLM 结束发出）。
 * 后端两种 shape 共用 stats 字段，用 inputTokens / context_pct 键名区分。
 */
export interface TokenStats {
  requests?: number
  inputTokens?: number
  outputTokens?: number
  cachedTokens?: number
  sysHash?: string
}

export interface ServerEvent {
  type: ServerTypeValue
  /** delta/reasoning/status 的文本分片（delta 与 reasoning 均为追加语义） */
  content?: string
  toolName?: string
  /** 工具入参：JSON 字符串（可能因流式而不完整） */
  toolArgs?: string
  /** 工具输出全文 */
  toolOut?: string
  error?: string
  settings?: Settings
  sessions?: SessionInfo[]
  /** 事件归属的会话：不属于当前会话的事件不得污染视图 */
  sessionId?: string
  stats?: SessionStats
  messages?: HistoryMessage[]
  /** 待确认工具调用的标识，应答时原样带回 */
  askId?: string
}

/* ---------------- 设置对象（internal/settings.Settings） ---------------- */

/** API Key 掩码占位符：回传时原样带回，服务端自动还原真实密钥 */
export const MASKED_API_KEY = '********'

export type ProviderType = 'openai' | 'claude' | 'ollama' | 'gemini'

export interface ProviderConfig {
  /** 厂商标识（内置名或自定义名），列表内唯一 */
  provider: string
  /** 展示名，空则回落 provider */
  name?: string
  /** 协议类型；空则按 provider 推断 */
  type?: string
  base_url?: string
  /** 读取时恒为掩码，写入时填真实值；保持掩码即不改 */
  api_key?: string
  /** 该厂商当前使用的模型 */
  model?: string
  /** 该厂商的模型清单（仅展示与选择） */
  models?: string[]
  /** 域名直连此 IP（SNI/证书校验仍用原域名），绕过 DNS 污染 */
  host_ip?: string
  /** 忽略 TLS 证书校验，仅限自签证书受控环境 */
  insecure_ssl?: boolean
}

export interface MCPServer {
  name: string
  type?: string
  command?: string
  args?: string[]
  url?: string
  cwd?: string
}

export type DnsMode = '' | 'system' | 'custom' | 'command'
export type DnsServerMode = 'plain' | 'dot' | 'doh'

export interface DnsServer {
  mode?: DnsServerMode
  server?: string
}

export interface DnsConfig {
  mode?: DnsMode
  servers?: DnsServer[]
  /** 并发查询服务器数（0=默认 2，-1=全部） */
  concurrency?: number
  /** 单次查询超时毫秒（0=默认 5000） */
  timeout_ms?: number
  /** 域名 -> IP 直连覆盖 */
  host_overrides?: Record<string, string>
}

export type ToolRule = 'allow' | 'ask' | 'deny'

/**
 * 全量设置。settings_set 是整体替换语义（后端反序列化成 Settings 后直接
 * 覆盖运行时状态），因此任何字段缺失都会落回 EnsureDefaults 的默认值 ——
 * 保存时必须回传读到的完整对象，只做局部改动。
 */
export interface Settings {
  provider: string
  base_url?: string
  api_key?: string
  model?: string
  providers?: ProviderConfig[]
  temperature?: number
  max_tokens?: number
  max_iterations?: number
  subagents?: boolean
  /** 旧版兼容字段：新交互一律走 tool_rules */
  ask_tools?: string[]
  deny_tools?: string[]
  compaction?: boolean
  title_gen?: boolean
  auto_allow?: boolean
  /** null/缺省 = 默认开启流式 */
  streaming?: boolean | null
  tool_rules?: Record<string, ToolRule>
  mcp_servers?: MCPServer[]
  shell_path?: string
  retry_max?: number
  sub_timeout?: number
  max_ctx_tokens?: number
  redact_secrets?: boolean
  cache_enabled?: boolean
  cache_ttl?: number
  prompt_cache?: boolean
  keepalive_sec?: number
  tool_auto_retry?: boolean
  tool_retry_max?: number
  shutdown_timeout?: number
  rag_enabled?: boolean
  rag_source?: string
  rag_top_files?: number
  dns?: DnsConfig
}

/* ---------------- 连接状态 ---------------- */

export type ConnectionState = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed'
