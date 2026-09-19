<script setup lang="ts">
import {
  X, Plus, Trash2, RefreshCw, CheckCircle2, Loader2, Settings as SettingsIcon,
  ArrowLeft, SlidersHorizontal, Server, Wrench, Blocks, Palette, Sun, Moon,
  Search, ChevronDown, ChevronRight, Cpu, Sparkles, Puzzle, RefreshCw as Refresh2,
} from 'lucide-vue-next'
import { Message, Button, Input, Switch, Chip, Select, Dialog, Empty } from 'fuxsto-design'
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

const tab = ref<'basic' | 'providers' | 'advanced' | 'mcp' | 'tools' | 'plugins' | 'appearance'>('basic')
const local = ref<Settings>({})
const mcpServers = ref<any[]>([])
const fetching = ref('')
const saving = ref(false)
const shells = ref<string[]>([])
const workspaceRoot = ref('')
const workspaceSaving = ref(false)

async function loadWorkspace() {
  try {
    const d = await useApi<{ root: string }>('/api/workspace')
    workspaceRoot.value = d.root || ''
  } catch {}
}

async function applyWorkspace() {
  const p = (workspaceRoot.value || '').trim()
  if (!p) {
    Message.warning('请填写工作目录')
    return
  }
  workspaceSaving.value = true
  try {
    const d = await useApi<{ root: string }>('/api/workspace', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ path: p }),
    })
    workspaceRoot.value = d.root || p
    Message.success('工作目录已切换（AI 工具立即生效）')
  } catch (e: any) {
    Message.error(e?.message || '设置失败')
  } finally {
    workspaceSaving.value = false
  }
}

const navItems = [
  { id: 'basic', label: '基础', icon: SlidersHorizontal },
  { id: 'providers', label: 'AI 厂商', icon: Server },
  { id: 'advanced', label: '高级', icon: Wrench },
  { id: 'mcp', label: 'MCP 连接', icon: Blocks },
  { id: 'tools', label: '工具管理', icon: Wrench },
  { id: 'plugins', label: '插件', icon: Puzzle },
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

// applyEditToLocal 把弹窗中的厂商修改合并回表单（返回是否成功）。
// 点击「保存设置」时会自动调用，避免弹窗里的获取结果/编辑丢失。
function applyEditToLocal(): boolean {
  if (!editOpen.value) return true
  const p = editP.value
  const id = (p.provider || p.name || p.type || 'custom').trim().toLowerCase().replace(/\s+/g, '-')
  const hasContent = !!(p.name || p.base_url || p.api_key || (p.models && p.models.length))
  if (!id) {
    if (editNew.value && !hasContent) return true // 空白的新建弹窗：忽略，不阻塞保存
    Message.warning('请填写厂商名称')
    return false
  }
  const list = providers.value.slice()
  if (editNew.value) {
    if (list.some((x) => x.provider === id)) {
      Message.error('厂商标识已存在')
      return false
    }
    p.provider = id
    list.push(JSON.parse(JSON.stringify(p)))
  } else {
    const i = list.findIndex((x) => x.provider === editKey.value)
    if (i < 0) return false
    p.provider = id
    list[i] = JSON.parse(JSON.stringify(p))
    if (local.value.provider === editKey.value) {
      local.value.provider = id
      local.value.base_url = p.base_url || ''
      local.value.api_key = p.api_key || ''
      if (p.model) local.value.model = p.model
    }
  }
  local.value.providers = list
  return true
}

function commitProvider() {
  if (!applyEditToLocal()) return
  editOpen.value = false
  Message.success(editNew.value ? '厂商已添加，点击保存生效' : '厂商已更新，点击保存生效')
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
  loadWorkspace()
  resetLocal()
})

let inited = false
let pendingSave = false
watch(
  () => state.settings,
  () => {
    // 仅在首次加载或本次页面主动保存后从服务端同步，避免后台设置事件
    // （如临时取模型、其他页面操作）把正在编辑的表单冲掉。
    if (state.settings && (!inited || pendingSave)) {
      inited = true
      pendingSave = false
      resetLocal()
    }
  },
)

watch(tab, (v) => {
  if (v === 'tools' && !toolLoaded.value && !toolLoading.value) loadTools()
})

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
  Message.info(`已切换当前厂商为「${p.name || p.provider}」，点击保存生效`)
}

async function fetchModels(p: ProviderRow) {
  if (!p.base_url && p.type !== 'claude') {
    Message.warning('请先填写该厂商的 API 地址')
    return
  }
  fetching.value = p.provider
  try {
    // 直接把目标厂商的地址/密钥发给后端取列表，不切换到该厂商，
    // 因此不会触发设置回传把当前编辑的内容冲掉。
    const res = await useApi<{ models: string[] }>('/api/models', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({
        type: p.type,
        base_url: p.base_url,
        api_key: p.api_key,
        provider: p.provider,
        model: p.model,
      }),
    })
    const fetched = res.models || []
    if (!p.models) p.models = []
    for (const m of fetched) if (!p.models.includes(m)) p.models.push(m)
    if (!fetched.length) Message.info('该厂商无公开模型列表（如 Claude），请手动填写模型名')
    else Message.success(`获取到 ${fetched.length} 个模型`)
  } catch (e: any) {
    Message.error(e?.message || '获取模型失败')
  } finally {
    fetching.value = ''
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
  if (!applyEditToLocal()) throw new Error('厂商信息不完整')
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
  pendingSave = true
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

// ===== 插件管理（进程插件） =====
interface PluginInfo {
  id: string
  name: string
  version: string
  apiVersion: number
  description?: string
  author?: string
  capabilities?: string[]
  permission_summary?: string[]
  enabled: boolean
  acked: boolean
  running: boolean
  error?: string
  tools?: { name: string; description?: string }[]
  commands?: { name: string; description?: string }[]
  panels?: { id: string; title?: string; description?: string }[]
  settings_schema?: any
  settings?: any
  prompt?: string
  logs?: string[]
  dir: string
}

const pluginList = ref<PluginInfo[]>([])
const pluginLoading = ref(false)
const pluginBusy = ref('')
const pluginForm = ref<Record<string, Record<string, any>>>({})
const pluginPanels = ref<Record<string, any>>({})
const pluginLogsOpen = ref<Record<string, boolean>>({})
const pluginZipInput = ref<HTMLInputElement | null>(null)

function defaultFor(v: any): any {
  if (v?.type === 'boolean') return false
  if (v?.type === 'number' || v?.type === 'integer') return 0
  if (v?.type === 'array') return []
  if (Array.isArray(v?.enum) && v.enum.length) return v.enum[0]
  return ''
}

function initPluginForm(p: PluginInfo) {
  const out: Record<string, any> = {}
  const props = p.settings_schema?.properties
  if (props && typeof props === 'object') {
    for (const [k, v] of Object.entries<any>(props)) {
      const cur = p.settings ? p.settings[k] : undefined
      out[k] = cur !== undefined ? cur : v?.default !== undefined ? v.default : defaultFor(v)
    }
  }
  pluginForm.value[p.id] = out
}

function schemaFields(p: PluginInfo): { key: string; label: string; type: string; options?: string[] }[] {
  const props = p.settings_schema?.properties
  if (!props || typeof props !== 'object') return []
  return Object.entries<any>(props).map(([key, v]) => {
    let type = 'string'
    if (v?.type === 'boolean') type = 'boolean'
    else if (v?.type === 'number' || v?.type === 'integer') type = 'number'
    else if (Array.isArray(v?.enum)) type = 'enum'
    else if (v?.type === 'array') type = 'array'
    return { key, label: v?.title || key, type, options: v?.enum }
  })
}

async function loadPlugins() {
  pluginLoading.value = true
  try {
    const d = await useApi<{ plugins?: PluginInfo[] }>('/api/plugins')
    pluginList.value = d.plugins || []
    for (const p of pluginList.value) {
      initPluginForm(p)
      if (p.running && p.panels?.length) {
        for (const panel of p.panels) loadPanel(p, panel.id)
      }
    }
  } catch (e: any) {
    Message.error(e?.message || '加载插件失败')
  } finally {
    pluginLoading.value = false
  }
}

async function postPlugin(path: string, body: any): Promise<any> {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    body: JSON.stringify(body),
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const err: any = new Error(data.error || `请求失败 (${res.status})`)
    err.data = data
    throw err
  }
  return data
}

async function enablePlugin(p: PluginInfo) {
  pluginBusy.value = p.id
  try {
    try {
      await postPlugin('/api/plugins/enable', { id: p.id })
    } catch (e: any) {
      if (!e?.data?.need_ack) throw e
      const lines = (e.data.permission_summary || []).join('\n') || '未声明特殊权限'
      Dialog.confirm({
        title: `启用插件「${p.name}」`,
        content: `该插件声明以下权限（进程插件无法强制沙箱，请只启用可信插件）：\n\n${lines}`,
        confirmText: '确认启用',
        onConfirm: async () => {
          await postPlugin('/api/plugins/enable', { id: p.id, ack: true })
          Message.success('插件已启用')
          loadPlugins()
        },
      })
      return
    }
    Message.success('插件已启用')
    loadPlugins()
  } catch (e: any) {
    Message.error(e?.message || '启用失败')
  } finally {
    pluginBusy.value = ''
  }
}

async function disablePlugin(p: PluginInfo) {
  pluginBusy.value = p.id
  try {
    await postPlugin('/api/plugins/disable', { id: p.id })
    Message.success('插件已停用')
    loadPlugins()
  } catch (e: any) {
    Message.error(e?.message || '停用失败')
  } finally {
    pluginBusy.value = ''
  }
}

async function reloadPlugin(p: PluginInfo) {
  pluginBusy.value = p.id
  try {
    await postPlugin('/api/plugins/reload', { id: p.id })
    Message.success('插件已重载')
    loadPlugins()
  } catch (e: any) {
    Message.error(e?.message || '重载失败')
  } finally {
    pluginBusy.value = ''
  }
}

async function removePlugin(p: PluginInfo) {
  Dialog.confirm({
    title: '删除插件',
    content: `确定删除「${p.name}」？插件目录会移入 .trash，可手动恢复。`,
    danger: true,
    confirmText: '删除',
    onConfirm: async () => {
      try {
        await postPlugin('/api/plugins/delete', { id: p.id })
        Message.success('已删除')
        loadPlugins()
      } catch (e: any) {
        Message.error(e?.message || '删除失败')
      }
    },
  })
}

async function savePluginSettings(p: PluginInfo) {
  pluginBusy.value = p.id
  try {
    await postPlugin('/api/plugins/settings', { id: p.id, settings: pluginForm.value[p.id] || {} })
    Message.success('插件设置已保存')
    loadPlugins()
  } catch (e: any) {
    Message.error(e?.message || '保存失败')
  } finally {
    pluginBusy.value = ''
  }
}

async function loadPanel(p: PluginInfo, panel: string) {
  try {
    const data = await useApi<any>(`/api/plugins/panel?id=${encodeURIComponent(p.id)}&panel=${encodeURIComponent(panel)}`)
    pluginPanels.value[p.id + ':' + panel] = data
  } catch (e: any) {
    Message.error(e?.message || '面板加载失败')
  }
}

async function installPlugin(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (!files?.length) return
  const fd = new FormData()
  fd.append('file', files[0])
  try {
    const res = await useApi<{ id: string }>('/api/plugins/install', { method: 'POST', body: fd })
    Message.success(`插件已安装：${res.id}（默认未启用）`)
    loadPlugins()
  } catch (err: any) {
    Message.error(err?.message || '安装失败')
  }
  ;(e.target as HTMLInputElement).value = ''
}

// ===== 工具管理（内嵌设置页，保留设置侧栏） =====
interface ToolInfo {
  name: string
  description: string
  source: 'builtin' | 'subagent' | 'mcp' | 'external' | 'skill'
  server?: string
  file?: string
  removable: boolean
  rule: string
}

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

const toolGroupDefs = [
  { id: 'builtin', label: '内置工具', icon: Wrench, hint: '随程序内置的编码工具' },
  { id: 'subagent', label: '子代理', icon: Cpu, hint: '把任务拆分给专用子代理执行' },
  { id: 'mcp', label: 'MCP 工具', icon: Blocks, hint: '来自 MCP 连接，删除请移除对应连接' },
  { id: 'external', label: '外部命令工具', icon: Server, hint: '来自 ~/.licode/tools/ 的自定义工具' },
  { id: 'plugin', label: '插件工具', icon: Puzzle, hint: '来自运行中的进程插件' },
  { id: 'skill', label: '技能', icon: Sparkles, hint: '来自 skills/ 的 Markdown 技能' },
] as const

const toolRuleOptions = [
  { label: '允许', value: 'allow' },
  { label: '询问', value: 'ask' },
  { label: '禁止', value: 'deny' },
]

const toolLoaded = ref(false)
const toolList = ref<ToolInfo[]>([])
const toolServers = ref<any[]>([])
const toolError = ref('')
const toolLoading = ref(false)
const toolSaving = ref('')
const toolQuery = ref('')
const toolOpen = reactive<Record<string, boolean>>({
  builtin: true,
  subagent: true,
  mcp: true,
  external: true,
  plugin: true,
  skill: true,
})

async function loadTools() {
  toolLoading.value = true
  try {
    const data = await useApi<{ tools: ToolInfo[]; mcp_servers?: any[]; mcp_error?: string }>('/api/tools')
    toolList.value = data.tools || []
    toolServers.value = data.mcp_servers || []
    toolError.value = data.mcp_error || ''
  } catch (e: any) {
    Message.error(e?.message || '加载工具列表失败')
  } finally {
    toolLoading.value = false
    toolLoaded.value = true
  }
}

function toolDisplayName(t: ToolInfo): string {
  if (t.source === 'mcp') {
    const parts = t.name.split('__')
    return parts.length >= 3 ? parts.slice(2).join('__') : t.name
  }
  if (t.source === 'skill') return t.name.replace(/^skill_/, '')
  return builtinZh[t.name]?.name || t.name
}

function toolDisplayDesc(t: ToolInfo): string {
  if (t.source === 'builtin' || t.source === 'subagent') return builtinZh[t.name]?.desc || t.description || '暂无说明'
  return t.description || '暂无说明（由工具提供方定义）'
}

function toolMatches(t: ToolInfo): boolean {
  const q = toolQuery.value.trim().toLowerCase()
  if (!q) return true
  return (
    t.name.toLowerCase().includes(q) ||
    toolDisplayName(t).toLowerCase().includes(q) ||
    toolDisplayDesc(t).toLowerCase().includes(q) ||
    (t.server || '').toLowerCase().includes(q)
  )
}

const toolFiltered = computed(() => toolList.value.filter(toolMatches))
function toolGroup(id: string): ToolInfo[] {
  return toolFiltered.value.filter((t) => t.source === id)
}
function toolServerTools(name: string): ToolInfo[] {
  return toolFiltered.value.filter((t) => t.source === 'mcp' && t.server === name)
}
const toolServerRows = computed(() => {
  const list = toolServers.value.slice()
  for (const t of toolFiltered.value) {
    if (t.source === 'mcp' && t.server && !list.some((s: any) => s.name === t.server)) {
      list.push({ name: t.server, type: 'http' })
    }
  }
  return list.filter((s: any) => {
    const q = toolQuery.value.trim().toLowerCase()
    if (!q) return true
    return String(s.name).toLowerCase().includes(q) || toolServerTools(s.name).some(toolMatches)
  })
})
function toolGroupCount(id: string): number {
  return id === 'mcp' ? toolServerRows.value.length : toolGroup(id).length
}
function toggleAllTools(open: boolean) {
  for (const g of toolGroupDefs) toolOpen[g.id] = open
}
async function setToolRule(t: ToolInfo, rule: string) {
  const prev = t.rule
  t.rule = rule
  toolSaving.value = t.name
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
    toolSaving.value = ''
  }
}
function removeTool(t: ToolInfo) {
  const isSkill = t.source === 'skill'
  const type = isSkill ? 'skill' : 'external'
  const name = isSkill ? t.name.replace(/^skill_/, '') : t.name
  Dialog.confirm({
    title: isSkill ? '删除技能' : '删除外部命令工具',
    content: `确定删除「${toolDisplayName(t)}」？将删除对应文件，不可恢复。`,
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
        loadTools()
      } catch (e: any) {
        Message.error(e?.message || '删除失败')
      }
    },
  })
}
function removeMcpToolServer(s: any) {
  Dialog.confirm({
    title: '删除 MCP 连接',
    content: `确定删除「${s.name}」？其工具将不再可用，相关权限规则一并清理。`,
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
        loadTools()
      } catch (e: any) {
        Message.error(e?.message || '删除失败')
      }
    },
  })
}

const toolSourceBadge: Record<string, string> = {
  builtin: '内置',
  subagent: '子代理',
  mcp: 'MCP',
  external: '外部命令',
  plugin: '插件',
  skill: '技能',
}
</script>

<template>
  <div class="flex h-full overflow-hidden">
    <!-- 左侧分区导航 -->
    <aside class="flex w-16 shrink-0 flex-col border-r border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900 sm:w-56">
      <div class="flex items-center justify-center gap-2 border-b border-zinc-200 px-2 py-4 dark:border-zinc-800 sm:justify-start sm:px-4">
        <button class="rounded-lg p-1.5 text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800" title="返回对话" @click="navigateTo('/')">
          <ArrowLeft :size="16" />
        </button>
        <span class="hidden text-base font-semibold sm:inline">设置</span>
      </div>
      <nav class="flex-1 space-y-1 p-2">
        <button
          v-for="item in navItems"
          :key="item.id"
          class="flex w-full items-center justify-center gap-2.5 rounded-lg px-2 py-2 text-sm transition-colors sm:justify-start sm:px-3"
          :class="
            tab === item.id
              ? 'bg-zinc-100 font-medium text-zinc-900 dark:bg-zinc-800 dark:text-zinc-50'
              : 'text-zinc-600 hover:bg-zinc-50 dark:text-zinc-400 dark:hover:bg-zinc-800/60'
          "
          @click="tab = item.id"
        >
          <component :is="item.icon" :size="16" class="shrink-0" />
          <span class="hidden sm:inline">{{ item.label }}</span>
        </button>
      </nav>
      <div class="border-t border-zinc-200 p-2 dark:border-zinc-800 sm:p-3">
        <Button
          variant="primary"
          class="w-full"
          :loading="saving"
          :disabled="!state.settings || state.wsStatus !== 'connected'"
          @click="save"
        >
          <span class="hidden sm:inline">保存设置</span>
          <span class="sm:hidden">保存</span>
        </Button>
      </div>
    </aside>

    <!-- 内容区 -->
    <main class="min-w-0 flex-1 overflow-y-auto">
      <div class="mx-auto px-3 py-4 sm:px-6 sm:py-6" :class="tab === 'tools' ? 'max-w-4xl' : 'max-w-3xl'">
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
            <span class="hidden text-xs text-zinc-400 sm:inline">同一时间使用一个厂商，可在对话输入框随时切换</span>
            <span class="flex-1" />
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
                  设为当前
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
          <div class="mb-4 rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
            <div class="mb-2 flex items-center justify-between">
              <span class="text-sm font-medium">AI 工作目录</span>
              <span class="text-[10px] text-zinc-400">文件读写边界、相对路径与 Shell 的基准目录</span>
            </div>
            <div class="flex items-center gap-2">
              <Input v-model="workspaceRoot" size="sm" class="flex-1" placeholder="如 /data/data/com.termux/files/home/project" @keydown.enter.prevent="applyWorkspace" />
              <Button size="sm" variant="outline" :loading="workspaceSaving" @click="applyWorkspace">切换</Button>
            </div>
          </div>
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

        <!-- 工具管理（内嵌，保留设置侧栏） -->
        <template v-else-if="tab === 'tools'">
          <div class="mb-4 flex flex-wrap items-center gap-2">
            <h2 class="text-lg font-semibold">工具管理</h2>
            <span class="hidden text-xs text-zinc-400 sm:inline">允许=直接执行 · 询问=每次确认 · 禁止=不可用</span>
            <span class="flex-1" />
            <Button size="sm" variant="outline" :icon="Refresh2" :loading="toolLoading" @click="loadTools">刷新</Button>
            <Button size="sm" variant="ghost" @click="toggleAllTools(true)">展开</Button>
            <Button size="sm" variant="ghost" @click="toggleAllTools(false)">折叠</Button>
          </div>
          <Input v-model="toolQuery" size="sm" class="mb-4" :prefix-icon="Search" placeholder="搜索工具名称或说明…" />

          <div v-if="toolLoading && !toolList.length" class="flex items-center justify-center gap-2 py-16 text-sm text-zinc-500">
            <Loader2 :size="18" class="animate-spin" />
            正在加载工具列表…（首次连接 MCP 可能需要几秒）
          </div>
          <Empty v-else-if="!toolFiltered.length" title="没有匹配的工具" description="换个关键词试试，或检查工具配置" class="mt-10" />
          <div v-else class="space-y-3">
            <div v-for="g in toolGroupDefs" :key="g.id" class="overflow-hidden rounded-xl border border-zinc-200 dark:border-zinc-800">
              <button
                class="flex w-full items-center gap-2 bg-zinc-50 px-3 py-2.5 text-left transition-colors hover:bg-zinc-100 dark:bg-zinc-900 dark:hover:bg-zinc-800"
                @click="toolOpen[g.id] = !toolOpen[g.id]"
              >
                <ChevronDown v-if="toolOpen[g.id]" :size="14" class="shrink-0 text-zinc-400" />
                <ChevronRight v-else :size="14" class="shrink-0 text-zinc-400" />
                <component :is="g.icon" :size="14" class="shrink-0 text-zinc-500" />
                <span class="shrink-0 text-sm font-medium">{{ g.label }}</span>
                <Chip size="sm" variant="secondary">{{ toolGroupCount(g.id) }}</Chip>
                <span class="min-w-0 flex-1" />
                <span class="hidden truncate text-[11px] text-zinc-400 sm:block">{{ g.hint }}</span>
              </button>

              <div v-if="toolOpen[g.id]" class="divide-y divide-zinc-100 bg-white dark:divide-zinc-800 dark:bg-zinc-900">
                <!-- MCP：按连接展示 -->
                <template v-if="g.id === 'mcp'">
                  <div v-if="toolError" class="px-3 py-2 text-[11px] text-amber-600 dark:text-amber-400">
                    部分 MCP 连接失败（{{ toolError }}），下方仍可管理配置。
                  </div>
                  <div v-if="!toolServerRows.length" class="px-3 py-4 text-xs text-zinc-400">暂无 MCP 连接（可在「MCP 连接」页添加）</div>
                  <div v-for="srv in toolServerRows" :key="srv.name">
                    <div class="flex items-center gap-2 px-3 py-2.5">
                      <Blocks :size="14" class="shrink-0 text-zinc-400" />
                      <span class="min-w-0 flex-1 truncate text-sm font-medium">{{ srv.name }}</span>
                      <Chip size="sm" variant="outline" class="shrink-0">{{ srv.type === 'http' ? '网络' : srv.type }}</Chip>
                      <Button size="sm" variant="ghost" danger :icon="Trash2" title="删除该连接" class="shrink-0" @click="removeMcpToolServer(srv)" />
                    </div>
                    <div v-if="!toolServerTools(srv.name).length" class="px-3 pb-3 pl-9 text-[11px] text-zinc-400">
                      {{ toolError ? '未连接成功，暂无工具列表' : '该连接没有提供工具' }}
                    </div>
                    <div
                      v-for="t in toolServerTools(srv.name)"
                      :key="t.name"
                      class="flex flex-wrap items-start gap-x-2 gap-y-1.5 border-t border-zinc-100 px-3 py-2.5 pl-4 dark:border-zinc-800 sm:pl-9"
                    >
                      <div class="min-w-[140px] flex-1">
                        <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5">
                          <span class="truncate text-sm">{{ toolDisplayName(t) }}</span>
                          <span class="max-w-full truncate font-mono text-[10px] text-zinc-400">{{ t.name }}</span>
                        </div>
                        <p class="mt-0.5 line-clamp-2 text-[11px] text-zinc-400">{{ toolDisplayDesc(t) }}</p>
                      </div>
                      <div class="flex shrink-0 items-center gap-1">
                        <Select
                          :model-value="t.rule"
                          size="sm"
                          :options="toolRuleOptions"
                          class="w-20"
                          :loading="toolSaving === t.name"
                          @update:model-value="setToolRule(t, String($event))"
                        />
                      </div>
                    </div>
                  </div>
                </template>

                <!-- 其他来源：平铺 -->
                <template v-else>
                  <div v-if="!toolGroup(g.id).length" class="px-3 py-4 text-xs text-zinc-400">暂无工具</div>
                  <div v-for="t in toolGroup(g.id)" :key="t.name" class="flex flex-wrap items-start gap-x-2 gap-y-1.5 px-3 py-2.5">
                    <div class="min-w-[160px] flex-1">
                      <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5">
                        <span class="truncate text-sm">{{ toolDisplayName(t) }}</span>
                        <span class="max-w-full truncate font-mono text-[10px] text-zinc-400">{{ t.name }}</span>
                        <Chip v-if="t.source !== 'builtin' && t.source !== 'subagent'" size="sm" variant="outline">{{ toolSourceBadge[t.source] }}</Chip>
                      </div>
                      <p class="mt-0.5 line-clamp-2 text-[11px] text-zinc-400">{{ toolDisplayDesc(t) }}</p>
                    </div>
                    <div class="flex shrink-0 items-center gap-1">
                      <Select
                        :model-value="t.rule"
                        size="sm"
                        :options="toolRuleOptions"
                        class="w-20"
                        :loading="toolSaving === t.name"
                        @update:model-value="setToolRule(t, String($event))"
                      />
                      <Button v-if="t.removable" size="sm" variant="ghost" danger :icon="Trash2" title="删除" @click="removeTool(t)" />
                    </div>
                  </div>
                </template>
              </div>
            </div>
          </div>
        </template>

        <!-- 插件（进程插件） -->
        <template v-else-if="tab === 'plugins'">
          <div class="mb-4 flex flex-wrap items-center gap-2">
            <h2 class="text-lg font-semibold">插件</h2>
            <span class="hidden text-xs text-zinc-400 sm:inline">进程插件：可贡献工具、斜杠命令、提示词、设置与面板</span>
            <span class="flex-1" />
            <Button size="sm" variant="outline" :icon="Refresh2" :loading="pluginLoading" @click="loadPlugins">刷新</Button>
            <input ref="pluginZipInput" type="file" accept=".zip" class="hidden" @change="installPlugin" />
            <Button size="sm" variant="outline" :icon="Plus" @click="pluginZipInput?.click()">安装插件（zip）</Button>
          </div>

          <div v-if="!pluginList.length" class="rounded-xl border border-dashed border-zinc-300 p-8 text-center text-sm text-zinc-400 dark:border-zinc-700">
            暂无插件。把插件目录（含 plugin.json）放入 ~/.licode/plugins/，或上传 zip 安装。
          </div>

          <div v-for="p in pluginList" :key="p.id" class="mb-3 rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-sm font-medium">{{ p.name || p.id }}</span>
              <Chip size="sm" variant="outline">v{{ p.version }}</Chip>
              <Chip size="sm" variant="secondary">{{ p.running ? '运行中' : p.enabled ? '已启用' : '未启用' }}</Chip>
              <span class="flex-1" />
              <Button v-if="!p.enabled" size="sm" variant="primary" :loading="pluginBusy === p.id" @click="enablePlugin(p)">启用</Button>
              <Button v-else size="sm" variant="outline" :loading="pluginBusy === p.id" @click="disablePlugin(p)">停用</Button>
              <Button size="sm" variant="ghost" :icon="Refresh2" title="重载" @click="reloadPlugin(p)" />
              <Button size="sm" variant="ghost" danger :icon="Trash2" title="删除" @click="removePlugin(p)" />
            </div>
            <p v-if="p.description" class="mt-1 text-xs text-zinc-500">{{ p.description }}</p>
            <p v-if="p.error" class="mt-1 flex items-center gap-1 text-xs text-red-500">
              <X :size="12" /> {{ p.error }}
            </p>
            <div v-if="(p.permission_summary || []).length" class="mt-2 flex flex-wrap gap-1">
              <Chip v-for="s in p.permission_summary" :key="s" size="sm" variant="outline">{{ s }}</Chip>
            </div>
            <div class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-zinc-400">
              <span v-if="p.tools?.length">工具 {{ p.tools.length }} 个</span>
              <span v-if="p.commands?.length">命令 {{ p.commands.map((c) => '/' + c.name).join(' ') }}</span>
              <span v-if="p.panels?.length">面板 {{ p.panels.length }} 个</span>
              <span class="font-mono">{{ p.id }} · {{ p.dir }}</span>
            </div>

            <!-- 声明式设置表单 -->
            <div v-if="schemaFields(p).length" class="mt-3 space-y-2 rounded-lg border border-zinc-200 p-3 dark:border-zinc-800">
              <div class="text-xs font-medium text-zinc-400">插件设置</div>
              <label v-for="f in schemaFields(p)" :key="f.key" class="flex items-center justify-between gap-2 text-sm">
                <span class="shrink-0 text-xs text-zinc-500">{{ f.label }}</span>
                <Switch
                  v-if="f.type === 'boolean'"
                  :model-value="!!(pluginForm[p.id] || {})[f.key]"
                  size="sm"
                  @update:model-value="(pluginForm[p.id] || {})[f.key] = !!$event"
                />
                <Select
                  v-else-if="f.type === 'enum'"
                  :model-value="(pluginForm[p.id] || {})[f.key]"
                  size="sm"
                  class="w-48"
                  :options="(f.options || []).map((o) => ({ label: String(o), value: String(o) }))"
                  @update:model-value="(pluginForm[p.id] || {})[f.key] = String($event)"
                />
                <Input
                  v-else-if="f.type === 'number'"
                  :model-value="(pluginForm[p.id] || {})[f.key]"
                  type="number"
                  size="sm"
                  class="w-48"
                  @update:model-value="(pluginForm[p.id] || {})[f.key] = Number($event) || 0"
                />
                <Input
                  v-else
                  :model-value="f.type === 'array' ? ((pluginForm[p.id] || {})[f.key] || []).join(',') : (pluginForm[p.id] || {})[f.key]"
                  size="sm"
                  class="w-64 max-w-[60%]"
                  @update:model-value="(pluginForm[p.id] || {})[f.key] = f.type === 'array' ? String($event).split(',').map((x) => x.trim()).filter(Boolean) : String($event)"
                />
              </label>
              <Button size="sm" variant="outline" :loading="pluginBusy === p.id" @click="savePluginSettings(p)">保存插件设置</Button>
            </div>

            <!-- 声明式面板 -->
            <div v-for="panel in p.panels || []" :key="panel.id" class="mt-3 rounded-lg border border-zinc-200 dark:border-zinc-800">
              <div class="flex items-center gap-2 border-b border-zinc-100 px-3 py-2 dark:border-zinc-800">
                <span class="text-xs font-medium">{{ panel.title || panel.id }}</span>
                <span class="min-w-0 flex-1 truncate text-[10px] text-zinc-400">{{ panel.description }}</span>
                <Button size="sm" variant="ghost" :icon="Refresh2" :disabled="!p.running" @click="loadPanel(p, panel.id)">刷新</Button>
              </div>
              <div class="max-h-72 overflow-y-auto p-3 text-xs">
                <template v-if="!pluginPanels[p.id + ':' + panel.id]" class="text-zinc-400">点击「刷新」加载内容</template>
                <pre
                  v-else-if="pluginPanels[p.id + ':' + panel.id].type === 'markdown'"
                  class="whitespace-pre-wrap break-words font-sans leading-relaxed text-zinc-600 dark:text-zinc-300"
                >{{ pluginPanels[p.id + ':' + panel.id].content }}</pre>
                <table v-else-if="pluginPanels[p.id + ':' + panel.id].type === 'table'" class="w-full">
                  <thead>
                    <tr>
                      <th
                        v-for="col in pluginPanels[p.id + ':' + panel.id].columns || []"
                        :key="col"
                        class="border-b border-zinc-200 px-2 py-1 text-left font-medium text-zinc-500 dark:border-zinc-700"
                      >
                        {{ col }}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(row, ri) in pluginPanels[p.id + ':' + panel.id].rows || []" :key="ri">
                      <td v-for="(cell, ci) in row" :key="ci" class="border-b border-zinc-100 px-2 py-1 text-zinc-500 dark:border-zinc-800">{{ cell }}</td>
                    </tr>
                  </tbody>
                </table>
                <div v-else-if="pluginPanels[p.id + ':' + panel.id].type === 'keyvalue'" class="space-y-1">
                  <div v-for="item in pluginPanels[p.id + ':' + panel.id].items || []" :key="item.key" class="flex gap-2">
                    <span class="w-32 shrink-0 text-zinc-400">{{ item.key }}</span>
                    <span class="min-w-0 flex-1 break-words text-zinc-600 dark:text-zinc-300">{{ item.value }}</span>
                  </div>
                </div>
                <div v-else-if="pluginPanels[p.id + ':' + panel.id].type === 'status'" class="text-zinc-600 dark:text-zinc-300">
                  {{ pluginPanels[p.id + ':' + panel.id].text }}
                </div>
                <pre v-else class="whitespace-pre-wrap break-words font-mono text-[11px] text-zinc-500">{{ JSON.stringify(pluginPanels[p.id + ':' + panel.id], null, 2) }}</pre>
              </div>
            </div>

            <!-- 日志 -->
            <div v-if="(p.logs || []).length" class="mt-2">
              <button class="text-[11px] text-zinc-400 hover:text-zinc-600" @click="pluginLogsOpen[p.id] = !pluginLogsOpen[p.id]">
                {{ pluginLogsOpen[p.id] ? '收起日志' : `查看日志（${p.logs.length}）` }}
              </button>
              <pre v-if="pluginLogsOpen[p.id]" class="mt-1 max-h-40 overflow-y-auto rounded-lg bg-zinc-50 p-2 font-mono text-[10px] leading-relaxed text-zinc-500 dark:bg-zinc-900">{{ p.logs.join('\n') }}</pre>
            </div>
          </div>
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
