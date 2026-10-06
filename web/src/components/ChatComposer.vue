<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useSessionStore } from '@/stores/session'
import { useToast } from '@/composables/useToast'
import AppIcon from '@/components/ui/AppIcon.vue'

/**
 * 输入区。
 *
 * 约束：
 *   - Enter 发送 / Shift+Enter 换行，输入法组词期间（isComposing）绝不触发；
 *   - 附件走 base64 内嵌，WS 单帧上限 8 MiB，因此这里按编码后大小拦截，
 *     而不是等后端断连（超限 gorilla 直接关连接，用户看到的是「莫名掉线」）；
 *   - 运行中允许继续输入但不允许发送（后端同会话串行，busy 时回 error）；
 *   - 草稿按会话存 localStorage，切会话/刷新不丢字。
 */
const session = useSessionStore()
const toast = useToast()

const draft = ref('')
const textarea = ref<HTMLTextAreaElement | null>(null)

interface Pending {
  name: string
  mime: string
  size: number
  data: string
  kind: 'image' | 'file'
  url?: string
}
const pending = ref<Pending[]>([])

const DRAFT_KEY = 'licod...aft'

/** WS 单帧 8 MiB；base64 膨胀 4/3，留 15% 余量给 JSON 信封与其它字段 */
const MAX_ENCODED = 6 * 1024 * 1024

// 草稿按会话隔离：离开时存、进入时取回
watch(
  () => session.activeId,
  (id, prev) => {
    const store = readDrafts()
    if (prev) store[prev] = draft.value
    writeDrafts(store)
    draft.value = store[id] ?? ''
    void nextTick(autoGrow)
  },
)

watch(draft, (v) => {
  const store = readDrafts()
  store[session.activeId] = v
  writeDrafts(store)
})

function readDrafts(): Record<string, string> {
  try {
    return JSON.parse(localStorage.getItem(DRAFT_KEY) ?? '{}') as Record<string, string>
  } catch {
    return {}
  }
}

function writeDrafts(v: Record<string, string>) {
  try {
    localStorage.setItem(DRAFT_KEY, JSON.stringify(v))
  } catch {
    /* 配额满：草稿丢失可接受 */
  }
}

function autoGrow() {
  const el = textarea.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 168)}px`
}

watch(draft, autoGrow)

function onKeydown(e: KeyboardEvent) {
  if (e.key !== 'Enter' || e.shiftKey || e.isComposing) return
  e.preventDefault()
  submit()
}

/** 全局快捷键：Esc 聚焦输入框（编辑器习惯，但输入框内不劫持 Esc） */
function onGlobalKey(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  const t = e.target as HTMLElement | null
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA')) return
  textarea.value?.focus()
}

window.addEventListener('keydown', onGlobalKey)
onBeforeUnmount(() => window.removeEventListener('keydown', onGlobalKey))

async function onFiles(files: FileList | File[] | null) {
  if (!files?.length) return
  for (const f of Array.from(files)) {
    const isImage = f.type.startsWith('image/')
    const data = await readAsBase64(f)
    // base64 长度即编码后字节数（无需再乘 4/3，readAsDataURL 已编码）
    if (data.length > MAX_ENCODED) {
      toast.error(`「${f.name}」过大，附件编码后需小于 6MB`)
      continue
    }
    pending.value.push({
      name: f.name,
      mime: f.type || 'application/octet-stream',
      size: f.size,
      data,
      kind: isImage ? 'image' : 'file',
      url: isImage ? `data:${f.type};base64,${data}` : undefined,
    })
  }
}

function readAsBase64(f: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader()
    r.onload = () => {
      const s = String(r.result ?? '')
      // 去掉 dataURL 头，只留 base64 正文
      resolve(s.includes(',') ? s.slice(s.indexOf(',') + 1) : s)
    }
    r.onerror = () => reject(r.error)
    r.readAsDataURL(f)
  })
}

function dropFile(i: number) {
  pending.value.splice(i, 1)
}

const canSend = computed(
  () => (draft.value.trim().length > 0 || pending.value.length > 0) && !session.busy,
)

function submit() {
  const text = draft.value.trim()
  if (!text && !pending.value.length) return
  if (session.busy) {
    toast.info('上一条消息仍在处理中')
    return
  }
  session.sendUserMessage(
    text,
    pending.value.map((p) => ({
      type: p.kind,
      mime_type: p.mime,
      data: p.data,
      filename: p.name,
      url: p.url,
    })),
  )
  draft.value = ''
  pending.value = []
}

function onPaste(e: ClipboardEvent) {
  const items = e.clipboardData?.items
  if (!items) return
  const imgs: File[] = []
  for (const it of items) {
    if (it.kind === 'file' && it.type.startsWith('image/')) {
      const f = it.getAsFile()
      if (f) imgs.push(new File([f], f.name || `pasted-${Date.now()}.png`, { type: f.type }))
    }
  }
  if (imgs.length) {
    e.preventDefault()
    void onFiles(imgs)
  }
}

/**
 * 供父视图填入建议文本（空态的建议芯片）。
 * 直接改 draft 而不是操作 DOM：外部写 textarea.value 不会触发 v-model，
 * 反而要和框架的输入状态打架。
 */
function setDraft(text: string) {
  draft.value = text
  void nextTick(() => {
    textarea.value?.focus()
    autoGrow()
  })
}

defineExpose({ setDraft })
</script>

<template>
  <div class="shell__composer shrink-0 border-t border-line px-3 py-2.5 pb-[max(0.625rem,env(safe-area-inset-bottom))] sm:px-6">
    <div class="mx-auto w-full max-w-[76ch]">
      <!-- 附件条 -->
      <div v-if="pending.length" class="mb-2 flex flex-wrap gap-2">
        <div
          v-for="(p, i) in pending"
          :key="i"
          class="group/att relative flex items-center gap-2 rounded-lg border border-line bg-white/[0.03] py-1 pl-1 pr-6"
        >
          <img v-if="p.url" :src="p.url" :alt="p.name" class="size-8 rounded object-cover" />
          <AppIcon v-else name="file" class="mx-1.5 size-4 text-ink-400" />
          <span class="max-w-[140px] truncate text-ink-200">{{ p.name }}</span>
          <button
            type="button"
            class="absolute right-1 top-1/2 -translate-y-1/2 rounded p-0.5 text-ink-500 transition-colors hover:text-danger"
            :aria-label="`移除 ${p.name}`"
            @click="dropFile(i)"
          >
            <AppIcon name="close" class="size-3" />
          </button>
        </div>
      </div>

      <div
        class="flex items-end gap-2 rounded-[12px] border border-line bg-white/[0.02] px-2 py-1.5 transition-colors focus-within:border-accent-line"
      >
        <label class="sr-only" for="composer">输入消息</label>
        <textarea
          id="composer"
          ref="textarea"
          v-model="draft"
          rows="1"
          :placeholder="session.busy ? '生成中…可继续输入，稍后发送' : '发消息，Enter 发送'"
          class="max-h-[168px] min-h-[24px] flex-1 resize-none bg-transparent py-0.5 leading-6 text-ink-100 outline-none placeholder:text-ink-500"
          @keydown="onKeydown"
          @paste="onPaste"
        />

        <label
          class="grid size-7 shrink-0 cursor-pointer place-items-center rounded-lg text-ink-400 transition-colors hover:bg-white/[0.05] hover:text-ink-200"
          title="添加附件"
        >
          <AppIcon name="plus" class="size-4" />
          <input type="file" multiple class="sr-only" @change="onFiles(($event.target as HTMLInputElement).files)" />
        </label>

        <button
          v-if="session.busy"
          type="button"
          class="flex h-7 shrink-0 items-center gap-1 rounded-lg border border-line px-2 text-ink-300 transition-colors hover:border-line-strong hover:bg-white/[0.05]"
          @click="session.interrupt()"
        >
          <AppIcon name="stop" class="size-3" />
          停止
        </button>
        <button
          v-else
          type="button"
          class="grid size-7 shrink-0 place-items-center rounded-lg bg-accent text-[var(--color-accent-ink)] transition-opacity hover:opacity-90 active:scale-[0.96] disabled:opacity-35"
          :disabled="!canSend"
          aria-label="发送"
          @click="submit"
        >
          <AppIcon name="send" class="size-3.5" :stroke-width="1.6" />
        </button>
      </div>

      <p class="mt-1 hidden px-1 text-[11px] text-ink-600 sm:block">
        Enter 发送 · Shift+Enter 换行 · 支持粘贴图片 · 指令 /clear /plan /build
      </p>
    </div>
  </div>
</template>
