import { ref, computed, onBeforeUnmount } from 'vue'
import { api } from '../api.js'
import { nudgeJobs } from '../jobs.js'

// useJob follows one background job (package installs, certificates…) by
// polling its state, and calls onDone when it has finished.
export function useJob(onDone) {
  const job = ref(null)
  let timer = 0

  function watch(j) {
    job.value = j
    clearTimeout(timer)
    nudgeJobs(j) // the top bar shows it too
    if (j.status === 'running') timer = setTimeout(poll, 1200)
  }

  async function poll() {
    if (!job.value) return
    try {
      const j = await api('GET', `api/jobs/${job.value.id}`)
      job.value = j
      if (j.status === 'running') timer = setTimeout(poll, 1200)
      else await onDone?.(j)
    } catch {
      timer = setTimeout(poll, 3000)
    }
  }

  // resume picks up a job that is still running, for example after a reload.
  async function resume(kinds) {
    try {
      const { jobs } = await api('GET', 'api/jobs')
      const running = jobs.find((j) => j.status === 'running' && (!kinds || kinds.includes(j.kind)))
      if (running && job.value?.id !== running.id) watch(running)
    } catch {
      // not important
    }
  }

  onBeforeUnmount(() => clearTimeout(timer))

  return { job, running: computed(() => job.value?.status === 'running'), watch, resume, clear: () => (job.value = null) }
}
