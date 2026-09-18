<script setup lang="ts">
import {
  X, Plus, Trash2, RefreshCw, CheckCircle2, Loader2, Settings as SettingsIcon,
  ArrowLeft, SlidersHorizontal, Server, Wrench, Blocks, Palette, Sun, Moon, ChevronRight,
} from 'lucide-vue-next'
import { Message, Button, Input, Switch, Chip, Select, Dialog } from 'fuxsto-design'
import type { ProviderConfig, Settings } from '~/composables/useLicode'
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
  { name: '阿里 DoH', region: '国内', mode: 'doh', server: 'https://dns.alidns.com/dns-query' },
  { name: '阿里 UDP', region: '国内', mode: 'plain', server: '223.5.5.5:53' },
  { name: '阿里 UDP 2', region: '国内', mode: 'plain', server: '223.6.6.6:53' },
  { name: '阿里 DoT', region: '国内', mode: 'dot', server: '223.5.5.5:853' },
  { name: '腾讯 DoH', region: '国内', mode: 'doh', server: 'https://doh.pub/dns-query' },
  { name: '腾讯 UDP', region: '国内', mode: 'plain', server: '119.28.28.28:53' },
  { name: '腾讯 DoT', region: '国内', mode: 'dot', server: '119.29.29.29:853' },
  { name: 'OneDNS DoH', region: '国内', mode: 'doh', server: 'https://doh.onedns.net/dns-query' },
  { name: 'OneDNS DoT', region: '国内', mode: 'dot', server: '1.2.4.8:853' },
  { name: '360 UDP', region: '国内', mode: 'plain', server: '101.226.4.6:53' },
  { name: '360 UDP 2', region: '国内', mode: 'plain', server: '218.30.118.6:53' },
  { name: '114 UDP', region: '国内', mode: 'plain', server: '114.114.114.114:53' },
  { name: '114 UDP 2', region: '国内', mode: 'plain', server: '114.114.115.115:53' },
  { name: '百度 DoH', region: '国内', mode: 'doh', server: 'https://doh.baidu.com/dns-query' },
  { name: 'CNNIC DoH', region: '国内', mode: 'doh', server: 'https://doh.cnnic.cn/dns-query' },
  { name: 'CF DoH', region: '国外', mode: 'doh', server: 'https://1.1.1.1/dns-query' },
  { name: 'CF UDP', region: '国外', mode: 'plain', server: '1.0.0.1:53' },
  { name: 'CF DoT', region: '国外', mode: 'dot', server: '1.1.1.1:853' },
  { name: 'CF 安全 DoH', region: '国外', mode: 'doh', server: 'https://security.cloudflare-dns.com/dns-query' },
  { name: 'Google DoH', region: '国外', mode: 'doh', server: 'https://dns.google/dns-query' },
  { name: 'Google DoT', region: '国外', mode: 'dot', server: '8.8.8.8:853' },
  { name: 'Google UDP', region: '国外', mode: 'plain', server: '8.8.4.4:53' },
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
const {
  mode: themeMode, skin, glassLevel, bgStyle, radius, anim,
  setSkin, setMode, setGlassLevel, setBgStyle, setRadius, setAnim, initTheme,
} = useTheme()

const tab = ref<'basic' | 'providers' | 'advanced' | 'mcp' | 'appearance'>('basic')
const local = ref<Settings>({})
const mcpServers = ref<any[]>([])
const fetching = ref('')
const saving = ref(false)
const shells = ref<string[]>([])

const navItems = [
  { id: 'basic', label: '基础', icon: SlidersHorizontal },
  { id: 'providers', label: 'AI 厂商', icon: Server },
  { id: 'advanced', label: '高级', icon: Wrench },
  { id: 'mcp', label: 'MCP 连接', icon: Blocks },
  { id: 'appearance', label: '外观', icon: Palette },
] as const

async function loadShells() {
  try {
    shells.value = (await useApi<string[]>('/api/shells')) || []
  } catch {}
}

// MCP 只支持网络服务（http/https）。
function addMcpServer() {
  mcpServers.value.push({ name: '', type: 'http', url: '' })
}

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

const fieldLabels: Record<string, string> = {
  retry_max: '调用失败重试次数',
  sub_timeout: '子代理超时（秒）',
  max_ctx_tokens: '上下文窗口上限（token）',
  cache_ttl: '缓存有效期（秒）',
  tool_retry_max: '工具重试次数',
  shutdown_timeout: '关停等待时间（秒）',
  rag_source: 'RAG 索引目录（留空关闭）',
  redact_secrets: '敏感信息脱敏',
  tool_auto_retry: '工具自动重试',
  rag_enabled: '启用 RAG 项目检索',
}

const typeOptions = [
  { label: 'OpenAI 兼容', value: 'openai' },
  { label: 'Claude', value: 'claude' },
  { label: 'Ollama', value: 'ollama' },
  { label: 'Gemini', value: 'gemini' },
]

function emptyProvider(): ProviderRow {
  return { provider: '', name: '', type: 'openai', base_url: '', api_key: '', model: '', models: [] }
}

// 厂商：列表为小卡片，点击弹出大卡片（左接口设置 / 右模型管理）。
const editOpen = ref(false)
const editNew = ref(false)
const editKey = ref('')
const editP = ref<ProviderRow>(emptyProvider())

function openProvider(p?: ProviderRow) {
  editNew.value = !p
  editKey.value = p?.provider || ''
  editP.value = p ? JSON.parse(JSON.stringify(p)) : emptyProvider()
  editOpen.value = true
}

function commitProvider() {
  const p = editP.value
  const id = (p.provider || p.name || p.type || 'custom').trim().toLowerCase().replace(/\s+/g, '-')
  if (!id) {
    Message.warning('请填写厂商名称')
    return
  }
  const list = providers.value.slice()
  if (editNew.value) {
    if (list.some((x) => x.provider === id)) {
      Message.error('厂商标识已存在')
      return
    }
    p.provider = id
    list.push(p)
    Message.success('厂商已添加，点击保存生效')
  } else {
    const i = list.findIndex((x) => x.provider === editKey.value)
    if (i < 0) return
    p.provider = id
    list[i] = p
    if (local.value.provider === editKey.value) {
      local.value.provider = id
      local.value.base_url = p.base_url || ''
      local.value.api_key = p.api_key || ''
      if (p.model) local.value.model = p.model
    }
    Message.success('厂商已更新，点击保存生效')
  }
  local.value.providers = list
  editOpen.value = false
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
  mcpServers.value = (local.value.mcp_servers || [])
    .filter((s: any) => s && (s.type === 'http' || s.url))
    .map((s: any) => ({ name: s.name || '', type: 'http', url: s.url || '' }))
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

function removeProvider(p: ProviderRow) {
  local.value.providers = providers.value.filter((x) => x.provider !== p.provider)
  if (local.value.provider === p.provider) {
    const next = providers.value[0]
    local.value.provider = next?.provider || ''
  }
  Message.success('厂商已移除，点击保存生效')
}

function buildSettings(): Settings {
  const s = JSON.parse(JSON.stringify(local.value)) as Settings
  for (const k of NUM_KEYS) s[k] = Number(s[k]) || 0
  s.streaming = !!s.streaming
  // 工具权限由「工具管理」页维护：完整保留原值，不做表单重建。
  s.mcp_servers = mcpServers.value
    .filter((x) => x && (x.url || '').trim())
    .map((x) => ({ name: (x.name || '').trim(), type: 'http', url: x.url.trim() }))
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
  // _newModel 是临时字段，不随设置持久化
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
const skinOptions = [
  { label: '默认', value: 'default' },
  { label: '液态玻璃', value: 'glass' },
]
const glassOptions = [
  { label: '轻柔', value: 'soft' },
  { label: '标准', value: 'medium' },
  { label: '浓厚', value: 'strong' },
]
const bgOptions = [
  { label: '极光渐变', value: 'gradient' },
  { label: '纯色', value: 'plain' },
]
const radiusOptions = [
  { label: '默认', value: 'normal' },
  { label: '大圆角', value: 'large' },
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
        <button
          class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-zinc-600 transition-colors hover:bg-zinc-50 dark:text-zinc-400 dark:hover:bg-zinc-800/60"
          @click="navigateTo('/tools')"
        >
          <Wrench :size="15" />
          工具管理
          <ChevronRight :size="14" class="ml-auto text-zinc-400" />
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

        <!-- 厂商：小卡片列表 + 大卡片编辑 -->
        <template v-else-if="tab === 'providers'">
          <div class="mb-4 flex items-center justify-between">
            <h2 class="text-lg font-semibold">AI 厂商</h2>
            <Button size="sm" variant="outline" :icon="Plus" @click="openProvider()">添加厂商</Button>
          </div>
          <div v-if="!providers.length" class="rounded-xl border border-dashed border-zinc-300 p-8 text-center text-sm text-zinc-400 dark:border-zinc-700">
            还没有厂商配置，点击右上角「添加厂商」开始
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <button
              v-for="p in providers"
              :key="p.provider"
              class="rounded-xl border p-3 text-left transition-colors hover:border-zinc-400 dark:hover:border-zinc-500"
              :class="
                p.provider === local.provider
                  ? 'border-zinc-900 dark:border-zinc-100'
                  : 'border-zinc-200 dark:border-zinc-800'
              "
              @click="openProvider(p)"
            >
              <div class="flex items-center gap-2">
                <CheckCircle2
                  v-if="p.provider === local.provider"
                  :size="14"
                  class="shrink-0 text-emerald-600 dark:text-emerald-400"
                />
                <span class="truncate text-sm font-medium">{{ p.name || p.provider }}</span>
                <Chip size="sm" variant="outline">{{ p.type || 'openai' }}</Chip>
              </div>
              <div class="mt-1.5 truncate text-xs text-zinc-500">{{ p.base_url || '未设置 API 地址' }}</div>
              <div class="mt-1 truncate text-[11px] text-zinc-400">
                模型：{{ p.model || (p.models && p.models[0]) || '未选择' }}（共 {{ (p.models || []).length }} 个）
              </div>
              <div class="mt-2 flex items-center gap-1" @click.stop>
                <Button
                  v-if="p.provider !== local.provider"
                  size="sm"
                  variant="secondary"
                  @click="activate(p)"
                >
                  激活
                </Button>
                <Chip v-else size="sm" variant="secondary">当前使用</Chip>
                <span class="flex-1" />
                <Button size="sm" variant="ghost" danger :icon="Trash2" title="删除" @click="removeProvider(p)" />
              </div>
            </button>
          </div>

          <!-- 厂商编辑大卡片：左接口 / 右模型 -->
          <Teleport to="body">
            <div
              v-if="editOpen"
              class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
              @click.self="editOpen = false"
            >
              <div class="flex max-h-[85vh] w-full max-w-3xl flex-col overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-2xl dark:border-zinc-800 dark:bg-zinc-900">
                <div class="flex shrink-0 items-center justify-between border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
                  <span class="text-sm font-semibold">{{ editNew ? '添加厂商' : '厂商设置' }} · {{ editP.name || editP.provider || '未命名' }}</span>
                  <Button variant="ghost" size="sm" :icon="X" @click="editOpen = false" />
                </div>
                <div class="grid min-h-0 flex-1 gap-4 overflow-y-auto p-4 md:grid-cols-2">
                  <!-- 左：接口设置 -->
                  <div class="space-y-3">
                    <div class="text-xs font-medium text-zinc-400">接口设置</div>
                    <label class="block space-y-1">
                      <span class="text-xs text-zinc-500">名称</span>
                      <Input v-model="editP.name" size="sm" placeholder="如 我的 Ollama" />
                    </label>
                    <label class="block space-y-1">
                      <span class="text-xs text-zinc-500">协议类型</span>
                      <Select
                        :model-value="editP.type || 'openai'"
                        size="sm"
                        :options="typeOptions"
                        @update:model-value="editP.type = String($event)"
                      />
                    </label>
                    <label class="block space-y-1">
                      <span class="text-xs text-zinc-500">API 地址</span>
                      <Input v-model="editP.base_url" size="sm" placeholder="https://api.openai.com/v1" />
                    </label>
                    <label class="block space-y-1">
                      <span class="text-xs text-zinc-500">API 密钥</span>
                      <Input v-model="editP.api_key" size="sm" type="password" placeholder="sk-..." />
                    </label>
                    <label class="block space-y-1">
                      <span class="text-xs text-zinc-500">指定 IP（可选，绕过 DNS 污染）</span>
                      <Input v-model="editP.host_ip" size="sm" placeholder="如 104.18.7.10" />
                    </label>
                    <label class="flex items-center justify-between gap-2 rounded-lg border border-zinc-200 p-2.5 text-sm dark:border-zinc-700">
                      <span>
                        忽略 SSL 证书校验
                        <span class="block text-[10px] text-zinc-400">仅自签名等受控场景</span>
                      </span>
                      <Switch :model-value="!!editP.insecure_ssl" size="sm" @update:model-value="editP.insecure_ssl = !!$event" />
                    </label>
                    <Button
                      size="sm"
                      variant="outline"
                      class="w-full"
                      :icon="RefreshCw"
                      :loading="fetching === editP.provider"
                      @click="fetchModels(editP)"
                    >
                      从服务商获取模型列表
                    </Button>
                  </div>

                  <!-- 右：模型管理 -->
                  <div class="space-y-3">
                    <div class="text-xs font-medium text-zinc-400">模型管理（可自定义增删）</div>
                    <div class="flex min-h-24 flex-wrap content-start gap-1 rounded-lg border border-zinc-200 p-2 dark:border-zinc-700">
                      <Chip
                        v-for="m in editP.models || []"
                        :key="m"
                        size="sm"
                        :variant="editP.model === m ? 'secondary' : 'outline'"
                        class="group cursor-default"
                      >
                        {{ m }}
                        <button
                          class="ml-1 text-zinc-400 group-hover:text-red-500"
                          :title="`移除「${m}」`"
                          @click="removeModel(editP, m)"
                        >
                          <X :size="11" />
                        </button>
                      </Chip>
                      <span v-if="!(editP.models && editP.models.length)" class="self-center px-1 text-xs text-zinc-400">
                        暂无模型，先获取或手动添加
                      </span>
                    </div>
                    <div class="flex items-center gap-2">
                      <Input
                        v-model="editP._newModel"
                        size="sm"
                        placeholder="模型名（如 gpt-4o-mini）"
                        class="flex-1"
                        @keydown.enter.prevent="addModel(editP)"
                      />
                      <Button size="sm" variant="outline" :icon="Plus" @click="addModel(editP)">添加</Button>
                    </div>
                    <label class="block space-y-1">
                      <span class="text-xs text-zinc-500">当前使用模型</span>
                      <Select
                        v-if="modelOpts(editP).length"
                        :model-value="editP.model"
                        size="sm"
                        searchable
                        :options="modelOpts(editP)"
                        placeholder="选择模型"
                        @update:model-value="editP.model = String($event)"
                      />
                      <Input v-else v-model="editP.model" size="sm" placeholder="如 gpt-4o-mini" />
                    </label>
                    <p class="text-[10px] text-zinc-400">模型名会随设置保存；生成请求时使用「当前使用模型」。</p>
                  </div>
                </div>
                <div class="flex shrink-0 items-center justify-end gap-2 border-t border-zinc-200 px-4 py-3 dark:border-zinc-800">
                  <Button variant="ghost" @click="editOpen = false">取消</Button>
                  <Button variant="primary" @click="commitProvider">保存厂商</Button>
                </div>
              </div>
            </div>
          </Teleport>
        </template>

        <!-- 高级 -->
        <template v-else-if="tab === 'advanced'">
          <h2 class="mb-4 text-lg font-semibold">高级设置</h2>
          <div class="grid grid-cols-2 gap-3">
            <label class="col-span-2 space-y-1">
              <span class="text-xs text-zinc-500">Shell 路径</span>
              <Input v-model="local.shell_path" size="sm" placeholder="如 /bin/sh 或 /data/data/com.termux/files/usr/bin/bash" />
              <span v-if="shells.length" class="flex flex-wrap gap-1 pt-1">
                <button
                  v-for="s in shells"
                  :key="s"
                  class="rounded-md border border-zinc-200 px-1.5 py-0.5 font-mono text-[10px] text-zinc-500 hover:bg-zinc-100 dark:border-zinc-700 dark:hover:bg-zinc-800"
                  @click="local.shell_path = s"
                >
                  {{ s }}
                </button>
              </span>
              <span v-else class="text-[10px] text-zinc-400">未检测到其他 shell，可直接手动填写路径</span>
            </label>
            <label v-for="k in ['retry_max', 'sub_timeout', 'max_ctx_tokens', 'cache_ttl', 'tool_retry_max', 'shutdown_timeout']" :key="k" class="space-y-1">
              <span class="text-xs text-zinc-500">{{ fieldLabels[k] || k }}</span>
              <Input v-model="local[k]" type="number" />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-zinc-500">{{ fieldLabels.rag_source }}</span>
              <Input v-model="local.rag_source" placeholder="留空关闭" />
            </label>
          </div>

          <div class="mt-4 grid grid-cols-2 gap-2 rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
            <label class="flex items-center justify-between gap-2 text-sm">
              <span>{{ fieldLabels.redact_secrets }}</span>
              <Switch :model-value="!!local.redact_secrets" size="sm" @update:model-value="local.redact_secrets = !!$event" />
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

        <!-- MCP：仅支持网络服务 -->
        <template v-else-if="tab === 'mcp'">
          <div class="mb-4 flex items-center justify-between">
            <h2 class="text-lg font-semibold">MCP 连接</h2>
            <Button size="sm" variant="outline" :icon="Plus" @click="addMcpServer">添加连接</Button>
          </div>
          <p class="mb-3 text-xs text-zinc-500">
            仅支持网络方式（Streamable HTTP / SSE 的 http/https 地址）；本地命令方式已不再提供。
          </p>
          <div v-if="!mcpServers.length" class="rounded-xl border border-dashed border-zinc-300 p-8 text-center text-sm text-zinc-400 dark:border-zinc-700">
            暂无 MCP 连接
          </div>
          <div
            v-for="(srv, i) in mcpServers"
            :key="i"
            class="mb-2 rounded-lg border border-zinc-200 p-3 dark:border-zinc-800"
          >
            <div class="mb-2 flex items-center gap-2">
              <Chip size="sm" variant="secondary">网络</Chip>
              <Input v-model="srv.name" size="sm" placeholder="连接名称（如 github）" class="w-48" />
              <span class="flex-1" />
              <Button size="sm" variant="ghost" danger :icon="Trash2" @click="mcpServers.splice(i, 1)" />
            </div>
            <Input v-model="srv.url" size="sm" placeholder="服务地址（https://example.com/mcp）" />
          </div>
          <p class="mt-3 text-xs text-zinc-400">
            保存后可在「工具管理」页查看与设置其工具的权限；工具名格式为 mcp__连接名__工具名。
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
                class="flex flex-1 items-center justify-center gap-2 rounded-lg border p-2.5 text-sm transition-colors"
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

            <template v-if="skin === 'glass'">
              <div class="mt-4 space-y-3">
                <div>
                  <div class="mb-1.5 text-xs text-zinc-500">玻璃强度</div>
                  <div class="flex gap-2">
                    <button
                      v-for="opt in glassOptions"
                      :key="opt.value"
                      class="flex-1 rounded-lg border py-1.5 text-xs transition-colors"
                      :class="
                        glassLevel === opt.value
                          ? 'border-zinc-900 bg-zinc-100 dark:border-zinc-100 dark:bg-zinc-800'
                          : 'border-zinc-200 hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800/60'
                      "
                      @click="setGlassLevel(opt.value as any)"
                    >
                      {{ opt.label }}
                    </button>
                  </div>
                </div>
                <div>
                  <div class="mb-1.5 text-xs text-zinc-500">背景风格</div>
                  <div class="flex gap-2">
                    <button
                      v-for="opt in bgOptions"
                      :key="opt.value"
                      class="flex-1 rounded-lg border py-1.5 text-xs transition-colors"
                      :class="
                        bgStyle === opt.value
                          ? 'border-zinc-900 bg-zinc-100 dark:border-zinc-100 dark:bg-zinc-800'
                          : 'border-zinc-200 hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800/60'
                      "
                      @click="setBgStyle(opt.value as any)"
                    >
                      {{ opt.label }}
                    </button>
                  </div>
                </div>
              </div>
            </template>
          </div>

          <div class="mt-4 grid grid-cols-2 gap-2 rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
            <div>
              <div class="mb-1.5 text-xs text-zinc-500">圆角</div>
              <div class="flex gap-2">
                <button
                  v-for="opt in radiusOptions"
                  :key="opt.value"
                  class="flex-1 rounded-lg border py-1.5 text-xs transition-colors"
                  :class="
                    radius === opt.value
                      ? 'border-zinc-900 bg-zinc-100 dark:border-zinc-100 dark:bg-zinc-800'
                      : 'border-zinc-200 hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800/60'
                  "
                  @click="setRadius(opt.value as any)"
                >
                  {{ opt.label }}
                </button>
              </div>
            </div>
            <label class="flex items-center justify-between gap-2 self-end rounded-lg border border-zinc-200 p-2.5 text-sm dark:border-zinc-700">
              <span>界面动画</span>
              <Switch :model-value="anim" size="sm" @update:model-value="setAnim(!!$event)" />
            </label>
          </div>
        </template>
      </div>
    </main>
  </div>
</template>
