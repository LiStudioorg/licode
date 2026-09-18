<script setup lang="ts">
import { Loader2 } from 'lucide-vue-next'
import { Empty, Message } from 'fuxsto-design'
const licode = useLicode()
const { state } = licode
const listRef = ref<HTMLElement | null>(null)
const pinned = ref(true)

function onScroll() {
  const el = listRef.value
  if (!el) return
  pinned.value = el.scrollTop + el.clientHeight >= el.scrollHeight - 80
}

function scrollToBottom() {
  nextTick(() => {
    const el = listRef.value
    if (el && pinned.value) el.scrollTop = el.scrollHeight
  })
}

watch(() => [state.messages, state.statusText], scrollToBottom, { deep: true })

function onRootClick(e: MouseEvent) {
  const t = (e.target as HTMLElement).closest('.md-copy') as HTMLElement | null
  if (t?.dataset.code) {
    navigator.clipboard
      .writeText(decodeURIComponent(t.dataset.code))
      .then(() => Message.success('代码已复制'))
      .catch(() => Message.error('复制失败'))
  }
}
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <div ref="listRef" class="min-h-0 flex-1 overflow-y-auto" @scroll="onScroll" @click="onRootClick">
      <div class="mx-auto w-full max-w-3xl px-4 py-6">
        <div v-if="!state.messages.length" class="flex flex-col items-center pt-24">
          <h1 class="text-xl font-semibold tracking-tight">licode</h1>
          <p class="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
            本地 AI 编程助手 · 纯 Go · 子代理编排 · 工具管理
          </p>
        </div>

        <MessageItem v-for="m in state.messages" :key="m.id" :msg="m" />

        <div v-if="state.busy && state.statusText" class="mt-2 flex items-center gap-2 text-xs text-zinc-500 dark:text-zinc-400">
          <Loader2 :size="13" class="animate-spin" />
          {{ state.statusText }}
        </div>
      </div>
    </div>

    <AskBar v-if="state.ask" />
    <ChatInput />
  </div>
</template>
