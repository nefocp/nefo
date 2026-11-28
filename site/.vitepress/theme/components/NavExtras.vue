<script setup lang="ts">
// The right end of the nav bar: language, light/dark, GitHub and Install.
// VitePress keeps its own versions of these in the phone menu.
import { computed, inject, onBeforeUnmount, onMounted, ref } from 'vue'
import { useData } from 'vitepress'
import Ico from './Ico.vue'
import { GITHUB, useLanguages, useNf } from '../lib'

const { isDark, frontmatter, site, theme, localeIndex } = useData()
const nf = useNf()
const langs = useLanguages()
const current = computed(() => site.value.locales[localeIndex.value]?.label ?? 'English')
const toggle = inject<() => void>('toggle-appearance', () => {
  isDark.value = !isDark.value
})
const installHref = computed(() => (frontmatter.value.layout === 'home' ? '#install' : `${nf.value.root}docs/install`))

const open = ref(false)
const menu = ref<HTMLElement | null>(null)
const onDoc = (e: Event) => {
  if (menu.value && !menu.value.contains(e.target as Node)) open.value = false
}
const onKey = (e: KeyboardEvent) => {
  if (e.key === 'Escape') open.value = false
}
onMounted(() => {
  document.addEventListener('click', onDoc)
  document.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', onDoc)
  document.removeEventListener('keydown', onKey)
})
</script>

<template>
  <div class="nf-navx">
    <div ref="menu" class="nf-lang">
      <button class="nf-icon-btn" type="button" :aria-expanded="open" aria-haspopup="true" :title="theme.langMenuLabel" @click="open = !open">
        <Ico name="lang" /><span class="nf-lang-label">{{ current }}</span><Ico name="down" class="nf-chev" />
      </button>
      <div v-show="open" class="nf-lang-menu">
        <a v-for="l in langs" :key="l.key" :href="l.link" :hreflang="l.lang" :lang="l.lang" :class="{ on: l.current }" @click="open = false">{{ l.label }}<span>{{ l.code }}</span></a>
      </div>
    </div>
    <button class="nf-icon-btn nf-theme" type="button" :title="isDark ? nf.theme[0] : nf.theme[1]" :aria-label="isDark ? nf.theme[0] : nf.theme[1]" @click="toggle()">
      <Ico name="sun" class="sun" /><Ico name="moon" class="moon" />
    </button>
    <a class="nf-icon-btn nf-gh" :href="GITHUB" aria-label="GitHub"><Ico name="github" /><span>GitHub</span></a>
    <a class="nf-btn-install" :href="installHref"><Ico name="download" />{{ nf.install }}</a>
  </div>
</template>
