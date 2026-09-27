<script setup lang="ts">
/**
 * 中栏三件套：空态 / 加载骨架 / 错误态。
 *
 * 统一放一个组件里，因为三者共用同一块消息流区域且互斥，
 * 拆成三个文件只会让调用方维护一份同样的 v-if 链。
 * 每态的视觉都贴合最终布局（骨架用与真实消息同形的块，不用转圈）。
 */
defineProps<{
  /** 三态之一；ready 表示正常内容，由调用方渲染 */
  state: 'empty' | 'loading' | 'error' | 'ready'
  /** 错误态的具体原因，直接展示，不吞掉信息 */
  errorMessage?: string
  /** 空态主文案 */
  emptyTitle?: string
  /** 空态副文案 */
  emptyHint?: string
}>()

const emit = defineEmits<{
  (e: 'retry'): void
  (e: 'create'): void
}>()
</script>

<template>
  <!-- 加载骨架：形状贴合「消息 + 工具卡片」，不用通用转圈 -->
  <div v-if="state === 'loading'" class="mx-auto w-full max-w-[76ch]" aria-busy="true" aria-live="polite">
    <span class="sr-only">正在加载会话</span>
    <div class="flex flex-col gap-4">
      <div class="flex flex-col items-end gap-2">
        <div class="h-3 w-16 animate-pulse rounded-lg bg-white/[0.05] motion-reduce:animate-none" />
        <div class="h-10 w-[52%] animate-pulse rounded-lg bg-white/[0.05] motion-reduce:animate-none" />
      </div>
      <div class="flex flex-col items-start gap-2">
        <div class="h-3 w-20 animate-pulse rounded-lg bg-white/[0.05] motion-reduce:animate-none" />
        <div class="h-4 w-[76%] animate-pulse rounded-lg bg-white/[0.05] motion-reduce:animate-none" />
        <div class="h-4 w-[64%] animate-pulse rounded-lg bg-white/[0.05] motion-reduce:animate-none" />
        <div class="h-24 w-full animate-pulse rounded-lg border border-line bg-white/[0.03] motion-reduce:animate-none" />
      </div>
    </div>
  </div>

  <!-- 错误态：就地给出原因与唯一主操作 -->
  <div v-else-if="state === 'error'" class="grid h-full place-items-center">
    <div class="max-w-[42ch] text-center">
      <div
        class="mx-auto mb-3 grid size-9 place-items-center rounded-lg border border-danger/40 text-danger"
        aria-hidden="true"
      >
        <svg viewBox="0 0 20 20" class="size-4.5" fill="none" stroke="currentColor" stroke-width="1.25">
          <path d="M10 6.5v4M10 13.2v.1" stroke-linecap="round" />
          <circle cx="10" cy="10" r="7.25" />
        </svg>
      </div>
      <p class="text-ink-100">会话加载失败</p>
      <p class="mt-1 text-ink-400">{{ errorMessage ?? '连接已中断，请重试' }}</p>
      <button
        type="button"
        class="mt-3 rounded-lg border border-line px-3 py-1 text-ink-200 transition-colors hover:border-line-strong hover:bg-white/[0.04] active:scale-[0.98]"
        @click="emit('retry')"
      >
        重试
      </button>
    </div>
  </div>

  <!-- 空态：说清下一步该做什么，而不是只报「没有数据」 -->
  <div v-else-if="state === 'empty'" class="grid h-full place-items-center">
    <div class="max-w-[38ch] text-center">
      <div
        class="mx-auto mb-3 grid size-9 place-items-center rounded-lg border border-line text-ink-500"
        aria-hidden="true"
      >
        <svg viewBox="0 0 20 20" class="size-4.5" fill="none" stroke="currentColor" stroke-width="1.25">
          <path d="M3.5 6.5A2.5 2.5 0 016 4h8a2.5 2.5 0 012.5 2.5v5A2.5 2.5 0 0114 14H9l-4 2.5V14H6a2.5 2.5 0 01-2.5-2.5v-5z" />
        </svg>
      </div>
      <p class="text-ink-200">{{ emptyTitle ?? '这个会话还没有消息' }}</p>
      <p class="mt-1 text-ink-500">{{ emptyHint ?? '在下方输入内容，或从左侧切换到其它会话' }}</p>
      <button
        type="button"
        class="mt-3 rounded-lg border border-line px-3 py-1 text-ink-200 transition-colors hover:border-line-strong hover:bg-white/[0.04] active:scale-[0.98]"
        @click="emit('create')"
      >
        新建会话
      </button>
    </div>
  </div>
</template>
