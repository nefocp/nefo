<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { useJob } from '../composables/useJob.js'
import JobLog from './JobLog.vue'

const props = defineProps({ version: { type: String, required: true } })
const emit = defineEmits(['close'])
const { t } = useI18n()

const cfg = ref(null)
const tab = ref('ext')
const ini = ref({})
const error = ref('')
const notice = ref('')
const saving = ref(false)
const { job, running, watch: watchJob, clear: clearJob } = useJob(load)

// Grouped so the form reads in a sensible order.
const groups = [
  { title: 'php.group_general', keys: ['date.timezone', 'max_input_vars', 'max_input_time', 'default_socket_timeout', 'session.gc_maxlifetime', 'realpath_cache_size'] },
  { title: 'php.group_security', keys: ['display_errors', 'expose_php', 'allow_url_fopen', 'short_open_tag'] },
  { title: 'php.group_opcache', keys: ['opcache.enable', 'opcache.memory_consumption', 'opcache.max_accelerated_files', 'opcache.validate_timestamps', 'opcache.revalidate_freq'] },
]
const choices = {
  display_errors: ['On', 'Off'], expose_php: ['On', 'Off'], allow_url_fopen: ['On', 'Off'], short_open_tag: ['On', 'Off'],
  'opcache.enable': ['1', '0'], 'opcache.validate_timestamps': ['1', '0'],
}

async function load() {
  try {
    cfg.value = await api('GET', `api/php/${props.version}`)
    ini.value = { ...cfg.value.ini }
  } catch (e) {
    error.value = errorMessage(e)
  }
}
onMounted(load)

const extensions = computed(() => (cfg.value?.extensions || []).filter((e) => e.available || e.installed))
// The settings this PHP version has: the server lists their defaults.
const shown = (g) => g.keys.filter((k) => cfg.value && k in cfg.value.defaults)

async function toggle(ext) {
  error.value = notice.value = ''
  try {
    watchJob(await api(ext.installed ? 'DELETE' : 'POST', `api/php/${props.version}/extensions/${ext.name}`))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function saveIni() {
  error.value = notice.value = ''
  saving.value = true
  try {
    const values = {}
    for (const g of groups) for (const k of shown(g)) values[k] = (ini.value[k] ?? '').toString()
    cfg.value = await api('PUT', `api/php/${props.version}/ini`, { ini: values })
    ini.value = { ...cfg.value.ini }
    notice.value = t('php.saved')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="stack">
    <div class="row">
      <button class="btn btn-sm" :class="{ 'btn-primary': tab === 'ext' }" @click="tab = 'ext'">{{ t('php.extensions') }}</button>
      <button class="btn btn-sm" :class="{ 'btn-primary': tab === 'ini' }" @click="tab = 'ini'">{{ t('php.settings') }}</button>
    </div>
    <p v-if="cfg?.eol" class="alert alert-warn">{{ t('php.eol_warning', { v: version }) }}</p>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />

    <template v-if="cfg && tab === 'ext'">
      <p class="muted small">{{ t('php.extensions_hint') }}</p>
      <div class="list">
        <div v-for="e in extensions" :key="e.name" class="list-row">
          <div class="list-main">
            <div class="list-title"><span class="mono">{{ e.name }}</span>
              <span v-if="e.installed" class="tag tag-ok">{{ t('software.installed') }}</span>
            </div>
            <div class="muted small">{{ t(`php.ext_${e.name}`) }}</div>
          </div>
          <div class="list-side">
            <button class="btn btn-sm" :class="{ 'btn-danger': e.installed }" :disabled="running" @click="toggle(e)">
              {{ e.installed ? t('software.remove') : t('software.install') }}
            </button>
          </div>
        </div>
      </div>
    </template>

    <form v-if="cfg && tab === 'ini'" class="stack" @submit.prevent="saveIni">
      <p class="muted small">{{ t('php.settings_hint') }}</p>
      <fieldset v-for="g in groups" :key="g.title" class="stack ini-group">
        <legend>{{ t(g.title) }}</legend>
        <label v-for="k in shown(g)" :key="k" class="field ini-row">
          <span class="mono small">{{ k }}</span>
          <select v-if="choices[k]" v-model="ini[k]" class="input">
            <option value="">{{ t('php.default', { v: cfg.defaults[k] }) }}</option>
            <option v-for="c in choices[k]" :key="c" :value="c">{{ c }}</option>
          </select>
          <input v-else v-model="ini[k]" class="input mono" :placeholder="t('php.default', { v: cfg.defaults[k] })" />
        </label>
      </fieldset>
      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.close') }}</button>
        <button class="btn btn-primary" :disabled="saving">{{ t('files.save') }}</button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.ini-group { border: 0; padding: 0; margin: 0; }
.ini-group legend { font-weight: 600; font-size: 13px; margin-bottom: 6px; }
.ini-row { display: grid; grid-template-columns: minmax(200px, 1fr) 2fr; align-items: center; gap: 10px; }
@media (max-width: 640px) { .ini-row { grid-template-columns: 1fr; } }
</style>
