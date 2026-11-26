<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import CopyField from './CopyField.vue'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'

// Remote storage: S3-compatible storage and an SFTP server. Both can be set
// up at once; backups are then copied to each of them.
const props = defineProps({
  s3: { type: Object, default: null },
  sftp: { type: Object, default: null },
})
const emit = defineEmits(['changed'])

const { t } = useI18n()

const error = ref('')
const notice = ref('')
const warning = ref('')
const busy = ref(false)
const form = ref('') // '', 's3' or 'sftp': the form shown

function clearMessages() {
  error.value = ''
  notice.value = ''
  warning.value = ''
}

const s3Form = ref({})
const sftpForm = ref({})

function open(kind) {
  clearMessages()
  const s3 = props.s3 ?? {}
  s3Form.value = { endpoint: s3.endpoint ?? '', region: s3.region ?? '', bucket: s3.bucket ?? '', prefix: s3.prefix ?? '', access_key: s3.access_key ?? '', secret_key: '' }
  const f = props.sftp ?? {}
  sftpForm.value = { host: f.host ?? '', port: f.port ?? 22, username: f.username ?? '', auth: f.auth ?? 'password', password: '', private_key: '', passphrase: '', folder: f.folder ?? '' }
  form.value = kind
}

// A secret is kept when the stored settings use the same way of signing in.
const keepsPassword = computed(() => props.sftp?.auth === 'password' && props.sftp?.has_password && sftpForm.value.auth === 'password')
const keepsKey = computed(() => props.sftp?.auth === 'key' && props.sftp?.has_key && sftpForm.value.auth === 'key')

async function saveS3() {
  clearMessages()
  busy.value = true
  try {
    await api('PUT', 'api/backups/storage', s3Form.value)
    form.value = ''
    notice.value = t('backups.storage_saved')
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function saveSFTP() {
  clearMessages()
  busy.value = true
  try {
    const res = await api('PUT', 'api/backups/sftp', { ...sftpForm.value, port: Number(sftpForm.value.port) || 22 })
    form.value = ''
    const key = `${res.sftp.host_key_type} ${res.sftp.host_key_fingerprint}`
    notice.value = t('backups.sftp_saved', { key })
    if (res.previous_fingerprint) warning.value = t('backups.sftp_key_replaced', { old: res.previous_fingerprint, key })
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function remove(kind) {
  clearMessages()
  if (!window.confirm(kind === 's3' ? t('backups.storage_remove_confirm') : t('backups.sftp_remove_confirm'))) return
  try {
    await api('DELETE', kind === 's3' ? 'api/backups/storage' : 'api/backups/sftp')
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e)
  }
}
</script>

<template>
  <section class="card">
    <div class="card-head">
      <h2>{{ t('backups.storage_title') }}</h2>
      <button v-if="!form && (!s3 || !sftp)" class="btn btn-sm" @click="open(s3 ? 'sftp' : 's3')">
        {{ s3 || sftp ? t('backups.storage_add') : t('backups.storage_setup') }}
      </button>
    </div>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="warning" class="alert alert-warn pre-wrap" role="status">{{ warning }}</p>
    <p v-if="notice" class="alert alert-ok pre-wrap" role="status">{{ notice }}</p>

    <template v-if="!form">
      <p v-if="!s3 && !sftp" class="muted">{{ t('backups.storage_none') }}</p>
      <div v-else class="table-wrap">
        <table class="table">
          <tbody>
            <tr v-if="s3">
              <td class="nowrap"><span class="dot dot-ok" aria-hidden="true"></span><span class="tag tag-info">{{ t('backups.remote_s3') }}</span></td>
              <td>
                <span class="mono">{{ s3.bucket }}/{{ s3.prefix }}</span>
                <div class="muted small">{{ s3.endpoint }}</div>
              </td>
              <td class="actions">
                <div class="row">
                  <button class="btn btn-sm" @click="open('s3')">{{ t('backups.storage_edit') }}</button>
                  <button class="btn btn-sm btn-danger" @click="remove('s3')">{{ t('websites.delete') }}</button>
                </div>
              </td>
            </tr>
            <tr v-if="sftp">
              <td class="nowrap"><span class="dot dot-ok" aria-hidden="true"></span><span class="tag tag-info">{{ t('backups.remote_sftp') }}</span></td>
              <td>
                <span class="mono">{{ sftp.username }}@{{ sftp.host }}:{{ sftp.port }}</span>
                <span class="mono muted"> {{ sftp.folder || '~' }}</span>
                <div class="muted small">
                  {{ t('backups.sftp_host_key') }}
                  <span class="mono">{{ sftp.host_key_type }} {{ sftp.host_key_fingerprint }}</span>
                </div>
              </td>
              <td class="actions">
                <div class="row">
                  <button class="btn btn-sm" @click="open('sftp')">{{ t('backups.storage_edit') }}</button>
                  <button class="btn btn-sm btn-danger" @click="remove('sftp')">{{ t('websites.delete') }}</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-if="s3 && sftp" class="muted small note">{{ t('backups.storage_both') }}</p>
    </template>

    <div v-else class="stack form-narrow">
      <div class="row" role="radiogroup" :aria-label="t('backups.storage_type')">
        <span class="muted small">{{ t('backups.storage_type') }}</span>
        <label class="check"><input v-model="form" type="radio" value="s3" @change="clearMessages" /><span>{{ t('backups.type_s3') }}</span></label>
        <label class="check"><input v-model="form" type="radio" value="sftp" @change="clearMessages" /><span>{{ t('backups.type_sftp') }}</span></label>
      </div>

      <!-- S3 -->
      <form v-if="form === 's3'" class="stack" @submit.prevent="saveS3">
        <p class="muted small">{{ t('backups.storage_hint') }}</p>
        <label class="field">
          <span>{{ t('backups.endpoint') }}</span>
          <input v-model="s3Form.endpoint" class="input mono" placeholder="https://s3.eu-central-1.amazonaws.com" required />
        </label>
        <div class="row">
          <label class="field grow">
            <span>{{ t('backups.bucket') }}</span>
            <input v-model="s3Form.bucket" class="input mono" required />
          </label>
          <label class="field grow">
            <span>{{ t('backups.region') }}</span>
            <input v-model="s3Form.region" class="input mono" placeholder="us-east-1 / auto" />
          </label>
        </div>
        <label class="field">
          <span>{{ t('backups.prefix') }}</span>
          <input v-model="s3Form.prefix" class="input mono" placeholder="panel/" />
        </label>
        <label class="field">
          <span>{{ t('backups.access_key') }}</span>
          <input v-model="s3Form.access_key" class="input mono" autocomplete="off" required />
        </label>
        <label class="field">
          <span>{{ t('backups.secret_key') }}</span>
          <input v-model="s3Form.secret_key" class="input mono" type="password" autocomplete="new-password" :required="!s3" />
          <small v-if="s3" class="muted">{{ t('backups.secret_keep') }}</small>
        </label>
        <div class="row">
          <button class="btn btn-primary" :disabled="busy">{{ busy ? t('backups.testing') : t('backups.storage_save') }}</button>
          <button type="button" class="btn btn-ghost" @click="form = ''">{{ t('common.cancel') }}</button>
        </div>
      </form>

      <!-- SFTP -->
      <form v-else class="stack" @submit.prevent="saveSFTP">
        <p class="muted small">{{ t('backups.sftp_hint') }}</p>
        <div class="row">
          <label class="field grow">
            <span>{{ t('backups.sftp_host') }}</span>
            <input v-model="sftpForm.host" class="input mono" placeholder="backup.example.com" autocomplete="off" spellcheck="false" required />
          </label>
          <label class="field">
            <span>{{ t('backups.sftp_port') }}</span>
            <input v-model.number="sftpForm.port" class="input input-auto mono" type="number" min="1" max="65535" required />
          </label>
        </div>
        <label class="field">
          <span>{{ t('backups.sftp_username') }}</span>
          <input v-model="sftpForm.username" class="input mono" autocomplete="off" spellcheck="false" autocapitalize="off" required />
        </label>
        <div class="row" role="radiogroup" :aria-label="t('backups.sftp_auth')">
          <span class="muted small">{{ t('backups.sftp_auth') }}</span>
          <label class="check"><input v-model="sftpForm.auth" type="radio" value="password" /><span>{{ t('backups.sftp_auth_password') }}</span></label>
          <label class="check"><input v-model="sftpForm.auth" type="radio" value="key" /><span>{{ t('backups.sftp_auth_key') }}</span></label>
        </div>
        <label v-if="sftpForm.auth === 'password'" class="field">
          <span>{{ t('backups.sftp_password') }}</span>
          <input v-model="sftpForm.password" class="input mono" type="password" autocomplete="new-password" :required="!keepsPassword" />
          <small v-if="keepsPassword" class="muted">{{ t('backups.sftp_password_keep') }}</small>
        </label>
        <template v-else>
          <label class="field">
            <span>{{ t('backups.sftp_private_key') }}</span>
            <textarea v-model="sftpForm.private_key" class="input mono" rows="5" spellcheck="false" autocomplete="off"
              placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" :required="!keepsKey"></textarea>
            <small v-if="keepsKey" class="muted">{{ t('backups.sftp_key_keep', { key: `${sftp.key_type} ${sftp.key_fingerprint}` }) }}</small>
            <small v-else class="muted">{{ t('backups.sftp_key_hint') }}</small>
          </label>
          <label class="field">
            <span>{{ t('backups.sftp_passphrase') }}</span>
            <input v-model="sftpForm.passphrase" class="input mono" type="password" autocomplete="new-password" />
            <small v-if="keepsKey && sftp.has_passphrase" class="muted">{{ t('backups.sftp_passphrase_keep') }}</small>
          </label>
          <CopyField v-if="keepsKey && sftp.public_key" :label="t('backups.sftp_public_key')" :value="sftp.public_key" />
        </template>
        <label class="field">
          <span>{{ t('backups.sftp_folder') }}</span>
          <input v-model="sftpForm.folder" class="input mono" placeholder="backups/panel" spellcheck="false" autocapitalize="off" />
          <small class="muted">{{ t('backups.sftp_folder_hint') }}</small>
        </label>
        <div class="field">
          <span>{{ t('backups.sftp_host_key') }}</span>
          <span v-if="sftp?.host_key_fingerprint" class="mono small">{{ sftp.host_key_type }} {{ sftp.host_key_fingerprint }}</span>
          <span v-else class="muted small">{{ t('backups.sftp_host_key_none') }}</span>
          <small class="muted">{{ t('backups.sftp_trust_hint') }}</small>
        </div>
        <div class="row">
          <button class="btn btn-primary" :disabled="busy">{{ busy ? t('backups.testing') : t('backups.storage_save') }}</button>
          <button type="button" class="btn btn-ghost" @click="form = ''">{{ t('common.cancel') }}</button>
        </div>
      </form>
    </div>
  </section>
</template>
