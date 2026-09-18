<script setup lang="ts">
import { ArrowLeft, Search, RefreshCw, Trash2, ChevronDown, ChevronRight, Loader2, Blocks, Wrench, Server, Sparkles, Cpu } from 'lucide-vue-next'
import { Message, Button, Input, Select, Chip, Dialog, Empty } from 'fuxsto-design'
import { useTheme } from '~/composables/useTheme'

interface ToolInfo {
  name: string
  description: string
  source: 'builtin' | 'subagent' | 'mcp' | 'external' | 'skill'
  server?: string
  file?: string
  removable: boolean
  rule: string
}

interface McpServer {
  name: string
  type?: string
  command?: string
  args?: string[]
  url?: string
}

const ruleOptions = [
  { label: '允许', value: 'allow' },
  { label: '询问', value: 'ask' },
  { label: '禁止', value: 'deny' },
]

// 内置工具的中文名称与说明（用户界面全中文）。
const builtinZh: Record<string, { name: string; desc: string }> = {
  Read: { name: '读取文件', desc: '读取文本文件内容，支持按行范围读取' },
  Write: { name: '写入文件', desc: '新建或覆盖写入文件（自动创建父目录）' },
  Edit: { name: '编辑文件', desc: '在文件中查找并替换文本' },
  ListDirectory: { name: '列出目录', desc: '列出目录下的文件与大小' },
  Grep: { name: '搜索内容', desc: '用正则表达式搜索文件内容' },
  Glob: { name: '查找文件', desc: '按通配符查找文件（支持 ** 递归）' },
  Shell: { name: '执行命令', desc: '执行 Shell 命令并返回输出' },
  Delete: { name: '删除文件', desc: '删除文件或空目录' },
  Move: { name: '移动/重命名', desc: '移动或重命名文件、目录' },
  Dispatch: { name: '调度子代理', desc: '把子任务分派给专用子代理并行执行' },
}

const groupDefs = [
  { id: 'builtin', label: '内置工具', icon: Wrench, hint: '随程序内置的编码工具' },
  { id: 'subagent', label: '子代理', icon: Cpu, hint: '把任务拆分给专用子代理执行' },
  { id: 'mcp', label: 'MCP 工具', icon: Blocks, hint: '来自 MCP 服务器，删除请移除对应服务器' },
  { id: 'external', label: '外部命令工具', icon: Server, hint: '来自 ~/.licode/tools/ 的自定义工具' },
  { id: 'skill', label: '技能', icon: Sparkles, hint: '来自 skills/ 的 Markdown 技能' },
]

const tools = ref<ToolInfo[]>([])
const mcpServers = ref<McpServer[]>([])
const mcpError = ref('')
const loading = ref(true)
const savingRule = ref('')
const query = ref('')
const open = reactive<Record<string, boolean>>({
  builtin: true,
  subagent: true,
  mcp: true,
  external: true,
  skill: true,
})

const { initTheme } = useTheme()
onMounted(() => {
  initTheme()
  load()
})

async function load() {
  loading.value = true
  try {
    const data = await useApi<{ tools: ToolInfo[]; mcp_servers?: McpServer[]; mcp_error?: string }>('/api/tools')
    tools.value = data.tools || []
    mcpServers.value = data.mcp_servers || []
    mcpError.value = data.mcp_error || ''
  } catch (e: any) {
    Message.error(e?.message || '加载工具列表失败')
  } finally {
    loading.value = false
  }
}

function displayName(t: ToolInfo): string {
  if (t.source === 'mcp') {
    const parts = t.name.split('__')
    return parts.length >= 3 ? parts.slice(2).join('__') : t.name
  }
  if (t.source === 'skill') return t.name.replace(/^skill_/, '')
  return builtinZh[t.name]?.name || t.name
}

function displayDesc(t: ToolInfo): string {
  if (t.source === 'builtin' || t.source === 'subagent') return builtinZh[t.name]?.desc || t.description || '暂无说明'
  return t.description || '暂无说明（由工具提供方定义）'
}

function matches(t: ToolInfo): boolean {
  const q = query.value.trim().toLowerCase()
  if (!q) return true
  return (
    t.name.toLowerCase().includes(q) ||
    displayName(t).toLowerCase().includes(q) ||
    displayDesc(t).toLowerCase().includes(q) ||
    (t.server || '').toLowerCase().includes(q)
  )
}

const filtered = computed(() => tools.value.filter(matches))

function groupTools(id: string): ToolInfo[] {
  return filtered.value.filter((t) => t.source === id)
}

function serverTools(name: string): ToolInfo[] {
  return filtered.value.filter((t) => t.source === 'mcp' && t.server === name)
}

// 仅有配置但未连上的服务器也要显示（否则删不掉）。
const mcpServerRows = computed(() => {
  const list = mcpServers.value.slice()
  for (const t of filtered.value) {
    if (t.source === 'mcp' && t.server && !list.some((s) => s.name === t.server)) {
      list.push({ name: t.server, type: 'stdio' })
    }
  }
  return list.filter((s) => {
    const q = query.value.trim().toLowerCase()
    if (!q) return true
    return s.name.toLowerCase().includes(q) || serverTools(s.name).some(matches)
  })
})

function groupCount(id: string): number {
  if (id === 'mcp') return mcpServerRows.value.length
  return groupTools(id).length
}

function toggleAll(openState: boolean) {
  for (const g of groupDefs) open[g.id] = openState
}

async function setRule(t: ToolInfo, rule: string) {
  const prev = t.rule
  t.rule = rule
  savingRule.value = t.name
  try {
    await useApi('/api/tools/rule', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ name: t.name, rule }),
    })
  } catch (e: any) {
    t.rule = prev
    Message.error(e?.message || '设置失败')
  } finally {
    savingRule.value = ''
  }
}

function removeTool(t: ToolInfo) {
  const isSkill = t.source === 'skill'
  const type = isSkill ? 'skill' : 'external'
  const name = isSkill ? t.name.replace(/^skill_/, '') : t.name
  Dialog.confirm({
    title: isSkill ? '删除技能' : '删除外部命令工具',
    content: `确定删除「${displayName(t)}」？将删除对应文件，不可恢复。`,
    danger: true,
    confirmText: '删除',
    onConfirm: async () => {
      try {
        await useApi('/api/tools/delete', {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ type, name }),
        })
        Message.success('已删除')
        load()
      } catch (e: any) {
        Message.error(e?.message || '删除失败')
      }
    },
  })
}

function removeMcpServer(s: McpServer) {
  Dialog.confirm({
    title: '删除 MCP 服务器',
    content: `确定删除「${s.name}」？该服务器及其工具将不再可用，相关权限规则一并清理。`,
    danger: true,
    confirmText: '删除',
    onConfirm: async () => {
      try {
        await useApi('/api/tools/delete', {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ type: 'mcp', name: s.name }),
        })
        Message.success('已删除')
        load()
      } catch (e: any) {
        Message.error(e?.message || '删除失败')
      }
    },
  })
}

const sourceBadge: Record<string, string> = {
  builtin: '内置',
  subagent: '子代理',
  mcp: 'MCP',
  external: '外部命令',
  skill: '技能',
}
</script>

<template>
  <div class="mx-auto flex h-full max-w-4xl flex-col px-6 py-6">
    <!-- 头部 -->
    <div class="mb-4 flex shrink-0 items-center gap-3">
      <button class="rounded-lg p-1.5 text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800" title="返回对话" @click="navigateTo('/')">
        <ArrowLeft :size="16" />
      </button>
      <h1 class="text-lg font-semibold">工具</h1>
      <span class="text-xs text-zinc-400">管理 AI 可调用的全部工具：权限与删除</span>
      <span class="flex-1" />
      <Button size="sm" variant="outline" :icon="RefreshCw" :loading="loading" @click="load">刷新</Button>
      <Button size="sm" variant="ghost" @click="toggleAll(true)">全部展开</Button>
      <Button size="sm" variant="ghost" @click="toggleAll(false)">全部折叠</Button>
    </div>

    <!-- 搜索与图例 -->
    <div class="mb-4 flex shrink-0 items-center gap-3">
      <Input v-model="query" size="sm" class="flex-1" :prefix-icon="Search" placeholder="搜索工具名称或说明…" />
      <span class="whitespace-nowrap text-[11px] text-zinc-400">
        允许=直接执行 · 询问=每次确认 · 禁止=模型不可调用
      </span>
    </div>

    <div v-if="loading" class="flex flex-1 flex-col items-center justify-center gap-3 text-sm text-zinc-500">
      <Loader2 :size="20" class="animate-spin" />
      正在加载工具列表…（首次连接 MCP 服务器可能需要几秒）
    </div>

    <Empty v-else-if="!filtered.length" title="没有匹配的工具" description="换个关键词试试，或检查工具配置" class="mt-16" />

    <!-- 分组列表 -->
    <div v-else class="min-h-0 flex-1 space-y-3 overflow-y-auto pb-6">
      <div v-for="g in groupDefs" :key="g.id" class="overflow-hidden rounded-xl border border-zinc-200 dark:border-zinc-800">
        <!-- 分组头 -->
        <button
          class="flex w-full items-center gap-2 bg-zinc-50 px-4 py-3 text-left transition-colors hover:bg-zinc-100 dark:bg-zinc-900 dark:hover:bg-zinc-800"
          @click="open[g.id] = !open[g.id]"
        >
          <ChevronDown v-if="open[g.id]" :size="15" class="text-zinc-400" />
          <ChevronRight v-else :size="15" class="text-zinc-400" />
          <component :is="g.icon" :size="15" class="text-zinc-500" />
          <span class="text-sm font-medium">{{ g.label }}</span>
          <Chip size="sm" variant="secondary">{{ groupCount(g.id) }}</Chip>
          <span class="flex-1" />
          <span class="hidden text-[11px] text-zinc-400 sm:block">{{ g.hint }}</span>
        </button>

        <div v-if="open[g.id]" class="divide-y divide-zinc-100 bg-white dark:divide-zinc-800 dark:bg-zinc-900">
          <!-- MCP：按服务器展示 -->
          <template v-if="g.id === 'mcp'">
            <div v-if="mcpError" class="px-4 py-2 text-[11px] text-amber-600 dark:text-amber-400">
              部分 MCP 服务器连接失败（{{ mcpError }}），下方仍可管理配置。
            </div>
            <div v-if="!mcpServerRows.length" class="px-4 py-4 text-xs text-zinc-400">暂未配置 MCP 服务器（可在「设置 → MCP 工具」中添加）</div>
            <div v-for="srv in mcpServerRows" :key="srv.name">
              <div class="flex items-center gap-2 px-4 py-2.5">
                <Blocks :size="14" class="text-zinc-400" />
                <span class="text-sm font-medium">{{ srv.name }}</span>
                <Chip size="sm" variant="outline">{{ srv.type === 'http' ? '远程服务' : '本地命令' }}</Chip>
                <span class="min-w-0 flex-1 truncate text-[11px] text-zinc-400">
                  {{ srv.type === 'http' ? srv.url : [srv.command, ...(srv.args || [])].join(' ') }}
                </span>
                <Button size="sm" variant="ghost" danger :icon="Trash2" title="删除该服务器" @click="removeMcpServer(srv)" />
              </div>
              <div v-if="!serverTools(srv.name).length" class="px-4 pb-3 pl-10 text-[11px] text-zinc-400">
                {{ mcpError ? '未连接成功，暂无工具列表' : '该服务器没有提供工具' }}
              </div>
              <div
                v-for="t in serverTools(srv.name)"
                :key="t.name"
                class="flex items-center gap-3 border-t border-zinc-100 px-4 py-2.5 pl-10 dark:border-zinc-800"
              >
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="truncate text-sm">{{ displayName(t) }}</span>
                    <span class="shrink-0 font-mono text-[10px] text-zinc-400">{{ t.name }}</span>
                  </div>
                  <p class="mt-0.5 line-clamp-2 text-[11px] text-zinc-400">{{ displayDesc(t) }}</p>
                </div>
                <Select
                  :model-value="t.rule"
                  size="sm"
                  :options="ruleOptions"
                  class="w-24 shrink-0"
                  :loading="savingRule === t.name"
                  @update:model-value="setRule(t, String($event))"
                />
              </div>
            </div>
          </template>

          <!-- 其他来源：平铺 -->
          <template v-else>
            <div v-if="!groupTools(g.id).length" class="px-4 py-4 text-xs text-zinc-400">暂无工具</div>
            <div v-for="t in groupTools(g.id)" :key="t.name" class="flex items-center gap-3 px-4 py-2.5">
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="truncate text-sm">{{ displayName(t) }}</span>
                  <span class="shrink-0 font-mono text-[10px] text-zinc-400">{{ t.name }}</span>
                  <Chip v-if="t.source !== 'builtin' && t.source !== 'subagent'" size="sm" variant="outline">
                    {{ sourceBadge[t.source] }}
                  </Chip>
                  <span v-if="t.file" class="truncate text-[10px] text-zinc-400">{{ t.file }}</span>
                </div>
                <p class="mt-0.5 line-clamp-2 text-[11px] text-zinc-400">{{ displayDesc(t) }}</p>
              </div>
              <Select
                :model-value="t.rule"
                size="sm"
                :options="ruleOptions"
                class="w-24 shrink-0"
                :loading="savingRule === t.name"
                @update:model-value="setRule(t, String($event))"
              />
              <Button
                v-if="t.removable"
                size="sm"
                variant="ghost"
                danger
                :icon="Trash2"
                title="删除"
                @click="removeTool(t)"
              />
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>
