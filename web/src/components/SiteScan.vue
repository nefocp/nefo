<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { useJob } from '../composables/useJob.js'
import JobLog from './JobLog.vue'

const props = defineProps({ site: { type: Object, required: true } })
const emit = defineEmits(['close'])
const { t, te, locale } = useI18n()

const rep = ref(null)
const error = ref('')
const clamav = ref(false)
const busy = ref('')
const { job, watch: watchJob, clear: clearJob } = useJob(load)
const running = computed(() => job.value?.status === 'running')

async function load() {
  try {
    rep.value = await api('GET', `api/sites/${props.site.id}/scan`)
  } catch (e) {
    error.value = errorMessage(e)
  }
}
onMounted(load)

async function scan() {
  error.value = ''
  try {
    watchJob(await api('POST', `api/sites/${props.site.id}/scan`, { clamav: clamav.value }))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function act(key, method, path, body) {
  error.value = ''
  busy.value = key
  try {
    await api(method, path, body)
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = ''
  }
}

const quarantine = (f) => act('q:' + f.path, 'POST', `api/sites/${props.site.id}/quarantine`, { path: f.path, rule: f.rule })
const restore = (q) => act('r:' + q.id, 'POST', `api/sites/${props.site.id}/quarantine/${q.id}/restore`, {})
function remove(q) {
  if (!window.confirm(t('scan.delete_confirm', { path: q.path }))) return
  act('d:' + q.id, 'DELETE', `api/sites/${props.site.id}/quarantine/${q.id}`)
}

const ruleName = (f) => (f.engine === 'clamav' ? `ClamAV: ${f.name}` : te(`scan.rule_${f.rule}`) ? t(`scan.rule_${f.rule}`) : f.name)
const fmt = (v) => new Date(v).toLocaleString(locale.value, { dateStyle: 'medium', timeStyle: 'short' })
const sevClass = { critical: 'tag-crit', high: 'tag-crit', medium: 'tag-warn' }
const clamNote = computed(() => {
  const n = rep.value?.clamav || ''
  if (!n || n === 'ok') return ''
  const [code, mb] = n.split(':')
  return t(`scan.clam_${code}`, { mb })
})
</script>

<template>
  <div class="stack">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p class="muted small">{{ t('scan.hint') }}</p>
    <div class="row">
      <button class="btn btn-primary" :disabled="running" @click="scan">{{ running ? t('scan.running') : t('scan.start') }}</button>
      <label v-if="rep?.clamav_installed" class="check"><input v-model="clamav" type="checkbox" :disabled="running" /><span>{{ t('scan.with_clamav') }}</span></label>
    </div>
    <JobLog v-if="job" :job="job" @close="clearJob" />

    <template v-if="rep">
      <p v-if="rep.finished_at" class="muted small">
        {{ t('scan.last', { date: fmt(rep.finished_at), files: rep.files }, rep.files) }}<template v-if="rep.skipped"> · {{ t('scan.skipped', { n: rep.skipped }, rep.skipped) }}</template>
      </p>
      <p v-else class="muted">{{ t('scan.never') }}</p>
      <p v-if="clamNote" class="alert alert-warn">{{ clamNote }}</p>

      <p v-if="rep.finished_at && !rep.findings.length" class="alert alert-ok">{{ t('scan.clean') }}</p>
      <div v-if="rep.findings.length" class="list findings">
        <div v-for="(f, i) in rep.findings" :key="i" class="list-row">
          <div class="list-main">
            <div class="list-title">
              <span class="tag" :class="sevClass[f.severity]">{{ t(`scan.sev_${f.severity}`) }}</span>
              <span class="mono path">{{ f.path }}<template v-if="f.line">:{{ f.line }}</template></span>
            </div>
            <div class="small">{{ ruleName(f) }}</div>
            <code v-if="f.snippet" class="snippet">{{ f.snippet }}</code>
          </div>
          <div class="list-side">
            <button class="btn btn-sm" :disabled="!!busy" @click="quarantine(f)">{{ t('scan.quarantine') }}</button>
          </div>
        </div>
      </div>

      <details v-if="rep.quarantined.length" open>
        <summary>{{ t('scan.quarantined', { n: rep.quarantined.length }) }}</summary>
        <p class="muted small">{{ t('scan.quarantined_hint') }}</p>
        <div class="list">
          <div v-for="q in rep.quarantined" :key="q.id" class="list-row">
            <div class="list-main">
              <span class="mono path">{{ q.path }}</span>
              <div class="muted small">{{ fmt(q.created_at) }}</div>
            </div>
            <div class="list-side row">
              <button class="btn btn-sm" :disabled="!!busy" @click="restore(q)">{{ t('scan.restore') }}</button>
              <button class="btn btn-sm btn-danger" :disabled="!!busy" @click="remove(q)">{{ t('scan.delete') }}</button>
            </div>
          </div>
        </div>
      </details>
    </template>
    <div class="modal-actions">
      <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.close') }}</button>
    </div>
  </div>
</template>

<style scoped>
.path { word-break: break-all; }
.snippet { display: block; margin-top: 4px; white-space: pre-wrap; word-break: break-all; font-size: 12px; background: var(--surface-2); padding: 4px 8px; border-radius: var(--radius-sm); }
.findings { max-height: 420px; overflow: auto; }
</style>
