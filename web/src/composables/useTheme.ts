import { computed, ref, watch } from 'vue'

/**
 * 主题状态：明暗模式 / 强调色 / 页面背景色，持久化到 localStorage。
 *
 * 机制：Tailwind v4 工具类编译为 var(--color-*) 引用，因此运行期覆写
 * :root 上的同名变量即可整站换肤。明暗切换靠 html 上的 .light 类
 * （样式表里的覆写块），强调色/背景色由本文件直接 setProperty。
 *
 * 注意：背景色覆写的是 --color-canvas 与派生的 --color-surface/sunken，
 * 只写自定义变量不会有任何效果。
 */

const LS_KEY = 'licod...e-v1'

export type ModeValue = 'dark' | 'light'

export interface AccentPreset {
  id: string
  label: string
  /** 主强调色 */
  accent: string
}

/** 预设取低饱和色：高饱和色块在满屏 UI 里廉价且刺眼 */
export const ACCENT_PRESETS: AccentPreset[] = [
  { id: 'blue', label: '靛蓝', accent: '#6f9bff' },
  { id: 'teal', label: '青碧', accent: '#4fc4b8' },
  { id: 'violet', label: '暮紫', accent: '#a78bfa' },
  { id: 'amber', label: '琥珀', accent: '#e0a458' },
  { id: 'rose', label: '绯红', accent: '#ef7d8e' },
  { id: 'slate', label: '石墨', accent: '#9aa7bd' },
]

interface ThemeState {
  mode: ModeValue
  accent: string
  /** 背景覆写色；'' = 主题默认 */
  canvas: string
}

function load(): ThemeState {
  const fallback: ThemeState = { mode: 'dark', accent: ACCENT_PRESETS[0].accent, canvas: '' }
  try {
    const raw = localStorage.getItem(LS_KEY)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as Partial<ThemeState>
    return {
      mode: parsed.mode === 'light' ? 'light' : 'dark',
      accent: typeof parsed.accent === 'string' && parsed.accent ? parsed.accent : fallback.accent,
      canvas: typeof parsed.canvas === 'string' ? parsed.canvas : '',
    }
  } catch {
    return fallback
  }
}

const state = ref<ThemeState>(load())

/** hex → 'r g b'，配 CSS 颜色乘透明度 */
function hexToRgb(hex: string): [number, number, number] | null {
  const m = /^#?([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(hex.trim())
  if (!m) return null
  let s = m[1]
  if (s.length === 3) s = s.split('').map((c) => c + c).join('')
  const n = parseInt(s, 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

/** 向白色混合（alpha>0）或向黑色混合（alpha<0），结果钳在 0..255 */
function mixWhite(hex: string, alpha: number): string {
  const [r, g, b] = hexToRgb(hex) ?? [128, 128, 128]
  const f = (c: number) => {
    const v = alpha >= 0 ? c + (255 - c) * alpha : c * (1 + alpha)
    return Math.max(0, Math.min(255, Math.round(v)))
  }
  return `#${((1 << 24) + (f(r) << 16) + (f(g) << 8) + f(b)).toString(16).slice(1)}`
}

function apply(s: ThemeState) {
  const root = document.documentElement
  root.classList.toggle('light', s.mode === 'light')

  const rgb = hexToRgb(s.accent)
  if (rgb) {
    const [r, g, b] = rgb
    root.style.setProperty('--color-accent', s.accent)
    root.style.setProperty('--color-accent-soft', `rgba(${r}, ${g}, ${b}, ${s.mode === 'dark' ? 0.13 : 0.14})`)
    root.style.setProperty('--color-accent-line', `rgba(${r}, ${g}, ${b}, 0.35)`)
    // 强调色上的文字：按感知亮度选黑/白，保证对比度
    const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255
    root.style.setProperty('--color-accent-ink', lum > 0.55 ? '#0b0c0f' : '#ffffff')
  }

  if (s.canvas && hexToRgb(s.canvas)) {
    // 背景三件套由背景色派生：surface 提亮/压暗一档，sunken 反向一档
    root.style.setProperty('--color-canvas', s.canvas)
    root.style.setProperty('--color-surface', s.mode === 'dark' ? mixWhite(s.canvas, 0.05) : mixWhite(s.canvas, 0.55))
    root.style.setProperty('--color-elevated', s.mode === 'dark' ? mixWhite(s.canvas, 0.08) : '#ffffff')
    root.style.setProperty('--color-sunken', s.mode === 'dark' ? mixWhite(s.canvas, -0.02) : mixWhite(s.canvas, 0.35))
  } else {
    root.style.removeProperty('--color-canvas')
    root.style.removeProperty('--color-surface')
    root.style.removeProperty('--color-elevated')
    root.style.removeProperty('--color-sunken')
  }
}

watch(state, (s) => {
  apply(s)
  try {
    localStorage.setItem(LS_KEY, JSON.stringify(s))
  } catch {
    /* 隐私模式下写入失败可忽略 */
  }
}, { immediate: true, deep: true })

export function useTheme() {
  const isDark = computed(() => state.value.mode === 'dark')

  function setMode(mode: ModeValue) {
    state.value.mode = mode
  }
  function toggleMode() {
    state.value.mode = state.value.mode === 'dark' ? 'light' : 'dark'
  }
  function setAccent(hex: string) {
    state.value.accent = hex
  }
  function setCanvas(hex: string) {
    state.value.canvas = hex
  }
  function resetCanvas() {
    state.value.canvas = ''
  }

  return {
    mode: computed(() => state.value.mode),
    accent: computed(() => state.value.accent),
    canvas: computed(() => state.value.canvas),
    isDark,
    setMode,
    toggleMode,
    setAccent,
    setCanvas,
    resetCanvas,
  }
}
