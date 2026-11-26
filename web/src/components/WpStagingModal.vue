<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import Icon from './Icon.vue'
import Modal from './Modal.vue'

// WpStagingModal starts a copy of a WordPress site to a new website,
// staging.<domain> unless another domain is typed. The copy runs as a job;
// "started" hands the job to the page.
const props = defineProps({
  site: { type: Object, required: true }, // { id, domain }
})
const emit = defineEmits(['close', 'started'])

const { t } = useI18n()
const domain = ref('staging.' + props.site.domain)
const busy = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  busy.value = true
  try {
    const res = await api('POST', `api/wordpress/${props.site.id}/staging`, { domain: domain.value.trim() })
    emit('started', res.job)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Modal :title="t('wp.clone_title', { domain: site.domain })" @close="emit('close')">
    <form class="stack" @submit.prevent="submit">
      <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
      <p>{{ t('wp.clone_body') }}</p>
      <label class="field">
        <span>{{ t('wp.clone_domain') }}</span>
        <input v-model="domain" class="input mono" autocomplete="off" spellcheck="false" required maxlength="253" />
        <small class="muted">{{ t('wp.clone_domain_hint') }}</small>
      </label>
      <ul class="small muted wp-steps">
        <li>{{ t('wp.clone_step_site') }}</li>
        <li>{{ t('wp.clone_step_db') }}</li>
        <li>{{ t('wp.clone_step_url') }}</li>
        <li>{{ t('wp.clone_step_noindex') }}</li>
      </ul>
      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="busy || !domain.trim()"><Icon name="copy" />{{ t('wp.clone_start') }}</button>
      </div>
    </form>
  </Modal>
</template>

<style scoped>
.wp-steps { margin: 0; padding-left: 18px; }
.wp-steps li { margin: 2px 0; }
</style>
