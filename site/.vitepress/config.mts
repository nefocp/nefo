import { defineConfig, type DefaultTheme, type HeadConfig, type LocaleConfig, type PageData } from 'vitepress'
import { mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { execFileSync } from 'node:child_process'
import en from './locales/en.json'
import vi from './locales/vi.json'
import zh from './locales/zh.json'
import es from './locales/es.json'
import pt from './locales/pt.json'
import ru from './locales/ru.json'
import id from './locales/id.json'
// The current release: version, status, date, the panel's measured memory. The
// only place these facts live; pages use {{ $release.version }} and so on.
import release from './data/release.json'
// The app store catalog, copied from internal/appstore by scripts/sync-apps.mjs.
import appsData from './data/apps.json'

const SITE = 'https://nefo.io'
const GITHUB = 'https://github.com/nefocp/nefo'
const SRC = fileURLToPath(new URL('..', import.meta.url))

// One list of docs pages for every language. The file name is the same in
// each: English at the root, every other language under /<code>/. The names
// shown come from .vitepress/locales/<code>.json.
const DOCS: { group: string; pages: string[] }[] = [
  { group: 'Getting started', pages: ['', 'requirements', 'install', 'first-steps', 'cli', 'update', 'uninstall'] },
  { group: 'Websites', pages: ['websites', 'import', 'ssl', 'website-protection', 'dns', 'git-deploy', 'wordpress', 'apps'] },
  { group: 'Data', pages: ['databases', 'postgresql', 'files', 'backups', 'restore-new-server'] },
  { group: 'Server', pages: ['software', 'docker', 'security', 'security-check', 'api', 'os', 'system', 'mail-alerts'] },
  { group: 'Help', pages: ['troubleshooting', 'faq'] },
]
// The pages outside the docs, in the order llms.txt lists them.
const PROJECT_PAGES = ['compare', 'changelog', 'roadmap', 'about', 'security-policy']

type Labels = typeof en

// The languages: the folder ('' for English at the root), the html lang
// attribute (also the hreflang), the Open Graph locale and the labels. A
// language is added with its labels file, its pages under site/<code>/ and a
// line here. `shots` is the folder of panel screenshots it uses.
const LANGS: { code: string; lang: string; og: string; shots: string; labels: Labels }[] = [
  { code: '', lang: 'en', og: 'en_US', shots: 'en', labels: en },
  { code: 'vi', lang: 'vi', og: 'vi_VN', shots: 'vi', labels: vi },
  { code: 'zh', lang: 'zh-CN', og: 'zh_CN', shots: 'zh', labels: zh },
  { code: 'es', lang: 'es', og: 'es_ES', shots: 'es', labels: es },
  { code: 'pt', lang: 'pt-BR', og: 'pt_BR', shots: 'pt', labels: pt },
  { code: 'ru', lang: 'ru', og: 'ru_RU', shots: 'ru', labels: ru },
  { code: 'id', lang: 'id', og: 'id_ID', shots: 'id', labels: id },
]
const LANG_BY_CODE = Object.fromEntries(LANGS.map((l) => [l.code, l]))
const CODES = LANGS.filter((l) => l.code).map((l) => l.code)

// ---------------------------------------------------------------- helpers

// {version}, {ram}, {apps}, {languages}, {status}, {date} in the locale
// strings, filled from the release data.
const FACTS: Record<string, string> = {
  version: release.version,
  status: release.status,
  date: release.date,
  ram: release.ram,
  languages: String(release.languages),
  apps: String(appsData.apps.length),
}
const fill = <T,>(v: T): T => {
  if (typeof v === 'string') return v.replace(/\{(version|status|date|ram|languages|apps)\}/g, (_, k) => FACTS[k]) as T
  if (Array.isArray(v)) return v.map(fill) as T
  if (v && typeof v === 'object') return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, fill(x)])) as T
  return v
}
// {{ $release.version }} and the like in Markdown and frontmatter.
const RELEASE_RE = /\{\{\s*\$release\.(\w+)\s*\}\}/g
const RELEASE: Record<string, string> = { ...FACTS, notes: release.notes }
const fillRelease = (s: string) => s.replace(RELEASE_RE, (m, k) => (k in RELEASE ? RELEASE[k] : m))

const splitLocale = (rel: string) => {
  const m = rel.match(/^([a-z]{2})\//)
  return m && CODES.includes(m[1]) ? { code: m[1], base: rel.slice(3) } : { code: '', base: rel }
}
// 'docs/install.md' -> '/docs/install', 'vi/index.md' -> '/vi/'
const urlPath = (rel: string) => '/' + rel.replace(/\.md$/, '').replace(/(^|\/)index$/, '$1')
const absUrl = (rel: string) => SITE + urlPath(rel)
const stripTags = (s: string) => s.replace(/<[^>]+>/g, '').replace(/&nbsp;/g, ' ').replace(/\s+/g, ' ').trim()
const isoDate = (ms: number) => new Date(ms).toISOString()

// Every Markdown page of the site, to know which languages have which page.
const ALL_PAGES = new Set<string>()
;(function walk(dir: string, rel: string) {
  for (const name of readdirSync(dir)) {
    if (name.startsWith('.') || name === 'node_modules' || name === 'public') continue
    const p = join(dir, name)
    if (statSync(p).isDirectory()) walk(p, rel + name + '/')
    else if (name.endsWith('.md')) ALL_PAGES.add(rel + name)
  }
})(SRC, '')
const alternatesOf = (base: string) =>
  LANGS.filter((l) => ALL_PAGES.has((l.code ? l.code + '/' : '') + base)).map((l) => ({ ...l, rel: (l.code ? l.code + '/' : '') + base }))

const is404 = (rel: string) => /(^|\/)404\.md$/.test(rel)
const docSlug = (base: string) => (base.startsWith('docs/') ? base.slice(5).replace(/\.md$/, '').replace(/^index$/, '') : null)
const groupOf = (slug: string) => DOCS.find((g) => g.pages.includes(slug))?.group

// The front matter of a Markdown file: title and description are enough here.
function readPage(rel: string) {
  const src = readFileSync(join(SRC, rel), 'utf8')
  const m = src.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n?/)
  const fm: Record<string, string> = {}
  if (m) for (const line of m[1].split(/\r?\n/)) {
    const kv = line.match(/^(\w+):\s*(.*)$/)
    if (kv) fm[kv[1]] = kv[2].replace(/^(['"])(.*)\1$/, '$2')
  }
  return { fm, body: m ? src.slice(m[0].length) : src }
}

// Pages changed since the last commit (or not committed yet): git's date
// would be too old for them, so they use the file's own time.
const UNCOMMITTED = new Set<string>()
try {
  const prefix = execFileSync('git', ['rev-parse', '--show-prefix'], { cwd: SRC, encoding: 'utf8' }).trim()
  const out = execFileSync('git', ['status', '--porcelain', '--untracked-files=all', '--', '.'], { cwd: SRC, encoding: 'utf8' })
  for (const line of out.split('\n')) {
    const p = line.slice(3).trim().replace(/^"|"$/g, '').split(' -> ').pop()
    if (p && p.startsWith(prefix)) UNCOMMITTED.add(p.slice(prefix.length))
  }
} catch {
  // not a git checkout: every page uses git's answer (none) and then its file time
}

// The date a page was last changed: its front matter (lastUpdated: 2026-11-27),
// else its last commit, else the file's own time (also the build date on a
// fresh checkout without history).
const DATES = new Map<string, number>()
function lastChanged(pageData: PageData) {
  const fm = pageData.frontmatter.lastUpdated
  if (fm instanceof Date) return +fm
  if (typeof fm === 'string' && !isNaN(Date.parse(fm))) return Date.parse(fm)
  const fileTime = () => {
    try {
      return statSync(join(SRC, pageData.filePath)).mtimeMs
    } catch {
      return Date.now()
    }
  }
  if (UNCOMMITTED.has(pageData.filePath)) return fileTime()
  if (pageData.lastUpdated && !isNaN(pageData.lastUpdated)) return pageData.lastUpdated
  return fileTime()
}

// Minutes to read the page: words for alphabetic scripts, characters for Chinese.
function readMinutes(rel: string) {
  const body = readPage(rel).body.replace(/```[\s\S]*?```/g, ' ')
  const cjk = (body.match(/[㐀-鿿]/g) || []).length
  const words = body.replace(/[㐀-鿿]/g, ' ').split(/\s+/).filter(Boolean).length
  return Math.max(1, Math.round(words / 220 + cjk / 400))
}

// What the docs FAQ and the install guide say, collected while Markdown is
// rendered, for their FAQPage and HowTo data. Keyed by the page's path.
type Block = { type: 'p' | 'ol' | 'ul' | 'code'; text?: string; items?: string[]; names?: string[] }
type Section = { id: string; title: string; blocks: Block[] }
const STRUCT = new Map<string, Section[]>()
const inlineText = (tok: any): string =>
  (tok?.children || [])
    .map((c: any) => (c.type === 'text' || c.type === 'code_inline' ? c.content : c.type === 'softbreak' || c.type === 'hardbreak' ? ' ' : ''))
    .join('')
    .replace(/\s+/g, ' ')
    .trim()

function markdownPlugins(md: any) {
  // {{ $release.version }} etc. become the value before anything else sees them
  md.core.ruler.before('normalize', 'nf-release', (state: any) => {
    state.src = fillRelease(state.src)
  })
  // tables sit in a box that scrolls sideways on a phone
  md.renderer.rules.table_open = () => '<div class="nf-table"><table>\n'
  md.renderer.rules.table_close = () => '</table></div>\n'
  md.core.ruler.push('nf-page', (state: any) => {
    const rel: string | undefined = state.env?.relativePath
    if (!rel) return
    const { base } = splitLocale(rel)
    const tokens = state.tokens
    // the version, date and reading time under the title of every page but the home and 404
    if (base !== 'index.md' && !is404(rel)) {
      const i = tokens.findIndex((t: any) => t.type === 'heading_close' && t.tag === 'h1')
      if (i >= 0) {
        const tok = new state.Token('html_block', '', 0)
        tok.content = '<DocMeta />\n'
        tokens.splice(i + 1, 0, tok)
      }
    }
    if (base !== 'docs/faq.md' && base !== 'docs/install.md') return
    const sections: Section[] = []
    let cur: Section | null = null
    for (let i = 0; i < tokens.length; i++) {
      const t = tokens[i]
      if (t.type === 'heading_open' && t.tag === 'h2') {
        cur = { id: t.attrGet('id') || '', title: inlineText(tokens[i + 1]), blocks: [] }
        sections.push(cur)
        continue
      }
      if (!cur || t.level !== 0) continue
      if (t.type === 'paragraph_open') cur.blocks.push({ type: 'p', text: inlineText(tokens[i + 1]) })
      else if (t.type === 'fence') cur.blocks.push({ type: 'code', text: t.content.trim() })
      else if (t.type === 'ordered_list_open' || t.type === 'bullet_list_open') {
        const close = t.type.replace('_open', '_close')
        const items: string[] = []
        const names: string[] = []
        let item: string[] | null = null
        let j = i + 1
        for (; j < tokens.length; j++) {
          const u = tokens[j]
          if (u.type === close && u.level === t.level) break
          if (u.type === 'list_item_open' && u.level === t.level + 1) item = []
          else if (u.type === 'list_item_close' && u.level === t.level + 1 && item) {
            items.push(item.join(' '))
            names.push(stepName(item))
            item = null
          } else if (item && u.type === 'inline') item.push(inlineText(u))
          else if (item && u.type === 'fence') item.push(u.content.trim())
        }
        cur.blocks.push({ type: t.type === 'ordered_list_open' ? 'ol' : 'ul', items, names })
        i = j
      }
    }
    STRUCT.set(rel, sections)
  })
}

// A step's name: its first sentence, with the command when that sentence ends in a colon.
function stepName(parts: string[]) {
  const first = (parts[0] || '').split(/(?<=[.:。：])\s/)[0]
  const code = /[:：]$/.test(first) && parts[1] ? ' ' + parts[1].split('\n')[0] : ''
  return (first + code).slice(0, 160)
}

const sectionText = (s: Section) =>
  s.blocks.map((b) => (b.type === 'p' || b.type === 'code' ? b.text : (b.items || []).map((x) => '• ' + x).join(' '))).join(' ').replace(/\s+/g, ' ').trim()

// ---------------------------------------------------------------- structured data

const ORG_ID = SITE + '/#organization'
const SITE_ID = SITE + '/#website'
const APP_ID = SITE + '/#software'
const ogImage = (code: string) => `${SITE}/og/${code || 'en'}.png`

function organization() {
  return {
    '@type': 'Organization',
    '@id': ORG_ID,
    name: 'Nefo',
    url: SITE + '/',
    logo: { '@type': 'ImageObject', url: SITE + '/logo-512.png', width: 512, height: 512 },
    sameAs: [GITHUB],
  }
}

function homeGraph(L: (typeof LANGS)[number], url: string) {
  const h = fill(L.labels.home)
  const root = L.code ? `/${L.code}/` : '/'
  const shots = ['dashboard', 'websites', 'apps', 'databases', 'backups', 'security']
  const features = Object.values(h.features.rows).map((r) => stripTags(r.h3))
  const also = Object.values(h.features.also).map((a) => `${a.title}: ${a.text}`)
  return [
    organization(),
    {
      '@type': 'WebSite',
      '@id': SITE_ID,
      url: SITE + '/',
      name: 'Nefo',
      description: stripTags(fill(en.home.metaDesc)),
      inLanguage: LANGS.map((l) => l.lang),
      publisher: { '@id': ORG_ID },
    },
    {
      '@type': 'SoftwareApplication',
      '@id': APP_ID,
      name: 'Nefo',
      description: stripTags(h.lead),
      url,
      applicationCategory: 'DeveloperApplication',
      applicationSubCategory: 'Server control panel',
      operatingSystem: 'Ubuntu 22.04, 24.04, 26.04; Debian 12, 13',
      processorRequirements: 'x86-64 (an ARM64 build is also published)',
      memoryRequirements: `About ${release.ram} of RAM for the panel itself; built for a VPS with 1 GB of RAM`,
      softwareVersion: release.version,
      releaseNotes: release.notes,
      dateModified: release.date,
      downloadUrl: GITHUB + '/releases/latest',
      installUrl: SITE + root + 'docs/install',
      license: 'https://www.gnu.org/licenses/agpl-3.0.html',
      isAccessibleForFree: true,
      offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
      inLanguage: LANGS.map((l) => l.lang),
      image: ogImage(L.code),
      screenshot: shots.map((s) => `${SITE}/screens/${L.shots}/${s}-light.webp`),
      featureList: [...features, ...also],
      author: { '@id': ORG_ID },
      publisher: { '@id': ORG_ID },
      sameAs: [GITHUB],
    },
    {
      '@type': 'FAQPage',
      '@id': url + '#faq',
      inLanguage: L.lang,
      mainEntity: h.faq.items.map((x) => ({
        '@type': 'Question',
        name: stripTags(x.q),
        acceptedAnswer: { '@type': 'Answer', text: stripTags(x.a) },
      })),
    },
  ]
}

function breadcrumbs(items: { name: string; url: string }[]) {
  return {
    '@type': 'BreadcrumbList',
    itemListElement: items.map((x, i) => ({ '@type': 'ListItem', position: i + 1, name: x.name, item: x.url })),
  }
}

function pageGraph(pageData: PageData, L: (typeof LANGS)[number], url: string, title: string, description: string) {
  const { base } = splitLocale(pageData.relativePath)
  const root = SITE + (L.code ? `/${L.code}/` : '/')
  const slug = docSlug(base)
  const modified = DATES.get(pageData.relativePath)
  const graph: any[] = []
  const crumbs = [{ name: 'Nefo', url: root }]
  if (slug !== null) {
    if (slug) crumbs.push({ name: L.labels.nav.docs, url: root + 'docs/' })
    crumbs.push({ name: slug ? title : L.labels.nav.docs, url })
  } else crumbs.push({ name: title, url })
  graph.push({
    '@type': slug !== null ? 'TechArticle' : base === 'about.md' ? 'AboutPage' : 'WebPage',
    '@id': url + '#page',
    headline: title,
    name: title,
    description,
    url,
    inLanguage: L.lang,
    ...(modified ? { dateModified: isoDate(modified) } : {}),
    image: ogImage(L.code),
    author: { '@id': ORG_ID },
    publisher: organization(),
    isPartOf: { '@type': 'WebSite', '@id': SITE_ID, name: 'Nefo', url: SITE + '/' },
    about: { '@type': 'SoftwareApplication', '@id': APP_ID, name: 'Nefo', softwareVersion: release.version },
    mainEntityOfPage: url,
  })
  graph.push(breadcrumbs(crumbs))

  const sections = STRUCT.get(pageData.relativePath)
  if (base === 'docs/faq.md' && sections?.length) {
    graph.push({
      '@type': 'FAQPage',
      '@id': url + '#faq',
      inLanguage: L.lang,
      mainEntity: sections
        .filter((s) => s.title && s.blocks.length)
        .map((s) => ({ '@type': 'Question', name: s.title, acceptedAnswer: { '@type': 'Answer', text: sectionText(s) } })),
    })
  }
  if (base === 'docs/install.md' && sections?.length) {
    // the steps under "Run the installer" and "Open the panel for the first
    // time", found by their anchors, or by position where a translation has none
    const find = (anchor: string, index: number) => sections.find((s) => s.id === anchor) || sections[index]
    const steps = [find('run-the-installer', 0), find('open-the-panel-for-the-first-time', 3)]
      .filter(Boolean)
      .flatMap((s) => {
        const ol = s!.blocks.find((b) => b.type === 'ol')
        return (ol?.items || []).map((text, i) => ({ text, name: ol!.names?.[i] || text, section: s! }))
      })
    if (steps.length)
      graph.push({
        '@type': 'HowTo',
        '@id': url + '#howto',
        name: title,
        description,
        inLanguage: L.lang,
        step: steps.map((s, i) => ({
          '@type': 'HowToStep',
          position: i + 1,
          name: s.name,
          text: s.text,
          url: url + (s.section.id ? '#' + s.section.id : ''),
        })),
      })
  }
  return graph
}

// ---------------------------------------------------------------- llms.txt

const MD_LINK = /\]\((?!https?:|mailto:|#)([^)\s]+)\)/g
function plainMarkdown(rel: string) {
  const { fm, body } = readPage(rel)
  const pageUrl = absUrl(rel)
  let text = fillRelease(body)
    .replace(/<script[\s\S]*?<\/script>|<style[\s\S]*?<\/style>/g, '')
    .replace(/^(#{1,6} .*?)\s*\{#[\w-]+\}\s*$/gm, '$1')
    .replace(MD_LINK, (_, href: string) => '](' + new URL(href, pageUrl).href.replace(/\.md(?=$|#)/, '') + ')')
  // ::: tip Title ... ::: becomes a quote
  text = text.replace(/^:::\s*(\w+)\s*(.*)\r?\n([\s\S]*?)^:::\s*$/gm, (_, kind: string, title: string, inner: string) => {
    const head = `> **${title || kind[0].toUpperCase() + kind.slice(1)}**`
    return [head, ...inner.trimEnd().split(/\r?\n/).map((l) => (l ? '> ' + l : '>'))].join('\n') + '\n'
  })
  return { title: fm.title || '', description: fm.description || '', url: pageUrl, text: text.trim() }
}

function writeLlms(outDir: string) {
  for (const L of LANGS) {
    const prefix = L.code ? L.code + '/' : ''
    const l = L.labels
    const ll = fill(l.llms || en.llms)
    const line = (rel: string) => {
      if (!ALL_PAGES.has(rel)) return null
      const p = readPage(rel)
      const title = fillRelease(p.fm.title || rel)
      return `- [${title}](${absUrl(rel)}): ${fillRelease(p.fm.description || '')}`
    }
    const docs = DOCS.flatMap((g) => g.pages.map((s) => line(`${prefix}docs/${s || 'index'}.md`))).filter(Boolean)
    const project = PROJECT_PAGES.map((s) => line(`${prefix}${s}.md`)).filter(Boolean)
    const others = LANGS.filter((x) => x !== L).map((x) => `- [${x.labels.langLabel}](${SITE}/${x.code ? x.code + '/' : ''}llms.txt): the same in ${x.labels.langLabel}`)
    const txt = [
      '# Nefo',
      '',
      `> ${ll.summary}`,
      '',
      ...ll.facts.map((f: string) => `- ${f}`),
      '',
      `Home page: ${SITE}/${prefix}`,
      '',
      '## Docs',
      '',
      ...docs,
      '',
      '## Project',
      '',
      ...project,
      `- [Source code on GitHub](${GITHUB}): the Go program, the web interface, the installer and this website`,
      `- [Releases](${GITHUB}/releases): the signed programs for amd64 and arm64, the installer and the checksums`,
      '',
      '## Optional',
      '',
      `- [All the docs in one file](${SITE}/${prefix}llms-full.txt): every docs page as Markdown`,
      ...others,
      '',
    ].join('\n')
    const full = [
      `# Nefo documentation (${l.langLabel})`,
      '',
      `> ${ll.summary}`,
      '',
      ...ll.facts.map((f: string) => `- ${f}`),
      '',
      ...DOCS.flatMap((g) =>
        g.pages
          .map((s) => `${prefix}docs/${s || 'index'}.md`)
          .filter((rel) => ALL_PAGES.has(rel))
          .map((rel) => {
            const p = plainMarkdown(rel)
            return `\n---\n\nURL: ${p.url}\n\n${p.text}\n`
          }),
      ),
    ].join('\n')
    mkdirSync(join(outDir, prefix), { recursive: true })
    writeFileSync(join(outDir, prefix, 'llms.txt'), txt)
    writeFileSync(join(outDir, prefix, 'llms-full.txt'), full)
  }
}

// ---------------------------------------------------------------- locales

// The docs sidebar of one language: the pages that language has.
function sidebar(code: string, base: string, l: Labels): DefaultTheme.SidebarItem[] {
  const groups = l.groups as Record<string, string>
  const pages = l.pages as Record<string, string>
  const has = (p: string) => ALL_PAGES.has(`${code ? code + '/' : ''}docs/${p || 'index'}.md`)
  return DOCS.map((g) => ({
    text: groups[g.group],
    items: g.pages.filter(has).map((p) => ({ text: pages[p] || p, link: base + p })),
  })).filter((g) => g.items.length)
}
// A docs page that isn't in DOCS would have no place in the sidebar: say so.
{
  const listed = new Set(DOCS.flatMap((g) => g.pages.map((p) => `docs/${p || 'index'}.md`)))
  const missing = [...ALL_PAGES].filter((p) => p.startsWith('docs/') && !listed.has(p))
  if (missing.length) console.warn('[nefo] docs pages missing from the sidebar list in config.mts:', missing.join(', '))
}

function themeConfig(code: string, root: string, l: Labels): DefaultTheme.Config & { nf: unknown } {
  return {
    nav: [
      { text: l.nav.docs, link: `${root}docs/`, activeMatch: `^${root}docs/(?!install$)` },
      { text: l.nav.install, link: `${root}docs/install`, activeMatch: `^${root}docs/install$` },
      { text: l.nav.compare, link: `${root}compare` },
      { text: l.nav.changelog, link: `${root}changelog` },
      {
        text: l.nav.more,
        items: [
          { text: l.nav.roadmap, link: `${root}roadmap` },
          { text: l.nav.securityPolicy, link: `${root}security-policy` },
          { text: l.nav.about, link: `${root}about` },
          { text: l.nav.releases, link: GITHUB + '/releases' },
        ],
      },
    ],
    sidebar: { [`${root}docs/`]: sidebar(code, `${root}docs/`, l) },
    editLink: { pattern: GITHUB + '/edit/main/site/:path', text: l.editLink },
    outline: { label: l.outline, level: [2, 3] },
    lastUpdated: { text: l.lastUpdated, formatOptions: { dateStyle: 'medium' } },
    docFooter: l.docFooter,
    darkModeSwitchLabel: l.darkModeSwitchLabel,
    lightModeSwitchTitle: l.lightModeSwitchTitle,
    darkModeSwitchTitle: l.darkModeSwitchTitle,
    sidebarMenuLabel: l.sidebarMenuLabel,
    returnToTopLabel: l.returnToTopLabel,
    langMenuLabel: l.langMenuLabel,
    skipToContentLabel: l.skipToContentLabel,
    notFound: l.notFound,
    // labels for the site's own components (footer, breadcrumbs, the
    // version line under titles); the home page gets its text per page
    nf: {
      root,
      nav: l.nav,
      footer: l.footer,
      groups: l.groups,
      pages: l.pages,
      docMeta: l.docMeta,
      install: l.home.ctaInstall,
      releaseLink: l.home.releaseBar.link,
      theme: [l.lightModeSwitchTitle, l.darkModeSwitchTitle],
    },
  }
}

const locales: LocaleConfig<DefaultTheme.Config> = {}
const searchLocales: Record<string, unknown> = {}
for (const { code, lang, labels: l } of LANGS) {
  const root = code ? `/${code}/` : '/'
  locales[code || 'root'] = {
    label: l.langLabel,
    lang,
    ...(code ? { link: root } : {}),
    description: l.description,
    themeConfig: themeConfig(code, root, l),
  }
  if (code) {
    const s = l.search
    searchLocales[code] = {
      translations: {
        button: { buttonText: s.buttonText, buttonAriaLabel: s.buttonAriaLabel },
        modal: {
          noResultsText: s.noResultsText,
          resetButtonTitle: s.resetButtonTitle,
          displayDetails: s.displayDetails,
          footer: s.footer,
        },
      },
    }
  }
}

// The favicon inline, so it works on every page without another request.
const FAVICON =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'%3E%3Crect width='100' height='100' rx='24' fill='%232dd4bf'/%3E%3Cg transform='translate(13 13) scale(3.08333)' fill='none' stroke='%2304201c' stroke-width='3.1' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M5.6 17.7V11.7a5.2 5.2 0 0 1 10.4 0v2.6a3.4 3.4 0 0 0 3.4 3.4'/%3E%3C/g%3E%3C/svg%3E"

// The fonts every page needs first: Latin text, regular and bold.
const preload = (f: string): HeadConfig => ['link', { rel: 'preload', href: `/fonts/${f}.woff2`, as: 'font', type: 'font/woff2', crossorigin: '' }]

export default defineConfig({
  title: 'Nefo',
  titleTemplate: ':title · Nefo',
  description: en.description,
  cleanUrls: true,
  lastUpdated: true,
  sitemap: {
    hostname: SITE,
    transformItems(items) {
      return items
        .filter((x) => !/(^|\/)404$/.test(x.url))
        .map((x) => {
          const rel = (x.url.replace(/\/$/, '/index') || 'index') + '.md'
          const ms = DATES.get(rel)
          const links = x.links?.length ? [...x.links, { lang: 'x-default', url: x.links.find((k) => k.lang === 'en')?.url ?? x.url }] : x.links
          return { ...x, ...(ms ? { lastmod: isoDate(ms) } : {}), ...(links ? { links } : {}) }
        })
    },
  },

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: FAVICON }],
    ['link', { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' }],
    ['meta', { name: 'theme-color', content: '#0b7a6e' }],
    ['meta', { property: 'og:site_name', content: 'Nefo' }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
  ],

  locales,

  markdown: {
    // code blocks are dark in both themes, as in the panel's terminal
    theme: { light: 'github-dark', dark: 'github-dark' },
    config: markdownPlugins,
  },

  transformPageData(pageData) {
    const { code, base } = splitLocale(pageData.relativePath)
    const L = LANG_BY_CODE[code]
    if (pageData.title) pageData.title = fillRelease(pageData.title)
    if (pageData.description) pageData.description = fillRelease(pageData.description)
    if (base === 'index.md') {
      const h = fill(L.labels.home)
      pageData.title = h.title
      pageData.description = h.metaDesc
      pageData.titleTemplate = false
      pageData.frontmatter.titleTemplate = false
      pageData.frontmatter.home = h
      pageData.frontmatter.shots = L.shots
      pageData.frontmatter.apps = appsData.apps.map((a) => ({
        id: a.id,
        name: a.name,
        cat: a.category,
        kind: a.kind,
        desc: (a.description as Record<string, string>)[L.code || 'en'] || a.description.en,
      }))
      DATES.set(pageData.relativePath, Date.parse(release.date))
      return
    }
    if (is404(pageData.relativePath)) {
      pageData.title = '404'
      return
    }
    const ms = lastChanged(pageData)
    pageData.lastUpdated = ms
    DATES.set(pageData.relativePath, ms)
    const slug = docSlug(base)
    const group = slug !== null ? groupOf(slug) : undefined
    // the languages this page exists in, for the language menu
    const langs = alternatesOf(base).map((a) => a.code || 'root')
    pageData.frontmatter.nf = { read: readMinutes(pageData.relativePath), group: group ?? null, langs }
  },

  transformHead({ pageData, title, description }) {
    const rel = pageData.relativePath
    const { code, base } = splitLocale(rel)
    const L = LANG_BY_CODE[code] ?? LANGS[0]
    const head: HeadConfig[] = []
    // fonts: Russian uses the system font (Be Vietnam Pro has no Cyrillic)
    if (L.code !== 'ru') {
      head.push(preload('be-vietnam-pro-latin-400-normal'), preload('be-vietnam-pro-latin-700-normal'))
      if (L.code === 'vi') head.push(preload('be-vietnam-pro-vietnamese-400-normal'))
    }
    if (pageData.isNotFound || is404(rel)) {
      head.push(['meta', { name: 'robots', content: 'noindex' }])
      return head
    }
    const url = absUrl(rel)
    const alts = alternatesOf(base)
    head.push(['link', { rel: 'canonical', href: url }])
    for (const a of alts) head.push(['link', { rel: 'alternate', hreflang: a.lang, href: absUrl(a.rel) }])
    if (alts.some((a) => a.code === '')) head.push(['link', { rel: 'alternate', hreflang: 'x-default', href: absUrl(base) }])

    const isHome = base === 'index.md'
    const ogTitle = title
    const imgAlt = stripTags(fill(L.labels.home.ogAlt))
    head.push(
      ['meta', { property: 'og:type', content: isHome ? 'website' : 'article' }],
      ['meta', { property: 'og:title', content: ogTitle }],
      ['meta', { property: 'og:description', content: description }],
      ['meta', { property: 'og:url', content: url }],
      ['meta', { property: 'og:locale', content: L.og }],
      ...alts.filter((a) => a !== L && a.code !== L.code).map((a): HeadConfig => ['meta', { property: 'og:locale:alternate', content: a.og }]),
      ['meta', { property: 'og:image', content: ogImage(L.code) }],
      ['meta', { property: 'og:image:type', content: 'image/png' }],
      ['meta', { property: 'og:image:width', content: '1200' }],
      ['meta', { property: 'og:image:height', content: '630' }],
      ['meta', { property: 'og:image:alt', content: imgAlt }],
      ['meta', { name: 'twitter:title', content: ogTitle }],
      ['meta', { name: 'twitter:description', content: description }],
      ['meta', { name: 'twitter:image', content: ogImage(L.code) }],
      ['meta', { name: 'twitter:image:alt', content: imgAlt }],
    )
    if (!isHome && DATES.has(rel)) head.push(['meta', { property: 'article:modified_time', content: isoDate(DATES.get(rel)!) }])

    const pageTitle = isHome ? title : pageData.title || title
    const graph = isHome ? homeGraph(L, url) : pageGraph(pageData, L, url, pageTitle, description)
    head.push(['script', { type: 'application/ld+json' }, JSON.stringify({ '@context': 'https://schema.org', '@graph': graph })])
    return head
  },

  buildEnd(siteConfig) {
    writeLlms(siteConfig.outDir)
  },

  themeConfig: {
    logo: { light: { src: '/mark-light.svg', width: 30, height: 30 }, dark: { src: '/mark-dark.svg', width: 30, height: 30 }, alt: '' },
    socialLinks: [{ icon: 'github', link: GITHUB, ariaLabel: 'GitHub' }],
    search: { provider: 'local', options: { locales: searchLocales } },
  },
})

