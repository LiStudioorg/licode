<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useSessionStore } from '@/stores/session'
import type { SessionInfo } from '@/api/protocol'

/**
 * 左栏会话列表：分组展示 + 拖拽重排 + 右键菜单。
 *
 * 拖拽约束：置顶会话不参与拖拽（服务端恒定把它们排在前面），
 * 因此这里只在同组内允许拖放，跨组拖放会被忽略。
 *
 * 拖拽用原生 HTML5 DnD 而非第三方库：需求只有「同组上下重排」，
 * 引入拖拽库的收益不抵它的体积与样式对抗成本。
 */
const session = useSessionStore()

const draggingId = ref<string | null>(null)
const dropTarget = ref<{ id: string; position: 'before' | 'after' } | null>(null)

interface MenuState {
  open: boolean
  x: number
  y: number
  target: SessionInfo | null
  renaming: boolean
  draft: string
}

const menu = ref<MenuState>({
  open: false,
  x: 0,
  y: 0,
  target: null,
  renaming: false,
  draft: '',
})

const renameInput = ref<HTMLInputElement | null>(null)

function openMenu(e: MouseEvent, target: SessionInfo) {
  e.preventDefault()
  menu.value = {
    open: true,
    // 用视口坐标，配合 fixed 定位，避免被列容器的 overflow 裁掉
    x: e.clientX,
    y: e.clientY,
    target,
    renaming: false,
    draft: target.title,
  }
}

function closeMenu() {
  if (menu.value.open) menu.value = { ...menu.value, open: false, renaming: false }
}

async function beginRename() {
  if (!menu.value.target) return
  menu.value.renaming = true
  await Promise.resolve()
  renameInput.value?.focus()
  renameInput.value?.select()
}

function commitRename() {
  const target = menu.value.target
  if (target) {
    const title = menu.value.draft.trim()
    if (title && title !== target.title) session.rename(target.id, title)
  }
  closeMenu()
}

function copyId() {
  const target = menu.value.target
  if (target) void navigator.clipboard?.writeText(target.id)
  closeMenu()
}

function togglePin() {
  const target = menu.value.target
  if (target) session.setPinned(target.id, !target.pinned)
  closeMenu()
}

function removeSession() {
  const target = menu.value.target
  if (target) session.remove(target.id)
  closeMenu()
}

/* ---------------- 拖拽 ---------------- */

function onDragStart(e: DragEvent, item: SessionInfo) {
  if (item.pinned) {
    e.preventDefault()
    return
  }
  draggingId.value = item.id
  e.dataTransfer?.setData('text/plain', item.id)
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onDragOver(e: DragEvent, item: SessionInfo) {
  if (!draggingId.value || draggingId.value === item.id) return
  const dragged = session.sessions.find((s) => s.id === draggingId.value)
  // 跨组（置顶与否）不允许落点，避免产生一个被服务端立刻纠正的假顺序
  if (!dragged || !!dragged.pinned !== !!item.pinned) return
  e.preventDefault()
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const position = e.clientY < rect.top + rect.height / 2 ? 'before' : 'after'
  dropTarget.value = { id: item.id, position }
}

function onDrop(e: DragEvent, item: SessionInfo) {
  e.preventDefault()
  const draggedId = draggingId.value
  const position = dropTarget.value?.position ?? 'before'
  if (draggedId && draggedId !== item.id) session.reorder(draggedId, item.id, position)
  onDragEnd()
}

function onDragEnd() {
  draggingId.value = null
  dropTarget.value = null
}

/** 键盘可达的排序：Alt+上下方向键等价于拖拽前后移动一格 */
function moveByKeyboard(item: SessionInfo, delta: -1 | 1) {
  const list = session.sessions.filter((s) => !!s.pinned === !!item.pinned)
  const idx = list.findIndex((s) => s.id === item.id)
  const next = list[idx + delta]
  if (!next) return
  session.reorder(item.id, next.id, delta === -1 ? 'before' : 'after')
}

function onEsc(e: KeyboardEvent) {
  if (e.key === 'Escape') closeMenu()
}

onMounted(() => {
  window.addEventListener('click', closeMenu)
  window.addEventListener('keydown', onEsc)
  window.addEventListener('resize', closeMenu)
})
onBeforeUnmount(() => {
  window.removeEventListener('click', closeMenu)
  window.removeEventListener('keydown', onEsc)
  window.removeEventListener('resize', closeMenu)
})

/** 菜单靠近视口边缘时向内收，避免被裁切 */
const menuStyle = computed(() => {
  const width = 168
  const height = menu.value.renaming ? 96 : 168
  const x = Math.min(menu.value.x, window.innerWidth - width - 8)
  const y = Math.min(menu.value.y, window.innerHeight - height - 8)
  return { left: `${Math.max(8, x)}px`, top: `${Math.max(8, y)}px`, width: `${width}px` }
})
</script>

<template>
  <div class="border-b border-line p-2">
    <label class="sr-only" for="session-search">搜索会话</label>
    <div class="relative">
      <svg
        viewBox="0 0 16 16"
        class="pointer-events-none absolute left-2 top-1/2 size-3.5 -translate-y-1/2 text-ink-500"
        fill="none"
        stroke="currentColor"
        stroke-width="1.25"
        aria-hidden="true"
      >
        <circle cx="7" cy="7" r="4.25" />
        <path d="M10.2 10.2L13.5 13.5" stroke-linecap="round" />
      </svg>
      <input
        id="session-search"
        v-model="session.query"
        type="search"
        class="h-8 w-full rounded-lg border border-line bg-white/[0.02] pl-7 pr-2 text-ink-100 transition-colors placeholder:text-ink-500 hover:border-line-strong focus:border-accent-line focus:outline-none"
        placeholder="搜索"
      />
    </div>
  </div>

  <nav class="min-h-0 flex-1 overflow-y-auto px-2 py-2" aria-label="会话列表">
    <div v-for="group in session.grouped" :key="group.group" class="mb-3 last:mb-0">
      <p class="px-2 pb-1 text-[11px] text-ink-500">{{ group.group }}</p>
      <ul>
        <li v-for="item in group.items" :key="item.id">
          <button
            type="button"
            :draggable="!item.pinned"
            class="group relative flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left transition-colors"
            :class="[
              item.id === session.activeId
                ? 'bg-white/[0.05] text-ink-100'
                : 'text-ink-300 hover:bg-white/[0.03] hover:text-ink-100',
              draggingId === item.id ? 'opacity-40' : '',
              dropTarget?.id === item.id
                ? dropTarget.position === 'before'
                  ? 'shadow-[inset_0_1px_0_0_var(--color-accent)]'
                  : 'shadow-[inset_0_-1px_0_0_var(--color-accent)]'
                : '',
            ]"
            :aria-current="item.id === session.activeId ? 'true' : undefined"
            @click="session.select(item.id)"
            @contextmenu="openMenu($event, item)"
            @dragstart="onDragStart($event, item)"
            @dragover="onDragOver($event, item)"
            @drop="onDrop($event, item)"
            @dragend="onDragEnd"
            @keydown.alt.up.prevent="moveByKeyboard(item, -1)"
            @keydown.alt.down.prevent="moveByKeyboard(item, 1)"
          >
            <!-- 置顶标识：真实语义状态，非装饰 -->
            <svg
              v-if="item.pinned"
              viewBox="0 0 16 16"
              class="size-3 shrink-0 text-accent"
              fill="none"
              stroke="currentColor"
              stroke-width="1.4"
              aria-label="已置顶"
            >
              <path d="M6 2h4l-.5 4 2 2.5H4.5l2-2.5L6 2zM8 8.5V14" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
            <span class="min-w-0 flex-1 truncate">{{ item.title }}</span>
            <span v-if="item.count" class="shrink-0 font-mono text-[10px] text-ink-500">{{
              item.count
            }}</span>
          </button>
        </li>
      </ul>
    </div>

    <p v-if="session.grouped.length === 0" class="px-2 py-6 text-center text-ink-500">
      {{ session.connection === 'open' ? '还没有会话' : '正在连接…' }}
    </p>
  </nav>

  <div class="border-t border-line p-2">
    <button
      type="button"
      class="flex w-full items-center gap-2 rounded-lg border border-line bg-white/[0.02] px-2 py-1.5 text-ink-200 transition-colors hover:border-line-strong hover:bg-white/[0.04] active:scale-[0.99]"
      @click="session.create()"
    >
      <svg viewBox="0 0 16 16" class="size-3.5" fill="none" stroke="currentColor" stroke-width="1.25" aria-hidden="true">
        <path d="M8 3.5v9M3.5 8h9" stroke-linecap="round" />
      </svg>
      新建会话
    </button>
  </div>

  <!-- 右键菜单 -->
  <div
    v-if="menu.open"
    class="fixed z-50 rounded-lg border border-line-strong bg-elevated p-1"
    :style="menuStyle"
    role="menu"
    @click.stop
  >
    <template v-if="!menu.renaming">
      <button
        type="button"
        role="menuitem"
        class="flex w-full items-center rounded-lg px-2 py-1 text-left text-ink-200 transition-colors hover:bg-white/[0.06]"
        @click="beginRename"
      >
        重命名
      </button>
      <button
        type="button"
        role="menuitem"
        class="flex w-full items-center rounded-lg px-2 py-1 text-left text-ink-200 transition-colors hover:bg-white/[0.06]"
        @click="togglePin"
      >
        {{ menu.target?.pinned ? '取消置顶' : '置顶' }}
      </button>
      <button
        type="button"
        role="menuitem"
        class="flex w-full items-center rounded-lg px-2 py-1 text-left text-ink-200 transition-colors hover:bg-white/[0.06]"
        @click="copyId"
      >
        复制会话 ID
      </button>
      <div class="my-1 h-px bg-line" />
      <button
        type="button"
        role="menuitem"
        class="flex w-full items-center rounded-lg px-2 py-1 text-left text-danger transition-colors hover:bg-white/[0.06]"
        @click="removeSession"
      >
        删除会话
      </button>
    </template>

    <div v-else class="p-1">
      <label class="sr-only" for="rename-input">会话名称</label>
      <input
        id="rename-input"
        ref="renameInput"
        v-model="menu.draft"
        class="h-7 w-full rounded-lg border border-line bg-white/[0.02] px-2 text-ink-100 focus:border-accent-line focus:outline-none"
        @keydown.enter.prevent="commitRename"
        @keydown.esc.prevent="closeMenu"
      />
      <p class="mt-1 text-[11px] text-ink-500">Enter 保存 / Esc 取消</p>
    </div>
  </div>
</template>
