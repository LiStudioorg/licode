<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AppShell from '@/components/AppShell.vue'
import ToolCard from '@/components/ToolCard.vue'
import ApprovalPrompt from '@/components/ApprovalPrompt.vue'
import ChatStates from '@/components/ChatStates.vue'
import SessionList from '@/components/SessionList.vue'
import { useSessionStore } from '@/stores/session'
import type { ApprovalDecision } from '@/types/tool'

const session = useSessionStore()
const draft = ref('')

/**
 * 三件套状态由真实连接状态驱动，不再有调试开关：
 *   - connecting/reconnecting 且无内容 -> 加载骨架
 *   - 连接错误或无会话 -> 错误态
 *   - 有会话但无消息 -> 空态
 */
const chatState = computed<'empty' | 'loading' | 'error' | 'ready'>(() => {
  const s = session.connection
  if (s === 'connecting' && session.timeline.length === 0) return 'loading'
  if (session.lastError) return 'error'
  if (s === 'closed' && session.timeline.length === 0) return 'error'
  return session.timeline.length === 0 ? 'empty' : 'ready'
})

function submit() {
  if (!draft.value.trim() || session.streaming) return
  session.send(draft.value)
  draft.value = ''
}

/** Enter 发送，Shift+Enter 换行；输入法组词期间不触发 */
function onKeydown(e: KeyboardEvent) {
  if (e.key !== 'Enter' || e.shiftKey || e.isComposing) return
  e.preventDefault()
  submit()
}

function decide(requestId: string, decision: ApprovalDecision) {
  session.decide(requestId, decision)
}

/** 待审批的工具名集合，用于在详情面板高亮 */
const pendingToolNames = computed(() => new Set(session.pending.map((p) => p.toolName)))

onMounted(() => session.connect())
onBeforeUnmount(() => session.disconnect())
</script>

<template>
  <AppShell>
    <!-- 左栏：搜索 + 会话列表（拖拽 / 右键菜单）+ 新建 -->
    <template #nav>
      <SessionList />
    </template>

    <!-- 中栏：会话头 + 消息流 + 输入区 -->
    <template #main>
      <div class="flex h-11 shrink-0 items-center gap-2 border-b border-line px-4">
        <h1 class="min-w-0 flex-1 truncate text-ink-100">
          {{ session.active?.title ?? '未选择会话' }}
        </h1>

        <!-- 连接状态：只呈现真实语义状态，无装饰 -->
        <span
          class="shrink-0 font-mono text-[11px]"
          :class="{
            'text-ink-500': session.connection === 'open',
            'text-warn': session.connection === 'connecting' || session.connection === 'reconnecting',
            'text-danger': session.connection === 'closed',
          }"
        >
          {{
            session.connection === 'open'
              ? '已连接'
              : session.connection === 'closed'
                ? '已断开'
                : '连接中'
          }}
        </span>

        <button
          v-if="session.streaming"
          type="button"
          class="shrink-0 rounded-lg border border-line px-1.5 py-0.5 text-[11px] text-ink-300 transition-colors hover:border-line-strong hover:bg-white/[0.04]"
          @click="session.interrupt()"
        >
          停止
        </button>
      </div>

      <div class="min-h-0 flex-1 overflow-y-auto px-4 py-5 sm:px-8">
        <ChatStates
          v-if="chatState !== 'ready'"
          :state="chatState"
          :error-message="session.lastError ?? '连接已断开，正在尝试重连'"
          @retry="session.connect()"
          @create="session.create()"
        />

        <div v-else class="mx-auto flex w-full max-w-[76ch] flex-col gap-4">
          <template v-for="item in session.timeline" :key="item.id">
            <article
              v-if="item.kind === 'message'"
              class="flex flex-col gap-1"
              :class="item.role === 'user' ? 'items-end' : 'items-start'"
            >
              <div class="flex items-center gap-2 px-1 text-[11px] text-ink-500">
                <span>{{ item.role === 'user' ? '你' : 'Licode' }}</span>
              </div>
              <p
                class="max-w-[68ch] rounded-lg px-3 py-2 whitespace-pre-wrap"
                :class="item.role === 'user' ? 'bg-white/[0.06] text-ink-100' : 'text-ink-200'"
              >
                {{ item.text }}
              </p>
            </article>

            <div v-else class="w-full">
              <ToolCard :call="item.call" />
            </div>
          </template>

          <!-- 流式尾部光标：有内容且仍在生成时提示未结束 -->
          <span
            v-if="session.streaming && session.timeline.length"
            class="inline-block h-3.5 w-1.5 animate-pulse bg-accent motion-reduce:animate-none"
            aria-hidden="true"
          />

          <ApprovalPrompt
            v-for="request in session.pending"
            :key="request.id"
            :request="request"
            @decide="(d) => decide(request.id, d)"
          />
        </div>
      </div>

      <div class="shrink-0 border-t border-line px-4 py-3 sm:px-8">
        <div class="mx-auto flex w-full max-w-[76ch] items-end gap-2">
          <label class="sr-only" for="composer">输入消息</label>
          <textarea
            id="composer"
            v-model="draft"
            rows="1"
            class="max-h-40 min-h-9 flex-1 resize-none rounded-lg border border-line bg-white/[0.02] px-3 py-2 text-ink-100 transition-colors hover:border-line-strong focus:border-accent-line focus:outline-none"
            @keydown="onKeydown"
          />
          <button
            type="button"
            class="grid size-9 shrink-0 place-items-center rounded-lg bg-accent text-canvas transition-opacity hover:opacity-90 active:scale-[0.97] disabled:opacity-40"
            :disabled="!draft.trim() || session.streaming"
            aria-label="发送"
            @click="submit"
          >
            <svg viewBox="0 0 16 16" class="size-4" fill="none" stroke="currentColor" stroke-width="1.4" aria-hidden="true">
              <path d="M8 13V3.5M8 3.5L4 7.5M8 3.5l4 4" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </button>
        </div>
      </div>
    </template>

    <!-- 右栏：会话事实 + 工具规则 + 本轮调用 -->
    <template #inspector>
      <div class="min-h-0 flex-1 overflow-y-auto">
        <section class="border-b border-line px-3 py-3">
          <p class="pb-2 text-[11px] text-ink-500">会话</p>
          <dl class="grid gap-1.5">
            <div class="flex items-baseline gap-3">
              <dt class="w-12 shrink-0 text-ink-500">状态</dt>
              <dd class="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-200">
                {{ session.connection === 'open' ? '已连接' : '未连接' }}
              </dd>
            </div>
            <div class="flex items-baseline gap-3">
              <dt class="w-12 shrink-0 text-ink-500">消息</dt>
              <dd class="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-200">
                {{ session.active?.count ?? 0 }}
              </dd>
            </div>
            <div class="flex items-baseline gap-3">
              <dt class="w-12 shrink-0 text-ink-500">生成</dt>
              <dd
                class="min-w-0 flex-1 truncate font-mono text-[12px]"
                :class="session.streaming ? 'text-accent' : 'text-ink-200'"
              >
                {{ session.streaming ? '进行中' : '空闲' }}
              </dd>
            </div>
          </dl>
        </section>

        <section class="border-b border-line px-3 py-3">
          <p class="pb-2 text-[11px] text-ink-500">待审批</p>
          <ul v-if="session.pending.length" class="grid gap-1.5">
            <li
              v-for="request in session.pending"
              :key="request.id"
              class="rounded-lg border border-warn/40 px-2 py-1.5"
            >
              <p class="font-mono text-[12px] text-ink-200">{{ request.toolName }}</p>
              <p class="mt-0.5 truncate text-[11px] text-ink-500">{{ request.detail }}</p>
            </li>
          </ul>
          <p v-else class="text-ink-500">没有待审批的调用</p>
        </section>

        <section class="px-3 py-3">
          <p class="pb-2 text-[11px] text-ink-500">本轮工具调用</p>
          <ul v-if="session.toolCalls.length" class="grid gap-1.5">
            <li
              v-for="item in session.toolCalls"
              :key="item.id"
              class="rounded-lg border px-2 py-1.5"
              :class="pendingToolNames.has(item.call.name) ? 'border-warn/40' : 'border-line'"
            >
              <p class="font-mono text-[12px] text-ink-200">{{ item.call.name }}</p>
              <p class="mt-0.5 truncate text-[11px] text-ink-500">{{ item.call.summary }}</p>
            </li>
          </ul>
          <p v-else class="text-ink-500">本轮没有调用工具</p>
        </section>
      </div>
    </template>
  </AppShell>
</template>
