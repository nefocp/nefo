import { createI18n } from 'vue-i18n'
import en from './locales/en.json'

// To add a language: copy locales/en.json, translate it, add a loader below
// and a line to LOCALES. English is the source and is always loaded (missing
// keys fall back to it); every other language is a separate file the
// browser fetches only when someone uses it.
// tag is the html lang attribute (fonts and hyphenation follow it).
export const LOCALES = [
  { code: 'en', name: 'English', tag: 'en' },
  { code: 'vi', name: 'Tiếng Việt', tag: 'vi' },
  { code: 'zh', name: '简体中文', tag: 'zh-CN' },
  { code: 'es', name: 'Español', tag: 'es' },
  { code: 'pt', name: 'Português', tag: 'pt-BR' },
  { code: 'ru', name: 'Русский', tag: 'ru' },
  { code: 'id', name: 'Bahasa Indonesia', tag: 'id' },
]

const loaders = {
  vi: () => import('./locales/vi.json'),
  zh: () => import('./locales/zh.json'),
  es: () => import('./locales/es.json'),
  pt: () => import('./locales/pt.json'),
  ru: () => import('./locales/ru.json'),
  id: () => import('./locales/id.json'),
}

const STORAGE_KEY = 'panel.locale'

function savedOrBrowserLocale() {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved && LOCALES.some((l) => l.code === saved)) return saved
  } catch {
    // storage blocked: fall through to the browser language
  }
  const nav = (navigator.language || 'en').toLowerCase().split('-')[0]
  return LOCALES.some((l) => l.code === nav) ? nav : 'en'
}

// Plural forms. English, Spanish and Portuguese messages have two
// ("one | other", the default rule). Chinese and Indonesian have one form.
// Russian has three: "one | few | many" (1, 21… | 2–4, 22–24… | the rest).
function russian(n, forms) {
  const m10 = n % 10
  const m100 = n % 100
  let i = 2
  if (m10 === 1 && m100 !== 11) i = 0
  else if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) i = 1
  return Math.min(i, forms - 1)
}
const oneForm = () => 0

export const i18n = createI18n({
  legacy: false,
  locale: 'en',
  fallbackLocale: 'en',
  messages: { en },
  pluralRules: { ru: russian, zh: oneForm, id: oneForm },
})

function htmlLang(code) {
  return LOCALES.find((l) => l.code === code)?.tag || code
}

// load fetches a language's messages once. A language that can't be loaded
// (offline, an old cached page) leaves the interface in English.
async function load(code) {
  if (code === 'en' || i18n.global.availableLocales.includes(code) || !loaders[code]) return true
  try {
    const m = await loaders[code]()
    i18n.global.setLocaleMessage(code, m.default)
    return true
  } catch {
    return false
  }
}

function apply(code) {
  i18n.global.locale.value = code
  document.documentElement.lang = htmlLang(code)
}

// initLocale picks the saved or the browser's language before the app shows
// anything, so the page doesn't flash English first.
export async function initLocale() {
  const code = savedOrBrowserLocale()
  apply((await load(code)) ? code : 'en')
}

export async function setLocale(code) {
  if (!(await load(code))) return
  apply(code)
  try {
    localStorage.setItem(STORAGE_KEY, code)
  } catch {
    // not saved; the choice lasts for this page only
  }
}

document.documentElement.lang = 'en'
