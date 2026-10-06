<script setup lang="ts">
import AppIcon from '@/components/ui/AppIcon.vue'

/** 统一下拉框：appearance-none + 自绘箭头，与 BaseInput 同一套边框语言 */
withDefaults(
  defineProps<{
    modelValue: string
    options: { value: string; label: string; disabled?: boolean }[]
    disabled?: boolean
    width?: string
  }>(),
  { disabled: false, width: undefined },
)

const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()
</script>

<template>
  <div class="relative inline-flex w-full items-center" :style="width ? { width } : undefined">
    <select
      :value="modelValue"
      :disabled="disabled"
      class="h-8 w-full cursor-pointer appearance-none rounded-lg border border-line bg-white/[0.02] pl-2.5 pr-7 text-ink-100 outline-none transition-colors hover:border-line-strong focus:border-accent-line disabled:cursor-not-allowed disabled:opacity-50"
      @change="emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
    >
      <option v-for="o in options" :key="o.value" :value="o.value" :disabled="o.disabled" class="bg-elevated text-ink-100">
        {{ o.label }}
      </option>
    </select>
    <AppIcon
      name="chevronDown"
      class="pointer-events-none absolute right-2 size-3 text-ink-500"
    />
  </div>
</template>
