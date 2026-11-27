<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

// "Encrypt backups before upload", the part of each storage form that sets
// the encryption of the copies: { encrypt, encrypt_passphrase, repeat }.
// saved: this storage already encrypts, with a saved passphrase.
const props = defineProps({ saved: { type: Boolean, default: false } })
const model = defineModel({ type: Object, required: true })
const { t } = useI18n()

const changing = computed(() => props.saved && model.value.encrypt && model.value.encrypt_passphrase !== '')
const turningOff = computed(() => props.saved && !model.value.encrypt)
</script>

<script>
// encryptionError checks the form before it is sent: '' when it is fine.
export function encryptionError(f, t) {
  if (!f.encrypt || !f.encrypt_passphrase) return ''
  if ([...f.encrypt_passphrase].length < 12) return t('backups.enc_short')
  if (f.encrypt_passphrase !== f.repeat) return t('backups.enc_mismatch')
  return ''
}

// encryptionFields is what the API gets of the form.
export function encryptionFields(f) {
  return { encrypt: !!f.encrypt, encrypt_passphrase: f.encrypt ? f.encrypt_passphrase : '' }
}
</script>

<template>
  <fieldset class="enc stack">
    <label class="check"><input v-model="model.encrypt" type="checkbox" /><span>{{ t('backups.enc_toggle') }}</span></label>
    <small class="muted">{{ t('backups.enc_hint') }}</small>
    <template v-if="model.encrypt">
      <p class="alert alert-warn enc-warn" role="note">{{ t('backups.enc_warning') }}</p>
      <label class="field">
        <span>{{ t('backups.enc_passphrase') }}</span>
        <input v-model="model.encrypt_passphrase" class="input mono" type="password" autocomplete="new-password" spellcheck="false" :required="!saved" />
        <small class="muted">{{ saved ? t('backups.enc_keep') : t('backups.enc_passphrase_hint') }}</small>
      </label>
      <label v-if="model.encrypt_passphrase" class="field">
        <span>{{ t('backups.enc_repeat') }}</span>
        <input v-model="model.repeat" class="input mono" type="password" autocomplete="new-password" spellcheck="false" required />
      </label>
      <p v-if="changing" class="muted small enc-note">{{ t('backups.enc_change_note') }}</p>
    </template>
    <p v-else-if="turningOff" class="muted small enc-note">{{ t('backups.enc_off_note') }}</p>
  </fieldset>
</template>

<style scoped>
.enc { border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 12px 14px; margin: 0; gap: 10px; }
.enc-warn, .enc-note { margin: 0; }
</style>
