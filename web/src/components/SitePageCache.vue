<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { toast } from '../toast.js'

// SitePageCache is the page cache of a PHP website: Nginx's FastCGI cache,
// one per site, with a switch and "Purge cache". OpenLiteSpeed has LSCache
// built in instead; Apache gets no tab.
const props = defineProps({
  site: { type: Object, required: true },
  server: { type: String, default: 'nginx' },
  busy: { type: Boolean, default: false },
})
const emit = defineEmits(['saved'])
const { t } = useI18n()
const saving = ref(false)

async function setCache() {
  saving.value = true
  try {
    emit('saved', await api('PUT', `api/sites/${props.site.id}/page-cache`, { enabled: !props.site.page_cache }))
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  } finally {
    saving.value = false
  }
}

async function purge() {
  saving.value = true
  try {
    await api('POST', `api/sites/${props.site.id}/page-cache/purge`, {})
    toast(t('pagecache.purged'))
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <section v-if="server === 'openlitespeed'" class="card">
    <p class="muted pc-text">{{ t('pagecache.ols') }}</p>
  </section>

  <template v-else>
    <div class="opt pc-opt">
      <div class="opt-text">
        <div class="opt-title">{{ t('pagecache.title') }}</div>
        <div class="opt-hint">{{ site.page_cache ? t('pagecache.on_hint') : t('pagecache.off_hint') }}</div>
      </div>
      <button type="button" class="switch" role="switch" :aria-checked="!!site.page_cache" :aria-label="t('pagecache.title')" :disabled="saving || busy" @click="setCache"></button>
    </div>
    <section class="card stack">
      <p class="muted small pc-text">{{ t('pagecache.hint') }}</p>
      <p class="muted small pc-text">{{ t('pagecache.check') }}</p>
      <div v-if="site.page_cache" class="row">
        <button type="button" class="btn" :disabled="saving || busy" @click="purge">{{ t('pagecache.purge') }}</button>
        <span class="muted small">{{ t('pagecache.purge_hint') }}</span>
      </div>
    </section>
  </template>
</template>

<style scoped>
.pc-opt { margin-bottom: 16px; }
.pc-text { margin: 0; }
</style>
