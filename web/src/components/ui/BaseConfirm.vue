<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'

/**
 * 确认弹窗：破坏性操作的唯一入口。
 *
 * 契约：danger=true 时主按钮用 danger 色，且需要显式点击（不响应 Enter），
 * 防止在输入框里回车顺手确认删除。
 */
const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    message?: string
    confirmText?: string
    danger?: boolean
  }>(),
  { message: undefined, confirmText: '确认', danger: false },
)

const emit = defineEmits<{ (e: 'confirm'): void; (e: 'update:open', v: boolean): void }>()

const root = ref<HTMLElement | null>(null)

watch(
  () => props.open,
  (open) => {
    if (open) {
      document.body.style.overflow = 'hidden'
      // 焦点给取消：确认必须是主动动作
      requestAnimationFrame(() => root.value?.querySelector<HTMLButtonElement>('[data-cancel]')?.focus())
    } else {
      document.body.style.overflow = ''
    }
  },
)

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('update:open', false)
}

onBeforeUnmount(() => {
  document.body.style.overflow = ''
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-[55] flex items-center justify-center p-6"
      role="alertdialog"
      aria-modal="true"
      :aria-label="title"
      @keydown="onKey"
    >
      <div class="absolute inset-0 bg-sunken/70" @click="emit('update:open', false)" />
      <div
        ref="root"
        class="relative w-full max-w-[380px] rounded-[14px] border border-line-strong bg-elevated p-4"
      >
        <h2 class="text-ink-100">{{ title }}</h2>
        <p v-if="message" class="mt-1.5 break-words text-ink-400">{{ message }}</p>
        <div class="mt-4 flex justify-end gap-2">
          <button
            data-cancel
            type="button"
            class="rounded-lg border border-line px-3 py-1 text-ink-200 transition-colors hover:border-line-strong hover:bg-white/[0.05] active:scale-[0.98]"
            @click="emit('update:open', false)"
          >
            取消
          </button>
          <button
            type="button"
            class="rounded-lg px-3 py-1 transition-opacity active:scale-[0.98]"
            :class="danger ? 'bg-danger text-white hover:opacity-90' : 'bg-accent text-[var(--color-accent-ink)] hover:opacity-90'"
            @click="emit('confirm')"
          >
            {{ confirmText }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
