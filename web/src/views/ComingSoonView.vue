<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const route = useRoute()
const feature = computed(() => route.meta.feature)
</script>

<template>
  <div>
    <h1 class="page-title">{{ t(`nav.${feature}`) }}</h1>
    <div class="card empty">
      <span class="tag">{{ t('soon.title') }}</span>
      <p class="empty-lead">{{ t(`soon.${feature}`) }}</p>
      <p class="muted">{{ t('soon.body') }}</p>
    </div>
  </div>
</template>
