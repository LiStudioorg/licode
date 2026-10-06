<script setup lang="ts">
import { computed, ref } from 'vue'
import { useSessionStore } from '@/stores/session'
import { formatMetric } from '@/utils/format'
import BaseModal from '@/components/ui/BaseModal.vue'
import { useTurnMetrics } from '@/composables/useTurnMetrics'

/**
 * 底部状态栏。固定格式：
 *   8轮 215步 · 273 tok/s · 30.7M tok · 缓存 96%
 *
 * 四个区域可点：
 *   轮/步 与 tok/s → 会话统计（模型用时、工具调用用时、首 tok 平均、输出速度）
 *   用量           → Token 用量（总用量、缓存命中、未缓存输入、缓存读取、输出）
 *   缓存           → 上下文已用（占比、已用/上限、系统提示词/工具定义/对话消息）
 *
 * 数据来源分两类：
 *   服务端——TokenStats（camelCase，无耗时字段）、SessionStats（snake_case）
 *   前端  ——耗时类指标由 useTurnMetrics 打点，刷新后丢失，此时显示 "-"
 */
const session = useSessionStore()
const metrics = useTurnMetrics()

const openPanel = ref<'none' | 'session' | 'token' | 'context'>('none')

const rounds = computed(() => session.turns.filter((t) => t.role === 'user').length)
const steps = computed(() =>
  session.turns.reduce((n, t) => n + t.blocks.filter((b) => b.kind === 'tool').length, 0),
)

/** 输出速度：前端打点得出，未观测到返回 null */
const tps = computed(() => metrics.tps.value)

const totalTokens = computed(() => {
  const t = session.tokens
  if (!t) return 0
  // 优先服务端本轮统计（含缓存读取），否则退化为会话累计
  const fromStats = (t.inputTokens ?? 0) + (t.outputTokens ?? 0)
  const fromConv = (session.stats?.conversation_in ?? 0) + (session.stats?.conversation_out ?? 0)
  return Math.max(fromStats, fromConv)
})

const cacheHit = computed(() => {
  const r = session.stats?.cache_hit_rate
  if (r === undefined || r === null) return null
  return r <= 1 ? Math.round(r * 100) : Math.round(r)
})

/* ---------------- 会话统计（全部前端打点） ---------------- */

function formatDurationCN(ms: number): string {
  const s = Math.round(ms / 1000)
  const m = Math.floor(s / 60)
  if (m <= 0) return `${s}秒`
  const h = Math.floor(m / 60)
  if (h <= 0) return `${m}分 ${s % 60}秒`
  return `${h}小时 ${m % 60}分`
}

const modelElapsed = computed(() =>
  metrics.modelMs.value ? formatDurationCN(metrics.modelMs.value) : '-',
)
const toolElapsed = computed(() =>
  metrics.toolTotalMs.value ? formatDurationCN(metrics.toolTotalMs.value) : '-',
)
const ttft = computed(() => {
  const v = metrics.ttftMs.value
  return v === null ? '-' : `${(v / 1000).toFixed(1)}秒`
})

/* ---------------- Token 用量 ---------------- */

const cachedRead = computed(() => session.tokens?.cachedTokens ?? session.stats?.usage_cached ?? 0)
const inputTokens = computed(() => session.tokens?.inputTokens ?? 0)
const uncachedInput = computed(() => Math.max(0, inputTokens.value - cachedRead.value))
const outputTokens = computed(() => session.tokens?.outputTokens ?? 0)

/* ---------------- 上下文 ---------------- */

const ctxUsed = computed(() => session.stats?.context_tokens ?? 0)
const ctxMax = computed(() => session.stats?.context_max ?? 0)
const ctxPct = computed(() => {
  const p = session.stats?.context_pct
  if (p !== undefined && p !== null) return p <= 1 ? Math.round(p * 100) : Math.round(p)
  if (!ctxMax.value) return 0
  return Math.round((ctxUsed.value / ctxMax.value) * 100)
})

/**
 * 上下文构成分解。服务端只给总量，不给分项，所以按可用信息估算：
 * 系统提示词与工具定义按常见量级固定，其余归对话消息。
 * 这三个数标注为估算，不当精确值展示。
 */
const sysTokens = computed(() => Math.round(ctxUsed.value * 0.012))
const toolTokens = computed(() => Math.round(ctxUsed.value * 0.043))
const msgTokens = computed(() => Math.max(0, ctxUsed.value - sysTokens.value - toolTokens.value))

const panelTitle = computed(
  () => ({ session: '会话统计', token: 'Token 用量', context: '上下文已用', none: '' })[openPanel.value],
)

/** 弹窗内用完整数字（千分位），状态栏用 M/K 缩写 */
const num = (n: number) => n.toLocaleString('en-US')
</script>

<template>
  <div class="shell__statusbar statusbar" role="status" aria-label="会话统计">
    <button type="button" class="statusbar__cell" @click="openPanel = 'session'">
      {{ rounds }}轮 {{ steps }}步
    </button>
    <span class="statusbar__sep">·</span>
    <button type="button" class="statusbar__cell" @click="openPanel = 'session'">
      {{ tps !== null ? `${tps} tok/s` : '- tok/s' }}
    </button>
    <span class="statusbar__sep">·</span>
    <button type="button" class="statusbar__cell" @click="openPanel = 'token'">
      {{ formatMetric(totalTokens) }} tok
    </button>
    <span class="statusbar__sep">·</span>
    <button type="button" class="statusbar__cell" @click="openPanel = 'context'">
      缓存 {{ cacheHit ?? 0 }}%
    </button>
  </div>

  <BaseModal :open="openPanel !== 'none'" :title="panelTitle" @close="openPanel = 'none'">
    <!-- 1. 会话统计 -->
    <dl v-if="openPanel === 'session'" class="sbdl">
      <div><dt>模型用时</dt><dd>{{ modelElapsed }}</dd></div>
      <div><dt>工具调用用时</dt><dd>{{ toolElapsed }}</dd></div>
      <div><dt>首 tok 平均 (TTFT)</dt><dd>{{ ttft }}</dd></div>
      <div><dt>输出速度 (TPS)</dt><dd>{{ tps !== null ? `${tps} tok/s` : '-' }}</dd></div>
      <div><dt>轮次</dt><dd>{{ rounds }}</dd></div>
      <div><dt>步数</dt><dd>{{ steps }}</dd></div>
      <p class="sbnote">耗时类指标由前端本次会话内打点，刷新页面后重新计。</p>
    </dl>

    <!-- 2. Token 用量 -->
    <dl v-else-if="openPanel === 'token'" class="sbdl">
      <div><dt>总用量</dt><dd>{{ num(totalTokens) }} tok</dd></div>
      <div><dt>缓存命中</dt><dd>{{ cacheHit ?? 0 }}%</dd></div>
      <div><dt>未缓存输入</dt><dd>{{ num(uncachedInput) }} tok</dd></div>
      <div><dt>缓存读取</dt><dd>{{ num(cachedRead) }} tok</dd></div>
      <div><dt>输出</dt><dd>{{ num(outputTokens) }} tok</dd></div>
    </dl>

    <!-- 3. 上下文已用 -->
    <div v-else-if="openPanel === 'context'">
      <p class="sbbig">{{ ctxPct }}%</p>
      <p class="sbsub">~{{ formatMetric(ctxUsed) }} / {{ formatMetric(ctxMax) }}</p>
      <div class="sbbar">
        <span :style="{ width: `${Math.min(100, ctxPct)}%` }" />
      </div>
      <dl class="sbdl sbdl--tight">
        <div><dt>系统提示词</dt><dd>~{{ formatMetric(sysTokens) }}</dd></div>
        <div><dt>工具定义</dt><dd>~{{ formatMetric(toolTokens) }}</dd></div>
        <div><dt>对话消息</dt><dd>~{{ formatMetric(msgTokens) }}</dd></div>
      </dl>
      <p class="sbnote">服务端只提供总量，分项为按比例估算，仅供参考。</p>
    </div>
  </BaseModal>
</template>

<style scoped>
.statusbar {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  height: 30px;
  padding: 0 8px;
  overflow-x: auto;
  border-top: 1px solid var(--color-shell-edge);
  background: var(--color-surface);
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 11px;
  color: var(--color-ink-400);
  white-space: nowrap;
}
.statusbar__cell {
  flex-shrink: 0;
  padding: 3px 6px;
  border-radius: 6px;
  color: inherit;
  transition: background 120ms var(--ease-std);
}
.statusbar__cell:hover {
  background: rgba(255, 255, 255, 0.06);
  color: var(--color-ink-100);
}
.statusbar__sep {
  flex-shrink: 0;
  color: var(--color-ink-700);
}

.sbdl {
  display: grid;
  gap: 9px;
}
.sbdl--tight {
  margin-top: 14px;
}
.sbdl > div {
  display: flex;
  align-items: baseline;
  gap: 12px;
}
.sbdl dt {
  width: 132px;
  flex-shrink: 0;
  color: var(--color-ink-500);
}
.sbdl dd {
  flex: 1;
  font-family: var(--font-mono, ui-monospace, monospace);
  color: var(--color-ink-100);
  text-align: right;
}
.sbnote {
  margin-top: 4px;
  font-size: 11px;
  line-height: 1.5;
  color: var(--color-ink-600);
}
.sbbig {
  font-size: 28px;
  line-height: 1.1;
  color: var(--color-ink-100);
}
.sbsub {
  margin-top: 2px;
  font-family: var(--font-mono, ui-monospace, monospace);
  color: var(--color-ink-500);
}
.sbbar {
  margin-top: 10px;
  height: 6px;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.07);
}
.sbbar > span {
  display: block;
  height: 100%;
  background: var(--color-accent);
}
</style>
