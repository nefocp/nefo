<script setup lang="ts">
// One line of news above the nav, on the home pages only.
import { computed } from 'vue'
import { useData } from 'vitepress'
import Ico from './Ico.vue'
import release from '../../data/release.json'

const { frontmatter } = useData()
const bar = computed(() => (frontmatter.value.layout === 'home' ? (frontmatter.value.home?.releaseBar as { text: string; short: string; link: string }) : null))
</script>

<template>
  <div v-if="bar" class="nf-topbar">
    <div class="nf-topbar-in">
      <span class="tag">{{ release.version }}</span>
      <span class="long">{{ bar.text }}</span>
      <span class="short">{{ bar.short }}</span>
      <a :href="release.notes">{{ bar.link }} <Ico name="arrow" /></a>
    </div>
  </div>
</template>
