import { ref } from 'vue'

/**
 * 全局轻提示。独立 composable（非 Pinia）：状态是一条短生命周期的队列，
 * 不需要 devtools 追踪，也不该被业务 store 依赖。
 */
export type ToastKind = 'ok' | 'error' | 'info'

export interface ToastItem {
  id: number
  kind: ToastKind
  text: string
}

const items = ref<ToastItem[]>([])
let seq = 0

function push(kind: ToastKind, text: string, ms: number) {
  const id = ++seq
  items.value.push({ id, kind, text })
  // 同时最多 4 条：超限时丢最旧，避免刷屏时铺满整屏
  if (items.value.length > 4) items.value.shift()
  setTimeout(() => dismiss(id), ms)
}

export function dismiss(id: number) {
  items.value = items.value.filter((t) => t.id !== id)
}

export function useToast() {
  return {
    items,
    ok: (text: string) => push('ok', text, 2600),
    error: (text: string) => push('error', text, 5200),
    info: (text: string) => push('info', text, 3200),
    dismiss,
  }
}
