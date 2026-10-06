<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUiStore } from '@/stores/ui'
import { useSessionStore } from '@/stores/session'
import { useTheme } from '@/composables/useTheme'
import AppIcon from '@/components/ui/AppIcon.vue'
import SessionList from '@/components/SessionList.vue'
import FileTree from '@/components/FileTree.vue'
import type { IconName } from '@/utils/icons'

/**
 * 外壳：左 264 + 中自适应 + 右 300（文件树）。**无顶栏**。
 *
 * 断点与 styles/shell.css 的媒体查询一一对应：
 *   ≥1280px    三栏常驻，左右均可折叠
 *   1024–1280  左常驻 240，右栏浮层
 *   768–1024   左抽屉，右栏浮层
 *   <768       单列，左右皆抽屉（触控目标 ≥44px）
 *   高度<500   手机横屏，按单列处理
 *
 * 主内容经默认插槽传入；本组件不渲染 RouterView，避免嵌套两层导致路由深度错位。
 */
const ui = useUiStore()
const session = useSessionStore()
// 解构后模板里才能自动解包（嵌套在对象里的 ref 不会自动 unwrap）
const { isDark, toggleMode } = useTheme()
const route = useRoute()
const router = useRouter()

const isChat = computed(() => route.name === 'chat')

const NAV: { name: string; label: string; icon: IconName; to: string }[] = [
  { name: 'chat', label: '对话', icon: 'chat', to: '/' },
  { name: 'files', label: '文件', icon: 'files', to: '/files' },
  { name: 'settings', label: '设置', icon: 'settings', to: '/settings' },
]

const currentLabel = computed(() => NAV.find((n) => n.name === route.name)?.label ?? '')

const connLabel = computed(() => {
  switch (session.connection) {
    case 'open':
      return '连接成功'
    case 'reconnecting':
      return '重连中'
    case 'closed':
      return '已断开'
    default:
      return '连接中'
  }
})

const connTone = computed(() => {
  if (session.connection === 'open') return 'bg-ok'
  if (session.connection === 'closed') return 'bg-danger'
  return 'bg-warn'
})

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') ui.closeAll()
}

/* ---------------- 软键盘 / 移动端底栏高度 ----------------
 * iOS 软键盘不触发 window.resize，只能靠 visualViewport。做法：
 *   键盘高度 = 布局视口高 - 可视视口高 - 可视视口顶部偏移
 * 把它写进 --kb-offset，底部固定的状态栏据此抬升，输入区自然跟着上移。
 */
const kbOffset = ref(0)

function syncViewport() {
  const vv = window.visualViewport
  if (!vv) {
    kbOffset.value = 0
  } else {
    const hidden = window.innerHeight - vv.height - vv.offsetTop
    // 阈值 80px：排除地址栏伸缩造成的小幅变化，只有真正的键盘才算
    kbOffset.value = hidden > 80 ? Math.round(hidden) : 0
  }
  /*
   * 只写 --kb-offset 一个变量。输入区不再依赖 --statusbar-h / --bottom-h：
   * 那两个值必须先量 DOM 才有，而量的时候元素可能尚未挂载 ——
   * 变量缺失会让 calc() 静默失效，输入框凭空消失且毫无报错。
   * 现在底部布局完全由 flex + dvh 决定，JS 只负责补键盘高度。
   */
  document.documentElement.style.setProperty('--kb-offset', `${kbOffset.value}px`)
}

// 切页自动收起抽屉，否则遮罩会挡住新页面
watch(
  () => route.fullPath,
  () => ui.closeAll(),
)

watch(
  () => ui.navOpen || ui.treeOpen,
  (open) => {
    document.body.style.overflow = open ? 'hidden' : ''
  },
)

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  syncViewport()
  window.visualViewport?.addEventListener('resize', syncViewport)
  window.visualViewport?.addEventListener('scroll', syncViewport)
  window.addEventListener('orientationchange', syncViewport)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  window.visualViewport?.removeEventListener('resize', syncViewport)
  window.visualViewport?.removeEventListener('scroll', syncViewport)
  window.removeEventListener('orientationchange', syncViewport)
  document.body.style.overflow = ''
})
</script>

<template>
  <div
    class="shell"
    :class="{
      'shell--nav-open': ui.navOpen,
      'shell--tree-open': ui.treeOpen,
      'shell--nav-collapsed': ui.navCollapsed,
      'shell--tree-collapsed': ui.treeCollapsed,
    }"
  >
    <!-- ============ 左侧栏（所有页面常驻） ============ -->
    <aside class="shell__nav" aria-label="侧边栏">
      <div class="shell__brand">
        <span class="shell__logo" aria-hidden="true">L</span>
        <span class="shell__name">licode</span>
        <button
          type="button"
          class="shell__icon-btn shell__nav-close"
          aria-label="收起侧边栏"
          @click="ui.closeAll()"
        >
          <AppIcon name="chevronLeft" class="size-4" />
        </button>
      </div>

      <!-- chat 页：会话列表；其它页：栏目名 + 返回对话 -->
      <SessionList v-if="isChat" />

      <div v-else class="shell__nav-body">
        <span class="shell__nav-label">{{ currentLabel }}</span>
        <button type="button" class="shell__nav-back" @click="router.push('/')">
          <AppIcon name="chat" class="size-4" />
          返回对话
        </button>
      </div>

      <!-- 导航贴底常驻 -->
      <nav class="shell__nav-links" aria-label="主导航">
        <RouterLink
          v-for="n in NAV"
          :key="n.name"
          :to="n.to"
          class="shell__nav-link"
          :class="{ 'is-active': route.name === n.name }"
          :aria-current="route.name === n.name ? 'page' : undefined"
        >
          <AppIcon :name="n.icon" class="size-4 shrink-0" />
          <span>{{ n.label }}</span>
        </RouterLink>
      </nav>

      <!-- 底部：连接状态 + 主题 -->
      <div class="shell__nav-foot">
        <span class="shell__conn" role="status">
          <span class="shell__conn-dot" :class="connTone" />
          <span class="shell__conn-text">{{ connLabel }}</span>
        </span>
        <RouterLink
          to="/diagnostics"
          class="shell__icon-btn ml-auto"
          aria-label="诊断"
          title="诊断"
        >
          <AppIcon name="info" class="size-4" />
        </RouterLink>
        <button
          type="button"
          class="shell__icon-btn"
          :aria-label="isDark ? '切换到浅色主题' : '切换到深色主题'"
          :title="isDark ? '浅色主题' : '深色主题'"
          @click="toggleMode()"
        >
          <AppIcon :name="isDark ? 'sun' : 'moon'" class="size-4" />
        </button>
      </div>
    </aside>

    <!-- ============ 中栏 ============ -->
    <main class="shell__main">
      <!-- 40px 上下文条：标题 + 运行状态 + 两侧开关 -->
      <div class="shell__topbar">
        <button
          type="button"
          class="shell__icon-btn shell__nav-toggle"
          aria-label="打开侧边栏"
          :aria-expanded="ui.navOpen"
          @click="ui.toggleNav()"
        >
          <AppIcon name="menu" class="size-4" />
        </button>

        <h1 class="shell__title">
          {{ isChat ? (session.active?.title ?? '未选择会话') : currentLabel }}
        </h1>

        <span v-if="session.runningCount > 0" class="shell__badge" :title="`${session.runningCount} 个会话正在生成`">
          <AppIcon name="spinner" class="size-3 animate-spin motion-reduce:animate-none" />
          {{ session.runningCount }} 个后台任务
        </span>

        <button
          type="button"
          class="shell__icon-btn shell__tree-toggle"
          aria-label="文件树"
          :aria-expanded="ui.treeOpen"
          @click="ui.toggleTree()"
        >
          <AppIcon name="files" class="size-4" />
        </button>
      </div>

      <div class="shell__content">
        <slot />
      </div>
    </main>

    <!-- ============ 右侧文件树 ============ -->
    <aside class="shell__tree" aria-label="文件树">
      <div class="shell__tree-head">
        <span class="shell__tree-title">文件</span>
        <button type="button" class="shell__icon-btn" aria-label="收起文件树" @click="ui.closeAll()">
          <AppIcon name="close" class="size-4" />
        </button>
      </div>
      <FileTree />
    </aside>

    <div
      v-if="ui.navOpen || ui.treeOpen"
      class="shell__scrim"
      aria-hidden="true"
      @click="ui.closeAll()"
    />
  </div>
</template>
