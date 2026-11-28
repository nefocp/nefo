<script setup lang="ts">
// Under "On this page": the current release, from .vitepress/data/release.json.
import { useData } from 'vitepress'
import { formatDate, useNf } from '../lib'
import Ico from './Ico.vue'
import release from '../../data/release.json'

const { lang } = useData()
const nf = useNf()
</script>

<template>
  <div class="nf-aside-ver">
    <span class="k">{{ nf.docMeta.release }}</span>
    <b>Nefo {{ release.version }} · {{ release.status }}</b>
    <span>{{ nf.docMeta.released.replace('{date}', formatDate(release.date, lang)) }}</span>
    <div class="links">
      <a :href="release.notes">{{ nf.docMeta.notes }} <Ico name="arrow" /></a>
      <a :href="`${nf.root}changelog`">{{ nf.nav.changelog }} <Ico name="arrow" /></a>
    </div>
  </div>
</template>
