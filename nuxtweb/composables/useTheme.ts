export type ThemeMode = 'light' | 'dark'
export type ThemeSkin = 'default' | 'glass'
export type GlassLevel = 'soft' | 'medium' | 'strong'
export type BgStyle = 'gradient' | 'plain'
export type RadiusStyle = 'normal' | 'large'

const KEY = 'licode_theme'
const SKIN_KEY = 'licode_skin'
const GLASS_KEY = 'licode_glass'
const BG_KEY = 'licode_bg'
const RADIUS_KEY = 'licode_radius'
const ANIM_KEY = 'licode_anim'

function read<T extends string>(key: string, allowed: readonly T[], fallback: T): T {
  if (!import.meta.client) return fallback
  const v = localStorage.getItem(key) as T | null
  return v && allowed.includes(v) ? v : fallback
}

export function useTheme() {
  const mode = useState<ThemeMode>('theme', () => 'light')
  const skin = useState<ThemeSkin>('themeSkin', () => 'default')
  const glassLevel = useState<GlassLevel>('themeGlass', () => 'medium')
  const bgStyle = useState<BgStyle>('themeBg', () => 'gradient')
  const radius = useState<RadiusStyle>('themeRadius', () => 'normal')
  const anim = useState<boolean>('themeAnim', () => true)

  function applyMode(m: ThemeMode) {
    mode.value = m
    if (import.meta.client) {
      document.documentElement.classList.toggle('dark', m === 'dark')
      localStorage.setItem(KEY, m)
    }
  }

  function applySkin(s: ThemeSkin) {
    skin.value = s
    if (import.meta.client) {
      document.documentElement.classList.toggle('glass', s === 'glass')
      localStorage.setItem(SKIN_KEY, s)
    }
  }

  function applyGlassLevel(v: GlassLevel) {
    glassLevel.value = v
    if (import.meta.client) {
      document.documentElement.dataset.glass = v
      localStorage.setItem(GLASS_KEY, v)
    }
  }

  function applyBg(v: BgStyle) {
    bgStyle.value = v
    if (import.meta.client) {
      document.documentElement.dataset.bg = v
      localStorage.setItem(BG_KEY, v)
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

  function initTheme() {
    if (!import.meta.client) return
    const saved = localStorage.getItem(KEY) as ThemeMode | null
    applyMode(saved === 'dark' ? 'dark' : 'light')
    applySkin(read<ThemeSkin>(SKIN_KEY, ['default', 'glass'], 'default'))
    applyGlassLevel(read<GlassLevel>(GLASS_KEY, ['soft', 'medium', 'strong'], 'medium'))
    applyBg(read<BgStyle>(BG_KEY, ['gradient', 'plain'], 'gradient'))
    applyRadius(read<RadiusStyle>(RADIUS_KEY, ['normal', 'large'], 'normal'))
    applyAnim(localStorage.getItem(ANIM_KEY) !== '0')
  }

  function toggleTheme() {
    applyMode(mode.value === 'dark' ? 'light' : 'dark')
  }

  return {
    mode,
    skin,
    glassLevel,
    bgStyle,
    radius,
    anim,
    initTheme,
    toggleTheme,
    setMode: applyMode,
    setSkin: applySkin,
    setGlassLevel: applyGlassLevel,
    setBgStyle: applyBg,
    setRadius: applyRadius,
    setAnim: applyAnim,
  }
}
