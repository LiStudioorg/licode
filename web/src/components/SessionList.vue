<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, nextTick, ref } from 'vue'
import { useSessionStore } from '@/stores/session'
import AppIcon from '@/components/ui/AppIcon.vue'
import BaseConfirm from '@/components/ui/BaseConfirm.vue'

/**
 * 会话列表：搜索 + 置顶/普通/空会话分组 + 拖拽重排 + 右键菜单 + 运行指示。
 *
 * 拖拽约束：置顶项由服务端恒定排在最前，不参与拖拽区间；跨区落点被忽略，
 * 避免产生一个立刻被服务端纠正的假顺序。顺序走本地乐观更新 + session_reorder。
 *
 * 键盘可达：列表项可聚焦，Alt+↑/↓ 等价于拖拽上下移一格。
 */
const session = useSessionStore()

interface MenuState {
  open: boolean
  x: number
  y: number
  id: string
  title: string
  renaming: boolean
  draft: string
}

const menu = ref<MenuState>({ open: false, x: 0, y: 0, id: '', title: '', renaming: false, draft: '' })
const renameInput = ref<HTMLInputElement | null>(null)

function openMenu(e: MouseEvent, id: string, title: string) {
  e.preventDefault()
  e.stopPropagation()
  menu.value = { open: true, x: e.clientX, y: e.clientY, id, title, renaming: false, draft: title }
}

function closeMenu() {
  if (menu.value.open) menu.value = { ...menu.value, open: false, renaming: false }
}

function beginRename() {
  menu.value.renaming = true
  void nextTick(() => {
    renameInput.value?.focus()
    renameInput.value?.select()
  })
}

function commitRename() {
  const { id, title, draft } = menu.value
  const next = draft.trim()
  if (next && next !== title) session.rename(id, next)
  closeMenu()
}

const target = computed(() => session.sessions.find((s) => s.id === menu.value.id) ?? null)

function togglePin() {
  if (target.value) session.setPinned(target.value.id, !target.value.pinned)
  closeMenu()
}

function copyId() {
  void navigator.clipboard?.writeText(menu.value.id)
  closeMenu()
}

function doBranch() {
  if (menu.value.id) session.branch(menu.value.id)
  closeMenu()
}

function doExport() {
  if (target.value) session.exportSession(target.value.id, target.value.title)
  closeMenu()
}

const pendingDelete = ref(false)

function askDelete() {
  closeMenu()
  pendingDelete.value = true
}

function confirmDelete() {
  // 删除要带当前 id：菜单已关闭，target 计算属性会失效
  const id = menu.value.id
  pendingDelete.value = false
  if (id) session.remove(id)
}

/* ---------------- 拖拽 ---------------- */

const draggingId = ref<string | null>(null)
const dropTarget = ref<{ id: string; position: 'before' | 'after' } | null>(null)

function onDragStart(e: DragEvent, id: string, pinned?: boolean) {
  if (pinned) {
    e.preventDefault()
    return
  }
  draggingId.value = id
  e.dataTransfer?.setData('text/plain', id)
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onDragOver(e: DragEvent, id: string, pinned?: boolean) {
  const dragged = draggingId.value
  if (!dragged || dragged === id) return
  const draggedItem = session.sessions.find((s) => s.id === dragged)
  // 跨区（置顶/普通）不允许落点：服务端会立刻纠正，本地先拒
  if (!draggedItem || !!draggedItem.pinned !== !!pinned) return
  e.preventDefault()
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  dropTarget.value = { id, position: e.clientY < rect.top + rect.height / 2 ? 'before' : 'after' }
}

function onDrop(e: DragEvent, id: string) {
  e.preventDefault()
  const dragged = draggingId.value
  const position = dropTarget.value?.position ?? 'before'
  if (dragged && dragged !== id) session.reorder(dragged, id, position)
  onDragEnd()
}

function onDragEnd() {
  draggingId.value = null
  dropTarget.value = null
}

function moveByKeyboard(id: string, delta: -1 | 1) {
  const item = session.sessions.find((s) => s.id === id)
  if (!item) return
  const peers = session.sessions.filter((s) => !!s.pinned === !!item.pinned)
  const idx = peers.findIndex((s) => s.id === id)
  const next = peers[idx + delta]
  if (!next) return
  session.reorder(id, next.id, delta === -1 ? 'before' : 'after')
}

const searchActive = computed(() => session.query.trim().length > 0)
const noMatch = computed(
  () => searchActive.value && session.grouped.every((g) => g.items.length === 0),
)

onMounted(() => {
  window.addEventListener('click', closeMenu)
  window.addEventListener('resize', closeMenu)
  window.addEventListener('scroll', closeMenu, true)
})
onBeforeUnmount(() => {
  window.removeEventListener('click', closeMenu)
  window.removeEventListener('resize', closeMenu)
  window.removeEventListener('scroll', closeMenu, true)
})

const menuStyle = computed(() => {
  const w = 176
  const h = menu.value.renaming ? 84 : 196
  const x = Math.max(8, Math.min(menu.value.x, window.innerWidth - w - 8))
  const y = Math.max(8, Math.min(menu.value.y, window.innerHeight - h - 8))
  return { left: `${x}px`, top: `${y}px`, width: `${w}px` }
})
</script>

<template>
  <div class="border-b border-line p-2">
    <div class="relative">
      <AppIcon
        name="search"
        class="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-ink-500"
      />
      <label class="sr-only" for="session-search">搜索会话</label>
      <input
        id="session-search"
        v-model="session.query"
        type="search"
        class="h-8 w-full rounded-lg border border-line bg-white/[0.02] pl-8 pr-2 text-ink-100 outline-none transition-colors placeholder:text-ink-500 hover:border-line-strong focus:border-accent-line"
        placeholder="搜索会话"
      />
    </div>
  </div>

  <nav class="min-h-0 flex-1 overflow-y-auto px-2 py-2" aria-label="会话列表">
    <div v-for="group in session.grouped" :key="group.key" class="mb-3 last:mb-0">
      <p class="px-2 pb-1 text-[11px] text-ink-500">{{ group.label }}</p>
      <ul class="space-y-px">
        <li v-for="item in group.items" :key="item.id">
          <button
            type="button"
            :draggable="!item.pinned"
            class="group relative flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left transition-colors"
            :class="[
              item.id === session.activeId
                ? 'bg-white/[0.06] text-ink-100'
                : 'text-ink-300 hover:bg-white/[0.03] hover:text-ink-100',
              draggingId === item.id ? 'opacity-40' : '',
              dropTarget?.id === item.id
                ? dropTarget.position === 'before'
                  ? 'shadow-[inset_0_1px_0_0_var(--color-accent)]'
                  : 'shadow-[inset_0_-1px_0_0_var(--color-accent)]'
                : '',
            ]"
            :aria-current="item.id === session.activeId ? 'true' : undefined"
            @click="session.switchSession(item.id)"
            @contextmenu="openMenu($event, item.id, item.title)"
            @dragstart="onDragStart($event, item.id, item.pinned)"
            @dragover="onDragOver($event, item.id, item.pinned)"
            @drop="onDrop($event, item.id)"
            @dragend="onDragEnd"
            @keydown.alt.up.prevent="moveByKeyboard(item.id, -1)"
            @keydown.alt.down.prevent="moveByKeyboard(item.id, 1)"
          >
            <!-- 激活标记：一条强调色竖线，比整块高亮克制 -->
            <span
              v-if="item.id === session.activeId"
              class="absolute -left-2 top-1/2 h-4 w-[2px] -translate-y-1/2 rounded-full bg-accent"
              aria-hidden="true"
            />
            <AppIcon v-if="item.pinned" name="pin" class="size-3 text-accent" />
            <span class="min-w-0 flex-1 truncate">{{ item.title }}</span>
            <!-- 该会话正在生成：转圈比数字更准确地传达「在跑」 -->
            <AppIcon
              v-if="session.isRunning(item.id)"
              name="spinner"
              class="size-3 animate-spin text-accent motion-reduce:animate-none"
            />
            <span v-else-if="item.count" class="shrink-0 font-mono text-[10px] text-ink-600">{{ item.count }}</span>
          </button>
        </li>
      </ul>
    </div>

    <p v-if="noMatch" class="px-2 py-8 text-center text-ink-500">没有匹配的会话</p>
    <p v-else-if="session.grouped.length === 0" class="px-2 py-8 text-center text-ink-500">
      {{ session.connection === 'open' ? '还没有会话' : '正在连接…' }}
    </p>
  </nav>

  <div class="space-y-1 border-t border-line p-2">
    <button
      type="button"
      class="flex w-full items-center gap-2 rounded-lg border border-line bg-white/[0.02] px-2 py-1.5 text-ink-200 transition-colors hover:border-line-strong hover:bg-white/[0.05] active:scale-[0.99]"
      @click="session.newSession()"
    >
      <AppIcon name="plus" class="size-3.5" />
      新建会话
      <span class="ml-auto font-mono text-[10px] text-ink-600">N</span>
    </button>
    <button
      type="button"
      class="flex w-full items-center gap-2 rounded-lg px-2 py-1 text-ink-400 transition-colors hover:bg-white/[0.03] hover:text-ink-200"
      @click="session.refreshSessions()"
    >
      <AppIcon name="refresh" class="size-3" />
      刷新列表
    </button>
  </div>

  <!-- 右键菜单：Teleport 出去避免被列容器 overflow 裁切 -->
  <Teleport to="body">
    <div
      v-if="menu.open"
      class="fixed z-50 rounded-[10px] border border-line-strong bg-elevated p-1"
      :style="menuStyle"
      role="menu"
      @click.stop
    >
      <template v-if="!menu.renaming">
        <button type="button" role="menuitem" class="menu-item" @click="beginRename">重命名</button>
        <button type="button" role="menuitem" class="menu-item" @click="togglePin">
          {{ target?.pinned ? '取消置顶' : '置顶' }}
        </button>
        <button type="button" role="menuitem" class="menu-item" @click="doBranch">从此分支</button>
        <button type="button" role="menuitem" class="menu-item" @click="doExport">导出 Markdown</button>
        <button type="button" role="menuitem" class="menu-item" @click="copyId">复制会话 ID</button>
        <div class="my-1 h-px bg-line" />
        <button type="button" role="menuitem" class="menu-item !text-danger" @click="askDelete">删除会话</button>
      </template>
      <div v-else class="p-1">
        <label class="sr-only" for="rename-input">会话名称</label>
        <input
          id="rename-input"
          ref="renameInput"
          v-model="menu.draft"
          class="h-7 w-full rounded-lg border border-line bg-white/[0.02] px-2 text-ink-100 outline-none focus:border-accent-line"
          @keydown.enter.prevent="commitRename"
          @keydown.esc.prevent="closeMenu"
        />
        <p class="mt-1 text-[11px] text-ink-500">Enter 保存 / Esc 取消</p>
      </div>
    </div>
  </Teleport>

  <BaseConfirm
    v-model:open="pendingDelete"
    title="删除这个会话？"
    :message="`「${target?.title ?? ''}」将移入回收目录 sessions/.trash。`"
    confirm-text="删除"
    danger
    @confirm="confirmDelete"
  />
</template>

<style scoped>
.menu-item {
  display: block;
  width: 100%;
  padding: 5px 8px;
  border-radius: 6px;
  text-align: left;
  color: var(--color-ink-200);
  transition: background 120ms var(--ease-std);
}
.menu-item:hover {
  background: rgba(255, 255, 255, 0.06);
}
</style>
