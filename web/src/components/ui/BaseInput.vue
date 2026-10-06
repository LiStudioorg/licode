<script setup lang="ts" generic="T extends string | number">
/**
 * 统一输入框。v-model 支持 string/number；number 模式在 emit 前完成转换，
 * 空串按 0 处理由调用方决定（这里保持 null 交给上层）。
 */
const props = withDefaults(
  defineProps<{
    modelValue: T | null | undefined
    type?: 'text' | 'number' | 'password' | 'search'
    placeholder?: string
    step?: number | string
    min?: number | string
    max?: number | string
    disabled?: boolean
    mono?: boolean
    width?: string
  }>(),
  { type: 'text', placeholder: '', step: undefined, min: undefined, max: undefined, disabled: false, mono: false, width: undefined },
)

const emit = defineEmits<{ (e: 'update:modelValue', v: T | null): void; (e: 'enter'): void }>()

function onInput(ev: Event) {
  const el = ev.target as HTMLInputElement
  if (props.type === 'number') {
    emit('update:modelValue', el.value === '' ? null : (Number(el.value) as T))
  } else {
    emit('update:modelValue', el.value as T)
  }
}
</script>

<template>
  <input
    :value="modelValue ?? ''"
    :type="type"
    :step="step"
    :min="min"
    :max="max"
    :placeholder="placeholder"
    :disabled="disabled"
    :style="width ? { width } : undefined"
    class="h-8 rounded-lg border border-line bg-white/[0.02] px-2.5 text-ink-100 outline-none transition-colors placeholder:text-ink-500 hover:border-line-strong focus:border-accent-line disabled:cursor-not-allowed disabled:opacity-50"
    :class="mono ? 'font-mono text-[12px]' : ''"
    @input="onInput"
    @keydown.enter="emit('enter')"
  />
</template>
