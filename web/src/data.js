import { reactive } from 'vue'
import { api } from './api.js'

// A small shared cache of GET answers, so the sidebar counts, the bell, the
// command palette and the dashboard do not each ask the server for the same
// lists. store[path] holds the last answer; pages that load a list themselves
// can put() it here.
export const store = reactive({})
const meta = {} // path -> { at, p }

// load returns the answer for path, from the cache when it is younger than
// ttl milliseconds. Calls made while a request is on its way share it.
export function load(path, { ttl = 30000, force = false } = {}) {
  const m = meta[path]
  if (m?.p) return m.p
  if (!force && m && Date.now() - m.at < ttl && path in store) return Promise.resolve(store[path])
  const p = api('GET', path)
    .then((d) => {
      store[path] = d
      return d
    })
    .finally(() => {
      meta[path] = { at: Date.now(), p: null }
    })
  meta[path] = { at: m?.at || 0, p }
  return p
}

export function put(path, data) {
  store[path] = data
  meta[path] = { at: Date.now(), p: null }
}

export function invalidate(path) {
  delete meta[path]
}

// Lists from Go may be null when empty.
export const list = (v) => (Array.isArray(v) ? v : [])
