import { ClientType, type OutgoingMessage, type ServerEvent } from './protocol'

/**
 * Licode WebSocket 客户端。
 *
 * 职责边界：只负责连接生命周期、收发编解码、心跳与重连；
 * 不持有任何 UI 或业务状态（那些在 Pinia store 里）。
 * 这样断线重连时状态不会与 socket 实例耦合。
 *
 * 关键行为：
 *   - 同源连接：浏览器下用当前 origin 推导 ws(s)://，天然通过 Go 端
 *     originMatchesHost 的同源校验（端口敏感，不能用硬编码 host）；
 *   - 指数退避重连，带抖动，避免后端重启时全体客户端同时打满；
 *   - 应用层 ping 保活（Go 端读超时 75s，这里 25s 一次）；
 *   - 断线期间发送的消息进入待发队列，连上后按序补发。
 */

export interface SocketClientOptions {
  /** 覆盖连接地址；默认按当前页面 origin 推导 */
  url?: string
  /** 收到任何服务端事件时的回调 */
  onEvent: (event: ServerEvent) => void
  /** 连接状态变化回调 */
  onState?: (state: 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed') => void
}

type State = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed'

const PING_INTERVAL = 25_000
const BASE_BACKOFF = 800
const MAX_BACKOFF = 20_000
/** 待发队列上限，防止长时间离线时内存无界增长 */
const MAX_PENDING = 100

export class LicodeSocket {
  private ws: WebSocket | null = null
  private state: State = 'idle'
  private attempt = 0
  private pingTimer: ReturnType<typeof setInterval> | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private pending: OutgoingMessage[] = []
  private closedByUser = false

  constructor(private readonly opts: SocketClientOptions) {}

  get readyState(): State {
    return this.state
  }

  private setState(next: State) {
    if (this.state === next) return
    this.state = next
    this.opts.onState?.(next)
  }

  /** 由当前页面 origin 推导 WebSocket 地址，保证与后端同源校验一致 */
  private resolveUrl(): string {
    if (this.opts.url) return this.opts.url
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${proto}//${window.location.host}/ws`
  }

  connect() {
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return
    }
    this.closedByUser = false
    this.setState(this.attempt === 0 ? 'connecting' : 'reconnecting')

    let ws: WebSocket
    try {
      ws = new WebSocket(this.resolveUrl())
    } catch {
      this.scheduleReconnect()
      return
    }
    this.ws = ws

    ws.onopen = () => {
      this.attempt = 0
      this.setState('open')
      this.startPing()
      this.flushPending()
    }

    ws.onmessage = (ev) => {
      let event: ServerEvent
      try {
        event = JSON.parse(ev.data as string) as ServerEvent
      } catch {
        // 非 JSON 帧（不应出现）直接忽略，不让坏帧打断连接
        return
      }
      if (!event || typeof event.type !== 'string') return
      this.opts.onEvent(event)
    }

    ws.onclose = () => {
      this.stopPing()
      this.ws = null
      if (this.closedByUser) {
        this.setState('closed')
        return
      }
      this.scheduleReconnect()
    }

    ws.onerror = () => {
      // onerror 后浏览器必然触发 onclose，重连逻辑统一走 onclose，避免双触发
    }
  }

  private startPing() {
    this.stopPing()
    // 应用层 ping：Go 端持有 75s 读超时，靠它顺延，防止空闲被判定为半开连接
    this.pingTimer = setInterval(() => {
      this.send({ type: ClientType.Ping })
    }, PING_INTERVAL)
  }

  private stopPing() {
    if (this.pingTimer) {
      clearInterval(this.pingTimer)
      this.pingTimer = null
    }
  }

  private scheduleReconnect() {
    if (this.closedByUser) return
    this.setState('reconnecting')
    this.attempt += 1
    // 指数退避 + 抖动：避免后端重启瞬间全体客户端同时重连
    const backoff = Math.min(BASE_BACKOFF * 2 ** (this.attempt - 1), MAX_BACKOFF)
    const jitter = Math.random() * backoff * 0.3
    this.reconnectTimer = setTimeout(() => this.connect(), backoff + jitter)
  }

  private flushPending() {
    if (this.pending.length === 0) return
    const queue = this.pending
    this.pending = []
    for (const msg of queue) this.send(msg)
  }

  /** 发送一条消息；未连接时入队，连上后补发 */
  send(msg: OutgoingMessage) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg))
      return
    }
    if (this.pending.length >= MAX_PENDING) {
      // 丢弃最旧的：宁可丢历史，也不让内存无界增长
      this.pending.shift()
    }
    this.pending.push(msg)
    this.connect()
  }

  /** 审批回执。always 为 true 时表示「始终允许」（仅当前会话生效） */
  replyAsk(askId: string, approve: boolean, always = false) {
    this.send({
      type: ClientType.AskReply,
      askId,
      askApprove: approve,
      askAlways: always,
    })
  }

  /** 用户主动断开：不再自动重连 */
  close() {
    this.closedByUser = true
    this.stopPing()
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.ws?.close()
    this.ws = null
    this.setState('closed')
  }
}
