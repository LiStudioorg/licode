/** 显示格式化工具：字节、耗时、相对时间、token 数 */

export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return '—'
  if (n < 1024) return `${n} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let v = n / 1024
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i += 1
  }
  return `${v >= 100 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

export function formatDuration(ms?: number): string | null {
  if (ms === undefined) return null
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

/** 12345 -> 1.2万；小于一万原样 */
export function formatCount(n?: number): string {
  if (n === undefined) return '—'
  if (n < 10000) return String(n)
  if (n < 100000000) return `${(n / 10000).toFixed(1)}万`
  return `${(n / 100000000).toFixed(2)}亿`
}

/** 相对时间：刚刚 / 5 分钟前 / 昨天 / MM-DD；用于会话列表 */
export function relativeTime(ts: number): string {
  const diff = Date.now() - ts
  if (diff < 60_000) return '刚刚'
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)} 分钟前`
  if (diff < 86400_000) return `${Math.floor(diff / 3600_000)} 小时前`
  if (diff < 172800_000) return '昨天'
  const d = new Date(ts)
  return `${d.getMonth() + 1}-${d.getDate()}`
}

export function formatTime(d: Date | number = Date.now()): string {
  const x = typeof d === 'number' ? new Date(d) : d
  return x.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

/** 截断单行摘要（按码点安全截断，避免劈开 emoji/生僻字） */
export function truncate(s: string, max = 80): string {
  const oneLine = s.replace(/\s+/g, ' ').trim()
  if (oneLine.length <= max) return oneLine
  return `${Array.from(oneLine).slice(0, max).join('')}…`
}

/** 工具入参的单行摘要：优先常见参数名，其次截断原始 JSON */
export function summarizeArgs(args: string): string {
  if (!args) return ''
  try {
    const parsed = JSON.parse(args) as Record<string, unknown>
    const candidate =
      parsed.file_path ?? parsed.path ?? parsed.pattern ?? parsed.query ?? parsed.command ?? parsed.cmd ?? parsed.url
    if (typeof candidate === 'string') return truncate(candidate)
  } catch {
    /* 流式未完成的 JSON：直接截断显示 */
  }
  return truncate(args)
}

/** 按工具名推断审批风险：仅影响视觉强调，不改变可用动作 */
export function riskOf(toolName: string): 'low' | 'medium' | 'high' {
  const t = toolName.toLowerCase()
  if (/(shell|bash|exec|delete|remove|move|kill|chmod|chown|install|deploy)/.test(t)) return 'high'
  if (/(write|edit|patch|create|mkdir|upload|save)/.test(t)) return 'medium'
  return 'low'
}

/**
 * 大数缩写：K / M / B（英文单位）。
 *
 * 与 formatCount 的分工：formatCount 用中文「万/亿」，用于笼统数量；
 * 这里用于状态栏的 token 计量，写法与主流 LLM 工具链一致（30.7M tok）。
 * 保留一位小数，整数位不带小数点。
 */
export function formatMetric(n?: number | null): string {
  if (n === undefined || n === null || !Number.isFinite(n)) return '0'
  const abs = Math.abs(n)
  const sign = n < 0 ? '-' : ''
  const trim = (v: number) => {
    const t = v.toFixed(1)
    return t.endsWith('.0') ? t.slice(0, -2) : t
  }
  if (abs >= 1e9) return `${sign}${trim(abs / 1e9)}B`
  if (abs >= 1e6) return `${sign}${trim(abs / 1e6)}M`
  if (abs >= 1e3) return `${sign}${trim(abs / 1e3)}K`
  return `${n}`
}
