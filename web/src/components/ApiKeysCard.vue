<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatDateTime, relativeTime } from '../format.js'
import Icon from './Icon.vue'
import Modal from './Modal.vue'
import CopyField from './CopyField.vue'

// API keys for scripts. A new key is shown once, in the dialog that made
// it, and forgotten when the dialog closes.
const { t, locale } = useI18n()
const keys = ref(null)
const max = ref(20)
const error = ref('')
const notice = ref('')

async function load() {
  try {
    const res = await api('GET', 'api/settings/api-keys')
    keys.value = res.keys || []
    max.value = res.max || 20
  } catch (e) {
    error.value = errorMessage(e)
  }
}
onMounted(load)

const blank = () => ({ name: '', access: 'read', addresses: '', expires_days: 90 })
const showNew = ref(false)
const form = ref(blank())
const formError = ref('')
const busy = ref(false)
const created = ref(null) // { key, token }

function openNew() {
  form.value = blank()
  formError.value = ''
  created.value = null
  notice.value = ''
  showNew.value = true
}

function closeNew() {
  showNew.value = false
  created.value = null
}

async function create() {
  formError.value = ''
  busy.value = true
  try {
    created.value = await api('POST', 'api/settings/api-keys', {
      name: form.value.name.trim(),
      access: form.value.access,
      addresses: form.value.addresses.split(/[\s,]+/).filter(Boolean),
      expires_days: Number(form.value.expires_days),
    })
    await load()
  } catch (e) {
    formError.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function revoke(k) {
  if (!window.confirm(t('apikeys.revoke_confirm', { name: k.name }))) return
  error.value = ''
  notice.value = ''
  try {
    await api('DELETE', `api/settings/api-keys/${k.id}`)
    notice.value = t('apikeys.revoked', { name: k.name })
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const formatDate = (v) => new Date(v).toLocaleDateString(locale.value, { dateStyle: 'medium' })
// The API of this panel, as a script reaches it (the page is /<entrance>/).
const apiBase = computed(() => `${window.location.origin}${window.location.pathname.replace(/[^/]*$/, '')}api/`)
const full = computed(() => (keys.value || []).length >= max.value)
</script>

<template>
  <section class="panel">
    <div class="panel-head">
      <h2 class="panel-title"><Icon name="key" :size="17" />{{ t('apikeys.title') }}</h2>
      <button type="button" class="btn btn-sm" :disabled="!keys || full" @click="openNew"><Icon name="plus" :size="14" />{{ t('apikeys.create') }}</button>
    </div>
    <div class="panel-body stack">
      <p class="muted small" style="margin: 0">{{ t('apikeys.hint') }}</p>
      <p v-if="error" class="alert alert-crit pre-wrap" role="alert" style="margin: 0">{{ error }}</p>
      <p v-if="notice" class="alert alert-ok" role="status" style="margin: 0">{{ notice }}</p>
    </div>
    <span v-if="!keys && !error" class="skel" style="height: 120px; margin: 0 18px 18px; display: block; border-radius: 12px"></span>
    <div v-else-if="keys && !keys.length" class="empty"><p class="empty-lead">{{ t('apikeys.none') }}</p></div>
    <div v-else-if="keys" class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>{{ t('apikeys.col_name') }}</th>
            <th>{{ t('apikeys.col_access') }}</th>
            <th>{{ t('apikeys.col_addresses') }}</th>
            <th>{{ t('apikeys.col_last_used') }}</th>
            <th>{{ t('apikeys.col_expires') }}</th>
            <th><span class="sr-only">{{ t('common.more') }}</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="k in keys" :key="k.id">
            <td>
              <strong>{{ k.name }}</strong>
              <div class="name-ico-sub mono">{{ k.prefix }}…</div>
            </td>
            <td><span class="tag" :class="{ 'tag-warn': k.access === 'full' }">{{ k.access === 'full' ? t('apikeys.full') : t('apikeys.read') }}</span></td>
            <td class="small">
              <span v-if="!k.addresses.length" class="muted">{{ t('apikeys.any_address') }}</span>
              <span v-else class="mono addr-list">{{ k.addresses.join(', ') }}</span>
            </td>
            <td class="nowrap">
              <template v-if="k.last_used_at">
                <span :title="formatDateTime(k.last_used_at, locale)">{{ relativeTime(k.last_used_at, locale) }}</span>
                <div class="name-ico-sub mono">{{ k.last_used_ip }}</div>
              </template>
              <span v-else class="muted">{{ t('apikeys.never_used') }}</span>
            </td>
            <td class="nowrap">
              <span v-if="k.expired" class="tag tag-crit">{{ t('apikeys.expired') }}</span>
              <span v-else-if="k.expires_at">{{ formatDate(k.expires_at) }}</span>
              <span v-else class="muted">{{ t('apikeys.never') }}</span>
            </td>
            <td class="num">
              <button type="button" class="icon-btn icon-btn-sm danger" :aria-label="t('apikeys.revoke', { name: k.name })" :title="t('apikeys.revoke', { name: k.name })" @click="revoke(k)"><Icon name="trash" :size="15" /></button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <Modal v-if="showNew" :title="created ? t('apikeys.created_title') : t('apikeys.create')" @close="closeNew">
      <div v-if="created" class="stack">
        <p class="alert alert-warn" style="margin: 0">{{ t('apikeys.once') }}</p>
        <CopyField :label="t('apikeys.key_label')" :value="created.token" />
        <div class="stack-tight">
          <span class="muted small">{{ t('apikeys.example') }}</span>
          <pre class="code-block">curl -H "Authorization: Bearer $NEFO_KEY" \
  {{ apiBase }}sites</pre>
        </div>
        <div class="modal-actions">
          <button type="button" class="btn btn-primary" @click="closeNew">{{ t('apikeys.done') }}</button>
        </div>
      </div>
      <form v-else class="stack" @submit.prevent="create">
        <p v-if="formError" class="alert alert-crit pre-wrap" role="alert" style="margin: 0">{{ formError }}</p>
        <label class="field">
          <span>{{ t('apikeys.name') }}</span>
          <input v-model="form.name" class="input" maxlength="32" autocomplete="off" spellcheck="false" placeholder="deploy-bot" required />
        </label>
        <div class="field">
          <span>{{ t('apikeys.col_access') }}</span>
          <div class="tabs access-pick" role="radiogroup" :aria-label="t('apikeys.col_access')">
            <button type="button" role="radio" :aria-checked="form.access === 'read'" :aria-pressed="form.access === 'read'" @click="form.access = 'read'"><Icon name="eye" :size="14" />{{ t('apikeys.read') }}</button>
            <button type="button" role="radio" :aria-checked="form.access === 'full'" :aria-pressed="form.access === 'full'" @click="form.access = 'full'"><Icon name="edit" :size="14" />{{ t('apikeys.full') }}</button>
          </div>
          <small class="muted">{{ form.access === 'full' ? t('apikeys.full_hint') : t('apikeys.read_hint') }}</small>
        </div>
        <label class="field">
          <span>{{ t('apikeys.addresses') }}</span>
          <textarea v-model="form.addresses" class="input mono" rows="2" spellcheck="false" autocomplete="off" placeholder="203.0.113.7 198.51.100.0/24"></textarea>
          <small class="muted">{{ t('apikeys.addresses_hint') }}</small>
        </label>
        <label class="field">
          <span>{{ t('apikeys.expires') }}</span>
          <select v-model.number="form.expires_days" class="input">
            <option v-for="n in [30, 90, 365]" :key="n" :value="n">{{ t('apikeys.days', { n }) }}</option>
            <option :value="0">{{ t('apikeys.no_expiry') }}</option>
          </select>
        </label>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="closeNew">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="busy">{{ t('apikeys.create') }}</button>
        </div>
      </form>
    </Modal>
  </section>
</template>

<style scoped>
.access-pick { display: flex; width: 100%; }
.access-pick button { flex: 1; justify-content: center; }
.addr-list { overflow-wrap: anywhere; display: inline-block; max-width: 260px; }
</style>
