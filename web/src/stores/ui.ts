import { defineStore } from 'pinia'
import { ref } from 'vue'

/** 壳层 UI 状态：抽屉开合与宽屏折叠。与业务数据分开，避免会话切换引发布局重排。 */
export const useUiStore = defineStore('ui', () => {
  /** 窄屏下左栏以 transform 抽屉出现 */
  const navOpen = ref(false)
  /** <1280px 时右栏（文件树）以浮层出现 */
  const treeOpen = ref(false)
  /** 宽屏下用户手动折叠左栏 */
  const navCollapsed = ref(false)
  /** 宽屏下用户手动折叠右栏 */
  const treeCollapsed = ref(false)

  function toggleNav() {
    navOpen.value = !navOpen.value
    if (navOpen.value) treeOpen.value = false
  }
  function toggleTree() {
    treeOpen.value = !treeOpen.value
    if (treeOpen.value) navOpen.value = false
  }
  function closeAll() {
    navOpen.value = false
    treeOpen.value = false
  }
  function toggleNavCollapsed() {
    navCollapsed.value = !navCollapsed.value
  }
  function toggleTreeCollapsed() {
    treeCollapsed.value = !treeCollapsed.value
  }

  return {
    navOpen,
    treeOpen,
    navCollapsed,
    treeCollapsed,
    toggleNav,
    toggleTree,
    closeAll,
    toggleNavCollapsed,
    toggleTreeCollapsed,
  }
})
