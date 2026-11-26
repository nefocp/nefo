import { reactive } from 'vue'
import { api } from './api.js'
import { store, list } from './data.js'
import { appName } from './logos.js'

// The panel runs one background job at a time (installs, certificates,
// backups…). The top bar shows it as a pill. GET /api/jobs/summary has the
// jobs without their logs; it is asked every 3 s while a job runs, every
// 20 s otherwise, and at once when a page starts a job (nudgeJobs).
export const jobs = reactive({ list: [], running: null, finished: null })

let timer = 0
let started = false
let lastFinishedId = ''

async function poll() {
  clearTimeout(timer)
  if (document.visibilityState !== 'hidden') {
    try {
      const res = await api('GET', 'api/jobs/summary')
      const list = res.jobs || []
      const before = jobs.running
      jobs.list = list
      jobs.running = list.find((j) => j.status === 'running') || null
      // A job that was running and has now finished is shown for a while.
      if (before && !jobs.running) {
        const done = list.find((j) => j.id === before.id)
        if (done && done.id !== lastFinishedId) {
          lastFinishedId = done.id
          jobs.finished = done
          setTimeout(() => {
            if (jobs.finished?.id === done.id) jobs.finished = null
          }, done.status === 'failed' ? 30000 : 8000)
        }
      }
    } catch {
      // signed out or offline: try again later
    }
  }
  if (started) timer = setTimeout(poll, jobs.running ? 3000 : 20000)
}

let listening = false

export function startJobsPolling() {
  if (started) return
  started = true
  poll()
  if (!listening) {
    listening = true
    document.addEventListener('visibilitychange', () => {
      if (started && document.visibilityState === 'visible') poll()
    })
  }
}

export function stopJobsPolling() {
  started = false
  clearTimeout(timer)
}

// nudgeJobs asks again soon. A page that started job j passes it, so even
// a job that ends before the next answer is shown as finished.
export function nudgeJobs(j) {
  if (!started) return
  if (j?.status === 'running' && jobs.running?.id !== j.id) {
    jobs.running = { id: j.id, kind: j.kind, target: j.target, status: 'running', last: '' }
  }
  clearTimeout(timer)
  timer = setTimeout(poll, 300)
}

export function dismissFinished() {
  jobs.finished = null
}

// jobTitle names a job for people ("Installing Umami on stats.example.com").
// App store installs are "install_<app>": they share one sentence.
export function jobTitle(j, t, te) {
  const key = `jobs.${j.kind}`
  if (te(key)) return t(key, { target: j.target })
  if (j.kind.startsWith('install_')) {
    const id = j.kind.slice('install_'.length)
    const app = list(store['api/apps']?.apps).find((a) => a.id === id)?.name || appName(id)
    return j.target && j.target !== id ? t('jobs.install_app_on', { app, target: j.target }) : t('jobs.install_app', { app })
  }
  return j.kind
}
