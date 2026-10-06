<script setup lang="ts">
/** 开关：唯一的双态控件。键盘可达（原生 button + aria-pressed）。 */
withDefaults(
  defineProps<{ modelValue: boolean; disabled?: boolean }>(),
  { disabled: false },
)

const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()
</script>

<template>
  <button
    type="button"
    role="switch"
    :aria-checked="modelValue"
    :disabled="disabled"
    class="relative h-[18px] w-[32px] shrink-0 rounded-full border transition-colors duration-150 disabled:cursor-not-allowed disabled:opacity-50"
    :class="
      modelValue
        ? 'border-transparent bg-accent'
        : 'border-line-strong bg-white/[0.06]'
    "
    @click="emit('update:modelValue', !modelValue)"
  >
    <span
      class="absolute top-1/2 size-[12px] -translate-y-1/2 rounded-full bg-white shadow-sm transition-[left] duration-150"
      :class="modelValue ? 'left-[16px]' : 'left-[2px]'"
    />
  </button>
</template>
