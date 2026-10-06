import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

/**
 * 路由表。
 *
 * 用 history 模式而非 hash：后端对未知路径统一回落到内嵌 index.html
 * （前端 SPA 的兜底），所以 /files、/settings 直接刷新也能正常打开。
 *
 * 只有登录页是懒加载 —— 它是唯一「大多数人永远看不到」的页面。
 * 其余页面体积小且是核心路径，直接静态 import 避免首屏多一次往返。
 */
const routes: RouteRecordRaw[] = [
  { path: '/', name: 'chat', component: () => import('@/views/ChatView.vue') },
  { path: '/files', name: 'files', component: () => import('@/views/FilesView.vue') },
  { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') },
  { path: '/diagnostics', name: 'diagnostics', component: () => import('@/views/DiagnosticsView.vue') },
  // 「信息」页已取消：会话统计归底部状态栏，排障事实归诊断页。
  // 保留重定向，避免旧书签落到空白页。
  { path: '/info', redirect: '/diagnostics' },
  { path: '/tools', redirect: '/settings' },
  { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue') },
  // 未知路径回对话页而不是空白 404：单用户本地工具，没有「找不到」的语义
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})
