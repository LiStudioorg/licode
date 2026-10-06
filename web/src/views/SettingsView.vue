<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  MASKED_API_KEY,
  type DnsServer,
  type MCPServer,
  type Settings,
  type ToolRule,
} from '@/api/protocol'
import { useSessionStore } from '@/stores/session'
import { useSettingsStore } from '@/stores/settings'
import { ACCENT_PRESETS, useTheme } from '@/composables/useTheme'
import { useToast } from '@/composables/useToast'
import AppIcon from '@/components/ui/AppIcon.vue'
import BaseModal from '@/components/ui/BaseModal.vue'
import BaseConfirm from '@/components/ui/BaseConfirm.vue'

/**
 * 设置页。
 *
 * 最关键的契约：settings_set 是**全量替换**而不是增量合并。
 * 服务端把收到的 JSON 反序列化成一个全新的 Settings，只对零值字段补默认，
 * 其余以收到的为准。所以本页必须：
 *   1. 以服务端快照为基底维护一个「工作副本」，只改用户显式改过的字段；
 *   2. api_key 回传时保留 "********" 占位（服务端据此还原真实密钥），
 *      绝不能把占位符变成空串 —— 那等于悄悄清空密钥；
 *   3. 本页不提供的字段（providers 列表细节、mcp_servers…）原样透传。
 *
 * 设置的读写**只走 WebSocket**（settings_get / settings_set），
 * 没有 REST 端点，因此本页依赖 session store 的连接状态。
 */
const session = useSessionStore()
const settings = useSettingsStore()
const theme = useTheme()
const toast = useToast()

/** 本地工作副本；null = 尚未拿到服务端快照 */
const draft = ref<Settings | null>(null)

/** 脏判定：与「载入时的快照」比较，而不是与随时可能被服务端刷新的 settings 比较 */
let baseline = ''

function snapshot(s: Settings): string {
  const clone = JSON.parse(JSON.stringify(s)) as Record<string, unknown>
  return JSON.stringify(clone)
}

const dirty = computed(() => draft.value !== null && snapshot(draft.value) !== baseline)

watch(
  () => settings.settings,
  (s) => {
    // 只在「没有未保存改动」时用服务端快照刷新，避免覆盖用户正在编辑的内容
    if (s && !dirty.value) adopt(s)
  },
  { immediate: true },
)

function adopt(s: Settings) {
  draft.value = JSON.parse(JSON.stringify(s)) as Settings
  ensureShape(draft.value)
  baseline = snapshot(draft.value)
}

/** 补齐可选容器，模板里就不用到处写 `?.` 与 `!` */
function ensureShape(s: Settings) {
  if (!s.providers) s.providers = []
  if (!s.mcp_servers) s.mcp_servers = []
  if (!s.tool_rules) s.tool_rules = {}
  if (!s.dns) s.dns = { servers: [] }
  if (!s.dns.servers) s.dns.servers = []
}

onMounted(() => {
  // 设置本身由 session store 在连接建立时自动 settings_get，这里只拉 REST 侧元数据
  void settings.loadTools().catch(() => undefined)
  void settings.loadShells().catch(() => undefined)
  void settings.loadCaCerts().catch(() => undefined)
})

/** 当前草稿里对应 provider 的配置项（api_key/base_url 的落点） */
const providerEntry = computed(() => {
  const d = draft.value
  if (!d) return null
  return d.providers?.find((p) => p.provider === d.provider) ?? null
})

const modelOptions = computed(() => providerEntry.value?.models ?? [])

/**
 * base_url / api_key 同时写进顶层与当前 provider 条目：
 * 后端读取时以 provider 条目为准，但顶层字段在旧配置里仍可能是唯一来源，
 * 两边都写才不会出现「改了没生效」。
 */
function onBaseUrl(v: string) {
  const d = draft.value
  if (!d) return
  d.base_url = v
  const p = providerEntry.value
  if (p) p.base_url = v
}

function onApiKey(v: string) {
  const d = draft.value
  if (!d) return
  d.api_key = v
  const p = providerEntry.value
  if (p) p.api_key = v
}

async function pullModels() {
  try {
    const models = await settings.fetchModels({ provider: draft.value?.provider })
    if (!models?.length) {
      toast.info('当前厂商未提供模型清单，请手动填写')
      return
    }
    const p = providerEntry.value
    if (p) p.models = models
    toast.ok(`已获取 ${models.length} 个模型`)
  } catch (e) {
    toast.error((e as Error).message)
  }
}

/* ---------------- 工具权限 ---------------- */

const RULES: { value: ToolRule; label: string }[] = [
  { value: 'allow', label: '允许' },
  { value: 'ask', label: '询问' },
  { value: 'deny', label: '拒绝' },
]

const ruleRows = computed(() =>
  Object.entries(draft.value?.tool_rules ?? {})
    .map(([name, rule]) => ({ name, rule }))
    .sort((a, b) => a.name.localeCompare(b.name)),
)

function setRule(name: string, rule: ToolRule) {
  const d = draft.value
  if (!d) return
  d.tool_rules = { ...(d.tool_rules ?? {}), [name]: rule }
}

const newRuleName = ref('')
function addRule() {
  const n = newRuleName.value.trim()
  if (!n) return
  setRule(n, 'ask')
  newRuleName.value = ''
}
function removeRule(name: string) {
  const d = draft.value
  if (!d?.tool_rules) return
  const next = { ...d.tool_rules }
  delete next[name]
  d.tool_rules = next
}

/* ---------------- DNS ---------------- */

const dnsServers = computed<DnsServer[]>(() => draft.value?.dns?.servers ?? [])
const newDns = ref('')

function addDns() {
  const s = newDns.value.trim()
  const dns = draft.value?.dns
  if (!s || !dns) return
  dns.servers = [...(dns.servers ?? []), { mode: 'plain', server: s }]
  newDns.value = ''
}
function removeDns(i: number) {
  const dns = draft.value?.dns
  if (!dns) return
  const next = [...(dns.servers ?? [])]
  next.splice(i, 1)
  dns.servers = next
}

/* ---------------- MCP ---------------- */

const mcpOpen = ref(false)
const mcpDraft = ref({ name: '', type: 'stdio', command: '', args: '', url: '' })

function addMcp() {
  const d = draft.value
  const m = mcpDraft.value
  if (!d || !m.name.trim()) return
  const entry: MCPServer = {
    name: m.name.trim(),
    type: m.type,
    command: m.command.trim(),
    args: m.args.trim() ? m.args.trim().split(/\s+/) : [],
    url: m.url.trim(),
  }
  d.mcp_servers = [...(d.mcp_servers ?? []), entry]
  mcpOpen.value = false
  mcpDraft.value = { name: '', type: 'stdio', command: '', args: '', url: '' }
}
function removeMcp(i: number) {
  const d = draft.value
  if (!d?.mcp_servers) return
  const next = [...d.mcp_servers]
  next.splice(i, 1)
  d.mcp_servers = next
}

/* ---------------- CA 证书 ---------------- */

const caInput = ref<HTMLInputElement | null>(null)
async function onCaPicked(files: FileList | null) {
  const f = files?.[0]
  if (!f) return
  try {
    await settings.uploadCa(f)
    toast.ok('证书已导入')
  } catch (e) {
    toast.error((e as Error).message)
  } finally {
    if (caInput.value) caInput.value.value = ''
  }
}

const caPendingDelete = ref<string | null>(null)
async function confirmDeleteCa() {
  const name = caPendingDelete.value
  caPendingDelete.value = null
  if (!name) return
  try {
    await settings.deleteCa(name)
    toast.ok('证书已删除')
  } catch (e) {
    toast.error((e as Error).message)
  }
}

/* ---------------- 保存 ---------------- */

const saving = computed(() => session.savingSettings || settings.saving)

async function save() {
  const d = draft.value
  if (!d) return
  // 掩码占位与「用户清空了密钥输入框」都按「保持原密钥」处理
  if (d.api_key === '') d.api_key = MASKED_API_KEY
  for (const p of d.providers ?? []) if (p.api_key === '') p.api_key = MASKED_API_KEY
  session.saveSettings(d)
  toast.ok('已提交保存')
}

function discard() {
  if (settings.settings) adopt(settings.settings)
}
</script>

<template>
  <div class="min-h-0 flex-1 overflow-y-auto px-4 py-5 sm:px-8">
    <div class="mx-auto flex w-full max-w-[78ch] flex-col gap-5">
      <header class="flex items-center gap-3">
        <div>
          <h1 class="text-ink-100 max-md:pl-9">设置</h1>
          <p class="mt-0.5 text-ink-500">保存时整体回传全部字段（服务端为全量替换语义）</p>
        </div>
        <div class="ml-auto flex items-center gap-2">
          <button v-if="dirty" type="button" class="tb-btn" @click="discard">放弃修改</button>
          <button
            type="button"
            class="rounded-lg bg-accent px-3 py-1.5 text-[var(--color-accent-ink)] transition-opacity hover:opacity-90 disabled:opacity-40"
            :disabled="!draft || saving"
            @click="save"
          >
            {{ saving ? '保存中…' : dirty ? '保存' : '已保存' }}
          </button>
        </div>
      </header>

      <p
        v-if="session.connection !== 'open'"
        class="rounded-lg border border-warn/40 bg-warn/5 px-3 py-2 text-warn"
      >
        当前未连接：设置的读写依赖 WebSocket，请等待重连后再生效。
      </p>
      <p v-if="session.lastSettingsError" class="rounded-lg border border-danger/40 px-3 py-2 text-danger">
        {{ session.lastSettingsError }}
      </p>
      <p v-if="!draft" class="py-10 text-center text-ink-500">正在读取设置…</p>

      <template v-else>
        <!-- 模型服务 -->
        <section class="card">
          <h2 class="card-title">模型服务</h2>
          <div class="grid gap-3 sm:grid-cols-2">
            <div class="field">
              <label for="provider">厂商</label>
              <input id="provider" v-model="draft.provider" class="input mono" placeholder="openai" />
              <p class="hint">内置候选：openai / claude / ollama / gemini</p>
            </div>
            <div class="field">
              <label for="model">模型</label>
              <input id="model" v-model="draft.model" class="input mono" list="model-list" placeholder="gpt-4o-mini" />
              <datalist id="model-list">
                <option v-for="m in modelOptions" :key="m" :value="m" />
              </datalist>
              <p class="hint">
                <button type="button" class="link" @click="pullModels">拉取模型清单</button>
                （部分厂商不提供）
              </p>
            </div>
            <div class="field sm:col-span-2">
              <label for="base-url">Base URL</label>
              <input
                id="base-url"
                :value="providerEntry?.base_url ?? draft.base_url ?? ''"
                class="input mono"
                placeholder="https://api.openai.com/v1"
                @input="onBaseUrl(($event.target as HTMLInputElement).value)"
              />
            </div>
            <div class="field sm:col-span-2">
              <label for="api-key">API Key</label>
              <input
                id="api-key"
                :value="providerEntry?.api_key ?? draft.api_key ?? ''"
                type="password"
                autocomplete="off"
                class="input mono"
                @input="onApiKey(($event.target as HTMLInputElement).value)"
              />
              <p class="hint">
                已保存的密钥显示为 <code class="mono">{{ MASKED_API_KEY }}</code
                >，保持原样即不修改
              </p>
            </div>
          </div>
        </section>

        <!-- 生成 -->
        <section class="card">
          <h2 class="card-title">生成</h2>
          <div class="grid gap-3 sm:grid-cols-2">
            <div class="field">
              <label for="temperature">Temperature（0 = 默认 1）</label>
              <input id="temperature" v-model.number="draft.temperature" type="number" step="0.1" min="0" max="2" class="input" />
            </div>
            <div class="field">
              <label for="max-tokens">上下文预算 max_tokens</label>
              <input id="max-tokens" v-model.number="draft.max_tokens" type="number" min="0" class="input" />
              <p class="hint">请求前按厂商上限钳制（Claude 32000 / OpenAI 16384 / Gemini 8192）</p>
            </div>
            <div class="field">
              <label for="max-iter">最大迭代次数（0 = 默认 16）</label>
              <input id="max-iter" v-model.number="draft.max_iterations" type="number" min="0" class="input" />
            </div>
            <div class="field">
              <label for="sub-timeout">子任务超时（秒，0 = 不限）</label>
              <input id="sub-timeout" v-model.number="draft.sub_timeout" type="number" min="0" class="input" />
            </div>
            <label class="toggle"><input v-model="draft.streaming" type="checkbox" /><span>流式输出</span></label>
            <label class="toggle"><input v-model="draft.title_gen" type="checkbox" /><span>自动生成标题</span></label>
            <label class="toggle"><input v-model="draft.subagents" type="checkbox" /><span>启用子代理</span></label>
            <label class="toggle"><input v-model="draft.compaction" type="checkbox" /><span>历史压缩</span></label>
          </div>
        </section>

        <!-- 工具权限 -->
        <section class="card">
          <h2 class="card-title">工具权限</h2>
          <p class="mb-2 text-ink-500">
            仅 allow / ask / deny 三档；通配符 <code class="mono">*</code> 作为未列出工具的兜底。
            只读工具（Read/ListDirectory/Glob/Grep）恒为 allow。实际生效值见「信息」页。
          </p>
          <ul class="divide-y divide-line/60">
            <li v-for="t in ruleRows" :key="t.name" class="flex items-center gap-2 py-1.5">
              <span class="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-200">{{ t.name }}</span>
              <div class="flex shrink-0 gap-0.5 rounded-lg border border-line p-0.5">
                <button
                  v-for="r in RULES"
                  :key="r.value"
                  type="button"
                  class="rounded px-2 py-0.5 text-[11px] transition-colors"
                  :class="t.rule === r.value ? 'bg-white/[0.08] text-ink-100' : 'text-ink-500 hover:text-ink-200'"
                  @click="setRule(t.name, r.value)"
                >
                  {{ r.label }}
                </button>
              </div>
              <button
                type="button"
                class="grid size-6 shrink-0 place-items-center rounded text-ink-500 transition-colors hover:text-danger"
                aria-label="移除该规则"
                @click="removeRule(t.name)"
              >
                <AppIcon name="close" class="size-3" />
              </button>
            </li>
            <li v-if="!ruleRows.length" class="py-3 text-center text-ink-500">未配置规则，使用服务端默认（写/命令/删除为询问）</li>
          </ul>
          <div class="mt-2 flex items-center gap-2">
            <input
              v-model="newRuleName"
              class="input h-7 flex-1 font-mono"
              placeholder="工具名，如 Shell；* 表示兜底"
              @keydown.enter.prevent="addRule"
            />
            <button type="button" class="tb-btn" @click="addRule">添加</button>
          </div>

          <div class="mt-3 grid gap-3 sm:grid-cols-2">
            <label class="toggle"><input v-model="draft.auto_allow" type="checkbox" /><span>跳过所有询问（auto_allow）</span></label>
            <label class="toggle"><input v-model="draft.tool_auto_retry" type="checkbox" /><span>工具无结果时自动重试</span></label>
            <div class="field">
              <label for="retry-max">重试次数（0 = 默认 3）</label>
              <input id="retry-max" v-model.number="draft.tool_retry_max" type="number" min="0" class="input" />
            </div>
            <div class="field">
              <label for="shell-path">Shell 路径（空 = /bin/sh）</label>
              <input id="shell-path" v-model="draft.shell_path" class="input mono" placeholder="/bin/sh" />
            </div>
          </div>
        </section>

        <!-- 网络与缓存 -->
        <section class="card">
          <h2 class="card-title">网络与缓存</h2>
          <div class="grid gap-3 sm:grid-cols-2">
            <div class="field sm:col-span-2">
              <label for="dns-mode">DNS 模式（空 = 自动）</label>
              <input id="dns-mode" v-model="draft.dns!.mode" class="input mono" placeholder="system / custom / command" />
            </div>
            <div class="field sm:col-span-2">
              <label>DNS 服务器</label>
              <ul v-if="dnsServers.length" class="mb-1.5 flex flex-wrap gap-1.5">
                <li
                  v-for="(s, i) in dnsServers"
                  :key="i"
                  class="flex items-center gap-1 rounded border border-line px-1.5 py-0.5 font-mono text-[11px] text-ink-300"
                >
                  <span class="text-ink-500">{{ s.mode }}</span>
                  <span>{{ s.server }}</span>
                  <button type="button" class="text-ink-500 hover:text-danger" aria-label="移除" @click="removeDns(i)">
                    <AppIcon name="close" class="size-2.5" />
                  </button>
                </li>
              </ul>
              <p v-else class="mb-1.5 text-ink-600">未配置，使用默认服务器</p>
              <div class="flex items-center gap-2">
                <input v-model="newDns" class="input h-7 flex-1 font-mono" placeholder="223.5.5.5:53" @keydown.enter.prevent="addDns" />
                <button type="button" class="tb-btn" @click="addDns">添加</button>
              </div>
            </div>
            <div class="field">
              <label for="dns-conc">并发数（0 = 默认 2，-1 = 全部）</label>
              <input id="dns-conc" v-model.number="draft.dns!.concurrency" type="number" class="input" />
            </div>
            <div class="field">
              <label for="dns-timeout">超时（毫秒，0 = 5000）</label>
              <input id="dns-timeout" v-model.number="draft.dns!.timeout_ms" type="number" min="0" class="input" />
            </div>
            <div class="field">
              <label for="cache-ttl">缓存 TTL（秒，0 = 3600）</label>
              <input id="cache-ttl" v-model.number="draft.cache_ttl" type="number" min="0" class="input" />
            </div>
            <div class="field">
              <label for="keepalive">保活间隔（秒，0 = 关闭）</label>
              <input id="keepalive" v-model.number="draft.keepalive_sec" type="number" min="0" class="input" />
            </div>
            <div class="field">
              <label for="shutdown">关停超时（秒，0 = 30）</label>
              <input id="shutdown" v-model.number="draft.shutdown_timeout" type="number" min="0" class="input" />
            </div>
            <label class="toggle"><input v-model="draft.cache_enabled" type="checkbox" /><span>启用结果缓存</span></label>
            <label class="toggle"><input v-model="draft.prompt_cache" type="checkbox" /><span>Prompt 缓存（仅 Claude）</span></label>
            <label class="toggle"><input v-model="draft.redact_secrets" type="checkbox" /><span>输出中脱敏密钥</span></label>
          </div>
        </section>

        <!-- RAG -->
        <section class="card">
          <h2 class="card-title">检索增强（RAG）</h2>
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="toggle"><input v-model="draft.rag_enabled" type="checkbox" /><span>启用 RAG</span></label>
            <div class="field">
              <label for="rag-top">返回文件数（0 = 默认 5）</label>
              <input id="rag-top" v-model.number="draft.rag_top_files" type="number" min="0" class="input" />
            </div>
            <div class="field sm:col-span-2">
              <label for="rag-src">索引来源（空 = 工作目录）</label>
              <input id="rag-src" v-model="draft.rag_source" class="input mono" />
            </div>
          </div>
        </section>

        <!-- MCP -->
        <section class="card">
          <div class="flex items-center gap-2">
            <h2 class="card-title mb-0">MCP 服务（{{ draft.mcp_servers?.length ?? 0 }}）</h2>
            <button type="button" class="tb-btn ml-auto" @click="mcpOpen = true">
              <AppIcon name="plus" class="size-3" />
              添加
            </button>
          </div>
          <ul v-if="draft.mcp_servers?.length" class="mt-2 divide-y divide-line/60">
            <li v-for="(m, i) in draft.mcp_servers" :key="i" class="flex items-center gap-2 py-1.5">
              <span class="shrink-0 font-mono text-[12px] text-ink-200">{{ m.name }}</span>
              <span class="shrink-0 rounded border border-line px-1 font-mono text-[10px] text-ink-400">{{ m.type || 'stdio' }}</span>
              <span class="min-w-0 flex-1 truncate font-mono text-[11px] text-ink-500">{{ m.command || m.url }}</span>
              <button
                type="button"
                class="grid size-6 shrink-0 place-items-center rounded text-ink-500 transition-colors hover:text-danger"
                aria-label="删除"
                @click="removeMcp(i)"
              >
                <AppIcon name="trash" class="size-3" />
              </button>
            </li>
          </ul>
          <p v-else class="mt-1 text-ink-500">未配置 MCP 服务。</p>
          <p v-if="settings.mcpError" class="mt-1 text-danger">MCP 加载异常：{{ settings.mcpError }}</p>
        </section>

        <!-- CA -->
        <section class="card">
          <div class="flex items-center gap-2">
            <h2 class="card-title mb-0">自签 CA 证书（{{ settings.caCerts.length }}）</h2>
            <button type="button" class="tb-btn ml-auto" @click="caInput?.click()">
              <AppIcon name="upload" class="size-3" />
              导入
            </button>
            <input ref="caInput" type="file" class="sr-only" accept=".pem,.crt,.cer,.key" @change="onCaPicked(($event.target as HTMLInputElement).files)" />
          </div>
          <ul v-if="settings.caCerts.length" class="mt-2 divide-y divide-line/60">
            <li v-for="c in settings.caCerts" :key="c.name" class="flex items-center gap-2 py-1.5">
              <span class="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-200">{{ c.subjects?.[0] ?? c.name }}</span>
              <span class="shrink-0 font-mono text-[11px]" :class="c.valid ? 'text-ink-500' : 'text-danger'">
                {{ c.valid ? c.mod_time : '无效' }}
              </span>
              <button
                type="button"
                class="grid size-6 shrink-0 place-items-center rounded text-ink-500 transition-colors hover:text-danger"
                aria-label="删除证书"
                @click="caPendingDelete = c.name"
              >
                <AppIcon name="trash" class="size-3" />
              </button>
            </li>
          </ul>
          <p v-else class="mt-1 text-ink-500">用于访问自签 HTTPS 的模型服务或 MCP。</p>
        </section>

        <!-- 外观（纯前端） -->
        <section class="card">
          <h2 class="card-title">外观</h2>
          <div class="grid gap-3 sm:grid-cols-2">
            <div class="field">
              <label>主题模式</label>
              <div class="flex gap-0.5 rounded-lg border border-line p-0.5">
                <button
                  type="button"
                  class="flex-1 rounded px-2 py-1 text-[11px] transition-colors"
                  :class="!theme.isDark.value ? 'bg-white/[0.08] text-ink-100' : 'text-ink-500 hover:text-ink-200'"
                  @click="theme.setMode('light')"
                >
                  浅色
                </button>
                <button
                  type="button"
                  class="flex-1 rounded px-2 py-1 text-[11px] transition-colors"
                  :class="theme.isDark.value ? 'bg-white/[0.08] text-ink-100' : 'text-ink-500 hover:text-ink-200'"
                  @click="theme.setMode('dark')"
                >
                  深色
                </button>
              </div>
            </div>
            <div class="field">
              <label>强调色</label>
              <div class="flex flex-wrap gap-1.5">
                <button
                  v-for="a in ACCENT_PRESETS"
                  :key="a.id"
                  type="button"
                  class="size-6 rounded-full border transition-transform hover:scale-110"
                  :class="theme.accent.value === a.accent ? 'border-ink-100' : 'border-line'"
                  :style="{ background: a.accent }"
                  :title="a.label"
                  :aria-label="a.label"
                  @click="theme.setAccent(a.accent)"
                />
              </div>
            </div>
          </div>
          <p class="mt-2 text-ink-600">主题只存本机 localStorage —— 设置接口里没有主题字段。</p>
        </section>
      </template>
    </div>

    <BaseModal :open="mcpOpen" title="添加 MCP 服务" @close="mcpOpen = false">
      <div class="grid gap-3">
        <div class="field">
          <label for="mcp-name">名称</label>
          <input id="mcp-name" v-model="mcpDraft.name" class="input mono" placeholder="filesystem" />
        </div>
        <div class="field">
          <label for="mcp-type">类型</label>
          <input id="mcp-type" v-model="mcpDraft.type" class="input mono" placeholder="stdio / http" />
        </div>
        <template v-if="mcpDraft.type !== 'http'">
          <div class="field">
            <label for="mcp-cmd">命令</label>
            <input id="mcp-cmd" v-model="mcpDraft.command" class="input mono" placeholder="npx" />
          </div>
          <div class="field">
            <label for="mcp-args">参数（空格分隔）</label>
            <input id="mcp-args" v-model="mcpDraft.args" class="input mono" placeholder="-y @modelcontextprotocol/server-filesystem /tmp" />
          </div>
        </template>
        <div v-else class="field">
          <label for="mcp-url">URL</label>
          <input id="mcp-url" v-model="mcpDraft.url" class="input mono" placeholder="http://127.0.0.1:3000/mcp" />
        </div>
      </div>
      <template #footer>
        <button type="button" class="tb-btn" @click="mcpOpen = false">取消</button>
        <button type="button" class="rounded-lg bg-accent px-3 py-1 text-[var(--color-accent-ink)] hover:opacity-90" @click="addMcp">
          添加
        </button>
      </template>
    </BaseModal>

    <BaseConfirm
      :open="caPendingDelete !== null"
      title="删除 CA 证书？"
      :message="`将移除「${caPendingDelete ?? ''}」`"
      confirm-text="删除"
      danger
      @update:open="(v) => { if (!v) caPendingDelete = null }"
      @confirm="confirmDeleteCa"
    />
  </div>
</template>

<style scoped>
.card {
  border: 1px solid var(--color-line);
  border-radius: var(--radius-card);
  background: rgba(255, 255, 255, 0.02);
  padding: 14px;
}
.card-title {
  margin-bottom: 10px;
  color: var(--color-ink-100);
}
.field label {
  display: block;
  margin-bottom: 4px;
  color: var(--color-ink-400);
}
.input {
  height: 30px;
  width: 100%;
  border: 1px solid var(--color-line);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.02);
  padding: 0 8px;
  color: var(--color-ink-100);
  outline: none;
}
.input:focus {
  border-color: var(--color-accent-line);
}
.mono {
  font-family: var(--font-mono, ui-monospace, monospace);
}
.hint {
  margin-top: 3px;
  color: var(--color-ink-600);
}
.toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--color-ink-300);
  cursor: pointer;
}
.toggle input {
  accent-color: var(--color-accent);
}
.link {
  color: var(--color-accent);
  text-decoration: underline;
}
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
</style>
