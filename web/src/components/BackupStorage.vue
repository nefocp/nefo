<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import CopyField from './CopyField.vue'
import Icon from './Icon.vue'
import BackupEncryption, { encryptionError, encryptionFields } from './BackupEncryption.vue'
import BackupFound from './BackupFound.vue'
import { api, ApiError } from '../api.js'
import { errorMessage } from '../errors.js'

// Remote storage: S3-compatible storage, an SFTP server and Google Drive.
// All can be set up at once; backups are then copied to each of them,
// encrypted for the storages that ask for it.
const props = defineProps({
  s3: { type: Object, default: null },
  sftp: { type: Object, default: null },
  gdrive: { type: Object, default: null },
  gdriveDefaultFolder: { type: String, default: '' },
  gdriveBuiltin: { type: Boolean, default: false },
})
const emit = defineEmits(['changed', 'restore'])

const { t } = useI18n()

const error = ref('')
const notice = ref('')
const warning = ref('')
const busy = ref(false)
const form = ref('') // '', 's3', 'sftp' or 'gdrive': the form shown
const finding = ref('') // the storage whose unknown backups are shown
const labelOf = (k) => t({ s3: 'backups.remote_s3', sftp: 'backups.remote_sftp', gdrive: 'backups.remote_gdrive' }[k])

function clearMessages() {
  error.value = ''
  notice.value = ''
  warning.value = ''
}

const kinds = ['s3', 'sftp', 'gdrive']
const configured = computed(() => kinds.filter((k) => props[k]))
const firstMissing = computed(() => kinds.find((k) => !props[k]))

const s3Form = ref({})
const sftpForm = ref({})
const driveForm = ref({})
const encOf = (v) => ({ encrypt: !!v?.encrypt, encrypt_passphrase: '', repeat: '' })
const s3Enc = ref(encOf())
const sftpEnc = ref(encOf())
const driveEnc = ref(encOf())

function open(kind) {
  clearMessages()
  const s3 = props.s3 ?? {}
  s3Form.value = { endpoint: s3.endpoint ?? '', region: s3.region ?? '', bucket: s3.bucket ?? '', prefix: s3.prefix ?? '', access_key: s3.access_key ?? '', secret_key: '' }
  const f = props.sftp ?? {}
  sftpForm.value = { host: f.host ?? '', port: f.port ?? 22, username: f.username ?? '', auth: f.auth ?? 'password', password: '', private_key: '', passphrase: '', folder: f.folder ?? '' }
  const g = props.gdrive ?? {}
  driveForm.value = { client_id: g.client_id ?? '', client_secret: '', folder: g.folder ?? '' }
  s3Enc.value = encOf(props.s3)
  sftpEnc.value = encOf(props.sftp)
  driveEnc.value = encOf(props.gdrive)
  form.value = kind
}

// A secret is kept when the stored settings use the same way of signing in.
const keepsPassword = computed(() => props.sftp?.auth === 'password' && props.sftp?.has_password && sftpForm.value.auth === 'password')
const keepsKey = computed(() => props.sftp?.auth === 'key' && props.sftp?.has_key && sftpForm.value.auth === 'key')
const keepsDriveSecret = computed(() => !!props.gdrive && props.gdrive.client_id !== '' && driveForm.value.client_id.trim() === props.gdrive.client_id)

async function saveS3() {
  clearMessages()
  if ((error.value = encryptionError(s3Enc.value, t))) return
  busy.value = true
  try {
    await api('PUT', 'api/backups/storage', { ...s3Form.value, ...encryptionFields(s3Enc.value) })
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
  if ((error.value = encryptionError(sftpEnc.value, t))) return
  busy.value = true
  try {
    const res = await api('PUT', 'api/backups/sftp', { ...sftpForm.value, port: Number(sftpForm.value.port) || 22, ...encryptionFields(sftpEnc.value) })
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

// ---- Google Drive ----

// Changing the folder or the encryption needs no new sign-in.
async function saveDrive() {
  clearMessages()
  if ((error.value = encryptionError(driveEnc.value, t))) return
  busy.value = true
  try {
    await api('PUT', 'api/backups/gdrive', { folder: driveForm.value.folder, ...encryptionFields(driveEnc.value) })
    form.value = ''
    notice.value = t('backups.gdrive_saved')
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

// The device flow: the panel shows a code, the owner enters it at
// google.com/device, the panel waits for Google and saves the settings.
const signIn = ref(null) // { user_code, verification_url, expires_at }
const now = ref(Date.now())
let pollTimer = 0
let clockTimer = 0

function stopWaiting() {
  clearTimeout(pollTimer)
  clearInterval(clockTimer)
  pollTimer = clockTimer = 0
}

function waitFor(st) {
  signIn.value = st
  stopWaiting()
  now.value = Date.now()
  clockTimer = setInterval(() => (now.value = Date.now()), 1000)
  const poll = async () => {
    try {
      const s = await api('GET', 'api/backups/gdrive/connect')
      if (s.state === 'waiting') {
        signIn.value = s
        pollTimer = setTimeout(poll, 2500)
        return
      }
      stopWaiting()
      signIn.value = null
      if (s.state === 'done') {
        form.value = ''
        notice.value = t('backups.gdrive_connected', { account: s.drive?.account || '?', folder: s.drive?.folder || '' })
        emit('changed')
      } else if (s.state === 'failed') {
        error.value = errorMessage(new ApiError(400, s.error, { detail: s.detail }))
      } else {
        // "none": the panel restarted and forgot the sign-in.
        error.value = errorMessage(new ApiError(400, 'gdrive_expired', {}))
      }
    } catch (e) {
      pollTimer = setTimeout(poll, 5000) // the panel restarted or the network failed: try again
    }
  }
  pollTimer = setTimeout(poll, 2500)
}

async function connectDrive() {
  clearMessages()
  if ((error.value = encryptionError(driveEnc.value, t))) return
  busy.value = true
  try {
    const f = driveForm.value
    waitFor(await api('POST', 'api/backups/gdrive/connect', { client_id: f.client_id.trim(), client_secret: f.client_secret.trim(), folder: f.folder, ...encryptionFields(driveEnc.value) }))
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function cancelSignIn() {
  stopWaiting()
  signIn.value = null
  try {
    await api('DELETE', 'api/backups/gdrive/connect')
  } catch {
    // nothing to cancel any more
  }
}

const timeLeft = computed(() => {
  const s = Math.max(0, Math.round((new Date(signIn.value?.expires_at).getTime() - now.value) / 1000))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
})
const deviceHost = computed(() => {
  try {
    const u = new URL(signIn.value?.verification_url)
    return u.host.replace(/^www\./, '') + u.pathname
  } catch {
    return 'google.com/device'
  }
})

onMounted(async () => {
  // A sign-in started before the page was reloaded is shown again.
  try {
    const s = await api('GET', 'api/backups/gdrive/connect')
    if (s.state === 'waiting') {
      open('gdrive')
      waitFor(s)
    }
  } catch {
    // not important
  }
})
onBeforeUnmount(stopWaiting)

async function remove(kind) {
  clearMessages()
  const confirmKey = { s3: 'backups.storage_remove_confirm', sftp: 'backups.sftp_remove_confirm', gdrive: 'backups.gdrive_remove_confirm' }[kind]
  if (!window.confirm(t(confirmKey))) return
  try {
    await api('DELETE', { s3: 'api/backups/storage', sftp: 'api/backups/sftp', gdrive: 'api/backups/gdrive' }[kind])
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
      <button v-if="!form && firstMissing" class="btn btn-sm" @click="open(firstMissing)">
        {{ configured.length ? t('backups.storage_add') : t('backups.storage_setup') }}
      </button>
    </div>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="warning" class="alert alert-warn pre-wrap" role="status">{{ warning }}</p>
    <p v-if="notice" class="alert alert-ok pre-wrap" role="status">{{ notice }}</p>

    <template v-if="!form">
      <p v-if="!configured.length" class="muted">{{ t('backups.storage_none') }}</p>
      <div v-else class="table-wrap">
        <table class="table stack-table">
          <tbody>
            <tr v-if="s3">
              <td class="nowrap"><span class="dot dot-ok" aria-hidden="true"></span><span class="tag tag-info">{{ t('backups.remote_s3') }}</span></td>
              <td>
                <span class="mono">{{ s3.bucket }}/{{ s3.prefix }}</span>
                <span v-if="s3.encrypt" class="tag tag-ok enc-tag"><Icon name="lock" :size="12" />{{ t('backups.encrypted') }}</span>
                <div class="muted small">{{ s3.endpoint }}</div>
              </td>
              <td class="actions">
                <div class="row storage-actions">
                  <button class="btn btn-sm" :aria-pressed="finding === 's3'" @click="finding = 's3'"><Icon name="search" :size="14" />{{ t('backups.find') }}</button>
                  <button class="btn btn-sm" @click="open('s3')">{{ t('backups.storage_edit') }}</button>
                  <button class="btn btn-sm btn-danger" @click="remove('s3')">{{ t('websites.delete') }}</button>
                </div>
              </td>
            </tr>
            <tr v-if="sftp">
              <td class="nowrap"><span class="dot dot-ok" aria-hidden="true"></span><span class="tag tag-info">{{ t('backups.remote_sftp') }}</span></td>
              <td>
                <span class="mono">{{ sftp.username }}@{{ sftp.host }}:{{ sftp.port }}&nbsp;<span class="muted">{{ sftp.folder || '~' }}</span></span>
                <span v-if="sftp.encrypt" class="tag tag-ok enc-tag"><Icon name="lock" :size="12" />{{ t('backups.encrypted') }}</span>
                <div class="muted small">
                  {{ t('backups.sftp_host_key') }}
                  <span class="mono">{{ sftp.host_key_type }} {{ sftp.host_key_fingerprint }}</span>
                </div>
              </td>
              <td class="actions">
                <div class="row storage-actions">
                  <button class="btn btn-sm" :aria-pressed="finding === 'sftp'" @click="finding = 'sftp'"><Icon name="search" :size="14" />{{ t('backups.find') }}</button>
                  <button class="btn btn-sm" @click="open('sftp')">{{ t('backups.storage_edit') }}</button>
                  <button class="btn btn-sm btn-danger" @click="remove('sftp')">{{ t('websites.delete') }}</button>
                </div>
              </td>
            </tr>
            <tr v-if="gdrive">
              <td class="nowrap"><span class="dot dot-ok" aria-hidden="true"></span><span class="tag tag-info">{{ t('backups.remote_gdrive') }}</span></td>
              <td>
                <span class="mono">{{ gdrive.folder }}</span>
                <span v-if="gdrive.encrypt" class="tag tag-ok enc-tag"><Icon name="lock" :size="12" />{{ t('backups.encrypted') }}</span>
                <div class="muted small">
                  <span class="mono">{{ gdrive.account }}</span>
                  <template v-if="gdrive.folder_url"> · <a :href="gdrive.folder_url" target="_blank" rel="noopener noreferrer">{{ t('backups.gdrive_open') }}</a></template>
                </div>
              </td>
              <td class="actions">
                <div class="row storage-actions">
                  <button class="btn btn-sm" :aria-pressed="finding === 'gdrive'" @click="finding = 'gdrive'"><Icon name="search" :size="14" />{{ t('backups.find') }}</button>
                  <button class="btn btn-sm" @click="open('gdrive')">{{ t('backups.storage_edit') }}</button>
                  <button class="btn btn-sm btn-danger" @click="remove('gdrive')">{{ t('websites.delete') }}</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-if="configured.length > 1" class="muted small note">{{ configured.length === 2 ? t('backups.storage_both') : t('backups.storage_each') }}</p>
      <BackupFound v-if="finding && props[finding]" :key="finding" :remote="finding" :label="labelOf(finding)"
        @close="finding = ''" @changed="emit('changed')" @restore="(id) => emit('restore', id)" />
    </template>

    <div v-else class="stack form-narrow">
      <div class="row" role="radiogroup" :aria-label="t('backups.storage_type')">
        <span class="muted small">{{ t('backups.storage_type') }}</span>
        <label class="check"><input v-model="form" type="radio" value="s3" :disabled="!!signIn" @change="clearMessages" /><span>{{ t('backups.type_s3') }}</span></label>
        <label class="check"><input v-model="form" type="radio" value="sftp" :disabled="!!signIn" @change="clearMessages" /><span>{{ t('backups.type_sftp') }}</span></label>
        <label class="check"><input v-model="form" type="radio" value="gdrive" :disabled="!!signIn" @change="clearMessages" /><span>{{ t('backups.type_gdrive') }}</span></label>
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
        <BackupEncryption v-model="s3Enc" :saved="!!s3?.encrypt" />
        <div class="row">
          <button class="btn btn-primary" :disabled="busy">{{ busy ? t('backups.testing') : t('backups.storage_save') }}</button>
          <button type="button" class="btn btn-ghost" @click="form = ''">{{ t('common.cancel') }}</button>
        </div>
      </form>

      <!-- SFTP -->
      <form v-else-if="form === 'sftp'" class="stack" @submit.prevent="saveSFTP">
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
          <span v-if="sftp?.host_key_fingerprint" class="mono small wrap-any">{{ sftp.host_key_type }} {{ sftp.host_key_fingerprint }}</span>
          <span v-else class="muted small">{{ t('backups.sftp_host_key_none') }}</span>
          <small class="muted">{{ t('backups.sftp_trust_hint') }}</small>
        </div>
        <BackupEncryption v-model="sftpEnc" :saved="!!sftp?.encrypt" />
        <div class="row">
          <button class="btn btn-primary" :disabled="busy">{{ busy ? t('backups.testing') : t('backups.storage_save') }}</button>
          <button type="button" class="btn btn-ghost" @click="form = ''">{{ t('common.cancel') }}</button>
        </div>
      </form>

      <!-- Google Drive -->
      <template v-else>
        <div v-if="signIn" class="signin stack" role="status" aria-live="polite">
          <h3 class="signin-title"><Icon name="key" :size="17" />{{ t('backups.gdrive_sign_in') }}</h3>
          <ol class="signin-steps">
            <li>
              {{ t('backups.gdrive_step_open') }}
              <div><a :href="signIn.verification_url" target="_blank" rel="noopener noreferrer" class="mono signin-url">{{ deviceHost }}<Icon name="external" :size="13" /></a></div>
            </li>
            <li>
              {{ t('backups.gdrive_step_code') }}
              <CopyField :label="''" :value="signIn.user_code" class="signin-code" />
            </li>
            <li>{{ t('backups.gdrive_step_allow') }}</li>
          </ol>
          <p class="muted small row"><span class="spinner" aria-hidden="true"></span>{{ t('backups.gdrive_waiting', { time: timeLeft }) }}</p>
          <div class="row">
            <button type="button" class="btn btn-ghost" @click="cancelSignIn">{{ t('common.cancel') }}</button>
          </div>
        </div>

        <form v-else class="stack" @submit.prevent="gdrive ? saveDrive() : connectDrive()">
          <p class="muted small">{{ t('backups.gdrive_hint') }}</p>
          <details :open="!gdrive">
            <summary>{{ t('backups.gdrive_howto') }}</summary>
            <ol class="howto small">
              <li>{{ t('backups.gdrive_step1') }}</li>
              <li>{{ t('backups.gdrive_step2') }}</li>
              <li>{{ t('backups.gdrive_step3') }}</li>
              <li>{{ t('backups.gdrive_step4') }}</li>
              <li>{{ t('backups.gdrive_step5') }}</li>
            </ol>
          </details>
          <label class="field">
            <span>{{ t('backups.gdrive_client_id') }}</span>
            <input v-model="driveForm.client_id" class="input mono" placeholder="123456789012-abc….apps.googleusercontent.com" autocomplete="off" spellcheck="false" autocapitalize="off" :required="!gdriveBuiltin" />
          </label>
          <label class="field">
            <span>{{ t('backups.gdrive_client_secret') }}</span>
            <input v-model="driveForm.client_secret" class="input mono" type="password" autocomplete="new-password" spellcheck="false" :required="!gdriveBuiltin && !keepsDriveSecret" />
            <small v-if="keepsDriveSecret" class="muted">{{ t('backups.gdrive_secret_keep') }}</small>
            <small v-else-if="gdriveBuiltin" class="muted">{{ t('backups.gdrive_builtin') }}</small>
          </label>
          <label class="field">
            <span>{{ t('backups.gdrive_folder') }}</span>
            <input v-model="driveForm.folder" class="input mono" :placeholder="gdriveDefaultFolder" spellcheck="false" autocapitalize="off" />
            <small class="muted">{{ t('backups.gdrive_folder_hint') }}</small>
          </label>
          <BackupEncryption v-model="driveEnc" :saved="!!gdrive?.encrypt" />
          <p v-if="gdrive" class="muted small">{{ t('backups.gdrive_change_hint') }}</p>
          <div class="row">
            <button class="btn btn-primary" :disabled="busy">
              {{ busy ? t('backups.testing') : gdrive ? t('backups.storage_save') : t('backups.gdrive_connect') }}
            </button>
            <button v-if="gdrive" type="button" class="btn" :disabled="busy" @click="connectDrive">{{ t('backups.gdrive_reconnect') }}</button>
            <button type="button" class="btn btn-ghost" @click="form = ''">{{ t('common.cancel') }}</button>
          </div>
        </form>
      </template>
    </div>
  </section>
</template>

<style scoped>
.enc-tag { margin-left: 6px; vertical-align: 1px; }
.wrap-any { overflow-wrap: anywhere; }
.storage-actions { justify-content: flex-end; }
@media (max-width: 640px) {
  .storage-actions { justify-content: flex-start; }
}
/* On a phone each row is a small card: the buttons go under the text. */
@media (max-width: 640px) {
  .stack-table thead { display: none; }
  .stack-table tr { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; padding: 10px 2px; border-bottom: 1px solid var(--border); }
  .stack-table tr:last-child { border-bottom: 0; }
  .stack-table td { display: block; border: 0; padding: 0; min-width: 0; }
  .stack-table td.actions { flex-basis: 100%; }
}
.enc-tag .icon { margin-right: 3px; }
.howto { margin: 8px 0 0; padding-left: 20px; display: flex; flex-direction: column; gap: 4px; color: var(--text-2); }
.signin { border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 16px; background: var(--surface-2); }
.signin-title { display: flex; align-items: center; gap: 8px; margin: 0; font-size: 15px; }
.signin-steps { margin: 0; padding-left: 20px; display: flex; flex-direction: column; gap: 10px; }
.signin-code { margin-top: 6px; }
.signin-url { display: inline-flex; align-items: center; gap: 4px; font-size: 15px; font-weight: 700; margin-top: 4px; }
.signin-code :deep(.secret) { font-size: 20px; letter-spacing: 0.12em; font-weight: 700; }
.signin .row.muted { gap: 8px; }
</style>
