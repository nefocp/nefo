<script setup lang="ts">
// "Docs › Data › Backups" above the title of a docs page.
import { computed } from 'vue'
import { useData } from 'vitepress'
import Ico from './Ico.vue'
import { useNf } from '../lib'

const { page, frontmatter } = useData()
const nf = useNf()
const crumbs = computed(() => {
  const m = page.value.relativePath.match(/(?:^|\/)docs\/(.+)\.md$/)
  if (!m) return null
  const slug = m[1] === 'index' ? '' : m[1]
  const group = frontmatter.value.nf?.group as string | null
  return { slug, group: group ? nf.value.groups[group] : null }
})
</script>

<template>
  <nav v-if="crumbs && crumbs.slug" class="nf-crumbs" aria-label="Breadcrumb">
    <a :href="`${nf.root}docs/`">{{ nf.nav.docs }}</a>
    <template v-if="crumbs.group"><Ico name="chev" /><span>{{ crumbs.group }}</span></template>
    <Ico name="chev" /><span aria-current="page">{{ page.title }}</span>
  </nav>
</template>
