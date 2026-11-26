import { reactive } from 'vue'
import { api } from './api.js'

// features says which optional software is installed (GET /api/features),
// so the menu can hide pages for software that is not. Until the first
// answer the optional items stay hidden; if the server cannot be asked they
// all show, as before.
export const features = reactive({
  loaded: false,
  docker: false,
  redis: false,
  mariadb: false,
  node: false,
  python: false,
  wordpress_sites: false,
  web_server: '',
})

let pending = null
let last = 0

// refreshFeatures asks the server again. Calls within two seconds of the
// last answer are skipped unless force is set (after an install or removal).
export function refreshFeatures(force = false) {
  if (pending) return pending
  if (!force && features.loaded && Date.now() - last < 2000) return Promise.resolve()
  pending = api('GET', 'api/features')
    .then((f) => Object.assign(features, f, { loaded: true }))
    .catch(() => {
      if (!features.loaded) Object.assign(features, { loaded: true, docker: true, redis: true, wordpress_sites: true })
    })
    .finally(() => {
      pending = null
      last = Date.now()
    })
  return pending
}
