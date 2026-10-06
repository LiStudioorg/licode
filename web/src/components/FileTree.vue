<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiGet } from '@/api/http'
import { useSettingsStore } from '@/stores/settings'
import AppIcon from '@/components/ui/AppIcon.vue'

/**
 * 右栏文件树（只读浏览）。
 *
 * 契约：GET /api/files?path= 返回 {root, path, entries}，entries 可能是 null
 * （空目录服务端给 null 而非 []），isDir 是驼峰。这里只做一层懒展开，
 * 不做递归预取 —— 大仓库下递归会把整棵树打进内存。
 */
interface Entry {
  name: string
  path: string
  isDir: boolean
  size: number
}

const settings = useSettingsStore()

const root = ref('')
const entries = ref<Entry[]>([])
const expanded = ref<Set<string>>(new Set())
const children = ref<Record<string, Entry[]>>({})
const loading = ref(false)

async function loadDir(dir: string): Promise<Entry[]> {
  const r = await apiGet<{ root: string; path: string; entries: Entry[] | null }>(
    `/api/files?path=${encodeURIComponent(dir)}`,
  )
  root.value = r.root
  return r.entries ?? []
}

async function toggle(e: Entry) {
  if (!e.isDir) return
  const next = new Set(expanded.value)
  if (next.has(e.path)) {
    next.delete(e.path)
  } else {
    next.add(e.path)
    if (!children.value[e.path]) {
      try {
        children.value[e.path] = await loadDir(e.path)
      } catch {
        children.value[e.path] = []
      }
    }
  }
  expanded.value = next
}

onMounted(async () => {
  loading.value = true
  try {
    const r = await settings.loadWorkspace()
    entries.value = await loadDir(r)
  } catch {
    entries.value = []
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="tree">
    <p class="tree__root" :title="root">{{ root }}</p>
    <p v-if="loading" class="tree__empty">加载中…</p>
    <p v-else-if="!entries.length" class="tree__empty">空目录</p>
    <ul v-else class="tree__list">
      <template v-for="e in entries" :key="e.path">
        <li>
          <button type="button" class="tree__row" @click="toggle(e)">
            <AppIcon
              v-if="e.isDir"
              :name="expanded.has(e.path) ? 'chevronDown' : 'chevronRight'"
              class="size-3 shrink-0"
            />
            <AppIcon :name="e.isDir ? 'folder' : 'file'" class="size-3.5 shrink-0" />
            <span class="tree__name">{{ e.name }}</span>
          </button>
        </li>
        <li v-if="e.isDir && expanded.has(e.path)">
          <ul class="tree__sub">
            <li v-for="c in children[e.path] ?? []" :key="c.path">
              <button type="button" class="tree__row">
                <AppIcon :name="c.isDir ? 'folder' : 'file'" class="size-3.5 shrink-0" />
                <span class="tree__name">{{ c.name }}</span>
              </button>
            </li>
          </ul>
        </li>
      </template>
    </ul>
  </div>
</template>

<style scoped>
.tree {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}
.tree__root {
  flex-shrink: 0;
  padding: 8px 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--font-mono, monospace);
  font-size: 11px;
  color: var(--color-ink-500);
}
.tree__list,
.tree__sub {
  overflow-y: auto;
  min-height: 0;
}
.tree__sub {
  padding-left: 14px;
}
.tree__row {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 5px 10px;
  border-radius: 6px;
  color: var(--color-ink-300);
  text-align: left;
}
.tree__row:hover {
  background: rgba(255, 255, 255, 0.04);
}
.tree__name {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tree__empty {
  padding: 16px 10px;
  text-align: center;
  color: var(--color-ink-500);
}
</style>
