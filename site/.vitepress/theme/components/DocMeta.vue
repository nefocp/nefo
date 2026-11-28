<script setup lang="ts">
// Under the title of a page: the current release, when the page was last
// changed and how long it takes to read. Rendered on the server, so the date
// is in the HTML that search engines read.
import { computed } from 'vue'
import { useData } from 'vitepress'
import Ico from './Ico.vue'
import { formatDate, useNf } from '../lib'
import release from '../../data/release.json'

const { page, frontmatter, lang } = useData()
const nf = useNf()
const updated = computed(() => (page.value.lastUpdated ? { iso: new Date(page.value.lastUpdated).toISOString(), text: formatDate(page.value.lastUpdated, lang.value) } : null))
const read = computed(() => frontmatter.value.nf?.read as number | undefined)
</script>

<template>
  <div class="nf-meta">
    <a class="chip" :href="release.notes"><Ico name="tag" />Nefo {{ release.version }}</a>
    <span v-if="updated" class="chip"><Ico name="clock" /><span>{{ nf.docMeta.updated }} <time :datetime="updated.iso">{{ updated.text }}</time></span></span>
    <span v-if="read" class="chip"><Ico name="book" />{{ nf.docMeta.readTime.replace('{n}', String(read)) }}</span>
  </div>
</template>
