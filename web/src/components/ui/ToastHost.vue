<script setup lang="ts">
import { useToast } from '@/composables/useToast'
import AppIcon from '@/components/ui/AppIcon.vue'
import type { IconName } from '@/utils/icons'

const { items, dismiss } = useToast()

const ICON: Record<string, IconName> = { ok: 'ok', error: 'error', info: 'info' }
const TONE: Record<string, string> = {
  ok: 'border-ok/35 text-ok',
  error: 'border-danger/35 text-danger',
  info: 'border-line-strong text-accent',
}
</script>

<template>
  <Teleport to="body">
    <div class="pointer-events-none fixed inset-x-0 top-3 z-[60] flex flex-col items-center gap-1.5 px-3">
      <TransitionGroup name="toast">
        <div
          v-for="t in items"
          :key="t.id"
          class="pointer-events-auto flex w-full max-w-[460px] items-start gap-2 rounded-[10px] border bg-elevated/95 px-3 py-2 backdrop-blur-md"
          :class="TONE[t.kind]"
          role="status"
        >
          <AppIcon :name="ICON[t.kind]" class="mt-0.5 size-3.5" />
          <p class="min-w-0 flex-1 break-words text-ink-200">{{ t.text }}</p>
          <button
            type="button"
            class="-mr-1 shrink-0 rounded p-0.5 text-ink-500 transition-colors hover:text-ink-200"
            aria-label="关闭提示"
            @click="dismiss(t.id)"
          >
            <AppIcon name="close" class="size-3" />
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition:
    opacity 180ms var(--ease-std),
    transform 180ms var(--ease-std);
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
.toast-leave-active {
  position: absolute;
}
</style>
