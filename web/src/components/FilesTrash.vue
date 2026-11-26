<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytes } from '../format.js'
import Icon from './Icon.vue'

// FilesTrash shows the trash of the website (or the server) that holds
// `path`: restore or delete items, or empty it. "changed" asks the parent
// to reload its listing after a restore.
const props = defineProps({
  path: { type: String, required: true },
})
const emit = defineEmits(['changed', 'close'])
const { t, locale } = useI18n()

const data = ref(null)
const loading = ref(false)
const busy = ref('')
const error = ref('')
const notice = ref('')

const scopeName = computed(() => data.value?.site || t('files.trash_server'))

async function load() {
  loading.value = true
  try {
    data.value = await api('GET', `api/files/trash?path=${encodeURIComponent(props.path)}`)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

async function act(key, fn) {
  error.value = ''
  notice.value = ''
  busy.value = key
  try {
    await fn()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = ''
    await load()
  }
}

const restore = (e) =>
  act('r:' + e.id, async () => {
    await api('POST', 'api/files/trash/restore', { path: props.path, id: e.id })
    notice.value = t('files.trash_restored', { path: e.path })
    emit('changed')
  })

function remove(e) {
  if (!window.confirm(t('files.trash_delete_confirm', { path: e.path }))) return
  act('d:' + e.id, () => api('POST', 'api/files/trash/delete', { path: props.path, id: e.id }))
}

function emptyAll() {
  if (!window.confirm(t('files.trash_empty_confirm', { n: data.value.count }, data.value.count))) return
  act('empty', async () => {
    await api('POST', 'api/files/trash/empty', { path: props.path })
    notice.value = t('files.trash_emptied')
  })
}

const icon = (e) => (e.type === 'dir' ? 'folder' : e.type === 'link' ? 'link' : 'file')
const fmt = (v) => new Date(v).toLocaleString(locale.value, { dateStyle: 'short', timeStyle: 'short' })

onMounted(load)
</script>

<template>
  <div class="stack">
    <p class="muted small">{{ t('files.trash_hint', { scope: scopeName, days: data?.max_age_days ?? 30 }, data?.max_age_days ?? 30) }}</p>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>

    <p v-if="loading && !data" class="muted">…</p>
    <template v-else-if="data">
      <p v-if="!data.count" class="muted">{{ t('files.trash_empty') }}</p>
      <template v-else>
        <div class="row">
          <span class="small">{{ t('files.trash_summary', { n: data.count, size: formatBytes(data.total_size) }, data.count) }}</span>
          <span class="toolbar-gap"></span>
          <button class="btn btn-sm btn-danger" :disabled="!!busy" @click="emptyAll"><Icon name="trash" :size="15" />{{ t('files.trash_empty_all') }}</button>
        </div>
        <div class="list entries">
          <div v-for="e in data.entries" :key="e.id" class="list-row">
            <div class="list-main">
              <div class="entry-path">
                <span class="ftype" :class="{ 'ftype-dir': e.type === 'dir' }" aria-hidden="true"><Icon :name="icon(e)" :size="16" /></span>
                <span class="mono path">{{ e.path }}</span>
              </div>
              <div class="list-meta">
                <span v-if="e.type === 'file' || e.type === 'dir'">{{ formatBytes(e.size) }}</span>
                <span>{{ t('files.trash_deleted_at', { date: fmt(e.deleted_at) }) }}</span>
                <span v-if="e.owner">{{ e.owner }}:{{ e.group }}</span>
              </div>
              <p v-if="e.occupied" class="small warn-text">{{ t('files.trash_occupied') }}</p>
            </div>
            <div class="list-side">
              <button class="btn btn-sm" :disabled="!!busy || e.occupied" @click="restore(e)">{{ t('files.trash_restore') }}</button>
              <button class="btn btn-sm btn-danger" :disabled="!!busy" @click="remove(e)">{{ t('files.trash_delete') }}</button>
            </div>
          </div>
        </div>
        <p v-if="data.truncated" class="muted small">{{ t('files.trash_truncated') }}</p>
      </template>
    </template>

    <div class="modal-actions">
      <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.close') }}</button>
    </div>
  </div>
</template>

<style scoped>
.entries { max-height: 55vh; overflow: auto; margin: 0; border: 1px solid var(--border); border-radius: var(--radius-sm); }
.entries .list-row { padding: 10px 14px; }
.entry-path { display: flex; align-items: flex-start; }
.path { word-break: break-all; }
.warn-text { color: var(--warn); margin: 4px 0 0; }
</style>
