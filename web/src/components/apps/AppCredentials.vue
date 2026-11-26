<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../../api.js'
import { errorMessage } from '../../errors.js'
import Modal from '../Modal.vue'
import SecretField from './SecretField.vue'
import { tx } from './appstore.js'

// AppCredentials shows how to reach and sign in to an installed app. The
// panel logs every opening in the account's activity log.
const props = defineProps({
  name: { type: String, required: true }, // the instance
  title: { type: String, required: true },
})
defineEmits(['close'])
const { t, locale } = useI18n()
const c = ref(null)
const error = ref('')

onMounted(async () => {
  try {
    c.value = await api('GET', `api/apps/installed/${encodeURIComponent(props.name)}/credentials`)
  } catch (e) {
    error.value = errorMessage(e)
  }
})

// The admin password is also among the secrets: show each value once.
const secrets = computed(() => (c.value?.secrets || []).filter((s) => s.value && s.value !== c.value.password))
const extra = computed(() => Object.entries(c.value?.extra_ports || {}))
// What to do with these details, for the apps where it is not obvious.
const methodHint = computed(() => {
  const m = c.value?.method
  return ['invite', 'logs', 'signup', 'site_password'].includes(m) ? t(`apps.cr_hint_${m}`) : ''
})
</script>

<template>
  <Modal :title="t('apps.creds_of', { app: title })" @close="$emit('close')">
    <p v-if="error" class="alert alert-crit" role="alert">{{ error }}</p>
    <div v-else-if="!c" class="stack"><span class="skel" style="height: 54px"></span><span class="skel" style="height: 54px"></span></div>
    <div v-else class="stack cr">
      <p v-if="methodHint" class="muted small cr-hint">{{ methodHint }}</p>
      <SecretField v-if="c.url" :label="t('apps.cr_url')" :value="c.url" link />
      <SecretField v-if="c.admin_url && c.admin_url !== c.url" :label="t('apps.admin_url')" :value="c.admin_url" link />
      <SecretField v-if="c.user" :label="t('apps.cr_user')" :value="c.user" />
      <SecretField v-if="c.password" :label="t('apps.cr_password')" :value="c.password" secret />
      <SecretField v-if="c.connection" :label="t('apps.cr_connection')" :value="c.connection" secret />
      <div v-if="c.host && c.port" class="cr-addr">
        <SecretField :label="t('apps.cr_address')" :value="`${c.host}:${c.port}`" />
        <SecretField v-for="[inner, outer] in extra" :key="inner" :label="t('apps.cr_port_of', { port: inner })" :value="`${c.host}:${outer}`" />
      </div>
      <SecretField v-for="s in secrets" :key="s.name" :label="tx(s.label, locale) || s.name" :value="s.value" secret />
      <p v-if="tx(c.note, locale)" class="alert alert-info cr-note">{{ tx(c.note, locale) }}</p>
      <p class="muted small cr-audit">{{ t('apps.cr_audit') }}</p>
    </div>
    <div class="modal-actions">
      <button type="button" class="btn" @click="$emit('close')">{{ t('common.close') }}</button>
    </div>
  </Modal>
</template>

<style scoped>
.cr { gap: 12px; }
.cr-hint, .cr-audit, .cr-note { margin: 0; }
.cr-addr { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); }
</style>
