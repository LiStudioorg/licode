import { computed, ref, shallowRef } from 'vue'
import { defineStore } from 'pinia'
import { LicodeSocket } from '@/api/socket'
import {
  ClientType,
  ServerType,
  type ConnectionState,
  type HistoryMessage,
  type ServerEvent,
  type SessionInfo,
} from '@/api/protocol'
import type { ApprovalRequest, ToolCall, ToolState } from '@/types/tool'

/**
 * 真实数据流下的会话状态。
 *
 * 事件到 UI 的映射（与 cmd/serve.go 的发送逻辑一一对应）：
 *   delta      -> 追加到当前助手消息
 *   tool_start -> 新建一条 running 的 ToolCall
 *   tool_done  -> 按同名顺序回填最后一条 running 的 ToolCall
 *   ask        -> 生成 ApprovalRequest（等待审批）
 *   done       -> 结束本轮，解除 streaming
 *   error      -> 记录错误
 *   sessions   -> 刷新会话列表
 *   history    -> 回放消息历史
 *
 * 界面消息与 ToolCall 分两条线渲染（消息流按时间交错），
 * 因此这里维护一个有序的 timeline，而不是把工具塞进消息体里。
 */
export type TimelineItem =
  | { kind: 'message'; id: string; role: 'user' | 'assistant'; text: string }
  | { kind: 'tool'; id: string; call: ToolCall }

export const useSessionStore = defineStore('session', () => {
  const socket = shallowRef<LicodeSocket | null>(null)
  const connection = ref<ConnectionState>('idle')
  const lastError = ref<string | null>(null)

  const sessions = ref<SessionInfo[]>([])
  const activeId = ref<string>('')
  const streaming = ref(false)
  const timeline = ref<TimelineItem[]>([])
  const pending = ref<ApprovalRequest[]>([])
  const query = ref('')

  const active = computed(() => sessions.value.find((s) => s.id === activeId.value) ?? null)

  /** 分组：置顶无后端概念，按消息数粗分「有内容 / 空会话」，空会话即新会话 */
  const grouped = computed(() => {
    const q = query.value.trim().toLowerCase()
    const hit = (s: SessionInfo) => (q ? s.title.toLowerCase().includes(q) : true)
    const list = sessions.value.filter(hit)
    return [
      { group: '进行中', items: list.filter((s) => s.count > 0) },
      { group: '未开始', items: list.filter((s) => s.count === 0) },
    ].filter((g) => g.items.length > 0)
  })

  const toolCalls = computed(() =>
    timeline.value.filter((t): t is Extract<TimelineItem, { kind: 'tool' }> => t.kind === 'tool'),
  )

  let msgSeq = 0
  let toolSeq = 0

  function pushMessage(role: 'user' | 'assistant', text: string) {
    timeline.value.push({ kind: 'message', id: `m-${++msgSeq}`, role, text })
  }

  /** 追加增量文本到末尾的助手消息；没有则新建（首个 delta 到来时） */
  function appendDelta(text: string) {
    const last = timeline.value[timeline.value.length - 1]
    if (last && last.kind === 'message' && last.role === 'assistant') {
      last.text += text
      return
    }
    pushMessage('assistant', text)
  }

  function startTool(name: string, args: string, sessionId?: string) {
    // 只接受当前会话的事件，避免切换会话后串台
    if (sessionId && activeId.value && sessionId !== activeId.value) return
    toolSeq += 1
    timeline.value.push({
      kind: 'tool',
      id: `t-${toolSeq}`,
      call: {
        id: `t-${toolSeq}`,
        name,
        summary: summarize(args),
        state: 'running',
        output: '',
      },
    })
  }

  function finishTool(name: string, out: string, isError: boolean, sessionId?: string) {
    if (sessionId && activeId.value && sessionId !== activeId.value) return
    // 从末尾往前找最后一条同名且仍在 running 的调用：
    // 同一工具可被并发调用，必须按「后进先出」配对
    for (let i = timeline.value.length - 1; i >= 0; i -= 1) {
      const item = timeline.value[i]
      if (item.kind === 'tool' && item.call.name === name && item.call.state === 'running') {
        const state: ToolState = isError ? 'error' : 'success'
        item.call = {
          ...item.call,
          state,
          output: out,
          error: isError ? out : undefined,
        }
        return
      }
    }
  }

  /** 历史回放：把 ai.Message 列表映射成 timeline */
  function replayHistory(messages: HistoryMessage[]) {
    timeline.value = []
    for (const m of messages) {
      if (m.role === 'user' || m.role === 'assistant') {
        if (!m.content) continue
        pushMessage(m.role, m.content)
      } else if (m.role === 'tool') {
        toolSeq += 1
        timeline.value.push({
          kind: 'tool',
          id: `t-${toolSeq}`,
          call: {
            id: `t-${toolSeq}`,
            name: m.tool_name ?? 'tool',
            summary: m.tool_call_id ?? '',
            state: 'success',
            output: m.content ?? '',
          },
        })
      }
    }
  }

  function handleEvent(event: ServerEvent) {
    switch (event.type) {
      case ServerType.Delta:
        lastError.value = null
        if (event.content) appendDelta(event.content)
        break

      case ServerType.ToolStart:
        startTool(event.toolName ?? 'tool', event.toolArgs ?? '', event.sessionId)
        break

      case ServerType.ToolDone:
        finishTool(event.toolName ?? 'tool', event.toolOut ?? '', false, event.sessionId)
        break

      case ServerType.Ask:
        pending.value.push({
          id: event.askId ?? `ask-${Date.now()}`,
          callId: event.askId ?? '',
          toolName: event.toolName ?? 'tool',
          detail: event.toolArgs ?? '',
          risk: riskOf(event.toolName ?? ''),
          requestedAt: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
        })
        break

      case ServerType.Done:
        streaming.value = false
        // 本轮结束仍无人应答的审批一律作废：
        // Go 端在 ctx 结束时已清理 pending 映射，继续展示会让用户以为还能批准
        pending.value = []
        break

      case ServerType.Error:
        streaming.value = false
        lastError.value = event.error ?? '未知错误'
        pending.value = []
        break

      case ServerType.Sessions:
        if (Array.isArray(event.sessions)) sessions.value = event.sessions
        if (event.sessionId) activeId.value = event.sessionId
        break

      case ServerType.History:
        if (event.messages) replayHistory(event.messages)
        break

      default:
        // status / reasoning / stats / settings 暂未在骨架中呈现，静默忽略
        break
    }
  }

  /** 建立连接并发起首次会话列表拉取 */
  function connect() {
    if (socket.value) return socket.value
    const c = new LicodeSocket({
      onEvent: handleEvent,
      onState: (s) => {
        connection.value = s
        // 重连成功后重新同步会话列表，补齐断线期间的变化
        if (s === 'open') {
          c.send({ type: ClientType.SessionsGet })
        }
      },
    })
    socket.value = c
    c.connect()
    return c
  }

  function disconnect() {
    socket.value?.close()
    socket.value = null
    connection.value = 'closed'
  }

  /** 发送一条用户消息；本轮开始，进入流式态 */
  function send(content: string) {
    const text = content.trim()
    if (!text) return
    pushMessage('user', text)
    streaming.value = true
    lastError.value = null
    connect().send({ type: ClientType.Message, content: text })
  }

  function select(id: string) {
    if (id === activeId.value) return
    activeId.value = id
    timeline.value = []
    pending.value = []
    streaming.value = false
    connect().send({ type: ClientType.SessionSwitch, sessionId: id })
  }

  function create() {
    connect().send({ type: ClientType.SessionNew })
  }

  function rename(id: string, title: string) {
    connect().send({ type: ClientType.SessionRename, sessionId: id, content: title })
  }

  function remove(id: string) {
    connect().send({ type: ClientType.SessionDelete, sessionId: id })
  }

  /** 中止当前生成 */
  function interrupt() {
    connect().send({ type: ClientType.Interrupt })
    streaming.value = false
  }

  /** 审批回执：always 对应「始终允许」 */
  function decide(requestId: string, decision: 'deny' | 'allow' | 'always') {
    pending.value = pending.value.filter((r) => r.id !== requestId)
    connect().replyAsk(requestId, decision !== 'deny', decision === 'always')
  }

  function refreshSessions() {
    connect().send({ type: ClientType.SessionsGet })
  }

  /**
   * 拖拽重排。
   *
   * 顺序由服务端持有：这里先本地乐观更新（避免拖拽后回弹的观感），
   * 再发 session_reorder，服务端回推 sessions 事件时以服务端顺序为准。
   * 置顶项不参与拖拽区间，前端计算顺序时需排除，否则会把置顶拖进普通区。
   */
  function reorder(draggedId: string, targetId: string, position: 'before' | 'after') {
    const pinned = sessions.value.filter((s) => s.pinned).map((s) => s.id)
    const normal = sessions.value.filter((s) => !s.pinned)
    const from = normal.findIndex((s) => s.id === draggedId)
    if (from === -1) return
    const moved = normal.splice(from, 1)[0]
    let to = normal.findIndex((s) => s.id === targetId)
    if (to === -1) {
      normal.push(moved)
    } else {
      if (position === 'after') to += 1
      normal.splice(to, 0, moved)
    }
    const order = [...pinned, ...normal.map((s) => s.id)]
    // 乐观更新：按客户端计算的顺序重排本地列表
    const byId = new Map(sessions.value.map((s) => [s.id, s]))
    sessions.value = order.map((id) => byId.get(id)!).filter(Boolean)
    connect().send({ type: ClientType.SessionReorder, order })
  }

  /** 置顶/取消置顶；顺序同样以服务端回推为准 */
  function setPinned(id: string, pinned: boolean) {
    connect().send({ type: ClientType.SessionPin, sessionId: id, pinned })
  }

  return {
    connection,
    lastError,
    sessions,
    activeId,
    active,
    streaming,
    timeline,
    pending,
    query,
    grouped,
    toolCalls,
    connect,
    disconnect,
    send,
    select,
    create,
    rename,
    remove,
    interrupt,
    decide,
    refreshSessions,
    reorder,
    setPinned,
  }
})

/** 工具调用的单行摘要：优先取常见参数名，其次截断原始 JSON */
function summarize(args: string): string {
  if (!args) return ''
  try {
    const parsed = JSON.parse(args) as Record<string, unknown>
    const candidate =
      parsed.file_path ?? parsed.path ?? parsed.pattern ?? parsed.query ?? parsed.command ?? parsed.cmd
    if (typeof candidate === 'string') return candidate
  } catch {
    // 非 JSON（流式未完成的参数）直接截断显示
  }
  const oneLine = args.replace(/\s+/g, ' ').trim()
  return oneLine.length > 80 ? `${oneLine.slice(0, 80)}…` : oneLine
}

/** 工具风险等级：影响审批卡片强调，不改变可用动作 */
function riskOf(toolName: string): ApprovalRequest['risk'] {
  const high = ['run_shell', 'shell', 'bash', 'delete', 'move']
  const medium = ['write', 'write_file', 'edit', 'patch']
  const lower = toolName.toLowerCase()
  if (high.some((h) => lower.includes(h))) return 'high'
  if (medium.some((m) => lower.includes(m))) return 'medium'
  return 'low'
}
