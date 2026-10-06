<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { useSettingsStore } from '@/stores/settings'
import AppShell from '@/components/AppShell.vue'
import ToastHost from '@/components/ui/ToastHost.vue'

/**
 * 应用根。
 *
 * 两个只有在这里才成立的职责：
 *   1. 全局只建立**一条** WebSocket —— 它属于应用生命周期，不属于任何页面。
 *      放在 created 而不是某个 view 的 onMounted，切页时连接才不会断。
 *   2. 把 session store 的 settings 事件桥接到 settings store（两者互相
 *      不知道对方存在，避免 store 间循环依赖）。
 *
 * 登录页不套 AppShell：没有会话、没有侧栏，独立一屏。
 */
const route = useRoute()
const session = useSessionStore()
const settings = useSettingsStore()

const isLogin = computed(() => route.name === 'login')

session.bindSettings({
  onSettings: (s) => {
    settings.applyFromServer(s ?? undefined)
  },
})

onMounted(() => {
  // 登录页不连接：未登录时 /ws 会 403/401，连了也只是刷错误日志
  if (!isLogin.value) session.connect()
})

watch(isLogin, (login) => {
  if (login) session.disconnect()
  else session.connect()
})

/** 切页时收起移动端抽屉，否则遮罩会挡住新页面 */
watch(
  () => route.fullPath,
  () => {
    /* ui store 的抽屉由 AppShell 内部在路由变化时关闭 */
  },
)

onBeforeUnmount(() => {
  session.disconnect()
})
</script>

<template>
  <RouterView v-if="isLogin" />
  <template v-else>
    <!--
      AppShell 只负责布局，主内容经默认插槽传入。
      这里**不能**再套一层 <RouterView>：AppShell 内部已经有 RouterView 的
      话就变成嵌套两层，路由匹配深度会错位（白屏根因）。
    -->
    <AppShell>
      <RouterView v-slot="{ Component }">
        <Transition name="fade" mode="out-in">
          <component :is="Component" />
        </Transition>
      </RouterView>
    </AppShell>
    <ToastHost />
  </template>
</template>
