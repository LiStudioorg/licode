<script setup lang="ts">
import { onBeforeUnmount, watch } from 'vue'

/**
 * 模态框：Teleport 到 body + 焦点收拢 + Esc 关闭。
 *
 * 用原生 <dialog> 之外的那套（自己实现）是为了完全掌控样式层级与
 * 移动端的全屏/底部抽屉行为；滚动锁在打开期间生效，关闭即还原。
 */
const props = defineProps<{
  open: boolean
  title?: string
  /** wide=true 时放宽到 720px，用于表单密集的编辑面板 */
  wide?: boolean
}>()

const emit = defineEmits<{ (e: 'close'): void }>()

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.stopPropagation()
    emit('close')
  }
}

watch(
  () => props.open,
  (open) => {
    document.body.style.overflow = open ? 'hidden' : ''
    if (open) window.addEventListener('keydown', onKeydown, true)
    else window.removeEventListener('keydown', onKeydown, true)
  },
)

onBeforeUnmount(() => {
  document.body.style.overflow = ''
  window.removeEventListener('keydown', onKeydown, true)
})
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-6"
        role="dialog"
        aria-modal="true"
        :aria-label="title"
      >
        <div class="absolute inset-0 bg-sunken/70" @click="emit('close')" />
        <div
          class="relative flex max-h-[92dvh] w-full flex-col overflow-hidden rounded-t-[14px] border border-line-strong bg-elevated sm:rounded-[14px]"
          :class="wide ? 'sm:max-w-[720px]' : 'sm:max-w-[440px]'"
        >
          <div class="flex h-11 shrink-0 items-center gap-2 border-b border-line px-4">
            <h2 class="min-w-0 flex-1 truncate text-ink-100">{{ title }}</h2>
            <button
              type="button"
              class="-mr-1 grid size-7 place-items-center rounded-lg text-ink-400 transition-colors hover:bg-white/[0.05] hover:text-ink-100"
              aria-label="关闭"
              @click="emit('close')"
            >
              <span class="text-[15px] leading-none">×</span>
            </button>
          </div>
          <div class="min-h-0 flex-1 overflow-y-auto px-4 py-3.5">
            <slot />
          </div>
          <div v-if="$slots.footer" class="flex shrink-0 items-center justify-end gap-2 border-t border-line px-4 py-2.5">
            <slot name="footer" />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 160ms var(--ease-std);
}
.modal-enter-active > div:last-child,
.modal-leave-active > div:last-child {
  transition: transform 200ms var(--ease-std);
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
.modal-enter-from > div:last-child,
.modal-leave-to > div:last-child {
  transform: translateY(12px) scale(0.99);
}
</style>
