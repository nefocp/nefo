<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import Icon from './Icon.vue'
import Modal from './Modal.vue'
import CopyField from './CopyField.vue'

// WpPasswordModal sets a new password for one of a WordPress site's
// administrators: a strong generated one, or one typed here. The new
// password is shown once; the panel does not keep it.
const props = defineProps({
  site: { type: Object, required: true }, // { id, domain }
})
const emit = defineEmits(['close'])

const { t } = useI18n()
const admins = ref(null)
const userId = ref(0)
const mode = ref('generate')
const password = ref('')
const busy = ref(false)
const error = ref('')
const result = ref(null)

onMounted(async () => {
  try {
    const res = await api('GET', `api/wordpress/${props.site.id}/admins`)
    admins.value = res.admins
    userId.value = res.admins[0]?.id || 0
  } catch (e) {
    admins.value = []
    error.value = errorMessage(e)
  }
})

async function submit() {
  error.value = ''
  busy.value = true
  try {
    result.value = await api('POST', `api/wordpress/${props.site.id}/password`, {
      user_id: Number(userId.value),
      password: mode.value === 'type' ? password.value : '',
    })
    password.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Modal :title="t('wp.password_title', { domain: site.domain })" @close="emit('close')">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>

    <div v-if="result" class="stack">
      <p class="alert alert-ok" role="status">{{ t('wp.password_done', { user: result.user }) }}</p>
      <p class="muted">{{ t('wp.password_once') }}</p>
      <div class="creds">
        <CopyField :label="t('wp.admin_user')" :value="result.user" />
        <CopyField :label="t('wp.new_password')" :value="result.password" />
      </div>
      <div class="modal-actions">
        <a class="btn" :href="result.login_url" target="_blank" rel="noopener noreferrer"><Icon name="external" />{{ t('wp.sign_in') }}</a>
        <button type="button" class="btn btn-primary" @click="emit('close')">{{ t('common.close') }}</button>
      </div>
    </div>

    <p v-else-if="admins === null" class="muted"><span class="spinner" aria-hidden="true"></span> {{ t('wp.loading_admins') }}</p>

    <form v-else class="stack" @submit.prevent="submit">
      <p v-if="!admins.length && !error" class="alert alert-warn">{{ t('wp.no_admins') }}</p>
      <template v-if="admins.length">
        <label class="field">
          <span>{{ t('wp.admin_user') }}</span>
          <select v-model="userId" class="input" required>
            <option v-for="u in admins" :key="u.id" :value="u.id">{{ u.login }}{{ u.email ? ` (${u.email})` : '' }}</option>
          </select>
        </label>
        <fieldset class="stack wp-choice">
          <legend class="sr-only">{{ t('wp.new_password') }}</legend>
          <label class="check"><input v-model="mode" type="radio" value="generate" /><span>{{ t('wp.password_generate') }}</span></label>
          <label class="check"><input v-model="mode" type="radio" value="type" /><span>{{ t('wp.password_type') }}</span></label>
        </fieldset>
        <label v-if="mode === 'type'" class="field">
          <span>{{ t('wp.new_password') }}</span>
          <input v-model="password" class="input" type="password" autocomplete="new-password" minlength="12" maxlength="128" required />
          <small class="muted">{{ t('wp.password_hint') }}</small>
        </label>
        <p class="muted small">{{ t('wp.password_signout') }}</p>
      </template>
      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="busy || !admins.length"><Icon name="key" />{{ t('wp.password_set') }}</button>
      </div>
    </form>
  </Modal>
</template>

<style scoped>
.wp-choice { border: 0; margin: 0; padding: 0; gap: 6px; }
</style>
