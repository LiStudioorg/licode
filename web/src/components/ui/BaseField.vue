<script setup lang="ts">
/**
 * 表单三件套：字段容器 / 输入框 / 下拉。
 * 统一外观与焦点态，设置页各表单只关心绑定，不再各自调样式。
 */
withDefaults(
  defineProps<{
    label?: string
    hint?: string
    /** 行内布局：label 在左、控件在右（设置行的标准排布） */
    inline?: boolean
  }>(),
  { label: undefined, hint: undefined, inline: false },
)
</script>

<template>
  <div class="flex gap-1.5" :class="inline ? 'flex-row items-center justify-between sm:flex-nowrap' : 'flex-col'">
    <div :class="inline ? 'min-w-0 pr-3' : ''">
      <label v-if="label" class="block text-ink-200">{{ label }}</label>
      <p v-if="hint" class="mt-0.5 text-[11px] leading-snug text-ink-500">{{ hint }}</p>
    </div>
    <div :class="inline ? 'shrink-0' : 'mt-1'">
      <slot />
    </div>
  </div>
</template>
