<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from './Icon.vue'
import JobLog from './JobLog.vue'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytes } from '../format.js'
import { useJob } from '../composables/useJob.js'

// The backups on one storage that the panel does not list: for a new
// server, or after the list was lost. The panel reads the manifest at the
// start of each file; the chosen ones are added to the backup list (and
// restored from there).
const props = defineProps({
  remote: { type: String, required: true },
  label: { type: String, required: true },
})
const emit = defineEmits(['close', 'changed', 'restore'])
const { t, locale } = useI18n()

const list = ref(null)
const error = ref('')
const notice = ref('')
const passphrase = ref('')
const busy = ref(false)
const { job, running, watch, clear } = useJob(() => load())

async function load() {
  try {
    const d = await api('GET', `api/backups/remote-scan?remote=${encodeURIComponent(props.remote)}`)
    list.value = d.found ? d : null
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function scan() {
  error.value = ''
  notice.value = ''
  try {
    watch(await api('POST', 'api/backups/remote-scan', { remote: props.remote, passphrase: passphrase.value }))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// Opening the list looks at the storage again: an earlier result may be
// from before the storage changed.
onMounted(scan)

const found = computed(() => list.value?.found || [])
const locked = computed(() => found.value.filter((f) => !f.readable && f.encrypted))
const importable = computed(() => found.value.filter((f) => f.readable && !f.imported))

async function importKeys(keys) {
  error.value = ''
  notice.value = ''
  busy.value = true
  try {
    const { ids } = await api('POST', 'api/backups/remote-import', { remote: props.remote, keys })
    await load()
    notice.value = t('backups.found_imported', { n: ids.length })
    emit('changed')
    return ids
  } catch (e) {
    error.value = errorMessage(e)
    return null
  } finally {
    busy.value = false
  }
}

async function restore(f) {
  const ids = await importKeys([f.key])
  if (ids?.length) emit('restore', ids[0])
}

// The usual reasons a file cannot be read are translated; others are shown as the server says them.
const reason = (f) => (f.error_code ? t(`backups.found_err_${f.error_code}`) : f.error)

const kindIcon = (k) => ({ site: 'globe', database: 'database', folder: 'folder', app: 'package' })[k] || 'archive'
const when = (f) => new Date(f.created_at || f.modified).toLocaleString(locale.value, { dateStyle: 'medium', timeStyle: 'short' })
</script>

<template>
  <section class="found stack">
    <div class="found-head">
      <h3 class="found-title"><Icon name="search" :size="16" />{{ t('backups.found_title', { storage: label }) }}</h3>
      <div class="row">
        <button type="button" class="btn btn-sm" :disabled="running" @click="scan"><Icon name="refresh" :size="14" />{{ t('backups.found_scan') }}</button>
        <button type="button" class="btn btn-sm btn-ghost" @click="emit('close')">{{ t('common.close') }}</button>
      </div>
    </div>
    <p class="muted small found-hint">{{ t('backups.found_hint') }}</p>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <JobLog v-if="job && (running || job.status === 'failed')" :job="job" @close="clear" />

    <template v-if="list && !running">
      <p v-if="!found.length" class="muted">{{ t('backups.found_none', { n: list.known }) }}</p>
      <template v-else>
        <form v-if="locked.length" class="stack-tight found-pass" @submit.prevent="scan">
          <span class="small">{{ t('backups.found_locked', { n: locked.length }) }}</span>
          <div class="row">
            <input v-model="passphrase" class="input mono grow" type="password" autocomplete="off" spellcheck="false" :aria-label="t('backups.enc_passphrase')" required />
            <button class="btn btn-sm">{{ t('backups.found_read') }}</button>
          </div>
        </form>
        <div class="table-wrap">
          <table class="table stack-table">
            <thead>
              <tr>
                <th>{{ t('backups.col_target') }}</th>
                <th class="found-meta">{{ t('backups.col_created') }}</th>
                <th class="found-meta">{{ t('backups.col_size') }}</th>
                <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="f in found" :key="f.key">
                <td>
                  <div class="name-ico">
                    <span class="tile" :class="'k-' + f.kind"><Icon :name="kindIcon(f.kind)" :size="16" /></span>
                    <div class="name-ico-text">
                      <div class="found-target" :class="{ mono: f.kind === 'folder' }">
                        {{ f.target || f.name }}
                        <span v-if="f.encrypted" class="tag tag-ok found-tag"><Icon name="lock" :size="12" />{{ t('backups.encrypted') }}</span>
                      </div>
                      <div v-if="f.target" class="name-ico-sub mono found-file">{{ f.name }}</div>
                      <div class="name-ico-sub found-meta-inline">{{ when(f) }} · {{ formatBytes(f.size) }}</div>
                      <div v-if="f.readable && (f.php_version || f.databases.length)" class="name-ico-sub">
                        <template v-if="f.php_version">PHP {{ f.php_version }}</template>
                        <template v-if="f.php_version && f.databases.length"> · </template>
                        <span v-if="f.databases.length" class="mono">{{ f.databases.join(', ') }}</span>
                      </div>
                      <div v-if="!f.readable" class="found-err small">{{ reason(f) }}</div>
                    </div>
                  </div>
                </td>
                <td class="nowrap found-meta">{{ when(f) }}</td>
                <td class="nowrap found-meta">{{ formatBytes(f.size) }}</td>
                <td class="actions">
                  <span v-if="f.imported" class="status-pill ok">{{ t('backups.found_in_list') }}</span>
                  <div v-else-if="f.readable" class="found-actions">
                    <button type="button" class="btn btn-sm" :disabled="busy" @click="importKeys([f.key])">{{ t('backups.found_import') }}</button>
                    <button type="button" class="btn btn-sm btn-primary" :disabled="busy" @click="restore(f)"><Icon name="refresh" :size="14" />{{ t('backups.restore') }}</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="importable.length > 1" class="row">
          <button type="button" class="btn btn-sm btn-primary" :disabled="busy" @click="importKeys(importable.map((f) => f.key))">
            {{ t('backups.found_import_all', { n: importable.length }) }}
          </button>
        </div>
      </template>
    </template>
  </section>
</template>

<style scoped>
.found { border-top: 1px solid var(--border); margin-top: 16px; padding-top: 14px; gap: 10px; }
.found-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.found-title { display: flex; align-items: center; gap: 8px; margin: 0; font-size: 15px; }
.found-hint { margin: 0; }
.found-target { font-weight: 600; overflow-wrap: anywhere; }
.found-file { overflow-wrap: anywhere; font-size: 11.5px; }
.found-tag { min-height: 20px; font-size: 11px; margin-left: 4px; vertical-align: 1px; }
.found-tag .icon { margin-right: 3px; }
.found-err { color: var(--crit); overflow-wrap: anywhere; }
.found-actions { display: flex; gap: 8px; justify-content: flex-end; }
.found-meta-inline { display: none; }
/* On a phone the date and size go under the name, so the buttons stay in view. */
@media (max-width: 640px) {
  .found-meta { display: none; }
  .found-meta-inline { display: block; }
  .found-actions { justify-content: flex-start; flex-wrap: wrap; }
}
/* On a phone each row is a small card: the buttons go under the text. */
@media (max-width: 640px) {
  .stack-table thead { display: none; }
  .stack-table tr { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; padding: 10px 2px; border-bottom: 1px solid var(--border); }
  .stack-table tr:last-child { border-bottom: 0; }
  .stack-table td { display: block; border: 0; padding: 0; min-width: 0; }
  .stack-table td.actions { flex-basis: 100%; }
  .stack-table th.found-meta, .stack-table td.found-meta { display: none; }
}
.found-pass { max-width: 560px; }
.tile.k-site { background: rgb(47 107 219 / 0.12); color: #2f6bdb; }
.tile.k-database { background: rgb(116 84 224 / 0.12); color: #7454e0; }
.tile.k-folder { background: rgb(217 154 11 / 0.14); color: #b7800a; }
</style>
