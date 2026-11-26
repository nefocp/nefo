<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from './Icon.vue'
import { jobTitle } from '../jobs.js'

// JobLog shows a background job's progress and log. embedded drops the
// card and the title (inside a Modal that already has them).
const props = defineProps({
  job: { type: Object, required: true },
  embedded: { type: Boolean, default: false },
})
defineEmits(['close'])

const { t, te } = useI18n()
const logEl = ref(null)

const title = computed(() => jobTitle(props.job, t, te))
const statusClass = computed(() => ({ running: 'tag-info', succeeded: 'tag-ok', failed: 'tag-crit' })[props.job.status] ?? '')
const lines = computed(() => props.job.log || [])

// Keep the newest output in view while the job runs.
watch(
  () => lines.value.length,
  async () => {
    await nextTick()
    if (logEl.value) logEl.value.scrollTop = logEl.value.scrollHeight
  },
  { immediate: true },
)
</script>

<template>
  <section :class="embedded ? 'joblog-embedded' : 'card joblog'" :aria-busy="job.status === 'running'">
    <div class="card-head">
      <h2 v-if="!embedded" class="joblog-title">
        <Icon :name="job.status === 'failed' ? 'alert' : job.status === 'succeeded' ? 'checkCircle' : 'activity'" :size="17" />{{ title }}
      </h2>
      <span class="tag" :class="statusClass">
        <span v-if="job.status === 'running'" class="spinner" aria-hidden="true"></span>
        {{ t(`jobs.${job.status}`) }}
      </span>
      <span class="muted small">{{ t('jobpill.lines', { n: lines.length }, lines.length) }}</span>
    </div>
    <p v-if="job.status === 'failed' && job.error" class="alert alert-crit pre-wrap" role="alert">{{ job.error }}</p>
    <pre ref="logEl" class="log" tabindex="0" :aria-label="t('jobpill.log_label')">{{ lines.join('\n') }}</pre>
    <div v-if="job.status !== 'running'" class="row">
      <button class="btn btn-sm" @click="$emit('close')">{{ t('common.close') }}</button>
    </div>
  </section>
</template>

<style scoped>
.joblog .card-head, .joblog-embedded .card-head { justify-content: flex-start; }
.joblog-title { display: flex; align-items: center; gap: 9px; margin: 0 auto 0 0; }
.joblog-title .icon { color: var(--accent); }
.joblog-embedded .card-head { margin-bottom: 12px; }
</style>
