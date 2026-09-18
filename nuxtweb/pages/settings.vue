<script setup lang="ts">
import {
  X, Plus, Trash2, RefreshCw, CheckCircle2, Loader2, Settings as SettingsIcon,
  ArrowLeft, SlidersHorizontal, Server, Wrench, Blocks, Palette, Sun, Moon, Info,
} from 'lucide-vue-next'
import { Message, Button, Input, Switch, Chip, Select, Dialog } from 'fuxsto-design'
import type { DNSConfig, ProviderConfig, Settings } from '~/composables/useLicode'
import { useTheme } from '~/composables/useTheme'

const dnsModeOptions = [
  { label: '跟随系统 DNS', value: 'system' },
  { label: '自定义 DNS 服务器', value: 'custom' },
  { label: '系统命令获取（getent/ping）', value: 'command' },
]

const dnsProtocolOptions = [
  { label: '普通 DNS (UDP/TCP)', value: 'plain' },
  { label: 'DNS over TLS', value: 'dot' },
  { label: 'DNS over HTTPS', value: 'doh' },
]

// DNS 预设：国内 + 国外，覆盖 plain / DoT / DoH 三种协议，点击一键添加。
const dnsPresets: { name: string; region: '国内' | '国外'; mode: string; server: string }[] = [
  // 国内 - 阿里
  { name: '阿里 DoH', region: '国内', mode: 'doh', server: 'https://dns.alidns.com/dns-query' },
  { name: '阿里 UDP', region: '国内', mode: 'plain', server: '223.5.5.5:53' },
  { name: '阿里 UDP 2', region: '国内', mode: 'plain', server: '223.6.6.6:53' },
  { name: '阿里 DoT', region: '国内', mode: 'dot', server: '223.5.5.5:853' },
  // 国内 - 腾讯
  { name: '腾讯 DoH', region: '国内', mode: 'doh', server: 'https://doh.pub/dns-query' },
  { name: '腾讯 UDP', region: '国内', mode: 'plain', server: '119.28.28.28:53' },
  { name: '腾讯 DoT', region: '国内', mode: 'dot', server: '119.29.29.29:853' },
  // 国内 - 其他
  { name: 'OneDNS DoH', region: '国内', mode: 'doh', server: 'https://doh.onedns.net/dns-query' },
  { name: 'OneDNS DoT', region: '国内', mode: 'dot', server: '1.2.4.8:853' },
  { name: '360 UDP', region: '国内', mode: 'plain', server: '101.226.4.6:53' },
  { name: '360 UDP 2', region: '国内', mode: 'plain', server: '218.30.118.6:53' },
  { name: '114 UDP', region: '国内', mode: 'plain', server: '114.114.114.114:53' },
  { name: '114 UDP 2', region: '国内', mode: 'plain', server: '114.114.115.115:53' },
  { name: '百度 DoH', region: '国内', mode: 'doh', server: 'https://doh.baidu.com/dns-query' },
  { name: 'CNNIC DoH', region: '国内', mode: 'doh', server: 'https://doh.cnnic.cn/dns-query' },
  // 国外 - Cloudflare
  { name: 'CF DoH', region: '国外', mode: 'doh', server: 'https://1.1.1.1/dns-query' },
  { name: 'CF UDP', region: '国外', mode: 'plain', server: '1.0.0.1:53' },
  { name: 'CF DoT', region: '国外', mode: 'dot', server: '1.1.1.1:853' },
  { name: 'CF 安全 DoH', region: '国外', mode: 'doh', server: 'https://security.cloudflare-dns.com/dns-query' },
  // 国外 - Google
  { name: 'Google DoH', region: '国外', mode: 'doh', server: 'https://dns.google/dns-query' },
  { name: 'Google DoT', region: '国外', mode: 'dot', server: '8.8.8.8:853' },
  { name: 'Google UDP', region: '国外', mode: 'plain', server: '8.8.4.4:53' },
  // 国外 - 其他
  { name: 'Quad9 DoH', region: '国外', mode: 'doh', server: 'https://dns.quad9.net/dns-query' },
  { name: 'Quad9 DoT', region: '国外', mode: 'dot', server: '9.9.9.9:853' },
  { name: 'Quad9 UDP', region: '国外', mode: 'plain', server: '9.9.9.10:53' },
  { name: 'OpenDNS DoH', region: '国外', mode: 'doh', server: 'https://doh.opendns.com/dns-query' },
  { name: 'OpenDNS UDP', region: '国外', mode: 'plain', server: '208.67.222.222:53' },
  { name: 'AdGuard DoH', region: '国外', mode: 'doh', server: 'https://dns.adguard-dns.com/dns-query' },
  { name: 'AdGuard DoT', region: '国外', mode: 'dot', server: '94.140.14.14:853' },
  { name: 'AdGuard UDP', region: '国外', mode: 'plain', server: '94.140.14.15:53' },
  { name: 'NextDNS DoH', region: '国外', mode: 'doh', server: 'https://dns.nextdns.io/dns-query' },
  { name: 'Mullvad DoH', region: '国外', mode: 'doh', server: 'https://dns.mullvad.net/dns-query' },
  { name: 'Control D DoH', region: '国外', mode: 'doh', server: 'https://freedns.controld.com/p0' },
]

function addDnsPreset(p: (typeof dnsPresets)[0]) {
  ensureDns()
  if ((local.value.dns?.servers || []).some((s) => s.server === p.server)) {
    Message.info(`已存在 ${p.name}`)
    return
  }
  local.value.dns!.servers!.push({ mode: p.mode as any, server: p.server })
}

const _newDnsServer = ref('')

function ensureDns() {
  if (!local.value.dns) local.value.dns = {}
  if (!local.value.dns.servers) local.value.dns.servers = []
}

function addDnsServer() {
  const s = (_newDnsServer.value || '').trim()
  if (!s) return
  ensureDns()
  const mode = s.startsWith('http') ? 'doh' : s.includes(':853') ? 'dot' : 'plain'
  local.value.dns!.servers!.push({ mode, server: s })
  _newDnsServer.value = ''
}

function removeDnsServer(i: number) {
  if (local.value.dns?.servers) local.value.dns.servers.splice(i, 1)
}

type ProviderRow = ProviderConfig & { _newModel?: string }

const licode = useLicode()
const { state } = licode
const { mode: themeMode, skin, setSkin, setMode, initTheme } = useTheme()

const tab = ref<'basic' | 'providers' | 'advanced' | 'mcp' | 'appearance'>('basic')
const local = ref<Settings>({})
const toolRows = ref<{ tool: string; rule: string }[]>([])
const mcpServers = ref<any[]>([])
const fetching = ref('')
const saving = ref(false)
const shellOptions = ref<{ label: string; value: string }[]>([])

const navItems = [
  { id: 'basic', label: '基础', icon: SlidersHorizontal },
  { id: 'providers', label: 'AI 厂商', icon: Server },
  { id: 'advanced', label: '高级', icon: Wrench },
  { id: 'mcp', label: 'MCP 工具', icon: Blocks },
  { id: 'appearance', label: '外观', icon: Palette },
] as const

async function loadShells() {
  try {
    const shells = await useApi<string[]>('/api/shells')
    shellOptions.value = (shells || []).map((s) => ({ label: s, value: s }))
  } catch {}
}

const mcpPresets: Record<string, any> = {
  filesystem: { name: 'filesystem', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-filesystem', '/'] },
  git: { name: 'git', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-git'] },
  github: { name: 'github', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-github'] },
  postgres: { name: 'postgres', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-postgres'] },
  sqlite: { name: 'sqlite', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-sqlite'] },
  memory: { name: 'memory', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-memory'] },
  puppeteer: { name: 'puppeteer', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-puppeteer'] },
  'brave-search': { name: 'brave-search', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-brave-search'] },
  fetch: { name: 'fetch', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-fetch'] },
}

function isMcpAdded(name: string) {
  return mcpServers.value.some((s) => s.name === name && s.type !== 'http')
}

function addMcpPreset(name: string) {
  const p = mcpPresets[name]
  if (p && !isMcpAdded(name)) {
    mcpServers.value.push({ ...p })
  }
}

function addMcpCustom(type: 'stdio' | 'http' = 'stdio') {
  if (type === 'http') mcpServers.value.push({ name: 'custom-http', type: 'http', url: '' })
  else mcpServers.value.push({ name: 'custom', type: 'stdio', command: '', args: [] })
}

const mcpTypeOptions = [
  { label: '本地命令（stdio，如 python/uvx）', value: 'stdio' },
  { label: '远程服务（http/https）', value: 'http' },
]

function addMcpTyped() {
  addMcpCustom(String((newMcpType.value as any)) === 'http' ? 'http' : 'stdio')
}

const newMcpType = ref<'stdio' | 'http'>('stdio')

const NUM_KEYS = [
  'temperature',
  'max_tokens',
  'max_iterations',
  'retry_max',
  'sub_timeout',
  'max_ctx_tokens',
  'cache_ttl',
  'tool_retry_max',
  'shutdown_timeout',
  'rag_top_files',
] as const

const COMMA_KEYS = ['ask_tools', 'deny_tools'] as const

const fieldLabels: Record<string, string> = {
  retry_max: '调用失败重试次数',
  sub_timeout: '子代理超时（秒）',
  max_ctx_tokens: '上下文窗口上限（token）',
  cache_ttl: '缓存有效期（秒）',
  tool_retry_max: '工具重试次数',
  shutdown_timeout: '关停等待时间（秒）',
  rag_source: 'RAG 索引目录（留空关闭）',
  ask_tools: '询问类工具（逗号分隔）',
  deny_tools: '禁止类工具（逗号分隔）',
  redact_secrets: '敏感信息脱敏',
  sandbox: '沙箱执行（Docker）',
  tool_auto_retry: '工具自动重试',
  rag_enabled: '启用 RAG 项目检索',
}

const typeOptions = [
  { label: 'OpenAI 兼容', value: 'openai' },
  { label: 'Claude', value: 'claude' },
  { label: 'Ollama', value: 'ollama' },
  { label: 'Gemini', value: 'gemini' },
]

const ruleOptions = [
  { label: '允许', value: 'allow' },
  { label: '询问', value: 'ask' },
  { label: '禁止', value: 'deny' },
]

const newProvider = ref<ProviderRow>({ provider: '', name: '', type: 'openai', base_url: '', api_key: '', model: '', models: [] })

// 厂商详细设置小弹窗：指定 IP / 忽略 SSL / 协议类型 / 获取模型等集中配置。
const detailOpen = ref(false)
const detailProvider = ref<ProviderRow | null>(null)

function openDetail(p: ProviderRow) {
  detailProvider.value = p
  detailOpen.value = true
}

// 用户自定义 CA证书管理（~/.licode/certs/）。
interface CAInfo { name: string; size: number; mod_time: string; subjects?: string[]; valid: boolean }
const caList = ref<CAInfo[]>([])
const caDir = ref('')
const caInput = ref<HTMLInputElement | null>(null)

async function loadCAs() {
  try {
    const d = await useApi<{ dir: string; certs?: CAInfo[] }>('/api/ca')
    caDir.value = d.dir || '~/.licode/certs'
    caList.value = d.certs || []
  } catch {}
}

async function onCAChange(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (!files?.length) return
  for (const f of Array.from(files)) {
    const fd = new FormData()
    fd.append('file', f)
    try {
      const res = await useApi<{ name: string }>('/api/ca/upload', { method: 'POST', body: fd })
      Message.success(`已上传 ${res.name}`)
    } catch (err: any) {
      Message.error(err?.message || '上传失败')
    }
  }
  ;(e.target as HTMLInputElement).value = ''
  loadCAs()
}

function deleteCA(name: string) {
  Dialog.confirm({
    title: '删除 CA 证书',
    content: `确定删除「${name}」？相关 TLS 将不再信任该 CA。`,
    danger: true,
    confirmText: '删除',
    onConfirm: async () => {
      try {
        await useApi('/api/ca/delete', {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ name }),
        })
        Message.success('已删除')
        loadCAs()
      } catch (err: any) {
        Message.error(err?.message || '删除失败')
      }
    },
  })
}

function resetLocal() {
  if (!state.settings) return
  local.value = JSON.parse(JSON.stringify(state.settings))
  if (local.value.streaming === null || local.value.streaming === undefined) local.value.streaming = true
  if (!local.value.dns) local.value.dns = {}
  if (!local.value.dns.servers) local.value.dns.servers = []
  if (!local.value.dns.mode) local.value.dns.mode = 'custom'
  toolRows.value = Object.entries(local.value.tool_rules || {}).map(([tool, rule]) => ({
    tool,
    rule: String(rule),
  }))
  mcpServers.value = local.value.mcp_servers ? JSON.parse(JSON.stringify(local.value.mcp_servers)) : []
}

onMounted(() => {
  initTheme()
  licode.connect()
  loadShells()
  loadCAs()
  resetLocal()
})

watch(
  () => state.settings,
  () => resetLocal(),
)

const providers = computed<ProviderRow[]>(() => (local.value.providers as ProviderRow[]) || [])

// 当前模型的候选列表：已配置列表 + 当前使用模型（去重），保证旧配置/自定义模型都能显示。
function modelOpts(p: ProviderRow): { label: string; value: string }[] {
  const set = new Set<string>()
  for (const m of p.models || []) set.add(m)
  if (p.model) set.add(p.model)
  return [...set].map((m) => ({ label: m, value: m }))
}

function addModel(p: ProviderRow) {
  const m = (p._newModel || '').trim()
  if (!m) return
  if (!p.models) p.models = []
  if (!p.models.includes(m)) p.models.push(m)
  p.model = m
  p._newModel = ''
}

function removeModel(p: ProviderRow, m: string) {
  if (p.models) p.models = p.models.filter((x) => x !== m)
  if (p.model === m) p.model = ''
}

function activate(p: ProviderRow) {
  local.value.provider = p.provider
  local.value.base_url = p.base_url || ''
  local.value.api_key = p.api_key || ''
  if (p.model) local.value.model = p.model
  Message.info(`已切换激活厂商为「${p.name || p.provider}」，点击保存生效`)
}

async function fetchModels(p: ProviderRow) {
  if (!p.base_url && p.type !== 'claude') {
    Message.warning('请先填写该厂商的 API 地址')
    return
  }
  saving.value = true
  const base = state.settings
  const prev = base ? JSON.parse(JSON.stringify(base)) : null
  const switched = !!(base && base.provider !== p.provider)
  try {
    // /api/models 使用「激活厂商」的 key。目标厂商不是当前激活时，临时切到该厂商获取列表，
    // 完成后立即还原原激活厂商（不改变已保存的设置）。
    if (switched && prev) {
      licode.saveSettings({
        ...prev,
        provider: p.provider,
        base_url: p.base_url || '',
        api_key: p.api_key || '',
        model: p.model || prev.model || '',
      })
    }
    fetching.value = p.provider
    const q = new URLSearchParams()
    if (p.type) q.set('type', p.type)
    if (p.base_url) q.set('base', p.base_url)
    const res = await useApi<{ models: string[] }>(`/api/models?${q.toString()}`)
    const fetched = res.models || []
    if (!p.models) p.models = []
    for (const m of fetched) if (!p.models.includes(m)) p.models.push(m)
    if (!fetched.length) Message.info('该厂商无公开模型列表（如 Claude），请手动填写模型名')
    else Message.success(`获取到 ${fetched.length} 个模型`)
  } catch (e: any) {
    Message.error(e?.message || '获取模型失败')
  } finally {
    if (switched && prev) licode.saveSettings(prev)
    fetching.value = ''
    saving.value = false
  }
}

function addProvider() {
  const np = newProvider.value
  const id = (np.provider || np.name || np.type || 'custom')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '-')
  if (!id) {
    Message.warning('请填写厂商标识或名称')
    return
  }
  if (providers.value.some((p) => p.provider === id)) {
    Message.error('厂商标识已存在')
    return
  }
  local.value.providers = [
    ...providers.value,
    {
      provider: id,
      name: np.name || '',
      type: np.type || 'openai',
      base_url: np.base_url || '',
      api_key: np.api_key || '',
      model: np.model || '',
    },
  ]
  newProvider.value = { provider: '', name: '', type: 'openai', base_url: '', api_key: '', model: '' }
  Message.success('厂商已添加，点击保存生效')
}

function removeProvider(p: ProviderRow) {
  local.value.providers = providers.value.filter((x) => x.provider !== p.provider)
  if (local.value.provider === p.provider) {
    const next = providers.value[0]
    local.value.provider = next?.provider || ''
  }
  Message.success('厂商已移除，点击保存生效')
}

function splitList(v: unknown): string[] {
  return String(v ?? '')
    .split(/[,，]/)
    .map((x) => x.trim())
    .filter(Boolean)
}

function buildSettings(): Settings {
  const s = JSON.parse(JSON.stringify(local.value)) as Settings
  for (const k of NUM_KEYS) s[k] = Number(s[k]) || 0
  s.streaming = !!s.streaming
  const rules: Record<string, string> = {}
  for (const r of toolRows.value) {
    if (r.tool.trim()) rules[r.tool.trim()] = r.rule || 'ask'
  }
  s.tool_rules = rules
  for (const k of COMMA_KEYS) s[k] = splitList(s[k])
  s.mcp_servers = mcpServers.value
    .filter((x) => x && (x.type === 'http' ? x.url?.trim() : x.command?.trim()))
    .map((x) => {
      if (x.type === 'http') return { name: x.name, type: 'http', url: x.url }
      return { name: x.name, type: 'stdio', command: x.command, args: x.args || [] }
    })
  const dns = s.dns
  if (dns) {
    const mode = dns.mode === 'system' || dns.mode === 'command' ? dns.mode : 'custom'
    if (mode === 'custom') {
      const servers = (dns.servers || []).filter((x) => x && (x.server || '').trim())
      if (!servers.length) {
        Message.warning('自定义 DNS 至少保留一条服务器')
        throw new Error('自定义 DNS 至少保留一条服务器')
      }
      s.dns = {
        mode,
        servers: servers.map((x) => ({ mode: x.mode === 'doh' || x.mode === 'dot' ? x.mode : 'plain', server: x.server!.trim() })),
        concurrency: Math.max(1, Math.min(16, Number(dns.concurrency) || 0)) || undefined,
        timeout_ms: Math.max(500, Math.min(30000, Number(dns.timeout_ms) || 0)) || undefined,
      }
    } else {
      s.dns = { mode }
    }
  }
  // _newModel 与 __models 都是临时字段，不随设置持久化
  s.providers = (s.providers || []).map((p) => {
    const { _newModel: _a, ...rest } = p as ProviderRow
    return rest
  })
  const act = (s.providers || []).find((p) => p.provider === s.provider)
  if (act) {
    act.base_url = s.base_url || ''
    act.api_key = s.api_key || ''
    act.model = s.model || act.model || ''
  }
  return s
}

function save() {
  if (!state.settings) {
    Message.warning('设置尚未加载，请稍后重试')
    return
  }
  if (state.wsStatus !== 'connected') {
    Message.error('无法连接 licode 后端，设置未保存')
    return
  }
  let s: Settings
  try {
    s = buildSettings()
  } catch (e: any) {
    Message.error(e?.message || '保存失败')
    return
  }
  licode.saveSettings(s)
  Message.success('设置已保存')
}

const themeOptions = [
  { label: '浅色', value: 'light' },
  { label: '深色', value: 'dark' },
]
</script>

<template>
  <div class="flex h-full overflow-hidden">
    <!-- 左侧分区导航 -->
    <aside class="flex w-56 shrink-0 flex-col border-r border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
      <div class="flex items-center gap-2 border-b border-zinc-200 px-4 py-4 dark:border-zinc-800">
        <button class="rounded-lg p-1.5 text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800" title="返回对话" @click="navigateTo('/')">
          <ArrowLeft :size="16" />
        </button>
        <span class="text-base font-semibold">设置</span>
      </div>
      <nav class="flex-1 space-y-1 p-2">
        <button
          v-for="item in navItems"
          :key="item.id"
          class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors"
          :class="
            tab === item.id
              ? 'bg-zinc-100 font-medium text-zinc-900 dark:bg-zinc-800 dark:text-zinc-50'
              : 'text-zinc-600 hover:bg-zinc-50 dark:text-zinc-400 dark:hover:bg-zinc-800/60'
          "
          @click="tab = item.id"
        >
          <component :is="item.icon" :size="15" />
          {{ item.label }}
        </button>
      </nav>
      <div class="border-t border-zinc-200 p-3 dark:border-zinc-800">
        <Button
          variant="primary"
          class="w-full"
          :loading="saving"
          :disabled="!state.settings || state.wsStatus !== 'connected'"
          @click="save"
        >
          保存设置
        </Button>
      </div>
    </aside>

    <!-- 内容区 -->
    <main class="min-w-0 flex-1 overflow-y-auto">
      <div class="mx-auto max-w-3xl px-6 py-6">
        <!-- 设置未加载（WS 未就绪） -->
        <div v-if="!state.settings" class="flex flex-col items-center gap-3 pt-24 text-sm text-zinc-500">
          <Loader2 :size="20" class="animate-spin" />
          正在加载设置…请确认已连接 licode 后端
          <Button size="sm" variant="outline" @click="licode.connect()">重新连接</Button>
        </div>

        <!-- 基础 -->
        <template v-else-if="tab === 'basic'">
          <h2 class="mb-4 text-lg font-semibold">基础设置</h2>
          <div class="grid grid-cols-3 gap-3">
            <label class="space-y-1">
              <span class="text-xs text-zinc-500">温度（0-2，越大越随机）</span>
              <Input v-model="local.temperature" type="number" step="0.1" min="0" max="2" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-zinc-500">单次最大输出（token）</span>
              <Input v-model="local.max_tokens" type="number" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-zinc-500">最大迭代次数</span>
              <Input v-model="local.max_iterations" type="number" />
            </label>
          </div>

          <div class="mt-4 grid grid-cols-2 gap-2 rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
            <label class="flex items-center justify-between gap-2 text-sm">
              <span>流式输出</span>
              <Switch :model-value="!!local.streaming" size="sm" @update:model-value="local.streaming = !!$event" />
            </label>
            <label class="flex items-center justify-between gap-2 text-sm">
              <span>自动允许风险工具</span>
              <Switch :model-value="!!local.auto_allow" size="sm" @update:model-value="local.auto_allow = !!$event" />
            </label>
            <label class="flex items-center justify-between gap-2 text-sm">
              <span>子代理编排</span>
              <Switch :model-value="!!local.subagents" size="sm" @update:model-value="local.subagents = !!$event" />
            </label>
            <label class="flex items-center justify-between gap-2 text-sm">
              <span>上下文压缩</span>
              <Switch :model-value="!!local.compaction" size="sm" @update:model-value="local.compaction = !!$event" />
            </label>
            <label class="flex items-center justify-between gap-2 text-sm">
              <span>自动生成标题</span>
              <Switch :model-value="!!local.title_gen" size="sm" @update:model-value="local.title_gen = !!$event" />
            </label>
            <label class="flex items-center justify-between gap-2 text-sm">
              <span>语义缓存</span>
              <Switch :model-value="!!local.cache_enabled" size="sm" @update:model-value="local.cache_enabled = !!$event" />
            </label>
          </div>

          <div class="mt-4 rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
            <div class="mb-3 flex items-center justify-between">
              <span class="text-sm font-medium">DNS 解析方式</span>
              <span class="text-[10px] text-zinc-400">影响 AI 接口与 MCP 的域名解析</span>
            </div>
            <Select
              :model-value="local.dns?.mode || 'custom'"
              size="sm"
              class="w-full"
              :options="dnsModeOptions"
              @update:model-value="ensureDns(); local.dns!.mode = String($event)"
            />
            <p v-if="(local.dns?.mode || 'custom') === 'system'" class="mt-2 text-xs text-zinc-500">
              使用操作系统当前的 DNS 配置，最简单；若系统 DNS 被劫持或不可用，可改用其他方式。
            </p>
            <p v-else-if="local.dns?.mode === 'command'" class="mt-2 text-xs text-zinc-500">
              通过系统命令（getent → ping → nslookup）获取 IP，适合系统 DNS 异常但网络可通的场景。
            </p>
            <template v-else>
              <div class="mt-3 space-y-2">
                <div
                  v-for="(srv, i) in local.dns?.servers || []"
                  :key="i"
                  class="rounded-lg border border-zinc-200 p-2 dark:border-zinc-700"
                >
                  <div class="mb-1.5 flex items-center gap-2">
                    <span class="text-[10px] font-medium text-zinc-400">{{ i === 0 ? '主' : i === 1 ? '备' : `#${i + 1}` }}</span>
                    <Select
                      :model-value="srv.mode || 'plain'"
                      size="sm"
                      class="w-36"
                      :options="dnsProtocolOptions"
                      @update:model-value="srv.mode = String($event)"
                    />
                    <span class="flex-1" />
                    <Button size="sm" variant="ghost" danger :icon="Trash2" @click="removeDnsServer(i)" />
                  </div>
                  <Input
                    :model-value="srv.server || ''"
                    size="sm"
                    class="w-full"
                    placeholder="223.5.5.5:53 或 https://dns.alidns.com/dns-query"
                    @update:model-value="srv.server = String($event)"
                  />
                </div>
                <div class="flex items-center gap-2">
                  <Input
                    v-model="_newDnsServer"
                    size="sm"
                    class="flex-1"
                    placeholder="自定义服务器（回车添加，如 223.5.5.5:53）"
                    @keydown.enter.prevent="addDnsServer"
                  />
                  <Button size="sm" variant="outline" :icon="Plus" @click="addDnsServer">添加</Button>
                </div>
              </div>
              <div class="mt-3">
                <div class="mb-1 text-[10px] font-medium text-zinc-400">预设（点击一键添加）</div>
                <div class="flex flex-wrap gap-1">
                  <Button
                    v-for="p in dnsPresets"
                    :key="p.server"
                    size="sm"
                    variant="outline"
                    class="text-[11px]"
                    @click="addDnsPreset(p)"
                  >
                    {{ p.region }} · {{ p.name }}
                  </Button>
                </div>
              </div>
              <div class="mt-3 grid grid-cols-2 gap-2">
                <label class="space-y-1">
                  <span class="text-[10px] text-zinc-400">并发查询数（0=默认 2，主备同时查取最快）</span>
                  <Input
                    :model-value="local.dns?.concurrency ?? ''"
                    type="number" min="1" max="16" placeholder="2"
                    @update:model-value="ensureDns(); local.dns!.concurrency = Number($event) || 0"
                  />
                </label>
                <label class="space-y-1">
                  <span class="text-[10px] text-zinc-400">单次查询超时（毫秒，0=默认 5000）</span>
                  <Input
                    :model-value="local.dns?.timeout_ms ?? ''"
                    type="number" min="500" max="30000" placeholder="5000"
                    @update:model-value="ensureDns(); local.dns!.timeout_ms = Number($event) || 0"
                  />
                </label>
              </div>
              <p class="mt-2 text-[11px] text-zinc-400">
                普通 DNS 走 UDP/TCP 53 端口 · DNS over TLS 走 853 · DNS over HTTPS 走加密 HTTPS。
              </p>
            </template>
          </div>
        </template>

        <!-- 厂商 -->
        <template v-else-if="tab === 'providers'">
          <h2 class="mb-4 text-lg font-semibold">AI 厂商</h2>
          <div class="space-y-3">
            <div
              v-for="p in providers"
              :key="p.provider"
              class="rounded-xl border p-3"
              :class="
                p.provider === local.provider
                  ? 'border-zinc-900 dark:border-zinc-100'
                  : 'border-zinc-200 dark:border-zinc-800'
              "
            >
              <div class="mb-2 flex items-center gap-2">
                <CheckCircle2
                  v-if="p.provider === local.provider"
                  :size="15"
                  class="text-emerald-600 dark:text-emerald-400"
                />
                <span class="text-sm font-medium">{{ p.name || p.provider }}</span>
                <Chip size="sm" variant="outline">{{ p.type || 'openai' }}</Chip>
                <span class="flex-1" />
                <Button size="sm" variant="ghost" :icon="SettingsIcon" title="详细设置（指定 IP / 忽略 SSL 等）" @click="openDetail(p)" />
                <Button
                  v-if="p.provider !== local.provider"
                  size="sm"
                  variant="secondary"
                  @click="activate(p)"
                >
                  激活
                </Button>
                <Button size="sm" variant="ghost" danger :icon="Trash2" @click="removeProvider(p)" />
              </div>
              <div class="grid grid-cols-2 gap-2">
                <Input v-model="p.name" size="sm" placeholder="显示名称" />
                <Input v-model="p.base_url" size="sm" placeholder="API 地址" />
                <Input v-model="p.api_key" size="sm" type="password" placeholder="API 密钥" class="col-span-2" />
                <div class="col-span-2 space-y-2 pt-1">
                  <div class="text-xs text-zinc-500">模型</div>
                  <div class="flex flex-wrap gap-1">
                    <Chip
                      v-for="m in p.models || []"
                      :key="m"
                      size="sm"
                      variant="secondary"
                      class="group cursor-default"
                    >
                      {{ m }}
                      <button
                        class="ml-1 text-zinc-400 group-hover:text-red-500"
                        :title="`移除「${m}」`"
                        @click="removeModel(p, m)"
                      >
                        <X :size="11" />
                      </button>
                    </Chip>
                    <span v-if="!(p.models && p.models.length)" class="self-center text-xs text-zinc-400">暂未添加模型</span>
                  </div>
                  <div class="flex items-center gap-2">
                    <Input
                      v-model="p._newModel"
                      size="sm"
                      placeholder="模型名（回车或点「添加」，如 gpt-4o-mini）"
                      class="flex-1"
                      @keydown.enter.prevent="addModel(p)"
                    />
                    <Button size="sm" variant="outline" :icon="Plus" @click="addModel(p)">添加</Button>
                  </div>
                  <Select
                    v-if="modelOpts(p).length"
                    :model-value="p.model"
                    size="sm"
                    searchable
                    :options="modelOpts(p)"
                    placeholder="当前模型"
                    @update:model-value="p.model = String($event)"
                  />
                  <Input v-else v-model="p.model" size="sm" placeholder="当前模型（如 gpt-4o-mini）" />
                </div>
              </div>
            </div>
          </div>

          <div class="mt-4 rounded-xl border border-dashed border-zinc-300 p-4 dark:border-zinc-700">
            <div class="mb-2 text-sm font-medium text-zinc-600 dark:text-zinc-400">新增厂商</div>
            <div class="grid grid-cols-2 gap-2">
              <Input v-model="newProvider.name" size="sm" placeholder="名称（如 我的 Ollama）" />
              <Select
                :model-value="newProvider.type"
                size="sm"
                :options="typeOptions"
                @update:model-value="newProvider.type = String($event)"
              />
              <Input v-model="newProvider.base_url" size="sm" placeholder="API 地址" />
              <Input v-model="newProvider.api_key" size="sm" type="password" placeholder="API 密钥" />
              <Input v-model="newProvider.model" size="sm" placeholder="模型名（可留空）" class="col-span-2" />
            </div>
            <Button size="sm" variant="secondary" class="mt-3 w-full" :icon="Plus" @click="addProvider">
              添加厂商
            </Button>
          </div>

          <!-- 厂商详细设置小弹窗 -->
          <Teleport to="body">
            <div
              v-if="detailOpen && detailProvider"
              class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
              @click.self="detailOpen = false"
            >
              <div class="w-full max-w-md space-y-3 rounded-2xl border border-zinc-200 bg-white p-4 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900">
                <div class="flex items-center justify-between">
                  <span class="text-sm font-semibold">详细设置 · {{ detailProvider.name || detailProvider.provider }}</span>
                  <Button variant="ghost" size="sm" :icon="X" @click="detailOpen = false" />
                </div>
                <label class="block space-y-1">
                  <span class="text-xs text-zinc-500">协议类型</span>
                  <Select
                    :model-value="detailProvider.type || 'openai'"
                    size="sm"
                    :options="typeOptions"
                    @update:model-value="detailProvider.type = String($event)"
                  />
                </label>
                <label class="block space-y-1">
                  <span class="text-xs text-zinc-500">指定 IP（可选，绕过 DNS 劫持/污染）</span>
                  <Input v-model="detailProvider.host_ip" placeholder="如 104.18.7.10；SNI/证书校验仍用原域名" />
                  <span class="block text-[10px] text-zinc-400">请求 API 域名时直接连此 IP，不再查询 DNS；TLS 证书按原域名正常校验</span>
                </label>
                <label class="flex items-center justify-between gap-2 rounded-lg border border-zinc-200 p-2.5 text-sm dark:border-zinc-700">
                  <span>
                    忽略 SSL 证书校验
                    <span class="block text-[10px] text-zinc-400">仅自签名证书等受控场景使用；配合指定 IP 可访问内网/自建网关</span>
                  </span>
                  <Switch :model-value="!!detailProvider.insecure_ssl" size="sm" @update:model-value="detailProvider.insecure_ssl = !!$event" />
                </label>
                <div class="flex items-center gap-2">
                  <Button size="sm" variant="outline" :icon="RefreshCw" :loading="fetching === detailProvider.provider" @click="fetchModels(detailProvider)">
                    获取模型
                  </Button>
                  <span class="flex-1" />
                  <Button size="sm" variant="primary" @click="detailOpen = false">完成</Button>
                </div>
              </div>
            </div>
          </Teleport>
        </template>

        <!-- 高级 -->
        <template v-else-if="tab === 'advanced'">
          <h2 class="mb-4 text-lg font-semibold">高级设置</h2>
          <div class="grid grid-cols-2 gap-3">
            <label class="space-y-1">
              <span class="text-xs text-zinc-500">Shell 路径（自动检测）</span>
              <Select
                :options="shellOptions"
                :model-value="local.shell_path || ''"
                size="sm"
                searchable
                clearable
                placeholder="选择已检测到的 Shell"
                @update:model-value="local.shell_path = String($event || '')"
              />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-zinc-500">沙箱镜像</span>
              <Input v-model="local.sandbox_image" placeholder="alpine" />
            </label>
            <label v-for="k in ['retry_max', 'sub_timeout', 'max_ctx_tokens', 'cache_ttl', 'tool_retry_max', 'shutdown_timeout']" :key="k" class="space-y-1">
              <span class="text-xs text-zinc-500">{{ fieldLabels[k] || k }}</span>
              <Input v-model="local[k]" type="number" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-zinc-500">{{ fieldLabels.rag_source }}</span>
              <Input v-model="local.rag_source" placeholder="留空关闭" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-zinc-500">{{ fieldLabels.ask_tools }}</span>
              <Input :model-value="(local.ask_tools || []).join(',')" @update:model-value="local.ask_tools = splitList($event)" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-zinc-500">{{ fieldLabels.deny_tools }}</span>
              <Input :model-value="(local.deny_tools || []).join(',')" @update:model-value="local.deny_tools = splitList($event)" />
            </label>
          </div>

          <div class="mt-4 grid grid-cols-2 gap-2 rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
            <label class="flex items-center justify-between gap-2 text-sm">
              <span>{{ fieldLabels.redact_secrets }}</span>
              <Switch :model-value="!!local.redact_secrets" size="sm" @update:model-value="local.redact_secrets = !!$event" />
            </label>
            <label class="flex items-center justify-between gap-2 text-sm">
              <span>{{ fieldLabels.sandbox }}</span>
              <Switch :model-value="!!local.sandbox" size="sm" @update:model-value="local.sandbox = !!$event" />
            </label>
            <label class="flex items-center justify-between gap-2 text-sm">
              <span>{{ fieldLabels.tool_auto_retry }}</span>
              <Switch :model-value="!!local.tool_auto_retry" size="sm" @update:model-value="local.tool_auto_retry = !!$event" />
            </label>
            <label class="flex items-center justify-between gap-2 text-sm">
              <span>{{ fieldLabels.rag_enabled }}</span>
              <Switch :model-value="!!local.rag_enabled" size="sm" @update:model-value="local.rag_enabled = !!$event" />
            </label>
          </div>

          <div class="mt-4 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium">工具权限规则</span>
              <span class="flex items-center gap-1 text-[10px] text-zinc-400">
                <Info :size="11" /> 更完整的工具管理请到「工具」页面
                <Button size="sm" variant="ghost" @click="navigateTo('/tools')">打开工具页面</Button>
              </span>
            </div>
            <div v-for="(r, i) in toolRows" :key="i" class="flex items-center gap-2">
              <Input v-model="r.tool" size="sm" placeholder="工具名（如 Shell / mcp__git__git_status）" class="flex-1" />
              <Select
                :model-value="r.rule"
                size="sm"
                :options="ruleOptions"
                class="w-24"
                @update:model-value="r.rule = String($event)"
              />
              <Button size="sm" variant="ghost" danger :icon="Trash2" @click="toolRows.splice(i, 1)" />
            </div>
            <Button size="sm" variant="outline" :icon="Plus" @click="toolRows.push({ tool: '', rule: 'ask' })">
              添加规则
            </Button>
          </div>

          <!-- 用户自定义 CA 证书 -->
          <div class="mt-4 rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
            <div class="mb-2 flex items-center justify-between">
              <span class="text-sm font-medium">自定义 CA 证书（TLS 额外信任）</span>
              <span class="text-[10px] text-zinc-400">与内置权威 CA 分开 · 即传即生效</span>
            </div>
            <div v-if="caList.length" class="mb-2 space-y-1">
              <div
                v-for="c in caList"
                :key="c.name"
                class="flex items-center gap-2 rounded-lg border border-zinc-200 px-2 py-1.5 dark:border-zinc-700"
              >
                <CheckCircle2 v-if="c.valid" :size="13" class="shrink-0 text-emerald-500" />
                <X v-else :size="13" class="shrink-0 text-red-400" />
                <span class="max-w-40 truncate font-mono text-xs">{{ c.name }}</span>
                <span class="min-w-0 flex-1 truncate text-[10px] text-zinc-400">
                  {{ c.valid ? c.subjects.join('；') : '无效证书' }}
                </span>
                <Button size="sm" variant="ghost" danger :icon="Trash2" @click="deleteCA(c.name)" />
              </div>
            </div>
            <p v-else class="mb-2 text-[10px] text-zinc-400">暂无自定义 CA（自签名/私有网关的根证书可上传到此处）</p>
            <input ref="caInput" type="file" accept=".pem,.crt,.cer" class="hidden" @change="onCAChange" />
            <Button size="sm" variant="outline" :icon="Plus" @click="caInput?.click()">上传 CA 证书</Button>
            <p class="mt-1 text-[10px] text-zinc-400">
              保存于 {{ caDir }} · AI 接口与 MCP 请求在内置权威 CA 之外额外信任这些证书
            </p>
          </div>
        </template>

        <!-- MCP -->
        <template v-else-if="tab === 'mcp'">
          <h2 class="mb-4 text-lg font-semibold">MCP 工具</h2>
          <div class="rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
            <div class="mb-2 text-sm font-medium text-zinc-600 dark:text-zinc-400">内置预设（点击添加）</div>
            <div class="flex flex-wrap gap-2">
              <Button
                v-for="(_, name) in mcpPresets"
                :key="name"
                size="sm"
                variant="outline"
                :icon="Plus"
                :disabled="isMcpAdded(name)"
                @click="addMcpPreset(name)"
              >
                {{ name }}
              </Button>
            </div>
          </div>

          <div class="mt-4 rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
            <div class="mb-2 flex items-center gap-2">
              <span class="flex-1 text-sm font-medium text-zinc-600 dark:text-zinc-400">已添加的 MCP 服务器</span>
              <Select v-model="newMcpType" :options="mcpTypeOptions" size="sm" class="w-64" />
              <Button size="sm" variant="outline" :icon="Plus" @click="addMcpTyped">添加</Button>
            </div>
            <div v-if="!mcpServers.length" class="text-xs text-zinc-400">暂未添加 MCP 服务器</div>
            <div
              v-for="(srv, i) in mcpServers"
              :key="i"
              class="mb-2 rounded-lg border border-zinc-200 p-2.5 dark:border-zinc-800"
            >
              <div class="mb-1.5 flex items-center gap-2">
                <Chip size="sm" variant="secondary">{{ srv.type === 'http' ? '远程服务' : '本地命令' }}</Chip>
                <Input v-model="srv.name" size="sm" placeholder="服务器名称" class="w-40" />
                <span class="flex-1" />
                <Button size="sm" variant="ghost" danger :icon="Trash2" @click="mcpServers.splice(i, 1)" />
              </div>
              <div v-if="srv.type === 'http'" class="grid grid-cols-1 gap-1">
                <Input v-model="srv.url" size="sm" placeholder="服务地址（https://...）" />
              </div>
              <div v-else class="grid grid-cols-2 gap-1">
                <Input v-model="srv.command" size="sm" placeholder="启动命令（如 python / uvx / ./server）" class="col-span-2" />
                <Input :model-value="(srv.args || []).join(' ')" size="sm" placeholder="参数（空格分隔，如 -m mcp_server_git）" class="col-span-2" @update:model-value="srv.args = String($event).split(' ')" />
              </div>
            </div>
          </div>
          <p class="mt-3 text-xs text-zinc-400">
            保存后，MCP 工具会以「工具」页面可管理；工具名格式为 mcp__服务器名__工具名。
          </p>
        </template>

        <!-- 外观 -->
        <template v-else-if="tab === 'appearance'">
          <h2 class="mb-4 text-lg font-semibold">外观</h2>
          <div class="rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
            <div class="mb-2 text-sm font-medium">明暗模式</div>
            <div class="flex gap-2">
              <button
                v-for="opt in themeOptions"
                :key="opt.value"
                class="flex flex-1 items-center justify-center gap-2 rounded-lg border p-3 text-sm transition-colors"
                :class="
                  themeMode === opt.value
                    ? 'border-zinc-900 bg-zinc-100 dark:border-zinc-100 dark:bg-zinc-800'
                    : 'border-zinc-200 hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800/60'
                "
                @click="setMode(opt.value as any)"
              >
                <Sun v-if="opt.value === 'light'" :size="15" />
                <Moon v-else :size="15" />
                {{ opt.label }}
              </button>
            </div>
          </div>

          <div class="mt-4 rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
            <div class="mb-2 text-sm font-medium">界面皮肤</div>
            <div class="flex gap-2">
              <button
                class="flex flex-1 flex-col items-start gap-2 rounded-lg border p-3 text-sm transition-colors"
                :class="
                  skin === 'default'
                    ? 'border-zinc-900 dark:border-zinc-100'
                    : 'border-zinc-200 hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800/60'
                "
                @click="setSkin('default')"
              >
                <span class="h-10 w-full rounded-md border border-zinc-200 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-800" />
                默认
              </button>
              <button
                class="flex flex-1 flex-col items-start gap-2 rounded-lg border p-3 text-sm transition-colors"
                :class="
                  skin === 'glass'
                    ? 'border-zinc-900 dark:border-zinc-100'
                    : 'border-zinc-200 hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800/60'
                "
                @click="setSkin('glass')"
              >
                <span class="h-10 w-full rounded-md border border-indigo-200 bg-gradient-to-br from-indigo-200 via-pink-100 to-sky-200 dark:border-indigo-900 dark:from-indigo-900 dark:via-fuchsia-900 dark:to-sky-900" />
                液态玻璃
              </button>
            </div>
            <p class="mt-2 text-xs text-zinc-500">液态玻璃：半透明毛玻璃面板 + 渐变背景，立即生效并记住选择。</p>
          </div>
        </template>
      </div>
    </main>
  </div>
</template>
