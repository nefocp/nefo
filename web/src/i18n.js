import { createI18n } from 'vue-i18n'
import en from './locales/en.json'
import vi from './locales/vi.json'

// To add a language: copy locales/en.json, translate it, import it here and
// add it to LOCALES. English is the source; missing keys fall back to it.
export const LOCALES = [
  { code: 'en', name: 'English' },
  { code: 'vi', name: 'Tiếng Việt' },
]

const STORAGE_KEY = 'panel.locale'

function initialLocale() {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved && LOCALES.some((l) => l.code === saved)) return saved
  } catch {
    // storage blocked: fall through to the browser language
  }
  const nav = (navigator.language || 'en').toLowerCase().split('-')[0]
  return LOCALES.some((l) => l.code === nav) ? nav : 'en'
}

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale(),
  fallbackLocale: 'en',
  messages: { en, vi },
})

document.documentElement.lang = i18n.global.locale.value

export function setLocale(code) {
  i18n.global.locale.value = code
  document.documentElement.lang = code
  try {
    localStorage.setItem(STORAGE_KEY, code)
  } catch {
    // not saved; the choice lasts for this page only
  }
}
