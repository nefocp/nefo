<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from './Icon.vue'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytes, formatDateTime } from '../format.js'

// DbSlowLog turns MariaDB's slow query log on or off and shows its newest
// entries.
const { t, locale } = useI18n()

const settings = ref(null)
const entries = ref([])
const limit = ref(200)
const enabled = ref(false)
const threshold = ref(1)
const error = ref('')
const notice = ref('')
const busy = ref(false)
const expanded = ref(null) // index of the entry whose query is shown in full

function fill(res) {
  settings.value = res.settings
  entries.value = res.entries || []
  limit.value = res.limit
  enabled.value = res.settings.enabled
  threshold.value = res.settings.threshold
}

async function load() {
  try {
    fill(await api('GET', 'api/db/slowlog'))
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function save() {
  error.value = ''
  notice.value = ''
  busy.value = true
  try {
    fill(await api('PUT', 'api/db/slowlog', { enabled: enabled.value, threshold: Number(threshold.value) }))
    notice.value = enabled.value ? t('databases.slow_enabled', { s: threshold.value }) : t('databases.slow_disabled')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function clearLog() {
  if (!window.confirm(t('databases.slow_clear_confirm'))) return
  error.value = ''
  notice.value = ''
  busy.value = true
  try {
    await api('DELETE', 'api/db/slowlog')
    notice.value = t('databases.slow_cleared')
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

const seconds = (v) => (v >= 10 ? v.toFixed(1) : v.toFixed(3))
const preview = (q) => (q.length > 160 ? q.slice(0, 160) + '…' : q)

onMounted(load)
</script>

<template>
  <div class="pg">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <span v-if="!settings && !error" class="skel" style="height: 160px; border-radius: 14px"></span>

    <template v-if="settings">
      <p v-if="!settings.running" class="alert alert-warn">{{ t('databases.slow_not_running') }}</p>
      <form class="opt slow-opt" @submit.prevent="save">
        <div class="opt-text">
          <div class="opt-title">{{ t('databases.slow_title') }}</div>
          <div class="opt-hint">{{ t('databases.slow_intro') }}</div>
        </div>
        <label class="slow-threshold">
          <span>{{ t('databases.slow_threshold') }}</span>
          <input v-model.number="threshold" class="input input-sm" type="number" min="0.1" max="60" step="any" required />
          <span class="muted">{{ t('databases.slow_seconds') }}</span>
        </label>
        <button type="button" class="switch" role="switch" :aria-checked="enabled" :aria-label="t('databases.slow_enable')" @click="enabled = !enabled"></button>
        <button class="btn btn-primary btn-sm" :disabled="busy || !settings.running">{{ t('databases.slow_save') }}</button>
      </form>

      <section class="panel">
        <div class="panel-tools">
          <span class="status-pill" :class="settings.enabled ? 'ok' : ''">{{ settings.enabled ? t('databases.slow_on') : t('databases.slow_off') }}</span>
          <span class="small muted slow-file">
            {{ t('databases.slow_count', { n: entries.length, max: limit }, entries.length) }} · <span class="mono">{{ settings.file }}</span> ({{ formatBytes(settings.size) }})
          </span>
          <span class="slow-gap"></span>
          <button type="button" class="btn btn-sm" :disabled="busy" @click="load"><Icon name="refresh" :size="15" />{{ t('files.refresh') }}</button>
          <button type="button" class="btn btn-sm btn-danger" :disabled="busy || !settings.size" @click="clearLog"><Icon name="trash" :size="15" />{{ t('databases.slow_clear') }}</button>
        </div>

        <div v-if="!entries.length" class="empty">
          <div class="empty-icon"><Icon name="clock" :size="24" /></div>
          <p class="empty-lead">{{ settings.enabled ? t('databases.slow_empty_on') : t('databases.slow_empty') }}</p>
        </div>
        <div v-else class="table-wrap slow-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>{{ t('databases.slow_col_time') }}</th>
                <th>{{ t('databases.slow_col_user') }}</th>
                <th>{{ t('databases.col_name') }}</th>
                <th class="num">{{ t('databases.slow_col_duration') }}</th>
                <th class="num">{{ t('databases.slow_col_rows') }}</th>
                <th>{{ t('databases.slow_col_query') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(q, i) in entries" :key="i">
                <td class="nowrap small">{{ q.time ? formatDateTime(q.time, locale) : '—' }}</td>
                <td class="mono small">{{ q.user }}</td>
                <td class="mono small">{{ q.database || '—' }}</td>
                <td class="num nowrap"><span class="status-pill" :class="q.query_time >= 5 ? 'crit' : q.query_time >= 1 ? 'warn' : ''">{{ seconds(q.query_time) }} s</span></td>
                <td class="num nowrap">{{ q.rows_examined.toLocaleString(locale) }}</td>
                <td class="query-cell">
                  <button type="button" class="query-btn mono small" :aria-expanded="expanded === i" @click="expanded = expanded === i ? null : i">
                    {{ expanded === i ? q.query : preview(q.query) }}
                  </button>
                  <span v-if="q.truncated && expanded === i" class="tag tag-warn">{{ t('databases.slow_truncated') }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.slow-opt { flex-wrap: wrap; gap: 14px; }
.slow-opt .opt-text { flex: 1 1 320px; }
.slow-threshold { display: inline-flex; align-items: center; gap: 8px; font-size: 13px; font-weight: 600; color: var(--text-2); }
.slow-threshold .input { width: 84px; }
.slow-gap { flex: 1; }
.slow-file { overflow-wrap: anywhere; }
.slow-wrap { max-height: 560px; overflow-y: auto; }
.num { text-align: right !important; }
.query-cell { min-width: 280px; max-width: 640px; }
.query-btn {
  display: block; width: 100%; padding: 0; border: 0; background: none; color: inherit; text-align: left;
  cursor: pointer; white-space: pre-wrap; overflow-wrap: anywhere;
}
.query-btn:hover { color: var(--accent); }
</style>
