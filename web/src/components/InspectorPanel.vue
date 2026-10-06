<script setup lang="ts">
import { computed } from 'vue'
import { useSessionStore } from '@/stores/session'
import { formatCount } from '@/utils/format'
import AppIcon from '@/components/ui/AppIcon.vue'

/**
 * 右栏检视器：会话事实 + 本轮工具调用 + 审批队列。
 *
 * 只呈现后端真正下发的数据（stats 快照与 token 账目），不编造指标。
 * stats 的 context_pct 用于上下文压力预警：接近上限时后端会自动压缩，
 * 用户应当知道什么时候该开新会话。
 */
const session = useSessionStore()

const stats = computed(() => session.stats)
const tokens = computed(() => session.tokens)

const pct = computed(() => {
  const p = stats.value?.context_pct ?? 0
  return Math.max(0, Math.min(100, p))
})

const pctTone = computed(() => {
  if (pct.value >= 85) return 'bg-danger'
  if (pct.value >= 60) return 'bg-warn'
  return 'bg-accent'
})

/** 本轮（最后一条助手回合）里的工具调用 */
const lastTurnTools = computed(() => {
  const turns = session.turns
  for (let i = turns.length - 1; i >= 0; i -= 1) {
    if (turns[i].role !== 'assistant') continue
    return turns[i].blocks.filter((b) => b.kind === 'tool')
  }
  return []
})

const alwaysAllow = computed(() => stats.value?.always_allow ?? [])
</script>

<template>
  <div class="min-h-0 flex-1 overflow-y-auto">
    <section class="border-b border-line px-3 py-3">
      <p class="pb-2 text-[11px] text-ink-500">会话</p>
      <dl class="grid gap-1.5">
        <div class="flex items-baseline gap-3">
          <dt class="w-14 shrink-0 text-ink-500">模型</dt>
          <dd class="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-200" :title="stats?.model">
            {{ stats?.model || '—' }}
          </dd>
        </div>
        <div class="flex items-baseline gap-3">
          <dt class="w-14 shrink-0 text-ink-500">协议</dt>
          <dd class="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-200">{{ stats?.provider || '—' }}</dd>
        </div>
        <div class="flex items-baseline gap-3">
          <dt class="w-14 shrink-0 text-ink-500">消息</dt>
          <dd class="font-mono text-[12px] text-ink-200">{{ stats?.messages ?? 0 }}</dd>
        </div>
        <div class="flex items-baseline gap-3">
          <dt class="w-14 shrink-0 text-ink-500">生成</dt>
          <dd class="font-mono text-[12px]" :class="session.busy ? 'text-accent' : 'text-ink-200'">
            {{ session.busy ? '进行中' : '空闲' }}
          </dd>
        </div>
      </dl>
    </section>

    <section class="border-b border-line px-3 py-3">
      <div class="flex items-baseline justify-between pb-1.5">
        <p class="text-[11px] text-ink-500">上下文占用</p>
        <p class="font-mono text-[11px]" :class="pct >= 85 ? 'text-danger' : 'text-ink-300'">
          {{ formatCount(stats?.context_tokens) }} / {{ formatCount(stats?.context_max) }} · {{ pct }}%
        </p>
      </div>
      <div class="h-1 overflow-hidden rounded-full bg-white/[0.06]">
        <div class="h-full rounded-full transition-[width] duration-300" :class="pctTone" :style="{ width: pct + '%' }" />
      </div>
      <p v-if="pct >= 85" class="pt-1.5 text-[11px] leading-snug text-warn">
        接近上限，后端将自动压缩历史。建议新开会话。
      </p>

      <dl v-if="tokens" class="mt-2.5 grid gap-1.5">
        <div class="flex items-baseline gap-3">
          <dt class="w-14 shrink-0 text-ink-500">本轮入/出</dt>
          <dd class="font-mono text-[12px] text-ink-200">
            {{ formatCount(tokens.inputTokens) }} / {{ formatCount(tokens.outputTokens) }}
          </dd>
        </div>
        <div class="flex items-baseline gap-3">
          <dt class="w-14 shrink-0 text-ink-500">缓存命中</dt>
          <dd class="font-mono text-[12px] text-ink-200">
            {{ formatCount(tokens.cachedTokens) }}
            <span v-if="stats?.cache_hit_rate !== undefined" class="text-ink-500">
              · {{ stats.cache_hit_rate }}%
            </span>
          </dd>
        </div>
      </dl>
    </section>

    <section class="border-b border-line px-3 py-3">
      <p class="pb-2 text-[11px] text-ink-500">待审批</p>
      <div
        v-if="session.ask"
        class="rounded-lg border border-warn/40 px-2 py-1.5"
      >
        <p class="font-mono text-[12px] text-ink-200">{{ session.ask.toolName }}</p>
        <p class="mt-0.5 truncate text-[11px] text-ink-500">{{ session.summarize(session.ask.toolArgs) }}</p>
      </div>
      <p v-else class="text-ink-600">没有待审批的调用</p>

      <template v-if="alwaysAllow.length">
        <p class="pt-2.5 pb-1 text-[11px] text-ink-500">本会话已放行</p>
        <ul class="flex flex-wrap gap-1">
          <li
            v-for="t in alwaysAllow"
            :key="t"
            class="rounded border border-line px-1 font-mono text-[11px] text-ink-400"
          >
            {{ t }}
          </li>
        </ul>
      </template>
    </section>

    <section class="px-3 py-3">
      <p class="pb-2 text-[11px] text-ink-500">本轮工具调用</p>
      <ul v-if="lastTurnTools.length" class="grid gap-1.5">
        <li
          v-for="b in lastTurnTools"
          :key="b.id"
          class="flex items-center gap-1.5 rounded-lg border px-2 py-1.5"
          :class="b.kind === 'tool' && b.state === 'error' ? 'border-danger/35' : 'border-line'"
        >
          <AppIcon
            :name="b.kind === 'tool' ? (b.state === 'running' ? 'spinner' : b.state === 'error' ? 'error' : 'check') : 'check'"
            class="size-3 shrink-0"
            :class="[
              b.kind === 'tool' && b.state === 'running' ? 'animate-spin text-accent motion-reduce:animate-none' : '',
              b.kind === 'tool' && b.state === 'error' ? 'text-danger' : '',
              b.kind === 'tool' && b.state === 'success' ? 'text-ink-500' : '',
            ]"
          />
          <span class="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-300">{{ b.name }}</span>
        </li>
      </ul>
      <p v-else class="text-ink-600">本轮没有调用工具</p>
    </section>
  </div>
</template>
