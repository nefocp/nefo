<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytes } from '../format.js'
import Icon from './Icon.vue'

// FilesSearch finds files and folders under `dir` by name and, optionally,
// by content. Clicking a result emits "open" with it.
const props = defineProps({
  dir: { type: String, required: true },
  initialName: { type: String, default: '' },
})
const emit = defineEmits(['open', 'close'])
const { t } = useI18n()

const name = ref(props.initialName)
const inContent = ref(false)
const content = ref('')
const result = ref(null)
const busy = ref(false)
const error = ref('')

const canSearch = () => name.value.trim() !== '' || (inContent.value && content.value.trim() !== '')

async function run() {
  if (!canSearch() || busy.value) return
  error.value = ''
  busy.value = true
  try {
    const q = new URLSearchParams({ path: props.dir, name: name.value.trim() })
    if (inContent.value && content.value.trim()) q.set('content', content.value)
    result.value = await api('GET', `api/files/search?${q}`)
  } catch (e) {
    error.value = errorMessage(e)
    result.value = null
  } finally {
    busy.value = false
  }
}

// Results are shown relative to the folder searched.
function rel(p) {
  const base = props.dir === '/' ? '/' : props.dir + '/'
  return p.startsWith(base) ? p.slice(base.length) : p
}

const icon = (h) => (h.type === 'dir' ? 'folder' : h.type === 'link' ? 'link' : 'file')

onMounted(run)
</script>

<template>
  <div class="stack">
    <form class="stack" @submit.prevent="run">
      <div class="row search-row">
        <label class="field search-field">
          <span>{{ t('files.search_name') }}</span>
          <input v-model="name" class="input mono" type="search" maxlength="200" spellcheck="false" autocomplete="off" />
        </label>
        <button class="btn btn-primary" :disabled="busy || !canSearch()">{{ busy ? t('files.searching') : t('files.search_run') }}</button>
      </div>
      <label class="check"><input v-model="inContent" type="checkbox" /><span>{{ t('files.search_in_content') }}</span></label>
      <label v-if="inContent" class="field">
        <span>{{ t('files.search_content') }}</span>
        <input v-model="content" class="input mono" maxlength="200" spellcheck="false" autocomplete="off" />
      </label>
      <small class="muted">{{ t('files.search_hint') }}</small>
    </form>

    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>

    <template v-if="result">
      <p v-if="result.truncated" class="alert alert-warn">{{ t('files.search_truncated') }}</p>
      <p class="muted small">
        {{ t('files.search_results', { n: result.hits.length }, result.hits.length) }}, {{ t('files.search_walked', { walked: result.walked }, result.walked) }}
        <template v-if="result.skipped"> · {{ t('files.search_skipped', { n: result.skipped }, result.skipped) }}</template>
      </p>
      <p v-if="!result.hits.length" class="muted">{{ t('files.search_none') }}</p>
      <div v-else class="list hits">
        <div v-for="h in result.hits" :key="h.path" class="list-row">
          <div class="list-main">
            <a href="#" class="hit-link" @click.prevent="emit('open', h)">
              <span class="ftype" :class="{ 'ftype-dir': h.type === 'dir' }" aria-hidden="true"><Icon :name="icon(h)" :size="16" /></span>
              <span class="mono path">{{ rel(h.path) }}</span>
            </a>
            <div class="list-meta">
              <span v-if="h.type === 'file'">{{ formatBytes(h.size) }}</span>
              <span v-if="h.line">{{ t('files.search_line', { n: h.line }) }}</span>
            </div>
            <code v-if="h.excerpt" class="snippet">{{ h.excerpt }}</code>
          </div>
        </div>
      </div>
    </template>

    <div class="modal-actions">
      <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.close') }}</button>
    </div>
  </div>
</template>

<style scoped>
.search-row { align-items: flex-end; flex-wrap: nowrap; }
.search-field { flex: 1; }
.hits { max-height: 52vh; overflow: auto; margin: 0; border: 1px solid var(--border); border-radius: var(--radius-sm); }
.hits .list-row { padding: 10px 14px; }
.hit-link { display: inline-flex; align-items: flex-start; color: var(--text); font-weight: 550; }
.path { word-break: break-all; }
.snippet { display: block; margin-top: 4px; white-space: pre-wrap; word-break: break-all; font-size: 12px; background: var(--surface-2); padding: 4px 8px; border-radius: var(--radius-sm); }
</style>
