<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useSessionStore } from '@/stores/session'
import { useSettingsStore } from '@/stores/settings'
import { renderMarkdown } from '@/utils/markdown'
import MessageTurn from '@/components/MessageTurn.vue'
import ChatComposer from '@/components/ChatComposer.vue'
import ApprovalCard from '@/components/ApprovalCard.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBar from '@/components/StatusBar.vue'

/**
 * 对话主视图：消息流 + 审批 + 输入区。
 *
 * 状态四态（骨架/错误/空/就绪）由 store 的 viewState 统一裁决，
 * 视图只负责呈现，不各自拼 v-if 条件。
 *
 * 滚动策略与工具卡同款：贴底时自动跟随流式输出，用户上滚即停止跟随，
 * 离开底部时出现「回到最新」的浮标。
 */
const session = useSessionStore()
const composer = ref<InstanceType<typeof ChatComposer> | null>(null)
const settings = useSettingsStore()

const scroller = ref<HTMLElement | null>(null)
const atBottom = ref(true)

function onScroll() {
  const el = scroller.value
  if (!el) return
  atBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 48
}

function toBottom(smooth = false) {
  const el = scroller.value
  if (!el) return
  el.scrollTo({ top: el.scrollHeight, behavior: smooth ? 'smooth' : 'auto' })
  atBottom.value = true
}

/**
 * 流式跟随：内容变化后滚到底。
 * flush 是同步的（store 每帧一次），这里放 nextTick 等 DOM 更新完成。
 */
watch(
  () => [session.turns.length, session.turns[session.turns.length - 1]?.blocks.length ?? 0],
  async () => {
    if (!atBottom.value) return
    await nextTick()
    toBottom()
  },
)

// 切换会话：直接落到底部（最新消息在最后），不做平滑动画
watch(
  () => session.activeId,
  async () => {
    await nextTick()
    toBottom()
  },
)

const reasoningHtml = computed(() => (session.reasoning ? renderMarkdown(session.reasoning) : ''))
const showReasoning = ref(false)

function onCopyClick(e: MouseEvent) {
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

function onKey(e: KeyboardEvent) {
  // 斜杠快捷：/ 开头直接落到输入框（编辑器习惯）；仅在输入框外且无修饰键时生效
  if (e.key !== '/' || e.metaKey || e.ctrlKey || e.altKey) return
  const t = e.target as HTMLElement | null
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA')) return
  e.preventDefault()
  composer.value?.setDraft('')
}

/** 空态建议芯片：填入输入框并聚焦，但不自动发送 —— 发送永远是用户的动作 */
function useSuggestion(text: string) {
  composer.value?.setDraft(text)
}

onMounted(() => {
  window.addEventListener('keydown', onKey)
  void settings.loadWorkspace().catch(() => undefined)
})
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))

/**
 * 软键盘弹出时视口高度骤减，输入框可能被顶出可视区。
 * 用 visualViewport 监听高度变化，把消息区重新贴底。
 * 不用 resize 事件：iOS 上软键盘不触发 window.resize。
 */
function onViewportResize() {
  if (atBottom.value) toBottom(false)
}

onMounted(() => {
  window.visualViewport?.addEventListener('resize', onViewportResize)
})
onBeforeUnmount(() => {
  window.visualViewport?.removeEventListener('resize', onViewportResize)
})

const rootName = computed(() => {
  const r = settings.workspace.root
  if (!r) return ''
  const parts = r.split('/').filter(Boolean)
  return parts[parts.length - 1] ?? r
})

/**
 * 输入框可见性：空态与就绪态都要给。
 *
 * 空态其实最需要输入框 —— 用户就是来发第一句话的。此前 ChatComposer
 * 只放在 ready 分支里，导致新会话根本看不到输入框，而空态文案却写着
 * 「在下方输入内容即可开始」，文案与结构自相矛盾。
 *
 * 加载态与错误态不给：前者会话尚未定位，后者连接不可用，
 * 给了也发不出去，只会把失败推迟到点击发送那一刻。
 */
const hasComposer = computed(
  () => session.viewState === 'ready' || session.viewState === 'empty',
)
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
  <!-- 骨架：形状贴合真实消息，不用通用转圈 -->
  <div v-if="session.viewState === 'loading'" class="min-h-0 flex-1 overflow-y-auto px-4 py-5 sm:px-8" aria-busy="true" aria-live="polite">
    <span class="sr-only">正在加载会话</span>
    <div class="mx-auto flex w-full max-w-[76ch] flex-col gap-4">
      <div class="flex flex-col items-end gap-2">
        <div class="h-9 w-[52%] animate-pulse rounded-[12px] bg-white/[0.05] motion-reduce:animate-none" />
      </div>
      <div class="flex flex-col items-start gap-2">
        <div class="h-4 w-[76%] animate-pulse rounded bg-white/[0.05] motion-reduce:animate-none" />
        <div class="h-4 w-[64%] animate-pulse rounded bg-white/[0.05] motion-reduce:animate-none" />
        <div class="h-24 w-full animate-pulse rounded-[10px] border border-line bg-white/[0.03] motion-reduce:animate-none" />
      </div>
    </div>
  </div>

  <!-- 错误：就地给原因与唯一动作 -->
  <div v-else-if="session.viewState === 'error'" class="grid min-h-0 flex-1 place-items-center px-6">
    <div class="max-w-[42ch] text-center">
      <div class="mx-auto mb-3 grid size-9 place-items-center rounded-[10px] border border-danger/40 text-danger">
        <AppIcon name="error" class="size-4" />
      </div>
      <p class="text-ink-100">无法连接后端</p>
      <p class="mt-1 text-ink-400">{{ session.lastError ?? '连接已断开，正在自动重连' }}</p>
      <button
        type="button"
        class="mt-3 rounded-lg border border-line px-3 py-1 text-ink-200 transition-colors hover:border-line-strong hover:bg-white/[0.05] active:scale-[0.98]"
        @click="session.connect()"
      >
        立即重试
      </button>
    </div>
  </div>

  <!-- 空态：说清下一步，而不是只报「没有数据」 -->
  <div v-else-if="session.viewState === 'empty'" class="grid min-h-0 flex-1 place-items-center px-6">
    <div class="max-w-[40ch] text-center">
      <div class="mx-auto mb-3 grid size-9 place-items-center rounded-[10px] border border-line text-ink-500">
        <AppIcon name="chat" class="size-4" />
      </div>
      <p class="text-ink-200">{{ session.activeId ? '这个会话还没有消息' : '开始一段新对话' }}</p>
      <p class="mt-1 text-ink-500">
        {{ session.activeId ? '在下方输入内容即可开始' : '左侧新建会话，或直接在这里输入' }}
      </p>
      <div class="mt-3 flex flex-wrap justify-center gap-1.5">
        <button
          v-for="s in ['解释这段代码', '帮我写一个单元测试', '重构这个函数']"
          :key="s"
          type="button"
          class="rounded-lg border border-line px-2.5 py-1 text-ink-300 transition-colors hover:border-line-strong hover:bg-white/[0.04]"
          @click="useSuggestion(s)"
        >
          {{ s }}
        </button>
      </div>
    </div>
  </div>

  <!-- 就绪：消息流 + 审批 + 输入区 + 状态栏 -->
  <template v-else>
    <!-- 运行状态条：工作目录与当前步骤。标题在左侧栏与外壳上下文条里已有，不重复 -->
    <div
      v-if="rootName || (session.busy && session.statusText)"
      class="flex h-8 shrink-0 items-center gap-2 border-b border-line px-4"
    >
      <span
        v-if="rootName"
        class="flex min-w-0 items-center gap-1 font-mono text-[11px] text-ink-500"
        :title="`工作目录：${settings.workspace.root}`"
      >
        <AppIcon name="folder" class="size-3 shrink-0" />
        <span class="truncate">{{ rootName }}</span>
      </span>
      <span
        v-if="session.busy && session.statusText"
        class="ml-auto shrink-0 truncate font-mono text-[11px] text-ink-400"
        >{{ session.statusText }}</span
      >
    </div>

    <div class="relative min-h-0 flex-1 overflow-hidden">
      <div ref="scroller" class="h-full overflow-y-auto px-4 py-5 sm:px-8" @scroll.passive="onScroll">
        <div class="mx-auto flex w-full max-w-[76ch] flex-col gap-5">
          <!-- 思考过程：可折叠，展开才渲染 -->
          <details
            v-if="reasoningHtml"
            class="rounded-[10px] border border-line bg-white/[0.02]"
            :open="showReasoning"
            @toggle="showReasoning = ($event.target as HTMLDetailsElement).open"
          >
            <summary class="flex cursor-pointer items-center gap-2 px-2.5 py-1.5 text-ink-400 select-none">
              <AppIcon name="brain" class="size-3.5" />
              思考过程
              <span v-if="session.busy" class="size-1.5 animate-pulse rounded-full bg-accent motion-reduce:animate-none" />
            </summary>
            <div class="md border-t border-line px-2.5 py-2 text-[12px] text-ink-400" @click="onCopyClick" v-html="reasoningHtml" />
          </details>

          <MessageTurn v-for="t in session.turns" :key="t.id" :turn="t" />

          <!-- 流式光标：本轮仍在生成时贴在末尾 -->
          <span
            v-if="session.busy"
            class="inline-block h-3.5 w-1.5 animate-pulse self-start bg-accent motion-reduce:animate-none"
            aria-hidden="true"
          />

          <ApprovalCard
            v-if="session.ask"
            :ask="session.ask"
            @decide="(d) => session.decide(session.ask!.askId, d)"
          />
        </div>
      </div>

      <button
        v-if="!atBottom"
        type="button"
        class="glass absolute bottom-4 left-1/2 flex -translate-x-1/2 items-center gap-1.5 rounded-full border border-line-strong px-3 py-1.5 text-ink-200 shadow-lg transition-transform hover:scale-[1.03] active:scale-[0.98]"
        @click="toBottom(true)"
      >
        <AppIcon name="chevronDown" class="size-3.5" />
        回到最新
      </button>
    </div>

  </template>

  <!-- 输入区与状态栏在四态之外：空态也必须能发消息 -->
  <ChatComposer v-if="hasComposer" ref="composer" />
  <StatusBar />
  </div>
</template>
