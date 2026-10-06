<script setup lang="ts">
/** 分段选择器：2-4 个互斥选项（比一排 radio 紧凑，比 select 少一次点击） */
withDefaults(
  defineProps<{
    modelValue: string
    options: { value: string; label: string }[]
    size?: 'sm' | 'md'
  }>(),
  { size: 'md' },
)

const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()
</script>

<template>
  <div class="inline-flex rounded-lg border border-line bg-white/[0.02] p-0.5" role="tablist">
    <button
      v-for="o in options"
      :key="o.value"
      type="button"
      role="tab"
      :aria-selected="modelValue === o.value"
      class="rounded-[6px] transition-colors"
      :class="[
        size === 'sm' ? 'px-2 py-0.5 text-[11px]' : 'px-2.5 py-1',
        modelValue === o.value ? 'bg-white/[0.08] text-ink-100' : 'text-ink-400 hover:text-ink-200',
      ]"
      @click="emit('update:modelValue', o.value)"
    >
      {{ o.label }}
    </button>
  </div>
</template>
