import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { apiGet, apiPost } from '@/api/http'
import { MASKED_API_KEY, type MCPServer, type Settings, type ToolRule } from '@/api/protocol'

/**
 * 设置与运行时元数据。
 *
 * 关键契约（后端源码为准）：
 *   - 设置读写只走 WebSocket，没有 REST 端点：settings_get 拉取、
 *     settings_set 保存；两者服务端都回推 settings 事件（已掩码）。
 *   - settings_set 是整体替换：必须回传读到的完整对象，缺字段会落回默认值。
 *   - api_key 读出恒为 "********"，原样回传则服务端保留旧密钥。
 *   - /api/models 返回扁平字符串数组，Claude 协议恒为 null。
 */

export interface ToolInfo {
  name: string
  description: string
  source: 'builtin' | 'subagent' | 'mcp' | 'external' | 'skill'
  server?: string
  file?: string
  removable: boolean
  rule: ToolRule
}

export interface ToolsPayload {
  tools: ToolInfo[] | null
  mcp_servers: MCPServer[] | null
  mcp_error: string
}

export interface CaCert {
  name: string
  size: number
  mod_time: string
  subjects?: string[]
  valid: boolean
}

export interface WorkspaceInfo {
  root: string
}

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Settings | null>(null)
  /** 已发出 settings_set、等待服务端回推确认 */
  const saving = ref(false)
  const lastSettingsError = ref<string | null>(null)

  const workspace = ref<WorkspaceInfo>({ root: '' })
  const version = ref<{ version: string; counter: number } | null>(null)
  const tools = ref<ToolInfo[]>([])
  const mcpError = ref('')
  const shells = ref<string[]>([])
  const caCerts = ref<CaCert[]>([])

  /**
   * 拉取设置。
   *
   * send 通常由 session store 注入（视图里直接调 requestGet() 即可）；
   * 未传时退化为「什么都没发生」——不再抛错，因为设置页在连接就绪前
   * 也可能被挂载，此时静默等待连接建立后的自动拉取更合理。
   */
  function requestGet(send?: (type: string) => void) {
    send?.('settings_get')
  }

  /** 服务端 settings 事件入口（由 session store 转发） */
  function applyFromServer(next: Settings | undefined, hadError = false) {
    if (next) settings.value = JSON.parse(JSON.stringify(next)) as Settings
    if (!hadError) saving.value = false
  }

  function markSaveFailed(err: string) {
    saving.value = false
    lastSettingsError.value = err
  }

  /**
   * 保存全量设置。调用方必须传入「读到的完整对象 + 局部改动」，
   * 这里只做掩码字段的自检，不猜测缺失字段。
   *
   * send 省略时直接返回 false（视图可用返回值提示「未连接」），
   * 避免在断线状态下把 saving 永久卡在 true。
   */
  function save(next: Settings, send?: (type: string, payload: Record<string, unknown>) => void): boolean {
    // 掩码原样回传即可；若被前端意外清空，恢复掩码以免抹掉真实密钥
    const clone = JSON.parse(JSON.stringify(next)) as Settings
    if (clone.api_key === '') clone.api_key = MASKED_API_KEY
    for (const p of clone.providers ?? []) if (p.api_key === '') p.api_key = MASKED_API_KEY
    if (!send) return false
    saving.value = true
    lastSettingsError.value = null
    send('settings_set', { settings: clone })
    return true
  }

  async function loadWorkspace(): Promise<string> {
    const r = await apiGet<WorkspaceInfo>('/api/workspace')
    workspace.value = r
    return r.root
  }

  async function setWorkspace(path: string): Promise<string> {
    const r = await apiPost<WorkspaceInfo & { ok: boolean }>('/api/workspace', { path })
    workspace.value = { root: r.root }
    return r.root
  }

  async function loadVersion() {
    version.value = await apiGet<{ version: string; counter: number }>('/api/version')
  }

  async function loadTools(): Promise<ToolsPayload> {
    const r = await apiGet<ToolsPayload>('/api/tools')
    tools.value = r.tools ?? []
    mcpError.value = r.mcp_error ?? ''
    return r
  }

  async function setToolRule(name: string, rule: ToolRule) {
    await apiPost('/api/tools/rule', { name, rule })
    const hit = tools.value.find((t) => t.name === name)
    if (hit) hit.rule = rule
  }

  async function deleteTool(type: 'mcp' | 'external' | 'skill', name: string) {
    await apiPost('/api/tools/delete', { type, name })
  }

  async function loadShells() {
    shells.value = await apiGet<string[]>('/api/shells')
  }

  async function loadCaCerts(): Promise<CaCert[]> {
    const r = await apiGet<{ dir: string; certs: CaCert[] | null }>('/api/ca')
    caCerts.value = r.certs ?? []
    return caCerts.value
  }

  async function uploadCa(file: File): Promise<{ ok: boolean; name: string; subjects?: string[] }> {
    const fd = new FormData()
    fd.append('file', file)
    const r = await apiPost<{ ok: boolean; name: string; subjects?: string[] }>('/api/ca/upload', fd)
    await loadCaCerts().catch(() => undefined)
    return r
  }

  async function deleteCa(name: string) {
    await apiPost('/api/ca/delete', { name })
    caCerts.value = caCerts.value.filter((c) => c.name !== name)
  }

  /** 拉取厂商模型清单；Claude 协议无公开列表接口 → null */
  async function fetchModels(p: Partial<{
    type: string
    base_url: string
    api_key: string
    provider: string
    model: string
  }> = {}): Promise<string[] | null> {
    const r = await apiPost<{ provider: string; type: string; models: string[] | null }>('/api/models', p)
    return r.models ?? null
  }

  const activeProvider = computed(() => {
    const s = settings.value
    if (!s) return null
    return (s.providers ?? []).find((p) => p.provider === s.provider) ?? null
  })

  return {
    settings,
    saving,
    lastSettingsError,
    activeProvider,
    workspace,
    version,
    tools,
    mcpError,
    shells,
    caCerts,
    requestGet,
    applyFromServer,
    markSaveFailed,
    save,
    loadWorkspace,
    setWorkspace,
    loadVersion,
    loadTools,
    setToolRule,
    deleteTool,
    loadShells,
    loadCaCerts,
    uploadCa,
    deleteCa,
    fetchModels,
  }
})
