<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import type { AskInfo, ApprovalDecision } from '@/types/chat'
import { riskOf } from '@/utils/format'
import AppIcon from '@/components/ui/AppIcon.vue'

/**
 * 工具审批卡。
 *
 * 安全契约：
 *   - 焦点默认落在「拒绝」：放行必须是主动动作；
 *   - 快捷键只绑在本组件上（不能绑 window）：消息流里可能同时有多张卡，
 *     绑 window 会让一次 Enter 放行全部请求；
 *   - 高/中风险强制展示完整参数，不给盲签机会。
 *
 * 后端语义：askAlways 记录「本会话始终允许」，且与批准与否无关（拒绝也会记），
 * 因此「始终允许」按钮显式限定作用范围文案。
 */
const props = defineProps<{ ask: AskInfo }>()
const emit = defineEmits<{ (e: 'decide', d: ApprovalDecision): void }>()

const root = ref<HTMLElement | null>(null)
const denyBtn = ref<HTMLButtonElement | null>(null)
const submitting = ref<ApprovalDecision | null>(null)

const risk = computed(() => riskOf(props.ask.toolName))

const riskLabel = computed(() => ({ low: '低风险', medium: '需确认', high: '高风险' })[risk.value])
const riskClass = computed(
  () =>
    ({
      low: 'border-line text-ink-400',
      medium: 'border-warn/40 text-warn',
      high: 'border-danger/40 text-danger',
    })[risk.value],
)

/** 工作目录外路径确认时后端发 toolName="Path" 且 args 是纯文本，不是 JSON */
const detail = computed(() => {
  const a = props.ask.toolArgs
  if (!a) return ''
  try {
    return JSON.stringify(JSON.parse(a), null, 2)
  } catch {
    return a
  }
})

const showDetail = computed(() => risk.value !== 'low' || detail.value.length < 120)

function decide(d: ApprovalDecision) {
  if (submitting.value !== null) return
  submitting.value = d
  emit('decide', d)
}

const shortcutsOn = ref(false)

function onKeydown(e: KeyboardEvent) {
  if (e.metaKey || e.ctrlKey || e.altKey || !shortcutsOn.value) return
  const t = e.target as HTMLElement | null
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA')) return
  if (e.key === 'Enter') {
    e.preventDefault()
    decide('allow')
  } else if (e.key === 'Escape') {
    e.preventDefault()
    decide('deny')
  }
}

function focusDeny() {
  void nextTick(() => denyBtn.value?.focus())
}
</script>

<template>
  <div
    ref="root"
    class="rounded-[10px] border bg-white/[0.02]"
    :class="risk === 'high' ? 'border-danger/45' : 'border-line-strong'"
    role="group"
    aria-label="工具调用审批"
    @keydown="onKeydown"
    @focusin="shortcutsOn = true"
    @focusout="
      (e) => {
        const next = e.relatedTarget as Node | null
        if (next && root?.contains(next)) return
        shortcutsOn = false
      }
    "
    @vue:mounted="focusDeny"
  >
    <div class="flex items-center gap-2 px-3 py-2">
      <AppIcon name="shield" class="size-3.5 shrink-0" :class="risk === 'high' ? 'text-danger' : 'text-warn'" />
      <span class="text-ink-100">需要确认</span>
      <span class="rounded border px-1.5 text-[11px]" :class="riskClass">{{ riskLabel }}</span>
      <span class="ml-auto font-mono text-[11px] text-ink-500">{{ ask.toolName }}</span>
    </div>

    <div class="border-t border-line px-3 py-2">
      <p v-if="ask.toolName === 'Path'" class="text-ink-300">请求访问工作目录之外的路径</p>
      <pre
        v-if="showDetail && detail"
        class="max-h-44 overflow-auto whitespace-pre-wrap break-words rounded-lg border border-line bg-sunken/60 px-2 py-1.5 font-mono text-[12px] leading-relaxed text-ink-200"
      >{{ detail }}</pre>
      <p v-else class="truncate font-mono text-[12px] text-ink-400">{{ detail }}</p>
    </div>

    <div class="flex flex-wrap items-center gap-2 border-t border-line px-3 py-2">
      <button
        ref="denyBtn"
        type="button"
        class="rounded-lg border border-line px-2.5 py-1 text-ink-200 transition-colors hover:border-line-strong hover:bg-white/[0.05] active:scale-[0.98] disabled:opacity-50"
        :disabled="submitting !== null"
        @click="decide('deny')"
      >
        拒绝
      </button>
      <button
        type="button"
        class="rounded-lg bg-accent px-3 py-1 text-[var(--color-accent-ink)] transition-opacity hover:opacity-90 active:scale-[0.98] disabled:opacity-50"
        :disabled="submitting !== null"
        @click="decide('allow')"
      >
        允许一次
      </button>
      <button
        type="button"
        class="rounded-lg border border-line px-2.5 py-1 text-ink-300 transition-colors hover:border-line-strong hover:bg-white/[0.05] active:scale-[0.98] disabled:opacity-50"
        :disabled="submitting !== null"
        @click="decide('always')"
      >
        始终允许
      </button>
      <span class="ml-auto font-mono text-[11px] text-ink-600">Enter 允许 / Esc 拒绝</span>
    </div>

    <p class="border-t border-line px-3 py-1.5 text-[11px] text-ink-500">
      「始终允许」仅在本会话内对该工具生效
    </p>
  </div>
</template>
