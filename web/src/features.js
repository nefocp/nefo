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
  mariadb: false, // MariaDB or MySQL: db_engine says which ("mariadb", "mysql" or "")
  db_engine: '',
  postgres: false,
  memcached: false,
  node: false,
  python: false,
  wordpress_sites: false,
  web_server: '',
  setup: false, // open the setup wizard after signing in
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

// dbEngineName is the database server's name for messages: "MySQL" or
// "MariaDB" (also while none is installed, as MariaDB is the default).
export function dbEngineName() {
  return features.db_engine === 'mysql' ? 'MySQL' : 'MariaDB'
}
