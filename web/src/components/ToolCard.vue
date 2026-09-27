<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import type { ToolCall } from '@/types/tool'

/**
 * 流式工具卡片。
 *
 * 状态机：running → success | error | denied，单向不可逆。
 * 折叠契约：
 *   - running 阶段自动展开，让用户看到实时输出；
 *   - 结束且输出超过阈值时自动折叠，避免长输出撑爆消息流；
 *   - 用户手动切换后不再被自动策略改动（manual 标记）。
 */
const props = defineProps<{
  call: ToolCall
}>()

const manual = ref<boolean | null>(null)

const AUTO_COLLAPSE_LINES = 12

const lineCount = computed(() => props.call.output.split('\n').length)

const open = computed(() => {
  if (manual.value !== null) return manual.value
  if (props.call.state === 'running') return true
  return lineCount.value <= AUTO_COLLAPSE_LINES
})

function toggle() {
  manual.value = !open.value
}

const stateLabel = computed(() => {
  switch (props.call.state) {
    case 'running':
      return '执行中'
    case 'success':
      return '完成'
    case 'error':
      return '失败'
    case 'denied':
      return '已拒绝'
  }
})

/** 耗时人类可读化；不做假精度，只给一位小数 */
const duration = computed(() => {
  const ms = props.call.durationMs
  if (ms === undefined) return null
  return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`
})

const toneClass = computed(() => {
  switch (props.call.state) {
    case 'running':
      return 'border-accent-line'
    case 'error':
      return 'border-danger/40'
    case 'denied':
      return 'border-line'
    case 'success':
      return 'border-line'
  }
})

const statusColor = computed(() => {
  switch (props.call.state) {
    case 'running':
      return 'text-accent'
    case 'error':
      return 'text-danger'
    case 'denied':
      return 'text-ink-400'
    case 'success':
      return 'text-ink-400'
  }
})

/**
 * 流式跟随：输出超过 max-h-64 后新内容会落在可视区外，
 * 若不跟随就看不到实时尾部。
 * 但用户一旦主动向上滚动，就停止自动跟随（sticky 标记），
 * 否则阅读历史输出时会被不断拽回底部。
 */
const outputEl = ref<HTMLElement | null>(null)
const follow = ref(true)

/** 距底部 24px 内视为「贴底」，留一点容差避免亚像素抖动误判 */
function onScroll() {
  const el = outputEl.value
  if (!el) return
  const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 24
  follow.value = atBottom
}

watch(
  () => props.call.output,
  async () => {
    if (props.call.state !== 'running' || !follow.value) return
    await nextTick()
    const el = outputEl.value
    if (el) el.scrollTop = el.scrollHeight
  },
)
</script>

<template>
  <div class="rounded-lg border bg-white/[0.02] transition-colors" :class="toneClass">
    <!-- 头部：整行可点，承担折叠开关 -->
    <button
      type="button"
      class="flex w-full items-center gap-2 px-2.5 py-2 text-left"
      :aria-expanded="open"
      @click="toggle"
    >
      <!-- 运行中转圈：唯一允许的持续动画，传达真实状态 -->
      <svg
        v-if="call.state === 'running'"
        viewBox="0 0 16 16"
        class="size-3.5 shrink-0 animate-spin text-accent motion-reduce:animate-none"
        fill="none"
        stroke="currentColor"
        stroke-width="1.4"
        aria-hidden="true"
      >
        <path d="M8 1.75a6.25 6.25 0 106.25 6.25" stroke-linecap="round" />
      </svg>
      <svg
        v-else-if="call.state === 'success'"
        viewBox="0 0 16 16"
        class="size-3.5 shrink-0 text-ink-400"
        fill="none"
        stroke="currentColor"
        stroke-width="1.4"
        aria-hidden="true"
      >
        <path d="M3.5 8.5l3 3 6-7" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
      <svg
        v-else-if="call.state === 'error'"
        viewBox="0 0 16 16"
        class="size-3.5 shrink-0 text-danger"
        fill="none"
        stroke="currentColor"
        stroke-width="1.4"
        aria-hidden="true"
      >
        <path d="M8 5v4M8 11.2v.1" stroke-linecap="round" />
        <circle cx="8" cy="8" r="6.25" />
      </svg>
      <svg
        v-else
        viewBox="0 0 16 16"
        class="size-3.5 shrink-0 text-ink-500"
        fill="none"
        stroke="currentColor"
        stroke-width="1.4"
        aria-hidden="true"
      >
        <circle cx="8" cy="8" r="6.25" />
        <path d="M5.5 8h5" stroke-linecap="round" />
      </svg>

      <span class="shrink-0 font-mono text-[12px] text-ink-200">{{ call.name }}</span>
      <span class="min-w-0 flex-1 truncate text-ink-400">{{ call.summary }}</span>

      <span class="shrink-0 text-[11px]" :class="statusColor">{{ stateLabel }}</span>
      <span v-if="duration" class="shrink-0 font-mono text-[11px] text-ink-500">{{ duration }}</span>

      <svg
        viewBox="0 0 16 16"
        class="size-3.5 shrink-0 text-ink-500 transition-transform duration-150"
        :class="open ? 'rotate-90' : ''"
        fill="none"
        stroke="currentColor"
        stroke-width="1.4"
        aria-hidden="true"
      >
        <path d="M6 4l4 4-4 4" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    </button>

    <!-- 输出区：等宽字体，横向可滚，不撑破中栏 -->
    <div v-if="open" class="border-t border-line px-2.5 py-2">
      <pre
        v-if="call.output"
        ref="outputEl"
        class="max-h-64 overflow-auto font-mono text-[12px] leading-relaxed whitespace-pre-wrap text-ink-300"
        @scroll="onScroll"
      >{{ call.output }}</pre>
      <p v-else-if="call.state === 'running'" class="font-mono text-[12px] text-ink-500">等待输出</p>
      <p v-else class="font-mono text-[12px] text-ink-500">无输出</p>

      <!-- 流式光标：running 时贴在输出末尾 -->
      <span
        v-if="call.state === 'running'"
        class="mt-1 inline-block h-3.5 w-1.5 animate-pulse bg-accent align-middle motion-reduce:animate-none"
        aria-hidden="true"
      />

      <p v-if="call.error" class="mt-2 border-t border-line pt-2 text-danger">{{ call.error }}</p>

      <p v-if="call.truncated" class="mt-2 text-[11px] text-ink-500">
        输出过长已截断，完整内容见日志
      </p>

      <!-- 用户上滚离开实时尾部时给出明确的恢复入口 -->
      <button
        v-if="call.state === 'running' && !follow"
        type="button"
        class="mt-2 rounded-lg border border-line px-2 py-0.5 text-[11px] text-ink-300 transition-colors hover:border-line-strong hover:bg-white/[0.04]"
        @click="
          () => {
            follow = true
            const el = outputEl
            if (el) el.scrollTop = el.scrollHeight
          }
        "
      >
        回到最新输出
      </button>
    </div>
  </div>
</template>
