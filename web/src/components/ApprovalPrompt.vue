<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { ApprovalDecision, ApprovalRequest } from '@/types/tool'

/**
 * 工具审批。
 *
 * 安全契约：
 *   - 三个动作的视觉权重刻意不同：允许是主操作，拒绝是中性操作，
 *     「始终允许」是带持久后果的次级操作，绝不做成最显眼的按钮；
 *   - 键盘默认焦点落在「拒绝」上，避免回车误放行；
 *   - 高风险请求额外用 danger 色，并强制展开 detail。
 */
const props = defineProps<{
  request: ApprovalRequest
}>()

const emit = defineEmits<{
  (e: 'decide', decision: ApprovalDecision): void
}>()

const denyButton = ref<HTMLButtonElement | null>(null)
const root = ref<HTMLElement | null>(null)
const submitting = ref<ApprovalDecision | null>(null)

const riskLabel = computed(() => {
  switch (props.request.risk) {
    case 'high':
      return '高风险'
    case 'medium':
      return '中等风险'
    case 'low':
      return '低风险'
  }
})

const riskClass = computed(() => {
  switch (props.request.risk) {
    case 'high':
      return 'border-danger/40 text-danger'
    case 'medium':
      return 'border-warn/40 text-warn'
    case 'low':
      return 'border-line text-ink-400'
  }
})

/** 高风险强制展示完整内容，不给「盲签」的机会 */
const showDetail = computed(() => props.request.risk !== 'low')

function decide(decision: ApprovalDecision) {
  if (submitting.value !== null) return
  submitting.value = decision
  emit('decide', decision)
}

/**
 * 键盘快捷键：Enter 允许 / Esc 拒绝。
 *
 * 关键约束：快捷键必须绑定在组件自身而非 window 上。
 * 消息流里可能同时存在多个待审批请求，若各自监听 window，
 * 一次 Enter 会同时放行全部请求（安全缺陷）。
 * 这里改为监听组件根节点的 keydown，只有焦点在卡片内时才生效。
 */
const shortcutsEnabled = ref(false)

function onKeydown(e: KeyboardEvent) {
  if (e.metaKey || e.ctrlKey || e.altKey) return
  if (!shortcutsEnabled.value) return
  const target = e.target as HTMLElement | null
  if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA')) return
  if (e.key === 'Enter') {
    e.preventDefault()
    decide('allow')
  } else if (e.key === 'Escape') {
    e.preventDefault()
    decide('deny')
  }
}

/** 焦点进入卡片时启用快捷键，离开即失效，避免跨卡片误触发 */
function onFocusIn() {
  shortcutsEnabled.value = true
}

function onFocusOut(e: FocusEvent) {
  const next = e.relatedTarget as Node | null
  if (next && root.value?.contains(next)) return
  shortcutsEnabled.value = false
}

onMounted(() => {
  // 默认焦点给「拒绝」：放行必须是主动行为
  denyButton.value?.focus()
})
</script>

<template>
  <div
    ref="root"
    class="rounded-lg border bg-white/[0.02]"
    :class="request.risk === 'high' ? 'border-danger/40' : 'border-line-strong'"
    role="group"
    aria-label="工具调用审批"
    @keydown="onKeydown"
    @focusin="onFocusIn"
    @focusout="onFocusOut"
  >
    <div class="flex items-center gap-2 px-3 py-2">
      <svg
        viewBox="0 0 16 16"
        class="size-3.5 shrink-0 text-warn"
        fill="none"
        stroke="currentColor"
        stroke-width="1.4"
        aria-hidden="true"
      >
        <path d="M8 5.5v3.5M8 11.2v.1" stroke-linecap="round" />
        <path d="M6.9 2.3L1.6 12a1.2 1.2 0 001.05 1.8h10.7A1.2 1.2 0 0014.4 12L9.1 2.3a1.2 1.2 0 00-2.2 0z" />
      </svg>
      <span class="text-ink-100">需要确认</span>
      <span class="rounded-lg border px-1.5 text-[11px]" :class="riskClass">{{ riskLabel }}</span>
      <span class="ml-auto font-mono text-[11px] text-ink-500">{{ request.requestedAt }}</span>
    </div>

    <div class="border-t border-line px-3 py-2">
      <p class="text-ink-300">
        <span class="font-mono text-[12px] text-ink-200">{{ request.toolName }}</span>
        将要执行以下操作
      </p>
      <pre
        v-if="showDetail"
        class="mt-1.5 max-h-40 overflow-auto rounded-lg border border-line bg-sunken/60 px-2 py-1.5 font-mono text-[12px] leading-relaxed whitespace-pre-wrap text-ink-200"
      >{{ request.detail }}</pre>
      <p v-else class="mt-1 truncate font-mono text-[12px] text-ink-400">{{ request.detail }}</p>
    </div>

    <div class="flex flex-wrap items-center gap-2 border-t border-line px-3 py-2">
      <button
        ref="denyButton"
        type="button"
        class="rounded-lg border border-line px-2.5 py-1 text-ink-200 transition-colors hover:border-line-strong hover:bg-white/[0.04] active:scale-[0.98] disabled:opacity-50"
        :disabled="submitting !== null"
        @click="decide('deny')"
      >
        拒绝
      </button>

      <button
        type="button"
        class="rounded-lg bg-accent px-3 py-1 text-canvas transition-opacity hover:opacity-90 active:scale-[0.98] disabled:opacity-50"
        :disabled="submitting !== null"
        @click="decide('allow')"
      >
        允许
      </button>

      <button
        type="button"
        class="rounded-lg border border-line px-2.5 py-1 text-ink-300 transition-colors hover:border-line-strong hover:bg-white/[0.04] active:scale-[0.98] disabled:opacity-50"
        :disabled="submitting !== null"
        @click="decide('always')"
      >
        始终允许
      </button>

      <span class="ml-auto font-mono text-[11px] text-ink-500">Enter 允许 / Esc 拒绝</span>
    </div>

    <p class="border-t border-line px-3 py-1.5 text-[11px] text-ink-500">
      「始终允许」仅在当前会话生效，切换会话后重新询问
    </p>
  </div>
</template>
