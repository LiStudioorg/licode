<script setup lang="ts">
import { ICONS, type IconName } from '@/utils/icons'

/**
 * 图标渲染：16 网格、stroke=currentColor、默认 1em 见方。
 * 尺寸由使用处的 class 控制（size-4 / size-3.5 等），与文字基线对齐。
 */
withDefaults(defineProps<{ name: IconName; strokeWidth?: number }>(), { strokeWidth: 1.25 })

const paths = ICONS
</script>

<template>
  <svg
    viewBox="0 0 16 16"
    class="size-[1em] shrink-0"
    fill="none"
    stroke="currentColor"
    :stroke-width="strokeWidth"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    v-html="paths[name]"
  />
</template>
