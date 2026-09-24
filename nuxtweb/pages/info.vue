<script setup lang="ts">
import { ArrowLeft } from 'lucide-vue-next'
import { Button } from 'fuxsto-design'
import { useLicode } from '~/composables/useLicode'

const { connect, state } = useLicode()

onMounted(() => {
  if (state.wsStatus === 'disconnected') connect()
})
</script>

<template>
  <div class="flex h-full flex-col overflow-hidden">
    <header
      class="flex h-12 shrink-0 items-center gap-2 border-b border-zinc-200 bg-white px-3 dark:border-zinc-800 dark:bg-zinc-900"
    >
      <Button variant="ghost" size="sm" :icon="ArrowLeft" title="返回对话" @click="navigateTo('/')" />
      <span class="text-sm font-semibold tracking-tight">信息</span>
    </header>
    <div class="min-h-0 flex-1 overflow-y-auto">
      <InfoPanel />
    </div>
  </div>
</template>
