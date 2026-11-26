<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import Icon from './Icon.vue'
import Modal from './Modal.vue'

// WpPushModal pushes a staging copy back to its live site: the files, the
// database or both, after typing the live domain. The push runs as a job;
// "started" hands the job to the page.
const props = defineProps({
  site: { type: Object, required: true }, // the staging copy: { id, domain }
  live: { type: Object, required: true }, // its live site: { id, domain }
})
const emit = defineEmits(['close', 'started'])

const { t } = useI18n()
const what = ref('both')
const typed = ref('')
const busy = ref(false)
const error = ref('')
const files = computed(() => what.value !== 'database')
const database = computed(() => what.value !== 'files')
const confirmed = computed(() => typed.value.trim().toLowerCase().replace(/\.$/, '') === props.live.domain)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    const res = await api('POST', `api/wordpress/${props.site.id}/push`, { files: files.value, database: database.value, confirm: typed.value.trim() })
    emit('started', res.job)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Modal danger :title="t('wp.push_title', { domain: live.domain })" @close="emit('close')">
    <form class="stack" @submit.prevent="submit">
      <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
      <p>{{ t('wp.push_body', { staging: site.domain, live: live.domain }) }}</p>
      <fieldset class="stack wp-what">
        <legend class="field-label">{{ t('wp.push_what') }}</legend>
        <label class="check"><input v-model="what" type="radio" value="both" /><span>{{ t('wp.push_both') }}</span></label>
        <label class="check"><input v-model="what" type="radio" value="files" /><span>{{ t('wp.push_files') }}</span></label>
        <label class="check"><input v-model="what" type="radio" value="database" /><span>{{ t('wp.push_database') }}</span></label>
      </fieldset>
      <ul class="small muted wp-steps">
        <li>{{ t('wp.push_step_backup') }}</li>
        <li>{{ t('wp.push_step_maintenance') }}</li>
        <li v-if="files">{{ t('wp.push_step_files') }}</li>
        <li v-if="database">{{ t('wp.push_step_db') }}</li>
      </ul>
      <p class="alert alert-warn">{{ database ? t('wp.push_warning_db') : t('wp.push_warning_files') }}</p>
      <label class="field">
        <span>{{ t('websites.delete_confirm', { domain: live.domain }) }}</span>
        <input v-model="typed" class="input mono" autocomplete="off" autocapitalize="off" spellcheck="false" required maxlength="253" />
      </label>
      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button class="btn btn-danger btn-solid" :disabled="busy || !confirmed"><Icon name="upload" />{{ t('wp.push_start') }}</button>
      </div>
    </form>
  </Modal>
</template>

<style scoped>
fieldset { border: 0; padding: 0; margin: 0; }
.field-label { font-size: 13px; font-weight: 600; margin-bottom: 4px; }
.wp-what { gap: 10px; }
.wp-steps { margin: 0; padding-left: 18px; }
.wp-steps li { margin: 2px 0; }
</style>
