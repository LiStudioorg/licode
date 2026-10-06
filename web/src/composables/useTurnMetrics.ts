import { computed, ref } from 'vue'

/**
 * 会话内计时指标。
 *
 * 背景：服务端 TokenStats 只有 requests/inputTokens/outputTokens/cachedTokens
 * 四个字段，**没有任何耗时字段**，因此输出速度、TTFT、模型用时只能由前端
 * 自己打点。
 *
 * 打点时机（由 session store 调用）：
 *   markSend(seq)       用户消息发出
 *   markFirstToken(seq) 收到本轮第一个 delta
 *   markDone(seq,out)   本轮 done，回填输出 token 数
 *   markToolStart/End   工具调用前后
 *
 * 数据只存在于内存：刷新页面即丢失，此时界面显示 "-"，不假装有值。
 */

interface Sample {
  /** 发出 → 首个 token 的毫秒数；0 表示未观测到 */
  ttftMs: number
  /** 发出 → 完成的毫秒数 */
  totalMs: number
  /** 本轮输出 token 数 */
  outputTokens: number
}

/** 求平均时只看最近若干轮：兼顾稳定性与对当前状态的敏感度 */
const WINDOW = 8

const samples = ref<Sample[]>([])
const toolMs = ref(0)

/** 进行中的轮次打点，key 为轮次序号 */
const pending = new Map<number, { sentAt: number; firstAt: number }>()
/** 进行中的工具打点，key 为工具块 id */
const toolOpen = new Map<string, number>()

export function useTurnMetrics() {
  function markSend(seq: number) {
    pending.set(seq, { sentAt: performance.now(), firstAt: 0 })
  }

  function markFirstToken(seq: number) {
    const p = pending.get(seq)
    if (p && !p.firstAt) p.firstAt = performance.now()
  }

  function markDone(seq: number, outputTokens: number) {
    const p = pending.get(seq)
    if (!p) return
    const now = performance.now()
    pending.delete(seq)
    samples.value = [
      ...samples.value,
      {
        ttftMs: p.firstAt ? p.firstAt - p.sentAt : 0,
        totalMs: now - p.sentAt,
        outputTokens,
      },
    ].slice(-50)
  }

  function markToolStart(id: string) {
    toolOpen.set(id, performance.now())
  }

  function markToolEnd(id: string) {
    const t0 = toolOpen.get(id)
    if (t0 === undefined) return
    toolOpen.delete(id)
    toolMs.value += performance.now() - t0
  }

  const recent = computed(() => samples.value.slice(-WINDOW))

  /** 首 token 平均（TTFT）；无样本返回 null */
  const ttftMs = computed(() => {
    const hit = recent.value.filter((s) => s.ttftMs > 0)
    if (!hit.length) return null
    return Math.round(hit.reduce((n, s) => n + s.ttftMs, 0) / hit.length)
  })

  /**
   * 输出速度。分母用 totalMs - ttftMs：扣掉排队与首 token 等待，
   * 得到的才是真实生成速度，否则长提示词会把速度压得毫无意义。
   */
  const tps = computed(() => {
    const usable = recent.value.filter((s) => s.outputTokens > 0 && s.totalMs > s.ttftMs)
    if (!usable.length) return null
    const tokens = usable.reduce((n, s) => n + s.outputTokens, 0)
    const ms = usable.reduce((n, s) => n + (s.totalMs - s.ttftMs), 0)
    if (ms <= 0) return null
    return Math.round((tokens / ms) * 1000)
  })

  /** 模型用时：所有样本总耗时累加（含等待） */
  const modelMs = computed(() => Math.round(samples.value.reduce((n, s) => n + s.totalMs, 0)))

  /** 工具调用累计用时 */
  const toolTotalMs = computed(() => Math.round(toolMs.value))

  function reset() {
    samples.value = []
    toolMs.value = 0
    pending.clear()
    toolOpen.clear()
  }

  return {
    markSend,
    markFirstToken,
    markDone,
    markToolStart,
    markToolEnd,
    ttftMs,
    tps,
    modelMs,
    toolTotalMs,
    reset,
  }
}
