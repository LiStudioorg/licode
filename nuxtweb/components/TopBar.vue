<script setup lang="ts">
import { Message, Button, Badge, Divider } from 'fuxsto-design'
import {
  Sun,
  Moon,
  PanelLeft,
  PanelRight,
  Info,
  FolderOpen,
  Bot,
} from 'lucide-vue-next'

const licode = useLicode()
const { state } = licode
const { mode, toggleTheme } = useTheme()

function toggleRight(tab: 'info' | 'files') {
  state.rightTab = state.rightTab === tab ? '' : tab
}
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
    <Button
      v-for="t in ([
        ['info', Info, '信息'],
        ['files', FolderOpen, '文件'],
      ] as const)"
      :key="t[0]"
      size="sm"
      :variant="state.rightTab === t[0] ? 'secondary' : 'ghost'"
      :icon="t[1]"
      :title="t[2]"
      @click="toggleRight(t[0])"
    >
      {{ t[2] }}
    </Button>
    <Divider direction="vertical" class="h-5" />
    <Button variant="ghost" size="sm" :icon="mode === 'dark' ? Sun : Moon" title="切换主题" @click="toggleTheme" />
    <Button
      variant="ghost"
      size="sm"
      :icon="PanelRight"
      title="信息面板"
      :class="{ 'opacity-40': !state.rightTab }"
      @click="state.rightTab = state.rightTab ? '' : 'info'"
    />
  </header>
</template>
