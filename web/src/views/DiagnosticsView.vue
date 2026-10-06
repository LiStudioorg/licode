<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useSessionStore } from '@/stores/session'
import { useSettingsStore } from '@/stores/settings'
import AppIcon from '@/components/ui/AppIcon.vue'

/**
 * 诊断页：运行时事实 + 能力清单。
 *
 * 会话统计（轮次/步数/token/缓存）不在这里 —— 那些归底部状态栏，
 * 点击即可展开详情，避免同一份数据两处维护。
 *
 * 刻意只做「展示后端如实告知的东西」，不做任何前端推算：
 *   - 工具与生效权限来自 /api/tools（EffectiveToolRule），不是设置页的历史配置；
 *   - /api/models 对 Claude 协议返回 null —— 那就显示「未提供」，绝不假装有；
 *   - /api/shells 在多数平台是空数组，同样如实显示。
 * 排障时这一页的价值就在于「后端到底认到了什么」一目了然。
 */
const session = useSessionStore()
const settings = useSettingsStore()

const loading = ref(true)
const err = ref('')
const toolQuery = ref('')

async function loadAll() {
  loading.value = true
  err.value = ''
  const results = await Promise.allSettled([
    settings.loadVersion(),
    settings.loadTools(),
    settings.loadShells(),
    settings.loadCaCerts(),
    settings.loadWorkspace(),
  ])
  const failed = results.filter((r) => r.status === 'rejected') as PromiseRejectedResult[]
  err.value = failed.length ? failed.map((f) => (f.reason as Error).message).join('；') : ''
  loading.value = false
}

onMounted(() => {
  void loadAll()
})

const filteredTools = computed(() => {
  const q = toolQuery.value.trim().toLowerCase()
  const list = settings.tools
  if (!q) return list
  return list.filter((t) => t.name.toLowerCase().includes(q) || t.description.toLowerCase().includes(q))
})

function ruleTone(rule: string): string {
  if (rule === 'allow') return 'border-ok/40 text-ok'
  if (rule === 'deny') return 'border-danger/40 text-danger'
  return 'border-warn/40 text-warn'
}

const connLabel = computed(() => {
  switch (session.connection) {
    case 'open':
      return '已连接'
    case 'reconnecting':
    case 'connecting':
      return '连接中'
    case 'closed':
      return '已断开'
    default:
      return '未开始'
  }
})

const connTone = computed(() => {
  if (session.connection === 'open') return 'text-ok'
  if (session.connection === 'closed') return 'text-danger'
  return 'text-warn'
})

/** 按来源分组：内置 / 子代理 / MCP / 外部 */
const toolsBySource = computed(() => {
  const map = new Map<string, number>()
  for (const t of settings.tools) map.set(t.source, (map.get(t.source) ?? 0) + 1)
  return [...map.entries()].sort((a, b) => b[1] - a[1])
})

const SOURCE_LABEL: Record<string, string> = {
  builtin: '内置',
  subagent: '子代理',
  mcp: 'MCP',
  external: '外部',
  skill: '技能',
}
</script>

<template>
  <div class="min-h-0 flex-1 overflow-y-auto px-4 py-5 sm:px-8">
    <div class="mx-auto flex w-full max-w-[78ch] flex-col gap-5">
      <header class="flex items-center gap-3">
        <div>
          <h1 class="text-ink-100 max-md:pl-9">诊断</h1>
          <p class="mt-0.5 text-ink-500">后端运行时状态与能力清单</p>
        </div>
        <button type="button" class="tb-btn ml-auto" :disabled="loading" @click="loadAll">
          <AppIcon name="refresh" class="size-3.5" :class="loading ? 'animate-spin motion-reduce:animate-none' : ''" />
          刷新
        </button>
      </header>

      <p v-if="err" class="rounded-lg border border-danger/40 px-3 py-2 text-danger">{{ err }}</p>

      <section class="card">
        <h2 class="card-title">运行状态</h2>
        <dl class="grid gap-2 sm:grid-cols-2">
          <div class="kv">
            <dt>连接</dt>
            <dd :class="connTone">{{ connLabel }}</dd>
          </div>
          <div class="kv">
            <dt>版本</dt>
            <dd class="mono">{{ settings.version?.version ?? '—' }}</dd>
          </div>
          <div class="kv">
            <dt>构建序号</dt>
            <dd class="mono">{{ settings.version?.counter ?? '—' }}</dd>
          </div>
          <div class="kv">
            <dt>运行中会话</dt>
            <dd class="mono">{{ session.runningCount }}</dd>
          </div>
          <div class="kv sm:col-span-2">
            <dt>工作目录</dt>
            <dd class="mono truncate" :title="settings.workspace.root">{{ settings.workspace.root || '—' }}</dd>
          </div>
        </dl>
      </section>

      <section class="card">
        <div class="flex items-center gap-2">
          <h2 class="card-title mb-0">工具（{{ settings.tools.length }}）</h2>
          <input
            v-model="toolQuery"
            type="search"
            placeholder="筛选"
            class="ml-auto h-7 w-36 rounded-lg border border-line bg-white/[0.02] px-2 text-ink-100 outline-none focus:border-accent-line sm:w-48"
          />
        </div>
        <p class="mt-1 text-ink-500">
          权限列是后端解析出的**生效值**（含只读工具恒 allow 与 <code class="mono">*</code> 兜底），
          与设置页里写的历史配置可能不同。
        </p>
        <ul v-if="toolsBySource.length" class="mt-2 flex flex-wrap gap-1.5">
          <li v-for="[src, n] in toolsBySource" :key="src" class="rounded border border-line px-1.5 py-0.5 text-[11px] text-ink-400">
            {{ SOURCE_LABEL[src] ?? src }} {{ n }}
          </li>
        </ul>
        <ul class="mt-2 divide-y divide-line/60">
          <li v-for="t in filteredTools" :key="t.name" class="flex items-start gap-2 py-1.5">
            <span class="shrink-0 font-mono text-[12px] text-ink-200">{{ t.name }}</span>
            <span class="shrink-0 rounded border px-1 font-mono text-[10px]" :class="ruleTone(t.rule)">{{ t.rule }}</span>
            <span class="min-w-0 flex-1 truncate text-ink-500" :title="t.description">{{ t.description }}</span>
          </li>
          <li v-if="!filteredTools.length" class="py-3 text-center text-ink-500">
            {{ loading ? '加载中…' : '没有匹配的工具' }}
          </li>
        </ul>
        <p v-if="settings.mcpError" class="mt-2 text-danger">MCP 异常：{{ settings.mcpError }}</p>
      </section>

      <section class="card">
        <h2 class="card-title">模型清单</h2>
        <p v-if="!settings.activeProvider?.models?.length" class="text-ink-500">
          当前厂商未提供模型清单（如 Claude 协议返回 null），可在设置页手动填写模型名。
        </p>
        <ul v-else class="flex flex-wrap gap-1.5">
          <li v-for="m in settings.activeProvider.models" :key="m" class="rounded border border-line px-1.5 py-0.5 font-mono text-[11px] text-ink-300">
            {{ m }}
          </li>
        </ul>
      </section>

      <section class="card">
        <h2 class="card-title">Shell（{{ settings.shells.length }}）</h2>
        <p v-if="!settings.shells.length" class="text-ink-500">
          后端未列出可用 shell（多数平台返回空数组，Agent 使用默认 /bin/sh）。
        </p>
        <ul v-else class="flex flex-wrap gap-1.5">
          <li v-for="s in settings.shells" :key="s" class="rounded border border-line px-1.5 py-0.5 font-mono text-[11px] text-ink-300">
            {{ s }}
          </li>
        </ul>
      </section>

      <section class="card">
        <h2 class="card-title">CA 证书（{{ settings.caCerts.length }}）</h2>
        <p v-if="!settings.caCerts.length" class="text-ink-500">没有额外导入的 CA 证书。</p>
        <ul v-else class="divide-y divide-line/60">
          <li v-for="c in settings.caCerts" :key="c.name" class="flex items-center gap-2 py-1.5">
            <AppIcon name="shield" class="size-3.5 shrink-0" :class="c.valid ? 'text-ink-500' : 'text-danger'" />
            <span class="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-200">{{ c.subjects?.[0] ?? c.name }}</span>
            <span class="shrink-0 font-mono text-[11px] text-ink-500">{{ c.valid ? c.mod_time : '无效' }}</span>
          </li>
        </ul>
      </section>

      <section class="card">
        <h2 class="card-title">会话安全上下文</h2>
        <dl class="grid gap-2 sm:grid-cols-2">
          <div class="kv">
            <dt>厂商 / 模型</dt>
            <dd class="mono truncate">{{ session.stats?.provider ?? '—' }} / {{ session.stats?.model ?? '—' }}</dd>
          </div>
          <div class="kv">
            <dt>本会话放行</dt>
            <dd class="mono truncate">{{ session.stats?.always_allow?.join(', ') || '—' }}</dd>
          </div>
        </dl>
        <p class="mt-2 text-ink-600">
          轮次 / 步数 / token 用量 / 缓存命中等统计见底部状态栏（点击可展开详情），此处不再重复。
        </p>
      </section>

      <footer class="pb-4 text-center text-ink-600">Licode · 单二进制 Web Agent · 前端产物离线自包含</footer>
    </div>
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
  margin-bottom: 8px;
  color: var(--color-ink-100);
}
.kv {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}
.kv dt {
  width: 84px;
  flex-shrink: 0;
  color: var(--color-ink-500);
}
.kv dd {
  min-width: 0;
  flex: 1;
  color: var(--color-ink-200);
}
.mono {
  font-family: var(--font-mono, ui-monospace, monospace);
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
