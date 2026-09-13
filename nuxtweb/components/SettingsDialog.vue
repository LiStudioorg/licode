<script setup lang="ts">
import { X, Plus, Trash2, RefreshCw, CheckCircle2, Loader2, Settings as SettingsIcon } from 'lucide-vue-next'
import { Message, Button, Input, Switch, Chip, Select, Tabs } from 'fuxsto-design'
import type { DNSConfig, ProviderConfig, Settings } from '~/composables/useLicode'

const dnsModeOptions = [
  { label: '普通 DNS (UDP)', value: 'plain' },
  { label: 'DoT (TLS)', value: 'dot' },
  { label: 'DoH (HTTPS)', value: 'doh' },
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

const tab = ref<'basic' | 'providers' | 'advanced' | 'mcp'>('basic')
const local = ref<Settings>({})
const toolRows = ref<{ tool: string; rule: string }[]>([])
const mcpJson = ref('')
const mcpServers = ref<any[]>([])
const fetching = ref('')
const saving = ref(false)
const shellOptions = ref<{ label: string; value: string }[]>([])

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
  { label: '本地命令 (stdio，如 node/npx/python)', value: 'stdio' },
  { label: '远程网页 (http/https)', value: 'http' },
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
  retry_max: 'LLM 重试次数',
  sub_timeout: '子代理超时（秒）',
  max_ctx_tokens: '上下文窗口（token）',
  cache_ttl: '缓存有效期（秒）',
  tool_retry_max: '工具重试次数',
  shutdown_timeout: '关停等待（秒）',
  rag_source: 'RAG 来源（留空关闭）',
  ask_tools: 'ask 工具（逗号分隔）',
  deny_tools: 'deny 工具（逗号分隔）',
  redact_secrets: '敏感信息脱敏',
  sandbox: '沙箱执行（Docker）',
  tool_auto_retry: '工具自动重试',
  rag_enabled: '启用 RAG',
}

const typeOptions = [
  { label: 'OpenAI 兼容', value: 'openai' },
  { label: 'Claude', value: 'claude' },
  { label: 'Ollama', value: 'ollama' },
  { label: 'Gemini', value: 'gemini' },
]

const ruleOptions = [
  { label: 'allow', value: 'allow' },
  { label: 'ask', value: 'ask' },
  { label: 'deny', value: 'deny' },
]

const newProvider = ref<ProviderRow>({ provider: '', name: '', type: 'openai', base_url: '', api_key: '', model: '', models: [] })

// 厂商详细设置小弹窗：指定 IP / 忽略 SSL / 协议类型 / 获取模型等集中配置。
const detailOpen = ref(false)
const detailProvider = ref<ProviderRow | null>(null)

function openDetail(p: ProviderRow) {
  detailProvider.value = p
  detailOpen.value = true
}

watch(
  () => state.settingsOpen,
  (v) => {
    if (v && state.settings) {
      loadShells()
      local.value = JSON.parse(JSON.stringify(state.settings))
      if (local.value.streaming === null || local.value.streaming === undefined) local.value.streaming = true
      toolRows.value = Object.entries(local.value.tool_rules || {}).map(([tool, rule]) => ({
        tool,
        rule: String(rule),
      }))
      mcpJson.value = JSON.stringify(local.value.mcp_servers || [], null, 2)
      try {
        mcpServers.value = local.value.mcp_servers ? JSON.parse(JSON.stringify(local.value.mcp_servers)) : []
      } catch {
        mcpServers.value = []
      }
      tab.value = 'basic'
    }
  },
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
  const rawMcp = mcpServers.value
    .filter((s) => s && (s.type === 'http' ? s.url?.trim() : s.command?.trim()))
    .map((s) => {
      if (s.type === 'http') return { name: s.name, type: 'http', url: s.url }
      return { name: s.name, type: 'stdio', command: s.command, args: s.args || [] }
    })
  s.mcp_servers = rawMcp
  mcpJson.value = JSON.stringify(rawMcp, null, 2)
  const dns = s.dns
  if (dns) {
    const servers = (dns.servers || []).filter((x) => x && (x.server || '').trim())
    if (servers.length) {
      s.dns = {
        servers: servers.map((x) => ({ mode: x.mode === 'doh' || x.mode === 'dot' ? x.mode : 'plain', server: x.server!.trim() })),
        concurrency: Math.max(1, Math.min(16, Number(dns.concurrency) || 0)) || undefined,
        timeout_ms: Math.max(500, Math.min(30000, Number(dns.timeout_ms) || 0)) || undefined,
      }
    } else {
      // 不允许清空 DNS：至少保留一条，避免退回系统解析
      s.dns = { servers: [{ mode: 'plain', server: '223.5.5.5:53' }] }
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
  state.settingsOpen = false
  Message.success('设置已保存')
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="state.settingsOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      @click.self="state.settingsOpen = false"
    >
      <div
        class="flex h-[82vh] w-full max-w-2xl flex-col overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-2xl dark:border-zinc-800 dark:bg-zinc-900"
      >
        <div class="flex shrink-0 items-center justify-between border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
          <span class="text-sm font-semibold">设置</span>
          <Button variant="ghost" size="sm" :icon="X" @click="state.settingsOpen = false" />
        </div>

        <div class="shrink-0 border-b border-zinc-200 px-4 pt-2 dark:border-zinc-800">
          <Tabs
            :model-value="tab"
            :options="[
              { label: '基础', value: 'basic' },
              { label: '厂商', value: 'providers' },
              { label: '高级', value: 'advanced' },
              { label: 'MCP', value: 'mcp' },
            ]"
            variant="line"
            @update:model-value="tab = $event as any"
          />
        </div>

        <div class="min-h-0 flex-1 space-y-4 overflow-y-auto p-4">
          <!-- 设置未加载（WS 未就绪） -->
          <div v-if="!state.settings" class="flex flex-col items-center gap-3 pt-16 text-sm text-zinc-500">
            <Loader2 :size="20" class="animate-spin" />
            正在加载设置…请确认已连接 licode 后端
            <Button size="sm" variant="outline" @click="licode.connect()">重新连接</Button>
          </div>

          <!-- 基础 -->
          <template v-else-if="tab === 'basic'">
            <div class="grid grid-cols-2 gap-3">
              <label class="space-y-1">
                <span class="text-xs text-zinc-500">温度 temperature</span>
                <Input v-model="local.temperature" type="number" step="0.1" min="0" max="2" />
              </label>
              <label class="space-y-1">
                <span class="text-xs text-zinc-500">最大输出 tokens</span>
                <Input v-model="local.max_tokens" type="number" />
              </label>
              <label class="space-y-1">
                <span class="text-xs text-zinc-500">最大迭代次数</span>
                <Input v-model="local.max_iterations" type="number" />
              </label>
            </div>
            <div class="grid grid-cols-2 gap-2 rounded-xl border border-zinc-200 p-3 dark:border-zinc-800">
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
                        <div class="rounded-xl border border-zinc-200 p-3 dark:border-zinc-800">
              <div class="mb-2 flex items-center justify-between">
                <span class="text-xs font-medium text-zinc-500">DNS 解析（不使用系统 DNS，仅用以下服务器）</span>
                <span class="text-[10px] text-zinc-400">默认阿里主 + 腾讯备 · 并发取最快</span>
              </div>
              <div class="space-y-2">
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
                      :options="dnsModeOptions"
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
              <div class="mt-2">
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
              <div class="mt-2 grid grid-cols-2 gap-2">
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
              <p class="mt-1 text-[10px] text-zinc-400">
                plain 普通 DNS (UDP/TCP 53) · dot DNS over TLS (853) · doh DNS over HTTPS ·
                不使用系统 DNS，API/模型请求域名均按以上服务器解析
              </p>
            </div>
          </template>

          <!-- 厂商 -->
          <template v-else-if="tab === 'providers'">
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

            <div class="rounded-xl border border-dashed border-zinc-300 p-3 dark:border-zinc-700">
              <div class="mb-2 text-xs font-medium text-zinc-500">新增厂商</div>
              <div class="grid grid-cols-2 gap-2">
                <Input v-model="newProvider.name" size="sm" placeholder="名称（如 我的 Ollama）" />
                <Select
                  :model-value="newProvider.type"
                  size="sm"
                  :options="typeOptions"
                  @update:model-value="newProvider.type = String($event)"
                />                <Input v-model="newProvider.base_url" size="sm" placeholder="API 地址" />
                <Input v-model="newProvider.api_key" size="sm" type="password" placeholder="API 密钥" />
                <Input v-model="newProvider.model" size="sm" placeholder="模型名（可留空）" class="col-span-2" />
              </div>
              <Button size="sm" variant="secondary" class="mt-2 w-full" :icon="Plus" @click="addProvider">
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
                    <span class="block text-[10px] text-zinc-400">请求 base_url 域名时直接连此 IP，不再查询 DNS；TLS 证书按原域名正常校验，不会被引到假服务器</span>
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
            <div class="grid grid-cols-2 gap-3">
              <label class="space-y-1">
                <span class="text-xs text-zinc-500">Shell 路径（自动检测）</span>
                <Select
                  :options="shellOptions"
                  :model-value="local.shell_path || ''"
                  size="sm"
                  searchable
                  clearable
                  placeholder="选择已检测到的 shell"
                  @update:model-value="local.shell_path = String($event || '')"
                />
                <span class="block text-[10px] text-zinc-400">如需其他路径，可在「高级 → 工具规则」旁手动编辑配置文件 shell_path 字段</span>
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

            <div class="grid grid-cols-2 gap-2 rounded-xl border border-zinc-200 p-3 dark:border-zinc-800">
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

            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-xs font-medium text-zinc-500">工具规则（tool_rules）</span>
                <Button size="sm" variant="ghost" :icon="Plus" @click="toolRows.push({ tool: '', rule: 'ask' })">
                  添加
                </Button>
              </div>
              <div v-for="(r, i) in toolRows" :key="i" class="flex items-center gap-2">
                <Input v-model="r.tool" size="sm" placeholder="工具名" class="flex-1" />
                <Select
                  :model-value="r.rule"
                  size="sm"
                  :options="ruleOptions"
                  class="w-28"
                  @update:model-value="r.rule = String($event)"
                />
                <Button size="sm" variant="ghost" danger :icon="Trash2" @click="toolRows.splice(i, 1)" />
              </div>
            </div>

            <label class="block space-y-1">
              <span class="text-xs text-zinc-500">MCP 服务器（JSON 数组）</span>
              <textarea
                v-model="mcpJson"
                rows="5"
                class="w-full rounded-lg border border-zinc-200 bg-transparent p-2 font-mono text-xs outline-none focus:border-zinc-400 dark:border-zinc-800 dark:focus:border-zinc-600"
                spellcheck="false"
              />
            </label>
          </template>

          <!-- MCP -->
          <template v-if="tab === 'mcp'">
            <div class="space-y-3 p-4 text-sm">
              <div class="rounded-xl border border-zinc-200 p-3 dark:border-zinc-800">
                <div class="mb-2 text-xs font-medium text-zinc-500">内置预设（点击添加）</div>
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

              <div class="rounded-xl border border-zinc-200 p-3 dark:border-zinc-800">
                <div class="mb-2 flex items-center gap-2">
                  <span class="flex-1 text-xs font-medium text-zinc-500">已添加的 MCP 服务器</span>
                  <Select v-model="newMcpType" :options="mcpTypeOptions" size="sm" class="w-56" />
                  <Button size="sm" variant="outline" :icon="Plus" @click="addMcpTyped">添加</Button>
                </div>
                <div v-if="!mcpServers.length" class="text-xs text-zinc-400">暂未添加 MCP 服务器</div>
                <div
                  v-for="(srv, i) in mcpServers"
                  :key="i"
                  class="mb-2 rounded-lg border border-zinc-200 p-2 dark:border-zinc-800"
                >
                  <div class="mb-1 flex items-center gap-2">
                    <Chip size="sm" variant="secondary">{{ srv.type || 'stdio' }}</Chip>
                    <span class="flex-1 truncate text-xs font-medium">{{ srv.name }}</span>
                    <Button size="sm" variant="ghost" danger :icon="Trash2" @click="mcpServers.splice(i, 1)" />
                  </div>
                  <div v-if="srv.type === 'http'" class="grid grid-cols-1 gap-1">
                    <Input v-model="srv.url" size="sm" placeholder="URL (https://...)" />
                  </div>
                  <div v-else class="grid grid-cols-2 gap-1">
                    <Input v-model="srv.command" size="sm" placeholder="命令（如 node / npx / python / ./server）" class="col-span-2" />
                    <Input :model-value="(srv.args || []).join(' ')" size="sm" placeholder="参数（空格分隔，如 -y @modelcontextprotocol/server-memory）" class="col-span-2" @update:model-value="srv.args = String($event).split(' ')" />
                  </div>
                </div>
              </div>

              <details class="rounded-xl border border-zinc-200 p-3 dark:border-zinc-800">
                <summary class="cursor-pointer text-xs text-zinc-500">高级：JSON 编辑</summary>
                <label class="mt-2 block space-y-1">
                  <textarea
                    v-model="mcpJson"
                    rows="6"
                    class="w-full rounded-lg border border-zinc-200 bg-transparent p-2 font-mono text-xs outline-none focus:border-zinc-400 dark:border-zinc-800 dark:focus:border-zinc-600"
                    spellcheck="false"
                  />
                </label>
              </details>
            </div>
          </template>
        </div>

        <div class="flex shrink-0 items-center justify-end gap-2 border-t border-zinc-200 px-4 py-3 dark:border-zinc-800">
          <Button variant="ghost" @click="state.settingsOpen = false">取消</Button>
          <Button variant="primary" :loading="saving" :disabled="!state.settings || state.wsStatus !== 'connected'" @click="save">保存</Button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
