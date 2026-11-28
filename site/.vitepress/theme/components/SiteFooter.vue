<script setup lang="ts">
// The footer of every page: product, docs and project links, the languages.
import { computed } from 'vue'
import { useSidebar } from 'vitepress/theme-without-fonts'
import Ico from './Ico.vue'
import { GITHUB, useLanguages, useNf } from '../lib'

const nf = useNf()
const langs = useLanguages()
const { hasSidebar } = useSidebar()

const cols = computed(() => {
  const n = nf.value
  const r = n.root
  return [
    [n.footer.product, [[n.footer.features, `${r}#features`], [n.footer.screens, `${r}#screens`], [n.footer.apps, `${r}#apps`], [n.nav.compare, `${r}compare`], [n.nav.changelog, `${r}changelog`], [n.nav.roadmap, `${r}roadmap`]]],
    [n.footer.docs, [[n.pages[''], `${r}docs/`], [n.pages.install, `${r}docs/install`], [n.pages.websites, `${r}docs/websites`], [n.pages.backups, `${r}docs/backups`], [n.pages.security, `${r}docs/security`], [n.pages.faq, `${r}docs/faq`]]],
    [n.footer.project, [['GitHub', GITHUB], [n.nav.releases, `${GITHUB}/releases`], [n.footer.issues, `${GITHUB}/issues`], [n.nav.securityPolicy, `${r}security-policy`], [n.nav.about, `${r}about`]]],
  ] as [string, [string, string][]][]
})
</script>

<template>
  <footer class="nf-foot" :class="{ 'has-sidebar': hasSidebar }">
    <div class="nf-foot-in">
      <div class="nf-foot-grid">
        <div class="nf-foot-brand">
          <a class="nf-brand" :href="nf.root"><img class="tl" src="/mark-light.svg" alt="" width="30" height="30" /><img class="td" src="/mark-dark.svg" alt="" width="30" height="30" />Nefo</a>
          <p>{{ nf.footer.about }}</p>
          <div class="nf-foot-icons">
            <a class="nf-icon-btn" :href="GITHUB" aria-label="GitHub"><Ico name="github" /></a>
            <a class="nf-icon-btn" :href="`${GITHUB}/releases`" :aria-label="nf.nav.releases"><Ico name="tag" /></a>
          </div>
        </div>
        <nav v-for="[h, items] in cols" :key="h" :aria-label="h">
          <p class="nf-foot-h">{{ h }}</p>
          <ul>
            <li v-for="[label, href] in items" :key="href"><a :href="href">{{ label }}</a></li>
          </ul>
        </nav>
        <nav :aria-label="nf.footer.languages">
          <p class="nf-foot-h">{{ nf.footer.languages }}</p>
          <ul>
            <li v-for="l in langs" :key="l.key"><a :href="l.link" :hreflang="l.lang" :lang="l.lang">{{ l.label }}</a></li>
          </ul>
        </nav>
      </div>
      <div class="nf-foot-bottom">
        <span>{{ nf.footer.copyright }}. {{ nf.footer.message }}</span>
        <span>{{ nf.footer.beta }}</span>
      </div>
    </div>
  </footer>
</template>
