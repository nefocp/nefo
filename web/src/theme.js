import { ref, computed } from 'vue'

// Theme: "system" follows prefers-color-scheme; "light" and "dark" are the
// administrator's choice, remembered in this browser. The choice is set as
// data-theme on <html>; style.css reads it.
const KEY = 'panel.theme'
const media = typeof matchMedia === 'function' ? matchMedia('(prefers-color-scheme: dark)') : null

function saved() {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : 'system'
  } catch {
    return 'system'
  }
}

export const themeMode = ref(saved())
const systemDark = ref(!!media?.matches)
media?.addEventListener?.('change', (e) => (systemDark.value = e.matches))

// isDark is the theme in effect.
export const isDark = computed(() => (themeMode.value === 'system' ? systemDark.value : themeMode.value === 'dark'))

function apply() {
  const el = document.documentElement
  if (themeMode.value === 'system') el.removeAttribute('data-theme')
  else el.setAttribute('data-theme', themeMode.value)
}
apply()

export function setTheme(mode) {
  themeMode.value = mode
  try {
    if (mode === 'system') localStorage.removeItem(KEY)
    else localStorage.setItem(KEY, mode)
  } catch {
    // not saved; lasts for this page
  }
  apply()
}

// toggleTheme switches to the other theme of the one in effect.
export function toggleTheme() {
  setTheme(isDark.value ? 'light' : 'dark')
}
