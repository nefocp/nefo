// Small helpers shared by the site's own components.
import { computed } from 'vue'
import { useData } from 'vitepress'

export type NfLabels = {
  root: string
  nav: Record<string, string>
  footer: Record<string, string>
  groups: Record<string, string>
  pages: Record<string, string>
  docMeta: Record<string, string>
  install: string
  releaseLink: string
  theme: [string, string]
}

export const GITHUB = 'https://github.com/nefocp/nefo'

// The labels of the current language (themeConfig.nf in config.mts).
export function useNf() {
  const { theme } = useData()
  return computed(() => (theme.value as { nf: NfLabels }).nf)
}

// The same page in every language, for the language menu and the footer.
// Where a language doesn't have the page yet (config.mts lists the ones that
// do in frontmatter.nf.langs), the link goes to that language's docs or home;
// a 404 links to each language's home.
export function useLanguages() {
  const { site, page, frontmatter, localeIndex } = useData()
  return computed(() => {
    const locales = site.value.locales
    const cur = localeIndex.value
    let rel = page.value.relativePath || ''
    if (cur !== 'root' && rel.startsWith(cur + '/')) rel = rel.slice(cur.length + 1)
    const notFound = page.value.isNotFound || /(^|\/)404\.md$/.test(rel)
    const path = notFound ? '' : rel.replace(/\.md$/, '').replace(/(^|\/)index$/, '$1')
    const has = frontmatter.value.nf?.langs as string[] | undefined
    return Object.entries(locales).map(([key, l]) => {
      const root = key === 'root' ? '/' : `/${key}/`
      const missing = has && !has.includes(key)
      return {
        key,
        code: key === 'root' ? 'en' : key,
        lang: l.lang || 'en',
        label: l.label || key,
        link: root + (missing ? (path.startsWith('docs/') ? 'docs/' : '') : path),
        current: key === cur,
      }
    })
  })
}

// "27 Nov 2026" in the page's language, the same on the server and in the browser.
export function formatDate(ms: number | string, lang: string) {
  try {
    return new Intl.DateTimeFormat(lang, { dateStyle: 'medium', timeZone: 'UTC' }).format(new Date(ms))
  } catch {
    return new Date(ms).toISOString().slice(0, 10)
  }
}
