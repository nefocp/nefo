<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { jobs, dismissFinished, jobTitle } from '../jobs.js'
import { useJob } from '../composables/useJob.js'
import Modal from './Modal.vue'
import JobLog from './JobLog.vue'
import Icon from './Icon.vue'

// JobPill shows the job the panel is running (or the one that just
// finished) in the top bar; a click opens its log.
const { t, te } = useI18n()
const shown = computed(() => jobs.running || jobs.finished)
const state = computed(() => (jobs.running ? 'run' : jobs.finished?.status === 'failed' ? 'fail' : 'ok'))
const label = computed(() => {
  const j = shown.value
  if (!j) return ''
  const name = jobTitle(j, t, te)
  if (state.value === 'run') return name
  return state.value === 'fail' ? t('jobpill.failed', { name }) : t('jobpill.done', { name })
})

const open = ref(false)
const { job, watch: follow, clear } = useJob()
async function show() {
  const j = shown.value
  if (!j) return
  open.value = true
  try {
    follow(await api('GET', `api/jobs/${j.id}`))
  } catch {
    // the job was forgotten: nothing to show
    open.value = false
  }
}
function close() {
  open.value = false
  clear()
  if (state.value !== 'run') dismissFinished()
}
</script>

<template>
  <button v-if="shown" type="button" class="job-pill" :class="{ ok: state === 'ok', fail: state === 'fail' }" :aria-label="label + ' · ' + t('jobpill.show_log')" :title="shown.last || label" @click="show">
    <span v-if="state === 'run'" class="spinner" aria-hidden="true"></span>
    <Icon v-else :name="state === 'fail' ? 'alert' : 'check'" :size="15" />
    <span class="truncate">{{ label }}</span>
  </button>
  <Modal v-if="open && job" :title="label" wide @close="close">
    <JobLog :job="job" embedded @close="close" />
  </Modal>
</template>
