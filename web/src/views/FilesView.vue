<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiPost, downloadUrl } from '@/api/http'
import { useSettingsStore } from '@/stores/settings'
import { useToast } from '@/composables/useToast'
import { formatBytes } from '@/utils/format'
import AppIcon from '@/components/ui/AppIcon.vue'
import BaseModal from '@/components/ui/BaseModal.vue'
import BaseConfirm from '@/components/ui/BaseConfirm.vue'

/**
 * 文件浏览 / 编辑 / 上传。
 *
 * 契约要点（cmd/serve.go + internal/files）：
 *   - GET  /api/files?path=   → {root, path, entries:[{name,path,isDir,size}] | null}
 *   - GET  /api/file?path=    → {path, content}
 *   - POST /api/file          → {path, content} 保存
 *   - POST /api/mkdir         → {path}
 *   - POST /api/delete        → {path, recursive}；目录非空且 recursive=false → 409
 *   - POST /api/upload        → multipart：file / dir，单文件上限 100MB
 *   - GET  /api/download?path= → 文件或目录打包
 *
 * 注意 entries 可能是 null（空目录服务端给 null 而非 []），isDir 是驼峰。
 * 二进制文件不做内容预览，只提供下载 —— 把 NUL 字节塞进 textarea 没有意义。
 */
const settings = useSettingsStore()
const toast = useToast()

interface Entry {
  name: string
  path: string
  isDir: boolean
  size: number
}

const root = ref('')
const cwd = ref('')
const entries = ref<Entry[]>([])
const loading = ref(false)
const error = ref('')

async function load(dir: string) {
  loading.value = true
  error.value = ''
  try {
    const r = await apiGet<{ root: string; path: string; entries: Entry[] | null }>(
      `/api/files?path=${encodeURIComponent(dir)}`,
    )
    root.value = r.root
    cwd.value = r.path
    entries.value = r.entries ?? []
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    const r = await settings.loadWorkspace()
    await load(r)
  } catch (e) {
    error.value = (e as Error).message
  }
})

/** 面包屑：从工作区根到当前目录，逐级可点 */
const crumbs = computed(() => {
  const norm = (s: string) => s.replace(/\/+$/, '')
  const rootN = norm(root.value)
  const cwdN = norm(cwd.value)
  if (!cwdN) return []
  const base = rootN.split('/').filter(Boolean).pop() || '/'
  if (cwdN === rootN) return [{ label: base, path: rootN }]
  const rel = cwdN.startsWith(rootN) ? cwdN.slice(rootN.length) : cwdN
  const out = [{ label: base, path: rootN }]
  let acc = rootN
  for (const p of rel.split('/').filter(Boolean)) {
    acc = `${acc}/${p}`
    out.push({ label: p, path: acc })
  }
  return out
})

function goUp() {
  const parent = cwd.value.replace(/\/+$/, '').split('/').slice(0, -1).join('/') || '/'
  void load(parent)
}

const sorted = computed(() =>
  [...entries.value].sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
    return a.name.localeCompare(b.name)
  }),
)

/* ---------------- 预览 / 编辑 ---------------- */

const preview = ref<{ path: string; name: string; text: boolean } | null>(null)
const draft = ref('')
const saving = ref(false)

async function openEntry(entry: Entry) {
  if (entry.isDir) {
    await load(entry.path)
    return
  }
  try {
    const r = await apiGet<{ path: string; content: string }>(
      `/api/file?path=${encodeURIComponent(entry.path)}`,
    )
    // NUL 字节 = 二进制；超大文本也不适合塞进 textarea
    const isText = !r.content.includes('\u0000') && r.content.length < 2_000_000
    preview.value = { path: r.path, name: entry.name, text: isText }
    draft.value = r.content
  } catch (e) {
    toast.error((e as Error).message)
  }
}

async function save() {
  if (!preview.value?.text) return
  saving.value = true
  try {
    await apiPost<{ ok: boolean }>('/api/file', { path: preview.value.path, content: draft.value })
    toast.ok('已保存')
  } catch (e) {
    toast.error((e as Error).message)
  } finally {
    saving.value = false
  }
}

/* ---------------- 新建目录 / 上传 / 删除 ---------------- */

const mkdirOpen = ref(false)
const mkdirName = ref('')

async function doMkdir() {
  const name = mkdirName.value.trim()
  if (!name) return
  try {
    await apiPost('/api/mkdir', { path: `${cwd.value}/${name}` })
    toast.ok('已创建目录')
    mkdirOpen.value = false
    mkdirName.value = ''
    await load(cwd.value)
  } catch (e) {
    toast.error((e as Error).message)
  }
}

const fileInput = ref<HTMLInputElement | null>(null)
const uploading = ref(false)

async function onUpload(files: FileList | null) {
  if (!files?.length) return
  uploading.value = true
  try {
    for (const f of Array.from(files)) {
      const fd = new FormData()
      fd.append('file', f)
      fd.append('dir', cwd.value)
      await apiPost('/api/upload', fd)
    }
    toast.ok('上传完成')
    await load(cwd.value)
  } catch (e) {
    toast.error((e as Error).message)
  } finally {
    uploading.value = false
    if (fileInput.value) fileInput.value.value = ''
  }
}

const delTarget = ref<Entry | null>(null)
const delRecursive = ref(false)

async function confirmDelete() {
  const t = delTarget.value
  if (!t) return
  try {
    await apiPost('/api/delete', { path: t.path, recursive: delRecursive.value })
    toast.ok('已删除')
    delTarget.value = null
    await load(cwd.value)
  } catch (e) {
    const msg = (e as Error).message
    // 409 = 目录非空且未递归：把递归开关打开让用户确认，而不是单纯报错
    if (t.isDir && !delRecursive.value) delRecursive.value = true
    toast.error(msg)
  }
}

/* ---------------- 工作区 ---------------- */

const wsEdit = ref('')
const wsEditing = ref(false)

function toggleWsEdit() {
  wsEdit.value = cwd.value
  wsEditing.value = !wsEditing.value
}

async function applyWorkspace() {
  const p = wsEdit.value.trim()
  if (!p) return
  try {
    await settings.setWorkspace(p)
    toast.ok('已切换工作区')
    wsEditing.value = false
    await load(p)
  } catch (e) {
    toast.error((e as Error).message)
  }
}
</script>

<template>
  <div class="flex h-full min-h-0 flex-1 flex-col">
    <!-- 工具条 -->
    <div class="flex h-11 shrink-0 items-center gap-1 border-b border-line pr-3 pl-11 md:pl-3">
      <button
        type="button"
        class="grid size-7 place-items-center rounded-lg text-ink-400 transition-colors hover:bg-white/[0.05] hover:text-ink-100 disabled:opacity-40"
        aria-label="向上一级"
        :disabled="cwd === root"
        @click="goUp"
      >
        <AppIcon name="chevronLeft" class="size-4" />
      </button>
      <button
        type="button"
        class="grid size-7 place-items-center rounded-lg text-ink-400 transition-colors hover:bg-white/[0.05] hover:text-ink-100"
        aria-label="刷新"
        @click="load(cwd)"
      >
        <AppIcon name="refresh" class="size-4" />
      </button>

      <nav class="flex min-w-0 flex-1 items-center gap-0.5 overflow-x-auto font-mono text-[12px]" aria-label="路径">
        <template v-for="(c, i) in crumbs" :key="c.path">
          <button
            type="button"
            class="shrink-0 rounded px-1 py-0.5 transition-colors hover:bg-white/[0.05] hover:text-ink-100"
            :class="i === crumbs.length - 1 ? 'text-ink-200' : 'text-ink-400'"
            @click="load(c.path)"
          >
            {{ c.label }}
          </button>
          <AppIcon v-if="i < crumbs.length - 1" name="chevronRight" class="size-2.5 shrink-0 text-ink-600" />
        </template>
        <span v-if="!crumbs.length" class="text-ink-500">/</span>
      </nav>

      <button type="button" class="tb-btn" @click="toggleWsEdit">
        <AppIcon name="folder" class="size-3.5" />
        <span class="hidden sm:inline">工作区</span>
      </button>
      <button type="button" class="tb-btn" @click="downloadUrl(`/api/download?path=${encodeURIComponent(cwd)}`)">
        <AppIcon name="download" class="size-3.5" />
        <span class="hidden sm:inline">打包下载</span>
      </button>
      <button type="button" class="tb-btn" :disabled="uploading" @click="fileInput?.click()">
        <AppIcon :name="uploading ? 'spinner' : 'upload'" class="size-3.5" :class="uploading ? 'animate-spin motion-reduce:animate-none' : ''" />
        <span class="hidden sm:inline">上传</span>
      </button>
      <button type="button" class="tb-btn" @click="mkdirOpen = true">
        <AppIcon name="plus" class="size-3.5" />
        <span class="hidden sm:inline">新建目录</span>
      </button>
      <input ref="fileInput" type="file" multiple class="sr-only" @change="onUpload(($event.target as HTMLInputElement).files)" />
    </div>

    <!-- 工作区输入条 -->
    <div v-if="wsEditing" class="flex items-center gap-2 border-b border-line px-3 py-2">
      <label for="ws-path" class="shrink-0 text-ink-400">工作区</label>
      <input
        id="ws-path"
        v-model="wsEdit"
        class="h-8 min-w-0 flex-1 rounded-lg border border-line bg-white/[0.02] px-2.5 font-mono text-[12px] text-ink-100 outline-none focus:border-accent-line"
        placeholder="绝对路径"
        @keydown.enter.prevent="applyWorkspace"
      />
      <button type="button" class="rounded-lg bg-accent px-3 py-1 text-[var(--color-accent-ink)] hover:opacity-90" @click="applyWorkspace">
        应用
      </button>
    </div>

    <!-- 列表 -->
    <div class="min-h-0 flex-1 overflow-y-auto">
      <p v-if="error" class="px-4 py-10 text-center text-danger">{{ error }}</p>
      <p v-else-if="loading && !sorted.length" class="px-4 py-12 text-center text-ink-500">加载中…</p>
      <p v-else-if="!sorted.length" class="px-4 py-12 text-center text-ink-500">空目录</p>
      <ul v-else>
        <li
          v-for="e in sorted"
          :key="e.path"
          class="group/row flex items-center gap-2 border-b border-line/50 px-3 py-1.5 transition-colors hover:bg-white/[0.02]"
        >
          <button type="button" class="flex min-w-0 flex-1 items-center gap-2 text-left" @click="openEntry(e)">
            <AppIcon
              :name="e.isDir ? 'folder' : 'file'"
              class="size-3.5 shrink-0"
              :class="e.isDir ? 'text-accent' : 'text-ink-500'"
            />
            <span class="min-w-0 flex-1 truncate text-ink-200">{{ e.name }}</span>
          </button>
          <span class="shrink-0 font-mono text-[11px] text-ink-600">{{ e.isDir ? '' : formatBytes(e.size) }}</span>
          <div class="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover/row:opacity-100 focus-within:opacity-100">
            <button
              type="button"
              class="grid size-6 place-items-center rounded text-ink-500 transition-colors hover:bg-white/[0.06] hover:text-ink-200"
              aria-label="下载"
              @click="downloadUrl(`/api/download?path=${encodeURIComponent(e.path)}`)"
            >
              <AppIcon name="download" class="size-3" />
            </button>
            <button
              type="button"
              class="grid size-6 place-items-center rounded text-ink-500 transition-colors hover:bg-white/[0.06] hover:text-danger"
              aria-label="删除"
              @click="delTarget = e"
            >
              <AppIcon name="trash" class="size-3" />
            </button>
          </div>
        </li>
      </ul>
    </div>

    <BaseModal :open="!!preview" :title="preview?.name ?? ''" wide @close="preview = null">
      <p v-if="preview && !preview.text" class="py-8 text-center text-ink-500">
        二进制文件，仅支持下载，不做内容预览。
      </p>
      <textarea
        v-else-if="preview"
        v-model="draft"
        class="h-[58vh] w-full resize-none rounded-lg border border-line bg-sunken/50 p-3 font-mono text-[12px] leading-relaxed text-ink-200 outline-none focus:border-accent-line"
        spellcheck="false"
        @keydown.ctrl.s.prevent="save"
      />
      <template #footer>
        <button type="button" class="btn-ghost" @click="preview = null">关闭</button>
        <button
          v-if="preview?.text"
          type="button"
          class="rounded-lg bg-accent px-3 py-1 text-[var(--color-accent-ink)] hover:opacity-90 disabled:opacity-50"
          :disabled="saving"
          @click="save"
        >
          {{ saving ? '保存中…' : '保存' }}
        </button>
      </template>
    </BaseModal>

    <BaseModal :open="mkdirOpen" title="新建目录" @close="mkdirOpen = false">
      <label for="mkdir-name" class="mb-1 block text-ink-300">目录名（创建于当前目录下）</label>
      <input
        id="mkdir-name"
        v-model="mkdirName"
        class="h-8 w-full rounded-lg border border-line bg-white/[0.02] px-2.5 text-ink-100 outline-none focus:border-accent-line"
        placeholder="src"
        @keydown.enter.prevent="doMkdir"
      />
      <template #footer>
        <button type="button" class="btn-ghost" @click="mkdirOpen = false">取消</button>
        <button type="button" class="rounded-lg bg-accent px-3 py-1 text-[var(--color-accent-ink)] hover:opacity-90" @click="doMkdir">
          创建
        </button>
      </template>
    </BaseModal>

    <BaseConfirm
      :open="!!delTarget"
      title="确认删除"
      :message="
        delTarget?.isDir && delRecursive
          ? `「${delTarget.name}」是非空目录，将连同内部所有内容一起删除。`
          : `「${delTarget?.name ?? ''}」将被永久删除。`
      "
      confirm-text="删除"
      danger
      @update:open="
        (v) => {
          if (!v) delTarget = null
        }
      "
      @confirm="confirmDelete"
    />
  </div>
</template>

<style scoped>
.tb-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border: 1px solid var(--color-line);
  border-radius: 8px;
  color: var(--color-ink-300);
  transition: all 120ms var(--ease-std);
}
.tb-btn:hover {
  border-color: var(--color-line-strong);
  background: rgba(255, 255, 255, 0.05);
}
.tb-btn:disabled {
  opacity: 0.5;
}
.btn-ghost {
  padding: 4px 12px;
  border: 1px solid var(--color-line);
  border-radius: 8px;
  color: var(--color-ink-200);
  transition: all 120ms var(--ease-std);
}
.btn-ghost:hover {
  border-color: var(--color-line-strong);
  background: rgba(255, 255, 255, 0.05);
}
</style>
