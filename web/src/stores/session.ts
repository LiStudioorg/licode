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
  type SessionStats,
  type Settings,
  type TokenStats,
} from '@/api/protocol'
import type { AskInfo, ApprovalDecision, Block, ToolState, Turn } from '@/types/chat'
import { riskOf, summarizeArgs } from '@/utils/format'
import { useTurnMetrics } from '@/composables/useTurnMetrics'

/**
 * 会话状态：WebSocket 事件 → 视图模型的唯一映射点。
 *
 * 后端语义（决定了这里的结构）：
 *   - 一个连接服务多个会话，且不同会话可并发运行 Agent（busy 按会话隔离），
 *     因此运行时状态必须按 sessionId 分桶，而不是只存「当前会话」；
 *   - 每个事件都带 sessionId，不属于当前视图的事件只更新对应桶，不动视图；
 *   - delta / reasoning 是追加分片；tool_start / tool_done 按「同名 + 最后
 *     一个 running」配对（同一工具可被并发调用）；
 *   - done/error 会清掉该会话未应答的审批：后端在上下文结束时已清理映射，
 *     继续展示会让用户以为还能批准。
 *
 * 性能：流式分片进入缓冲区，按帧（rAF）合并落盘一次，避免每个 token
 * 触发一轮响应式更新与 Markdown 重渲染。
 */

/** agent 用 toolOut 前缀表达工具失败（不另发 error 事件） */
const TOOL_ERROR_PREFIX = 'TOOL ERROR: '

interface SessionRuntime {
  turns: Turn[]
  busy: boolean
  ask: AskInfo | null
  reasoning: string
  status: string
  /** 会话快照（snake_case：context_pct/messages/provider…） */
  stats: SessionStats | null
  /** 每轮 LLM 的 token 账目（camelCase：inputTokens/outputTokens/cachedTokens） */
  tokens: TokenStats | null
  /** 是否已请求过 history：避免重复拉全量 */
  loaded: boolean
}

function emptyRuntime(): SessionRuntime {
  return { turns: [], busy: false, ask: null, reasoning: '', status: '', stats: null, tokens: null, loaded: false }
}

export const useSessionStore = defineStore('session', () => {
  const socket = shallowRef<LicodeSocket | null>(null)
  const connection = ref<ConnectionState>('idle')
  const lastError = ref<string | null>(null)

  const sessions = ref<SessionInfo[]>([])
  const activeId = ref('')
  const query = ref('')
  const runtime = ref<Record<string, SessionRuntime>>({})

  let turnSeq = 0
  let blockSeq = 0
  /** 计时打点：服务端没有耗时字段，速度/TTFT/模型用时全在前端算 */
  const metrics = useTurnMetrics()
  /** 每个会话各自的打点序号与首 token 标记，避免多会话互相污染 */
  const sendSeq = new Map<string, number>()
  const firstSeen = new Map<string, boolean>()

  function rt(id: string): SessionRuntime {
    let r = runtime.value[id]
    if (!r) {
      r = emptyRuntime()
      runtime.value[id] = r
    }
    return r
  }

  const active = computed(() => sessions.value.find((s) => s.id === activeId.value) ?? null)
  const current = computed(() => (activeId.value ? rt(activeId.value) : emptyRuntime()))
  const turns = computed(() => current.value.turns)
  const busy = computed(() => current.value.busy)
  const ask = computed(() => current.value.ask)
  const reasoning = computed(() => current.value.reasoning)
  const statusText = computed(() => current.value.status)
  const stats = computed(() => current.value.stats)
  /** 每轮 token 账目（camelCase shape），与 stats 分开保存 */
  const tokens = computed(() => current.value.tokens)
  /** 有任一会话在跑：顶栏给出「后台还有 N 个会话在生成」的全局信号 */
  const runningCount = computed(() => Object.values(runtime.value).filter((r) => r.busy).length)

  /** 侧栏分组：置顶（服务端恒定排前）→ 有内容 → 空会话；搜索过滤后保序 */
  const grouped = computed(() => {
    const q = query.value.trim().toLowerCase()
    const hit = (s: SessionInfo) => (q ? s.title.toLowerCase().includes(q) : true)
    const list = sessions.value.filter(hit)
    return [
      { key: 'pinned', label: '置顶', items: list.filter((s) => s.pinned) },
      { key: 'recent', label: '会话', items: list.filter((s) => !s.pinned && s.count > 0) },
      { key: 'empty', label: '空会话', items: list.filter((s) => !s.pinned && s.count === 0) },
    ].filter((g) => g.items.length > 0)
  })

  /**
   * 会话视图状态：驱动 ChatView 的骨架/空态/错误/就绪四态。
   * 未连接且无内容 = loading；明确断开 = error；有会话无消息 = empty。
   */
  const viewState = computed<'loading' | 'error' | 'empty' | 'ready'>(() => {
    if (turns.value.length > 0) return 'ready'
    if (lastError.value) return 'error'
    if (connection.value === 'connecting' || connection.value === 'idle') return 'loading'
    if (connection.value === 'closed') return 'error'
    return 'empty'
  })

  /* ---------------- 流式分片缓冲 ---------------- */

  const deltaBuf = new Map<string, string>()
  const reasoningBuf = new Map<string, string>()
  let rafId: number | null = null

  function scheduleFlush() {
    if (rafId !== null) return
    const raf =
      typeof requestAnimationFrame === 'function'
        ? requestAnimationFrame
        : (cb: () => void) => setTimeout(cb, 16) as unknown as number
    rafId = raf(flush)
  }

  function flush() {
    rafId = null
    for (const [sid, text] of deltaBuf) {
      const r = rt(sid)
      appendText(r, text)
    }
    deltaBuf.clear()
    for (const [sid, text] of reasoningBuf) {
      const r = rt(sid)
      r.reasoning += text
    }
    reasoningBuf.clear()
  }

  /** 立即落盘缓冲：切换会话/一轮结束时无条件调用，避免丢尾部 */
  function flushNow() {
    if (rafId !== null && typeof cancelAnimationFrame === 'function') cancelAnimationFrame(rafId)
    flush()
  }

  /* ---------------- 块级装配 ---------------- */

  function appendText(r: SessionRuntime, text: string) {
    if (!text) return
    let turn = r.turns[r.turns.length - 1]
    if (!turn || turn.role !== 'assistant') {
      turn = { id: ++turnSeq, role: 'assistant', blocks: [] }
      r.turns.push(turn)
    }
    const last: Block | undefined = turn.blocks[turn.blocks.length - 1]
    if (last && last.kind === 'text') last.text += text
    else turn.blocks.push({ kind: 'text', id: ++blockSeq, text })
  }

  function startTool(r: SessionRuntime, name: string, args: string) {
    let turn = r.turns[r.turns.length - 1]
    if (!turn || turn.role !== 'assistant') {
      turn = { id: ++turnSeq, role: 'assistant', blocks: [] }
      r.turns.push(turn)
    }
    turn.blocks.push({
      kind: 'tool',
      id: ++blockSeq,
      name,
      args,
      out: '',
      state: 'running',
      startedAt: Date.now(),
    })
  }

  /**
   * 同名 LIFO 配对：同一工具可并发调用，最后开始的先回填。
   *
   * 失败不另有事件：agent 把错误写成 toolOut 前缀 "TOOL ERROR: "，
   * 必须据此判定状态，否则失败的工具会显示成「完成」。
   */
  function finishTool(r: SessionRuntime, name: string, out: string) {
    const isError = out.startsWith(TOOL_ERROR_PREFIX)
    for (let i = r.turns.length - 1; i >= 0; i -= 1) {
      const blocks = r.turns[i].blocks
      for (let j = blocks.length - 1; j >= 0; j -= 1) {
        const b = blocks[j]
        if (b.kind === 'tool' && b.name === name && b.state === 'running') {
          const state: ToolState = isError ? 'error' : 'success'
          b.state = state
          b.out = out
          b.durationMs = Date.now() - b.startedAt
          return
        }
      }
    }
    // 没等到 tool_start（事件乱序/断线补发）：补一条已完成的记录，不丢输出
    startTool(r, name, '')
    const turn = r.turns[r.turns.length - 1]
    const b = turn.blocks[turn.blocks.length - 1]
    if (b.kind === 'tool') {
      b.state = isError ? 'error' : 'success'
      b.out = out
    }
  }

  /** 历史回放：assistant 的多段内容 + tool_calls + tool 结果合成一条回合 */
  function replayHistory(r: SessionRuntime, messages: HistoryMessage[]) {
    r.turns = []
    let pending: Turn | null = null

    const flushPending = () => {
      if (pending) r.turns.push(pending)
      pending = null
    }

    for (const m of messages) {
      if (m.role === 'user') {
        flushPending()
        const turn: Turn = { id: ++turnSeq, role: 'user', blocks: [] }
        if (m.content) turn.blocks.push({ kind: 'text', id: ++blockSeq, text: m.content })
        const atts = (m.attachments ?? []).map((a) => ({
          type: a.type ?? 'file',
          mime_type: a.mime_type ?? '',
          filename: a.filename ?? '',
          url:
            a.type === 'image' && a.data && a.mime_type
              ? `data:${a.mime_type};base64,${a.data}`
              : undefined,
        }))
        if (atts.length) turn.attachments = atts
        if (turn.blocks.length || atts.length) r.turns.push(turn)
        continue
      }
      if (m.role === 'assistant') {
        flushPending()
        pending = { id: ++turnSeq, role: 'assistant', blocks: [] }
        if (m.content) pending.blocks.push({ kind: 'text', id: ++blockSeq, text: m.content })
        for (const tc of m.tool_calls ?? []) {
          pending.blocks.push({
            kind: 'tool',
            id: ++blockSeq,
            name: tc.function?.name ?? 'tool',
            args: tc.function?.arguments ?? '',
            out: '',
            state: 'success',
            startedAt: 0,
          })
        }
        continue
      }
      if (m.role === 'tool') {
        // tool 消息回填到最近一个还没有输出的同名工具块
        const name = m.tool_name || 'tool'
        const owner = pending ?? r.turns[r.turns.length - 1]
        let filled = false
        if (owner) {
          for (let i = owner.blocks.length - 1; i >= 0; i -= 1) {
            const b = owner.blocks[i]
            if (b.kind === 'tool' && !b.out) {
              b.out = m.content ?? ''
              if (!b.name || b.name === 'tool') b.name = name
              filled = true
              break
            }
          }
        }
        if (!filled) {
          const turn: Turn = pending ?? { id: ++turnSeq, role: 'assistant', blocks: [] }
          turn.blocks.push({
            kind: 'tool',
            id: ++blockSeq,
            name,
            args: '',
            out: m.content ?? '',
            state: 'success',
            startedAt: 0,
          })
          if (pending === null) r.turns.push(turn)
          pending = turn
        }
        continue
      }
      // system 等其它角色不进入对话视图
    }
    flushPending()
    r.loaded = true
  }

  /* ---------------- 事件路由 ---------------- */

  function handleEvent(ev: ServerEvent) {
    const sid = ev.sessionId ?? activeId.value
    switch (ev.type) {
      case ServerType.Delta:
        if (ev.content) {
          lastError.value = null
          // 本轮第一个 delta = 首 token 到达时刻，TTFT 只记一次
          if (!firstSeen.get(sid)) {
            firstSeen.set(sid, true)
            metrics.markFirstToken(sendSeq.get(sid) ?? 0)
          }
          deltaBuf.set(sid, (deltaBuf.get(sid) ?? '') + ev.content)
          scheduleFlush()
        }
        break

      case ServerType.Reasoning:
        if (ev.content) {
          reasoningBuf.set(sid, (reasoningBuf.get(sid) ?? '') + ev.content)
          scheduleFlush()
        }
        break

      case ServerType.Status:
        rt(sid).status = ev.content ?? ''
        break

      case ServerType.ToolStart:
        flushNow()
        metrics.markToolStart(`${sid}:${ev.toolName ?? 'tool'}:${sendSeq.get(sid) ?? 0}:${blockSeq}`)
        startTool(rt(sid), ev.toolName ?? 'tool', ev.toolArgs ?? '')
        break

      case ServerType.ToolDone:
        flushNow()
        metrics.markToolEnd(`${sid}:${ev.toolName ?? 'tool'}:${sendSeq.get(sid) ?? 0}:${blockSeq}`)
        finishTool(rt(sid), ev.toolName ?? 'tool', ev.toolOut ?? '')
        break

      case ServerType.Ask: {
        const r = rt(sid)
        r.ask = {
          askId: ev.askId ?? `ask-${Date.now()}`,
          toolName: ev.toolName ?? 'tool',
          toolArgs: ev.toolArgs ?? '',
        }
        break
      }

      case ServerType.Done: {
        flushNow()
        const r = rt(sid)
        r.busy = false
        r.status = ''
        r.ask = null
        // 结算本轮耗时；输出 token 数用本轮 TokenStats
        firstSeen.set(sid, false)
        metrics.markDone(sendSeq.get(sid) ?? 0, tokens.value?.outputTokens ?? 0)
        break
      }

      case ServerType.Error: {
        flushNow()
        const r = rt(sid)
        r.busy = false
        r.status = ''
        r.ask = null
        const msg = ev.error ?? '未知错误'
        // 设置保存失败单独记录（设置页要就地提示，不能只弹全局 toast）
        if (savingSettings.value) lastSettingsError.value = msg
        if (sid === activeId.value) lastError.value = msg
        break
      }

      case ServerType.Sessions:
        if (Array.isArray(ev.sessions)) sessions.value = ev.sessions
        if (ev.sessionId) setActive(ev.sessionId)
        break

      case ServerType.History:
        if (ev.messages && sid) {
          const r = rt(sid)
          // 后台仍在跑的会话只重建消息，不清 busy/ask
          replayHistory(r, ev.messages)
        }
        break

      case ServerType.Stats: {
        // 同名事件两种 shape：含 inputTokens/cachedTokens 的是每轮 TokenStats，
        // 含 context_pct 的是会话快照。两者分别合并，互不覆盖。
        const s = ev.stats as (SessionStats & TokenStats) | undefined
        if (!s) break
        const r = rt(sid)
        if (s.inputTokens !== undefined || s.outputTokens !== undefined || s.requests !== undefined) {
          r.tokens = { ...r.tokens, ...s }
        } else {
          r.stats = { ...r.stats, ...s }
        }
        break
      }

      case ServerType.Settings:
        // 后端无论保存成败都会回发快照（失败时先发 error）：
        // 收到即可解除保存态，错误文案由上面的 error 分支记录。
        savingSettings.value = false
        onSettingsEvent(ev.settings)
        break

      default:
        break
    }
  }

  /* ---------------- 与 settings store 的边界 ---------------- */

  // settings store 初始化时挂载回调（session.ts 不反向 import settings.ts，
  // 避免 store 循环依赖）。后端在 settings_set 后无论成败都回发 settings 快照
  // （失败时先发 error 再发），所以 error 与 settings 两条路径都要通知设置侧。
  const savingSettings = ref(false)
  const lastSettingsError = ref<string | null>(null)
  let onSettingsEvent: (s: Settings | undefined) => void = () => {}

  function bindSettings(handlers: { onSettings: (s: Settings | undefined) => void }) {
    onSettingsEvent = handlers.onSettings
  }

  /* ---------------- 连接与动作 ---------------- */

  function connect(): LicodeSocket {
    if (socket.value) return socket.value
    const c = new LicodeSocket({
      onEvent: handleEvent,
      onState: (s) => {
        connection.value = s
        if (s === 'open') {
          lastError.value = null
          c.send({ type: ClientType.SessionsGet })
          c.send({ type: ClientType.SettingsGet })
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

  function send(type: string, payload: Record<string, unknown> = {}) {
    connect().send({ type, ...payload })
  }

  function setActive(id: string) {
    if (activeId.value === id) return
    flushNow()
    activeId.value = id
    // 首次进入该会话时拉全量历史（history 事件会置 loaded，不会循环）
    const r = rt(id)
    if (!r.loaded && socket.value?.readyState === 'open') {
      socket.value.send({ type: ClientType.SessionHistory, sessionId: id })
    }
  }

  /** 发送一条用户消息（或 /clear 之类的服务端指令） */
  function sendUserMessage(
    content: string,
    attachments?: { type: 'image' | 'file'; mime_type: string; data: string; filename?: string; url?: string }[],
  ) {
    const text = content.trim()
    if (!text && !(attachments && attachments.length)) return
    const sid = activeId.value
    const r = rt(sid)
    if (text === '/clear' && !attachments?.length) {
      r.turns = []
      r.reasoning = ''
      r.status = ''
      r.ask = null
    } else {
      const turn: Turn = { id: ++turnSeq, role: 'user', blocks: [] }
      if (text) turn.blocks.push({ kind: 'text', id: ++blockSeq, text })
      const atts = (attachments ?? []).map((a) => ({
        type: a.type,
        mime_type: a.mime_type,
        filename: a.filename ?? '',
        url: a.url,
      }))
      if (atts.length) turn.attachments = atts
      r.turns.push(turn)
      r.reasoning = ''
      r.status = '思考中…'
    }
    r.busy = true
    r.ask = null
    lastError.value = null
    // 计时起点：本轮序号自增并打点
    const seq = (sendSeq.get(sid) ?? 0) + 1
    sendSeq.set(sid, seq)
    firstSeen.set(sid, false)
    metrics.markSend(seq)
    connect().send({
      type: ClientType.Message,
      content: text,
      attachments: attachments?.map((a) => ({
        type: a.type,
        mime_type: a.mime_type,
        data: a.data,
        filename: a.filename,
      })),
    })
  }

  function switchSession(id: string) {
    if (id === activeId.value) return
    setActive(id)
    send(ClientType.SessionSwitch, { sessionId: id })
    // 后端 session_switch 只回 sessions 不回 history（无效 id 更是静默无应答），
    // 因此显式补一次历史拉取，保证切换后视图有内容。
    refreshHistory(id)
  }

  function refreshSessions() {
    send(ClientType.SessionsGet)
  }

  /** 该会话是否正在生成（供列表的运行指示） */
  function isRunning(id: string): boolean {
    return runtime.value[id]?.busy === true
  }

  function newSession() {
    send(ClientType.SessionNew)
  }

  function rename(id: string, title: string) {
    send(ClientType.SessionRename, { sessionId: id, content: title })
  }

  function remove(id: string) {
    send(ClientType.SessionDelete, { sessionId: id })
    delete runtime.value[id]
  }

  /** 从整段对话末尾分支（index=-1 由后端语义定为复制全部） */
  function branch(id: string) {
    send(ClientType.SessionBranch, { sessionId: id, index: -1 })
  }

  function refreshHistory(id: string) {
    send(ClientType.SessionHistory, { sessionId: id })
  }

  function setPinned(id: string, pinned: boolean) {
    send(ClientType.SessionPin, { sessionId: id, pinned })
  }

  /**
   * 拖拽重排：先本地乐观重排（避免回弹观感），再发 session_reorder，
   * 服务端回推 sessions 时以其顺序为准。置顶区与普通区不互换。
   */
  function reorder(draggedId: string, targetId: string, position: 'before' | 'after') {
    const pinned = sessions.value.filter((s) => s.pinned).map((s) => s.id)
    const normal = sessions.value.filter((s) => !s.pinned)
    const from = normal.findIndex((s) => s.id === draggedId)
    if (from === -1) return
    const moved = normal.splice(from, 1)[0]
    let to = normal.findIndex((s) => s.id === targetId)
    if (to === -1) normal.push(moved)
    else {
      if (position === 'after') to += 1
      normal.splice(to, 0, moved)
    }
    const order = [...pinned, ...normal.map((s) => s.id)]
    const byId = new Map(sessions.value.map((s) => [s.id, s]))
    sessions.value = order.map((id) => byId.get(id)!).filter(Boolean)
    send(ClientType.SessionReorder, { order })
  }

  function interrupt() {
    const sid = activeId.value
    send(ClientType.Interrupt)
    flushNow()
    const r = rt(sid)
    r.busy = false
    r.status = ''
    r.ask = null
    // 未完成的工具块不可能再收到 tool_done：标记为错误，避免永久转圈
    for (const turn of r.turns) {
      for (const b of turn.blocks) {
        if (b.kind === 'tool' && b.state === 'running') {
          b.state = 'error'
          b.durationMs = Date.now() - b.startedAt
        }
      }
    }
  }

  /** 审批回执；本地立刻撤下卡片（后端不回「已应答」事件） */
  function decide(requestId: string, decision: ApprovalDecision) {
    const sid = activeId.value
    const r = rt(sid)
    if (r.ask?.askId === requestId) r.ask = null
    connect().replyAsk(requestId, decision !== 'deny', decision === 'always')
  }

  function saveSettings(next: Settings) {
    savingSettings.value = true
    lastSettingsError.value = null
    send(ClientType.SettingsSet, { settings: next })
  }

  /** 供设置页在超时/失败时解除保存态 */
  function endSettingsSave(ok: boolean, err?: string) {
    savingSettings.value = false
    if (!ok && err) lastSettingsError.value = err
  }

  function exportSession(id: string, title: string) {
    const safe = (title || 'session').replace(/[\\/:*?"<>|]/g, '_')
    const a = document.createElement('a')
    a.href = `/api/session/export?session_id=${encodeURIComponent(id)}`
    a.download = `${safe}-${id}.md`
    document.body.appendChild(a)
    a.click()
    a.remove()
  }

  return {
    connection,
    lastError,
    sessions,
    activeId,
    active,
    query,
    grouped,
    turns,
    busy,
    ask,
    reasoning,
    statusText,
    stats,
    tokens,
    runningCount,
    viewState,
    savingSettings,
    lastSettingsError,
    bindSettings,
    connect,
    disconnect,
    send,
    sendUserMessage,
    switchSession,
    refreshSessions,
    isRunning,
    newSession,
    rename,
    remove,
    branch,
    refreshHistory,
    setPinned,
    reorder,
    interrupt,
    decide,
    saveSettings,
    endSettingsSave,
    exportSession,
    flushNow,
    summarize: summarizeArgs,
    riskOf,
  }
})
