<script setup lang="ts">
// The 404 page, in the language of the address that was not found. Also the
// layout of site/<code>/404.md, so each language has its own 404.html.
import { computed } from 'vue'
import { useData } from 'vitepress'
import Ico from './Ico.vue'
import { useNf } from '../lib'

const { theme } = useData()
const nf = useNf()
const t = computed(() => theme.value.notFound as { title: string; quote: string; linkLabel: string; linkText: string; docsText: string })
</script>

<template>
  <div class="nf-404">
    <p class="code">404</p>
    <h1>{{ t.title }}</h1>
    <p class="quote">{{ t.quote }}</p>
    <div class="actions">
      <a class="nf-btn-install" :href="nf.root" :aria-label="t.linkLabel"><Ico name="home" />{{ t.linkText }}</a>
      <a class="nf-btn-line" :href="`${nf.root}docs/`"><Ico name="book" />{{ t.docsText }}</a>
    </div>
  </div>
</template>
