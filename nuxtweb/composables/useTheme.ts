export type ThemeMode = 'light' | 'dark'
export type ThemeSkin = 'default' | 'glass'

const KEY = 'licode_theme'
const SKIN_KEY = 'licode_skin'

export function useTheme() {
  const mode = useState<ThemeMode>('theme', () => 'light')
  const skin = useState<ThemeSkin>('themeSkin', () => 'default')

  function apply(m: ThemeMode) {
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

  function initTheme() {
    if (import.meta.client) {
      const saved = localStorage.getItem(KEY) as ThemeMode | null
      apply(saved === 'dark' ? 'dark' : 'light')
      const savedSkin = localStorage.getItem(SKIN_KEY) as ThemeSkin | null
      applySkin(savedSkin === 'glass' ? 'glass' : 'default')
    }
  }

  function toggleTheme() {
    apply(mode.value === 'dark' ? 'light' : 'dark')
  }

  function setMode(m: ThemeMode) {
    apply(m)
  }

  function setSkin(s: ThemeSkin) {
    applySkin(s)
  }

  return { mode, skin, initTheme, toggleTheme, setMode, setSkin }
}
