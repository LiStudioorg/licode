<script setup lang="ts">
import { Button, Badge, Divider } from 'fuxsto-design'
import {
  Sun,
  Moon,
  PanelLeft,
  Info,
  FolderOpen,
  Bot,
} from 'lucide-vue-next'

const licode = useLicode()
const { state } = licode
const { mode, toggleTheme } = useTheme()
</script>

<template>
  <header
    class="flex h-12 shrink-0 items-center gap-2 border-b border-zinc-200 bg-white px-3 dark:border-zinc-800 dark:bg-zinc-900"
  >
    <Button variant="ghost" size="sm" :icon="PanelLeft" title="折叠侧边栏" @click="state.sidebarCollapsed = !state.sidebarCollapsed" />
    <div class="flex items-center gap-2">
      <span class="flex h-6 w-6 items-center justify-center rounded-md bg-zinc-900 text-white dark:bg-zinc-100 dark:text-zinc-900">
        <Bot :size="14" />
      </span>
      <span class="text-sm font-semibold tracking-tight">licode</span>
    </div>
    <Badge v-if="state.settings?.model" variant="secondary" size="sm" class="max-w-48 truncate">
      {{ state.settings.model }}
    </Badge>
    <div class="min-w-0 flex-1" />
    <Button size="sm" variant="ghost" :icon="Info" title="信息" @click="navigateTo('/info')">
      信息
    </Button>
    <Button size="sm" variant="ghost" :icon="FolderOpen" title="文件" @click="navigateTo('/files')">
      文件
    </Button>
    <Divider direction="vertical" class="h-5" />
    <Button variant="ghost" size="sm" :icon="mode === 'dark' ? Sun : Moon" title="切换主题" @click="toggleTheme" />
  </header>
</template>
