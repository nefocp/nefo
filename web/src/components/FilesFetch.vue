<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import Icon from './Icon.vue'
import JobLog from './JobLog.vue'

// FilesFetch asks for a URL and starts a download into folder `dir`. The
// parent follows the job (useJob) and passes it back in `job`, so the
// download goes on if this form is closed.
const props = defineProps({
  dir: { type: String, required: true },
  job: { type: Object, default: null },
})
const emit = defineEmits(['started', 'close'])
const { t } = useI18n()

const url = ref('')
const error = ref('')
const sending = ref(false)
const startedId = ref('')
const myJob = computed(() => (props.job && props.job.id === startedId.value ? props.job : null))

async function start() {
  error.value = ''
  sending.value = true
  try {
    const j = await api('POST', 'api/files/fetch', { dir: props.dir, url: url.value.trim() })
    startedId.value = j.id
    emit('started', j)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <div class="stack">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <form v-if="!myJob" class="stack" @submit.prevent="start">
      <label class="field">
        <span>{{ t('files.fetch_url') }}</span>
        <input v-model="url" class="input mono" type="url" inputmode="url" placeholder="https://" maxlength="2048" spellcheck="false" autocomplete="off" required />
        <small class="muted">{{ t('files.fetch_hint', { dir }) }}</small>
      </label>
      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="sending || !url.trim()"><Icon name="download" />{{ t('files.fetch_start') }}</button>
      </div>
    </form>
    <template v-else>
      <JobLog :job="myJob" @close="emit('close')" />
      <div v-if="myJob.status === 'running'" class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('files.fetch_hide') }}</button>
      </div>
    </template>
  </div>
</template>
