// The Nefo theme: VitePress's default theme (without its Inter font) with the
// site's own nav extras, footer, docs breadcrumbs and version line, and the
// look of the nefo.io design in custom.css. The home page component is
// imported by each index.md, so docs pages don't load it.
import DefaultTheme from 'vitepress/theme-without-fonts'
import type { Theme } from 'vitepress'
import { h } from 'vue'

import './fonts.css'
import './custom.css'

import release from '../data/release.json'
import AsideRelease from './components/AsideRelease.vue'
import DocCrumbs from './components/DocCrumbs.vue'
import DocMeta from './components/DocMeta.vue'
import NavExtras from './components/NavExtras.vue'
import NotFound from './components/NotFound.vue'
import ReleaseBar from './components/ReleaseBar.vue'
import SiteFooter from './components/SiteFooter.vue'

export default {
  extends: DefaultTheme,
  Layout: () =>
    h(DefaultTheme.Layout, null, {
      'layout-top': () => h(ReleaseBar),
      'nav-bar-content-after': () => h(NavExtras),
      'doc-before': () => h(DocCrumbs),
      'aside-outline-after': () => h(AsideRelease),
      'not-found': () => h(NotFound),
      'layout-bottom': () => h(SiteFooter),
    }),
  enhanceApp({ app }) {
    // <DocMeta /> is put under the title of each page by config.mts
    app.component('DocMeta', DocMeta)
    // the layout of site/<code>/404.md
    app.component('NfNotFound', NotFound)
    // {{ $release.version }} also works in Vue templates
    app.config.globalProperties.$release = release
  },
} satisfies Theme
