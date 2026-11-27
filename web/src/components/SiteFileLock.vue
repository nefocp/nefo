<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { toast } from '../toast.js'
import { appName } from '../logos.js'

// SiteFileLock is a website's file lock (internal/hosting/filelock.go): its
// code becomes read-only for PHP, the app and SFTP, except the writable
// folders; an optional daily check compares the PHP files with a snapshot.
const props = defineProps({ site: { type: Object, required: true } })
const emit = defineEmits(['saved'])
const { t, locale } = useI18n()

const info = ref(null)
const error = ref('')
const busy = ref(false)
const writable = ref('')
const alert = ref(false)

const fmtDate = (v) => new Date(v).toLocaleString(locale.value, { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
const isDocker = computed(() => (props.site.app || '').startsWith('docker:'))
const presetName = computed(() => (info.value?.preset ? appName(info.value.preset) || info.value.preset : ''))
const lines = () => writable.value.split('\n').map((l) => l.trim()).filter(Boolean)
const changed = computed(() => info.value && (lines().join('\n') !== (info.value.writable || []).join('\n') || alert.value !== info.value.alert))

function apply(res) {
  info.value = res
  writable.value = (res.writable || []).join('\n')
  alert.value = !!res.alert
}

onMounted(async () => {
  try {
    apply(await api('GET', `api/sites/${props.site.id}/filelock`))
  } catch (e) {
    error.value = errorMessage(e)
  }
})

function usePreset() {
  writable.value = (info.value.presets[info.value.preset] || []).join('\n')
}

async function set(on) {
  error.value = ''
  busy.value = true
  try {
    const res = await api('PUT', `api/sites/${props.site.id}/filelock`, { on, writable: lines(), alert: on && alert.value })
    const was = info.value.on
    apply(res)
    emit('saved', { ...props.site, file_lock: res.on })
    if (res.on !== was) toast(t(res.on ? 'filelock.locked_toast' : 'filelock.unlocked_toast'))
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function snapshot() {
  error.value = ''
  busy.value = true
  try {
    apply(await api('POST', `api/sites/${props.site.id}/filelock/snapshot`, {}))
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="!info && !error" class="muted">…</div>
  <div v-else class="stack">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="isDocker" class="alert alert-info">{{ t('filelock.docker') }}</p>
    <template v-else-if="info">
      <div class="fl-status">
        <span class="status-pill" :class="info.on ? 'ok' : ''">{{ info.on ? t('filelock.status_on', { date: fmtDate(info.locked_at) }) : t('filelock.status_off') }}</span>
        <span v-if="info.busy" class="tag tag-warn">{{ t('filelock.busy') }}</span>
      </div>
      <p class="muted small fl-text">{{ t('filelock.what') }}</p>
      <p class="muted small fl-text">{{ t('filelock.panel_ops') }}</p>
      <p v-if="site.app === 'wordpress'" class="muted small fl-text">{{ t('filelock.wp_updates') }}</p>

      <label class="field">
        <span>{{ t('filelock.writable') }}</span>
        <textarea v-model="writable" class="input mono" rows="4" spellcheck="false" placeholder="public/wp-content/uploads"></textarea>
        <small class="muted">{{ t('filelock.writable_hint', { root: site.root || '/var/www/' + site.domain }) }}</small>
      </label>
      <div v-if="info.preset" class="row">
        <button type="button" class="btn btn-sm" :disabled="busy" @click="usePreset">{{ t('filelock.use_preset', { app: presetName }) }}</button>
      </div>
      <p v-if="info.missing && info.missing.length" class="alert alert-warn small">{{ t('filelock.missing', { list: info.missing.join(', ') }) }}</p>

      <label class="check"><input v-model="alert" type="checkbox" /><span>{{ t('filelock.alert') }}</span></label>
      <small class="muted fl-text">{{ t('filelock.alert_hint') }}</small>

      <div v-if="info.on && info.alert" class="fl-scan">
        <p class="small">{{ info.scan_at ? t('filelock.scan_at', { date: fmtDate(info.scan_at) }) : t('filelock.scan_none') }}</p>
        <p v-if="info.scan_at && !info.change_count" class="small text-ok">{{ t('filelock.scan_clean') }}</p>
        <template v-if="info.change_count">
          <p class="small"><strong>{{ t('filelock.changes', { n: info.change_count }) }}</strong></p>
          <ul class="fl-changes small">
            <li v-for="c in info.changes" :key="c.path"><span class="tag" :class="c.kind === 'changed' ? 'tag-warn' : c.kind === 'added' ? 'tag-crit' : ''">{{ t('filelock.kind_' + c.kind) }}</span> <span class="mono">{{ c.path }}</span></li>
          </ul>
          <p v-if="info.partial" class="muted small">{{ t('filelock.partial') }}</p>
          <div class="row">
            <button type="button" class="btn btn-sm" :disabled="busy" @click="snapshot">{{ t('filelock.snapshot') }}</button>
            <span class="muted small">{{ t('filelock.snapshot_hint') }}</span>
          </div>
        </template>
      </div>

      <div class="modal-actions">
        <span v-if="busy" class="muted small">{{ t('filelock.working') }}</span>
        <button v-if="info.on" type="button" class="btn" :disabled="busy" @click="set(false)">{{ t('filelock.unlock') }}</button>
        <button v-if="info.on" type="button" class="btn btn-primary" :disabled="busy || !changed" @click="set(true)">{{ t('filelock.save') }}</button>
        <button v-else type="button" class="btn btn-primary" :disabled="busy" @click="set(true)">{{ t('filelock.lock') }}</button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.fl-status { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.fl-text { margin: 0; max-width: 72ch; }
.fl-scan { border-top: 1px solid var(--border); padding-top: 10px; }
.fl-scan p { margin: 0 0 6px; }
.fl-changes { margin: 0 0 8px; padding-left: 0; list-style: none; max-height: 220px; overflow: auto; }
.fl-changes li { padding: 2px 0; word-break: break-all; }
</style>
