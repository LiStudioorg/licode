<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import type { ToolBlock } from '@/types/chat'
import { formatDuration, truncate } from '@/utils/format'
import AppIcon from '@/components/ui/AppIcon.vue'

/**
 * 工具调用卡片。
 *
 * 折叠策略：running 自动展开看实时输出；结束后长输出自动折叠；
 * 用户手动开合后不再被自动策略覆盖（manual）。
 *
 * 流式跟随：新输出落在可视区外时自动滚到底；用户一旦上滚就停止跟随，
 * 并提供明确的「回到最新」入口 —— 否则读历史输出会被反复拽回底部。
 */
const props = defineProps<{ call: ToolBlock }>()

const manual = ref<boolean | null>(null)
const AUTO_COLLAPSE_LINES = 12

const lineCount = computed(() => props.call.out.split('\n').length)
const open = computed(() => {
  if (manual.value !== null) return manual.value
  if (props.call.state === 'running') return true
  return lineCount.value <= AUTO_COLLAPSE_LINES
})

function toggle() {
  manual.value = !open.value
}

/** 展示用参数摘要：JSON 成功解析时挑常见键，否则截断原文 */
const summary = computed(() => {
  const a = props.call.args
  if (!a) return ''
  try {
    const p = JSON.parse(a) as Record<string, unknown>
    for (const k of ['file_path', 'path', 'pattern', 'query', 'command', 'cmd', 'url', 'dir']) {
      if (typeof p[k] === 'string') return truncate(p[k] as string)
    }
    return truncate(a, 90)
  } catch {
    return truncate(a, 90)
  }
})

/** 展开时尽量给出格式化的入参；不是合法 JSON（如 Path 确认）就原样显示 */
const prettyArgs = computed(() => {
  const a = props.call.args
  if (!a) return ''
  try {
    return JSON.stringify(JSON.parse(a), null, 2)
  } catch {
    return a
  }
})

const stateLabel = computed(
  () =>
    ({ running: '执行中', success: '完成', error: '失败' })[props.call.state],
)

const icon = computed(
  () => ({ running: 'spinner', success: 'check', error: 'error' })[props.call.state] as 'spinner' | 'check' | 'error',
)

const tone = computed(
  () =>
    ({
      running: 'border-accent-line',
      success: 'border-line',
      error: 'border-danger/35',
    })[props.call.state],
)

const stateColor = computed(
  () =>
    ({ running: 'text-accent', success: 'text-ink-500', error: 'text-danger' })[props.call.state],
)

const duration = computed(() => formatDuration(props.call.durationMs))

/** 输出里被后端标记的截断提示单独强调（超 8000 字符时后端已裁过头尾） */
const outTruncated = computed(() => props.call.out.includes('输出过长'))

const outputEl = ref<HTMLElement | null>(null)
const follow = ref(true)

function onScroll() {
  const el = outputEl.value
  if (!el) return
  follow.value = el.scrollHeight - el.scrollTop - el.clientHeight < 24
}

watch(
  () => props.call.out,
  async () => {
    if (props.call.state !== 'running' || !follow.value) return
    await nextTick()
    const el = outputEl.value
    if (el) el.scrollTop = el.scrollHeight
  },
)

function backToLatest() {
  follow.value = true
  const el = outputEl.value
  if (el) el.scrollTop = el.scrollHeight
}
</script>

<template>
  <div class="overflow-hidden rounded-[10px] border bg-white/[0.02] transition-colors" :class="tone">
    <button
      type="button"
      class="flex w-full items-center gap-2 px-2.5 py-1.5 text-left"
      :aria-expanded="open"
      @click="toggle"
    >
      <AppIcon
        :name="icon"
        class="size-3.5 shrink-0"
        :class="[stateColor, call.state === 'running' ? 'animate-spin motion-reduce:animate-none' : '']"
      />
      <span class="shrink-0 font-mono text-[12px] text-ink-200">{{ call.name }}</span>
      <span class="min-w-0 flex-1 truncate text-ink-400">{{ summary }}</span>
      <span class="shrink-0" :class="stateColor">{{ stateLabel }}</span>
      <span v-if="duration" class="shrink-0 font-mono text-[11px] text-ink-600">{{ duration }}</span>
      <AppIcon name="chevronRight" class="size-3.5 shrink-0 text-ink-500 transition-transform duration-150" :class="open ? 'rotate-90' : ''" />
    </button>

    <div v-if="open" class="border-t border-line px-2.5 py-2">
      <template v-if="prettyArgs">
        <p class="pb-1 text-[11px] text-ink-500">参数</p>
        <pre class="max-h-40 overflow-auto whitespace-pre-wrap break-words font-mono text-[12px] leading-relaxed text-ink-300">{{ prettyArgs }}</pre>
      </template>

      <div class="mt-2 flex items-center gap-2">
        <p class="text-[11px] text-ink-500">输出</p>
        <span v-if="outTruncated" class="rounded border border-warn/35 px-1 text-[10px] text-warn">已截断</span>
      </div>
      <pre
        v-if="call.out"
        ref="outputEl"
        class="mt-1 max-h-60 overflow-auto whitespace-pre-wrap break-words font-mono text-[12px] leading-relaxed text-ink-300"
        @scroll="onScroll"
      >{{ call.out }}</pre>
      <p v-else class="mt-1 font-mono text-[12px] text-ink-500">
        {{ call.state === 'running' ? '等待输出…' : '无输出' }}
      </p>

      <button
        v-if="call.state === 'running' && !follow"
        type="button"
        class="mt-2 rounded-lg border border-line px-2 py-0.5 text-[11px] text-ink-300 transition-colors hover:border-line-strong hover:bg-white/[0.05]"
        @click="backToLatest"
      >
        回到最新输出
      </button>
    </div>
  </div>
</template>
