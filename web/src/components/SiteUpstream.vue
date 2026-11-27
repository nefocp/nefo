<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'

// SiteUpstream edits a reverse proxy website: the address it forwards to
// and how (Host header, WebSocket, X-Forwarded-* headers, time limit, the
// upstream's certificate). Saved sites go up with "saved"; "close" resets
// the form.
const props = defineProps({
  site: { type: Object, required: true },
  server: { type: String, default: 'nginx' },
})
const emit = defineEmits(['saved', 'close'])
const { t } = useI18n()

const p = props.site.proxy || {}
const form = ref({
  url: props.site.proxy_url || '',
  host_header: p.host_header || 'site',
  websocket: p.websocket !== false,
  forwarded: p.forwarded !== false,
  timeout: p.timeout || 60,
  insecure: !!p.insecure,
})
const isHTTPS = computed(() => /^https:\/\//i.test(form.value.url.trim()))
const upstreamHost = computed(() => {
  try {
    return new URL(form.value.url.trim()).host || '…'
  } catch {
    return '…'
  }
})
const error = ref('')
const busy = ref(false)

async function save() {
  error.value = ''
  busy.value = true
  try {
    const site = await api('PUT', `api/sites/${props.site.id}/proxy`, {
      proxy_url: form.value.url.trim(),
      proxy: {
        host_header: form.value.host_header,
        websocket: form.value.websocket,
        forwarded: form.value.forwarded,
        timeout: Number(form.value.timeout) || 60,
        insecure: isHTTPS.value && form.value.insecure,
      },
    })
    emit('saved', site)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="stack" @submit.prevent="save">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <label class="field">
      <span>{{ t('proxy.url') }}</span>
      <input v-model="form.url" class="input mono" placeholder="http://10.0.0.5:8080" maxlength="500" autocomplete="off" autocapitalize="off" spellcheck="false" required />
      <small class="muted">{{ t('proxy.url_hint') }}</small>
    </label>
    <label class="field">
      <span>{{ t('proxy.host_header') }}</span>
      <select v-model="form.host_header" class="input">
        <option value="site">{{ t('proxy.host_site', { domain: site.domain }) }}</option>
        <option value="upstream">{{ t('proxy.host_upstream', { host: upstreamHost }) }}</option>
      </select>
      <small class="muted">{{ t('proxy.host_hint') }}</small>
    </label>
    <div class="opt">
      <div class="opt-text">
        <div class="opt-title">{{ t('proxy.websocket') }}</div>
        <div class="opt-hint">{{ t('proxy.websocket_hint') }}</div>
      </div>
      <button type="button" class="switch" role="switch" :aria-checked="form.websocket" :aria-label="t('proxy.websocket')" @click="form.websocket = !form.websocket"></button>
    </div>
    <div class="opt">
      <div class="opt-text">
        <div class="opt-title">{{ t('proxy.forwarded') }}</div>
        <div class="opt-hint">{{ t('proxy.forwarded_hint') }}</div>
      </div>
      <button type="button" class="switch" role="switch" :aria-checked="form.forwarded" :aria-label="t('proxy.forwarded')" @click="form.forwarded = !form.forwarded"></button>
    </div>
    <label class="field">
      <span>{{ t('proxy.timeout') }}</span>
      <input v-model="form.timeout" class="input input-auto" type="number" min="5" max="3600" required />
      <small class="muted">{{ t('proxy.timeout_hint') }}</small>
    </label>
    <template v-if="isHTTPS && server !== 'openlitespeed'">
      <label class="check"><input v-model="form.insecure" type="checkbox" /><span>{{ t('proxy.insecure') }}</span></label>
      <p v-if="form.insecure" class="alert alert-warn">{{ t('proxy.insecure_warn') }}</p>
    </template>
    <p v-if="server === 'openlitespeed'" class="alert alert-warn small">{{ t('proxy.ols_note') }}</p>
    <div class="modal-actions">
      <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button class="btn btn-primary" :disabled="busy">{{ t('files.save') }}</button>
    </div>
  </form>
</template>
