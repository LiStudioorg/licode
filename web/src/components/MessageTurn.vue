<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Turn } from '@/types/chat'
import { renderMarkdown } from '@/utils/markdown'
import ToolCard from '@/components/ToolCard.vue'
import AppIcon from '@/components/ui/AppIcon.vue'

/**
 * 一条回合：用户输入（气泡）或助手回复（正文 + 内联工具卡）。
 *
 * Markdown 缓存：流式期间每个 token 都会改最后一个文本块，若每次都整篇
 * 重渲染，长回复会退化成 O(n²) 并把主线程打满。这里按块缓存 ——
 * 未变化的块直接复用上次 HTML，每帧只重渲染尾部活跃块。
 */
const props = defineProps<{ turn: Turn }>()

const htmlCache = new Map<number, { src: string; html: string }>()

/** 只有最后一个文本块可能在流式增长，前面的块永不变化 */
const lastTextId = computed(() => {
  for (let i = props.turn.blocks.length - 1; i >= 0; i -= 1) {
    const b = props.turn.blocks[i]
    if (b.kind === 'text') return b.id
  }
  return -1
})

const rendered = computed(() =>
  props.turn.blocks.map((b) => {
    if (b.kind === 'tool') return { kind: 'tool' as const, key: `b${b.id}`, block: b }
    const cached = htmlCache.get(b.id)
    let html: string
    if (cached && cached.src === b.text) {
      html = cached.html
    } else {
      // 非尾部块永远只算一次；尾部块每帧一次
      html = renderMarkdown(b.text)
      if (b.id !== lastTextId.value) htmlCache.set(b.id, { src: b.text, html })
    }
    return { kind: 'text' as const, key: `t${b.id}`, html }
  }),
)

const hasText = computed(() => props.turn.blocks.some((b) => b.kind === 'text'))

/* 复制本回合正文：代码块里的复制按钮由 markdown 的委托监听处理 */
const copied = ref(false)
const plainText = computed(() =>
  props.turn.blocks
    .filter((b) => b.kind === 'text')
    .map((b) => (b.kind === 'text' ? b.text : ''))
    .join('\n\n'),
)

function copyText() {
  void navigator.clipboard
    ?.writeText(plainText.value)
    .then(() => {
      copied.value = true
      setTimeout(() => (copied.value = false), 1400)
    })
    .catch(() => undefined)
}

/** 复制/折叠等交互由 markdown 内部的按钮触发，用事件委托统一处理 */
function onClick(e: MouseEvent) {
  const target = e.target as HTMLElement | null
  if (!target || !target.classList.contains('md-copy')) return
  const code = decodeURIComponent(target.dataset.code ?? '')
  void navigator.clipboard
    ?.writeText(code)
    .then(() => {
      target.textContent = '已复制'
      setTimeout(() => (target.textContent = '复制'), 1200)
    })
    .catch(() => {
      target.textContent = '失败'
      setTimeout(() => (target.textContent = '复制'), 1200)
    })
}
</script>

<template>
  <!-- 用户：右对齐气泡 -->
  <article v-if="turn.role === 'user'" class="flex flex-col items-end gap-1">
    <div class="flex max-w-[min(100%,56ch)] flex-col items-end gap-1.5">
      <div
        v-if="hasText"
        class="rounded-[12px] rounded-br-[4px] px-3 py-2 whitespace-pre-wrap break-words text-ink-100"
        style="background: var(--color-bubble)"
      >
        <template v-for="b in turn.blocks" :key="b.id">
          <p v-if="b.kind === 'text'" class="whitespace-pre-wrap">{{ b.text }}</p>
        </template>
      </div>
      <!-- 附件：图片出缩略图，其余出文件名徽标 -->
      <div v-if="turn.attachments?.length" class="flex flex-wrap justify-end gap-1.5">
        <template v-for="(a, i) in turn.attachments" :key="i">
          <img
            v-if="a.type === 'image' && a.url"
            :src="a.url"
            :alt="a.filename || '图片附件'"
            class="max-h-28 max-w-[180px] rounded-lg border border-line object-cover"
            loading="lazy"
          />
          <span
            v-else
            class="inline-flex max-w-[180px] items-center gap-1 truncate rounded-lg border border-line px-1.5 py-0.5 text-[11px] text-ink-300"
          >
            <AppIcon name="file" class="size-3 shrink-0" />
            <span class="truncate">{{ a.filename || a.mime_type }}</span>
          </span>
        </template>
      </div>
    </div>
  </article>

  <!-- 助手：左对齐正文，工具卡内联在文本流里 -->
  <article v-else class="group/msg flex flex-col gap-2">
    <template v-for="p in rendered" :key="p.key">
      <div
        v-if="p.kind === 'text'"
        class="md max-w-[76ch] text-ink-200"
        @click="onClick"
        v-html="p.html"
      />
      <ToolCard v-else :call="p.block" class="max-w-[76ch]" />
    </template>

    <div class="flex items-center gap-1 opacity-0 transition-opacity group-hover/msg:opacity-100 focus-within:opacity-100">
      <button
        v-if="plainText"
        type="button"
        class="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[11px] text-ink-500 transition-colors hover:bg-white/[0.05] hover:text-ink-200"
        @click="copyText"
      >
        <AppIcon :name="copied ? 'check' : 'copy'" class="size-3" />
        {{ copied ? '已复制' : '复制' }}
      </button>
    </div>
  </article>
</template>
