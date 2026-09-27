<script setup lang="ts">
import { onBeforeUnmount, onMounted, watch } from 'vue'
import { useUiStore } from '@/stores/ui'

/**
 * 三栏外壳。
 *
 * 布局契约（每个断点的塌陷方式都是显式声明，不依赖“Tailwind 会处理”）：
 *   >= xl (1280px)：左栏 264px 常驻 + 中栏自适应 + 右栏 320px 常驻
 *   >= md (768px)：左栏常驻 + 中栏自适应，右栏收进抽屉
 *   <  md：单栏，左栏与右栏都是覆盖式抽屉
 *
 * 视觉契约：所有分隔只用品红发丝线（--color-line），
 * 分层只靠 glass 与 sunken 两级背景差，不用阴影堆叠。
 */
const ui = useUiStore()

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') ui.closeAll()
}

// 抽屉打开时锁滚动，避免背景跟着滚
watch(
  () => ui.navOpen || ui.inspectorOpen,
  (open) => {
    document.body.style.overflow = open ? 'hidden' : ''
  },
)

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  document.body.style.overflow = ''
})
</script>

<template>
  <div class="flex h-dvh flex-col overflow-hidden bg-canvas text-ink-100">
    <!-- 顶栏：唯一常驻的毛玻璃层，承担品牌与全局动作 -->
    <header
      class="glass relative z-30 flex h-12 shrink-0 items-center gap-2 border-b border-line px-3"
    >
      <button
        type="button"
        class="-ml-1 grid size-8 place-items-center rounded-lg text-ink-300 transition-colors hover:bg-white/[0.04] hover:text-ink-100 active:scale-[0.96] md:hidden"
        aria-label="打开会话列表"
        :aria-expanded="ui.navOpen"
        @click="ui.toggleNav()"
      >
        <svg viewBox="0 0 16 16" class="size-4" fill="none" stroke="currentColor" stroke-width="1.25">
          <path d="M2 4.5h12M2 8h12M2 11.5h8" stroke-linecap="round" />
        </svg>
      </button>

      <div class="flex min-w-0 items-center gap-2">
        <span
          class="grid size-5 shrink-0 place-items-center rounded-[6px] border border-accent-line bg-accent-soft font-mono text-[10px] leading-none text-accent"
          aria-hidden="true"
          >L</span
        >
        <span class="truncate text-ink-100">Licode</span>
      </div>

      <div class="ml-auto flex items-center gap-1">
        <button
          type="button"
          class="grid size-8 place-items-center rounded-lg text-ink-300 transition-colors hover:bg-white/[0.04] hover:text-ink-100 active:scale-[0.96] xl:hidden"
          aria-label="打开详情面板"
          :aria-expanded="ui.inspectorOpen"
          @click="ui.toggleInspector()"
        >
          <svg viewBox="0 0 16 16" class="size-4" fill="none" stroke="currentColor" stroke-width="1.25">
            <rect x="2" y="3" width="12" height="10" rx="2" />
            <path d="M10 3v10" />
          </svg>
        </button>

        <button
          type="button"
          class="grid size-8 place-items-center rounded-lg text-ink-300 transition-colors hover:bg-white/[0.04] hover:text-ink-100 active:scale-[0.96]"
          aria-label="设置"
        >
          <svg viewBox="0 0 16 16" class="size-4" fill="none" stroke="currentColor" stroke-width="1.25">
            <circle cx="8" cy="8" r="2.25" />
            <path
              d="M8 1.75v1.6M8 12.65v1.6M14.25 8h-1.6M3.35 8h-1.6M12.4 3.6l-1.13 1.13M4.73 11.27L3.6 12.4M12.4 12.4l-1.13-1.13M4.73 4.73L3.6 3.6"
              stroke-linecap="round"
            />
          </svg>
        </button>
      </div>
    </header>

    <div class="flex min-h-0 flex-1">
      <!-- 左栏：>= md 常驻；< md 覆盖式抽屉 -->
      <aside
        class="glass fixed inset-y-0 left-0 z-40 flex w-[264px] shrink-0 flex-col border-r border-line transition-transform duration-200 ease-out md:static md:z-auto md:translate-x-0"
        :class="ui.navOpen ? 'translate-x-0' : '-translate-x-full'"
        aria-label="会话列表"
      >
        <div class="flex items-center justify-between border-b border-line px-3 py-2 md:hidden">
          <span class="text-ink-300">会话</span>
          <button
            type="button"
            class="grid size-7 place-items-center rounded-lg text-ink-400 transition-colors hover:bg-white/[0.04] hover:text-ink-100"
            aria-label="关闭会话列表"
            @click="ui.closeAll()"
          >
            <svg viewBox="0 0 16 16" class="size-4" fill="none" stroke="currentColor" stroke-width="1.25">
              <path d="M4 4l8 8M12 4l-8 8" stroke-linecap="round" />
            </svg>
          </button>
        </div>
        <slot name="nav" />
      </aside>

      <!-- 中栏：唯一的自适应列 -->
      <main class="relative flex min-h-0 min-w-0 flex-1 flex-col bg-canvas">
        <slot name="main" />
      </main>

      <!-- 右栏：>= xl 常驻；其余宽度覆盖式抽屉 -->
      <aside
        class="glass fixed inset-y-0 right-0 z-40 flex w-[320px] shrink-0 flex-col border-l border-line transition-transform duration-200 ease-out xl:static xl:z-auto xl:translate-x-0"
        :class="ui.inspectorOpen ? 'translate-x-0' : 'translate-x-full'"
        aria-label="会话详情"
      >
        <div class="flex items-center justify-between border-b border-line px-3 py-2 xl:hidden">
          <span class="text-ink-300">详情</span>
          <button
            type="button"
            class="grid size-7 place-items-center rounded-lg text-ink-400 transition-colors hover:bg-white/[0.04] hover:text-ink-100"
            aria-label="关闭详情面板"
            @click="ui.closeAll()"
          >
            <svg viewBox="0 0 16 16" class="size-4" fill="none" stroke="currentColor" stroke-width="1.25">
              <path d="M4 4l8 8M12 4l-8 8" stroke-linecap="round" />
            </svg>
          </button>
        </div>
        <slot name="inspector" />
      </aside>

      <!-- 抽屉遮罩：只在窄屏出现，且仅覆盖中栏 -->
      <div
        v-if="ui.navOpen || ui.inspectorOpen"
        class="fixed inset-0 z-30 bg-sunken/70 md:hidden"
        aria-hidden="true"
        @click="ui.closeAll()"
      />
    </div>
  </div>
</template>
