import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'
import './styles/main.css'
import './styles/shell.css'

/**
 * 应用入口。
 *
 * 显式注册 Pinia 与 Router —— 本项目没有任何 auto-import 插件，
 * SFC 里用到的每个 API、组件、composable 都必须自己 import。
 * 挂载前先装 router，App.vue 首帧就用到了 RouterView/useRoute。
 */
createApp(App).use(createPinia()).use(router).mount('#app')
