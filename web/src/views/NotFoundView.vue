<script setup>
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import Icon from '../components/Icon.vue'

const { t } = useI18n()
const route = useRoute()
</script>

<template>
  <div class="card empty notfound">
    <h1 class="sr-only">{{ t('notfound.title') }}</h1>
    <div class="empty-icon"><Icon name="fileX" :size="26" /></div>
    <p class="nf-code" aria-hidden="true">404</p>
    <p class="empty-lead">{{ t('notfound.lead') }}</p>
    <p class="muted">{{ t('notfound.body', { path: route.fullPath }) }}</p>
    <div class="row" style="justify-content: center">
      <RouterLink :to="{ name: 'dashboard' }" class="btn btn-primary"><Icon name="dashboard" :size="16" />{{ t('notfound.home') }}</RouterLink>
      <RouterLink :to="{ name: 'websites' }" class="btn"><Icon name="globe" :size="16" />{{ t('nav.websites') }}</RouterLink>
    </div>
  </div>
</template>

<style scoped>
.notfound { padding: 56px 20px; }
.nf-code { font-size: 44px; font-weight: 800; letter-spacing: -0.04em; margin: 0 0 4px; color: var(--text); line-height: 1; }
</style>
