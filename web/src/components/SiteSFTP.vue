<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import CopyField from './CopyField.vue'

const props = defineProps({ site: { type: Object, required: true } })
const emit = defineEmits(['saved', 'close'])
const { t } = useI18n()

const info = ref(null)
const enabled = ref(false)
const mode = ref('generate') // generate, set, keep, none
const password = ref('')
const keys = ref('')
const error = ref('')
const busy = ref(false)
const generated = ref('')
const host = window.location.hostname

function apply(i) {
  info.value = i
  enabled.value = i.enabled
  keys.value = i.keys_text
  password.value = ''
  mode.value = !i.enabled ? 'generate' : i.has_password ? 'keep' : 'none'
}

onMounted(async () => {
  try {
    apply(await api('GET', `api/sites/${props.site.id}/sftp`))
  } catch (e) {
    error.value = errorMessage(e)
  }
})

const sftpURL = computed(() => {
  if (!info.value) return ''
  const port = info.value.port && info.value.port !== 22 ? `:${info.value.port}` : ''
  return `sftp://${info.value.username}@${host}${port}${info.value.path}`
})

async function save() {
  error.value = ''
  generated.value = ''
  busy.value = true
  const body = { enabled: enabled.value }
  if (enabled.value) {
    body.keys = keys.value
    if (mode.value === 'generate') body.generate_password = true
    if (mode.value === 'set') body.password = password.value
    if (mode.value === 'none') body.remove_password = true
  }
  try {
    const i = await api('PUT', `api/sites/${props.site.id}/sftp`, body)
    apply(i)
    emit('saved', i)
    if (i.password) generated.value = i.password
    else emit('close')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="!info && !error" class="muted">…</div>
  <form v-else class="stack" @submit.prevent="save">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>

    <template v-if="info">
      <p v-if="!info.available" class="alert alert-warn">{{ t('sftp.no_ssh') }}</p>

      <div v-if="generated" class="alert alert-ok stack">
        <span>{{ t('sftp.password_generated') }}</span>
        <CopyField :label="t('sftp.password')" :value="generated" />
      </div>

      <div v-if="info.enabled" class="creds">
        <CopyField :label="t('sftp.host')" :value="host" />
        <CopyField :label="t('sftp.port')" :value="String(info.port)" />
        <CopyField :label="t('sftp.username')" :value="info.username" />
        <CopyField :label="t('sftp.folder')" :value="info.path" />
      </div>
      <p v-if="info.enabled" class="muted small mono">{{ sftpURL }}</p>

      <label class="check">
        <input v-model="enabled" type="checkbox" :disabled="!info.available" /><span>{{ t('sftp.enable') }}</span>
      </label>
      <small class="muted">{{ t('sftp.enable_hint', { path: info.path }) }}</small>

      <template v-if="enabled">
        <fieldset class="stack">
          <legend class="field-label">{{ t('sftp.password') }}</legend>
          <label class="check"><input v-model="mode" type="radio" value="generate" /><span>{{ t('sftp.mode_generate') }}</span></label>
          <label class="check"><input v-model="mode" type="radio" value="set" /><span>{{ t('sftp.mode_set') }}</span></label>
          <input v-if="mode === 'set'" v-model="password" class="input" type="password" autocomplete="new-password" minlength="12" maxlength="128" required :aria-label="t('sftp.password')" />
          <label v-if="info.enabled && info.has_password" class="check"><input v-model="mode" type="radio" value="keep" /><span>{{ t('sftp.mode_keep') }}</span></label>
          <label class="check"><input v-model="mode" type="radio" value="none" /><span>{{ t('sftp.mode_none') }}</span></label>
        </fieldset>

        <label class="field">
          <span>{{ t('sftp.keys') }}</span>
          <textarea v-model="keys" class="input mono" rows="4" spellcheck="false" placeholder="ssh-ed25519 AAAA… me@laptop"></textarea>
          <small class="muted">{{ t('sftp.keys_hint') }}</small>
        </label>
        <ul v-if="info.keys.length" class="key-list">
          <li v-for="k in info.keys" :key="k.fingerprint" class="small">
            <span class="tag">{{ k.type }}</span> <span class="mono">{{ k.fingerprint }}</span> <span class="muted">{{ k.comment }}</span>
          </li>
        </ul>
      </template>
      <small v-else-if="info.enabled" class="muted">{{ t('sftp.disable_hint') }}</small>
    </template>

    <div class="modal-actions">
      <button type="button" class="btn btn-ghost" @click="emit('close')">{{ generated ? t('common.close') : t('common.cancel') }}</button>
      <button v-if="info" class="btn btn-primary" :disabled="busy || !info.available">{{ t('files.save') }}</button>
    </div>
  </form>
</template>

<style scoped>
fieldset { border: 0; padding: 0; margin: 0; }
.field-label { font-size: 13px; font-weight: 600; margin-bottom: 4px; }
.key-list { list-style: none; margin: 0; padding: 0; display: grid; gap: 4px; }
.key-list .mono { word-break: break-all; }
</style>
