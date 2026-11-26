import { reactive } from 'vue'
import { api } from './api.js'

// What the sidebar and the dashboard show about this server: its info
// (asked once), its addresses, and live stats polled while the tab is
// visible. Views ask for an interval; the shortest one wins.
export const server = reactive({ info: null, ip: '', stats: null, online: true })

let infoP = null
export function loadServerInfo() {
  if (!infoP) {
    infoP = Promise.all([
      api('GET', 'api/system/info').then((i) => (server.info = i)),
      api('GET', 'api/system/addresses')
        .then((a) => (server.ip = a.primary || (a.addresses || [])[0] || ''))
        .catch(() => {}),
    ]).catch((e) => {
      infoP = null
      throw e
    })
  }
  return infoP
}

const wanted = new Map() // id -> interval ms
let seq = 0
let timer = 0
let current = 0

async function tick() {
  if (document.visibilityState !== 'hidden') {
    try {
      server.stats = await api('GET', 'api/system/stats')
      server.online = true
    } catch {
      server.online = false
    }
  }
  schedule()
}

function schedule() {
  clearTimeout(timer)
  timer = 0
  current = Math.min(...wanted.values())
  if (Number.isFinite(current)) timer = setTimeout(tick, current)
}

// watchStats polls the stats every ms while the returned stop() is not
// called. The first answer comes at once.
export function watchStats(ms) {
  const id = ++seq
  wanted.set(id, ms)
  if (!server.stats || ms < current) {
    clearTimeout(timer)
    tick()
  } else if (!timer) schedule()
  return () => {
    wanted.delete(id)
    schedule()
  }
}

document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible' && wanted.size) {
    clearTimeout(timer)
    tick()
  }
})

// level gives the colour step of a usage percentage: ok, warn (≥70) or crit (≥90).
export const level = (p) => (p >= 90 ? 'crit' : p >= 70 ? 'warn' : 'ok')
