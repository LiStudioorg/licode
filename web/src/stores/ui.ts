import { defineStore } from 'pinia'
import { ref } from 'vue'

/**
 * 壳层 UI 状态：抽屉开合与面板可见性。
 * 与业务数据分开，避免会话切换时误触发布局重排。
 */
export const useUiStore = defineStore('ui', () => {
  /** 窄屏下左栏以抽屉形式出现 */
  const navOpen = ref(false)
  /** 窄于 xl 时右栏以抽屉形式出现 */
  const inspectorOpen = ref(false)

  function toggleNav() {
    navOpen.value = !navOpen.value
    if (navOpen.value) inspectorOpen.value = false
  }

  function toggleInspector() {
    inspectorOpen.value = !inspectorOpen.value
    if (inspectorOpen.value) navOpen.value = false
  }

  function closeAll() {
    navOpen.value = false
    inspectorOpen.value = false
  }

  return { navOpen, inspectorOpen, toggleNav, toggleInspector, closeAll }
})
