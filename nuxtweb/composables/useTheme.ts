export type ThemeMode = 'light' | 'dark'
export type RadiusStyle = 'normal' | 'large'

const KEY = 'licode_theme'
const RADIUS_KEY = 'licode_radius'
const ANIM_KEY = 'licode_anim'
const ACCENT_KEY = 'licode_accent'
const APP_BG_KEY = 'licode_app_bg'
const BUBBLE_KEY = 'licode_bubble'

function read<T extends string>(key: string, allowed: readonly T[], fallback: T): T {
  if (!import.meta.client) return fallback
  const v = localStorage.getItem(key) as T | null
  return v && allowed.includes(v) ? v : fallback
}

function readColor(key: string, fallback: string): string {
  if (!import.meta.client) return fallback
  const v = localStorage.getItem(key)
  return v && /^#[0-9a-fA-F]{6}$/.test(v) ? v : fallback
}

export function useTheme() {
  const mode = useState<ThemeMode>('theme', () => 'light')
  const radius = useState<RadiusStyle>('themeRadius', () => 'normal')
  const anim = useState<boolean>('themeAnim', () => true)
  const accent = useState<string>('themeAccent', () => '#6366f1')
  const appBg = useState<string>('themeAppBg', () => '#f4f4f5')
  const bubble = useState<string>('themeBubble', () => '#18181b')

  function applyMode(m: ThemeMode) {
    mode.value = m
    if (import.meta.client) {
      document.documentElement.classList.toggle('dark', m === 'dark')
      localStorage.setItem(KEY, m)
    }
  }

  function applyRadius(v: RadiusStyle) {
    radius.value = v
    if (import.meta.client) {
      document.documentElement.classList.toggle('radius-lg', v === 'large')
      localStorage.setItem(RADIUS_KEY, v)
    }
  }

  function applyAnim(v: boolean) {
    anim.value = v
    if (import.meta.client) {
      document.documentElement.classList.toggle('no-anim', !v)
      localStorage.setItem(ANIM_KEY, v ? '1' : '0')
    }
  }

  function applyAccent(v: string) {
    accent.value = v
    if (import.meta.client) {
      document.documentElement.style.setProperty('--accent', v)
      localStorage.setItem(ACCENT_KEY, v)
    }
  }

  function applyAppBg(v: string) {
    appBg.value = v
    if (import.meta.client) {
      document.documentElement.style.setProperty('--app-bg', v)
      localStorage.setItem(APP_BG_KEY, v)
    }
  }

  function applyBubble(v: string) {
    bubble.value = v
    if (import.meta.client) {
      document.documentElement.style.setProperty('--bubble', v)
      localStorage.setItem(BUBBLE_KEY, v)
    }
  }

  function initTheme() {
    if (!import.meta.client) return
    const saved = localStorage.getItem(KEY) as ThemeMode | null
    applyMode(saved === 'dark' ? 'dark' : 'light')
    applyRadius(read<RadiusStyle>(RADIUS_KEY, ['normal', 'large'], 'normal'))
    applyAnim(localStorage.getItem(ANIM_KEY) !== '0')
    applyAccent(readColor(ACCENT_KEY, '#6366f1'))
    applyAppBg(readColor(APP_BG_KEY, '#f4f4f5'))
    applyBubble(readColor(BUBBLE_KEY, '#18181b'))
  }

  function toggleTheme() {
    applyMode(mode.value === 'dark' ? 'light' : 'dark')
  }

  return {
    mode,
    radius,
    anim,
    accent,
    appBg,
    bubble,
    initTheme,
    toggleTheme,
    setMode: applyMode,
    setRadius: applyRadius,
    setAnim: applyAnim,
    setAccent: applyAccent,
    setAppBg: applyAppBg,
    setBubble: applyBubble,
  }
}
