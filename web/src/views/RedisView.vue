<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytes, formatUptime } from '../format.js'
import AppLogo from '../components/AppLogo.vue'

// Redis: find keys, see their values, delete keys. A tab of the Databases
// page (embedded).
defineProps({ embedded: { type: Boolean, default: false } })
const { t, locale } = useI18n()

const info = ref(null)
const error = ref('')
const notice = ref('')
const loading = ref(true)

const db = ref(0)
const pattern = ref('*')
const keys = ref([])
const cursor = ref('0')
const scanned = ref(false)
const scanning = ref(false)

const key = ref(null) // the key shown in the detail panel
const keyError = ref('')
const keyLoading = ref(false)

async function loadInfo() {
  try {
    info.value = await api('GET', 'api/redis')
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

const ready = computed(() => info.value?.installed && info.value?.running)
const totalKeys = computed(() => (info.value?.databases || []).reduce((n, d) => n + d.keys, 0))

async function scan(more = false) {
  error.value = ''
  notice.value = ''
  scanning.value = true
  try {
    const q = new URLSearchParams({ db: String(db.value), pattern: pattern.value.trim() || '*', cursor: more ? cursor.value : '0' })
    const res = await api('GET', `api/redis/keys?${q}`)
    keys.value = more ? [...keys.value, ...res.keys.filter((k) => !keys.value.some((x) => x.id === k.id))] : res.keys
    cursor.value = res.cursor
    scanned.value = true
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    scanning.value = false
  }
}

function changeDB(n) {
  db.value = n
  key.value = null
  keys.value = []
  scanned.value = false
  scan()
}

async function openKey(k) {
  keyError.value = ''
  keyLoading.value = true
  try {
    const q = new URLSearchParams({ db: String(db.value), key: k.id })
    key.value = await api('GET', `api/redis/key?${q}`)
  } catch (e) {
    key.value = null
    keyError.value = errorMessage(e)
  } finally {
    keyLoading.value = false
  }
}

async function deleteKey() {
  const k = key.value
  if (!k || !window.confirm(t('redis.delete_confirm', { name: k.name, db: db.value }))) return
  keyError.value = ''
  try {
    await api('DELETE', 'api/redis/key', { db: db.value, key: k.id })
    notice.value = t('redis.deleted', { name: k.name })
    keys.value = keys.value.filter((x) => x.id !== k.id)
    key.value = null
    loadInfo()
  } catch (e) {
    keyError.value = errorMessage(e)
  }
}

// ---- flush ----
const flushing = ref(false)
const flushConfirm = ref('')
const flushBusy = ref(false)
const flushError = ref('')

function askFlush() {
  flushConfirm.value = ''
  flushError.value = ''
  flushing.value = true
}

async function flush() {
  flushBusy.value = true
  flushError.value = ''
  try {
    await api('POST', 'api/redis/flush', { db: db.value, confirm: flushConfirm.value.trim() })
    notice.value = t('redis.flushed', { db: db.value })
    flushing.value = false
    keys.value = []
    key.value = null
    cursor.value = '0'
    await loadInfo()
  } catch (e) {
    flushError.value = errorMessage(e)
  } finally {
    flushBusy.value = false
  }
}

function ttlText(s) {
  if (s === -1) return t('redis.no_expiry')
  if (s < 0) return '—'
  if (s < 60) return t('redis.seconds', { n: s })
  return formatUptime(s, t)
}

const typeClass = (type) => ({ string: 'tag-info', hash: 'tag-ok', list: 'tag-warn', set: 'tag-warn', zset: 'tag-ok' })[type] || ''
const dbKeys = (i) => info.value?.databases?.[i]?.keys ?? 0
const memPct = computed(() => (info.value?.max_memory ? Math.min(100, (info.value.used_memory / info.value.max_memory) * 100) : null))

onMounted(async () => {
  await loadInfo()
  if (ready.value) {
    // Start in the first database that has keys.
    const first = (info.value.databases || []).find((d) => d.keys > 0)
    if (first) db.value = first.db
    scan()
  }
})
</script>

<template>
  <div class="pg">
    <div v-if="!embedded" class="page-head sr-only"><h1 class="page-title">{{ t('redis.title') }}</h1></div>

    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>

    <span v-if="loading" class="skel" style="height: 220px; border-radius: 14px"></span>

    <template v-if="info && !loading">
      <div v-if="!info.installed" class="card empty">
        <AppLogo name="redis" size="lg" style="margin: 0 auto 14px" />
        <p class="empty-lead">{{ t('redis.not_installed') }}</p>
        <p class="muted">{{ t('redis.not_installed_hint') }}</p>
        <RouterLink class="btn btn-primary" :to="{ name: 'software' }">{{ t('nav.software') }}</RouterLink>
      </div>

      <div v-else-if="!info.running" class="card empty">
        <AppLogo name="redis" size="lg" dim style="margin: 0 auto 14px" />
        <p class="empty-lead">{{ t('redis.not_running') }}</p>
        <p v-if="info.error" class="muted mono small">{{ info.error }}</p>
        <RouterLink class="btn" :to="{ name: 'services' }">{{ t('nav.services') }}</RouterLink>
      </div>

      <template v-else>
        <div class="kpis">
          <div class="kpi">
            <AppLogo name="redis" />
            <div>
              <div class="kpi-val">Redis {{ info.version }}</div>
              <div class="kpi-label">{{ t('pg.redis_up', { d: formatUptime(info.uptime, t) }) }}</div>
            </div>
          </div>
          <div class="kpi">
            <span class="kpi-ico c-violet"><Icon name="memory" :size="20" /></span>
            <div style="flex: 1; min-width: 0">
              <div class="kpi-val">{{ formatBytes(info.used_memory) }}<span v-if="info.max_memory" class="muted kpi-of"> / {{ formatBytes(info.max_memory) }}</span></div>
              <div class="kpi-label">{{ t('redis.memory') }}</div>
              <div v-if="memPct !== null" class="ubar" style="width: 100%" aria-hidden="true"><span :style="{ width: memPct + '%' }"></span></div>
            </div>
          </div>
          <div class="kpi">
            <span class="kpi-ico c-blue"><Icon name="key" :size="20" /></span>
            <div>
              <div class="kpi-val">{{ totalKeys.toLocaleString(locale) }}</div>
              <div class="kpi-label">{{ t('redis.keys') }}</div>
            </div>
          </div>
        </div>

        <div class="redis-grid">
          <section class="panel">
            <form class="panel-tools" @submit.prevent="scan()">
              <label class="sr-only" for="redis-db">{{ t('redis.database') }}</label>
              <select id="redis-db" :value="db" class="input input-auto" @change="changeDB(Number($event.target.value))">
                <option v-for="i in 16" :key="i - 1" :value="i - 1">{{ t('redis.db_option', { n: String(i - 1), keys: dbKeys(i - 1) }, dbKeys(i - 1)) }}</option>
              </select>
              <label class="search-field grow">
                <Icon name="search" :size="15" />
                <input v-model="pattern" class="mono" maxlength="256" spellcheck="false" autocomplete="off" :placeholder="t('redis.pattern_placeholder')" :aria-label="t('redis.pattern')" />
              </label>
              <button class="btn btn-primary" :disabled="scanning">{{ t('redis.search') }}</button>
              <button type="button" class="icon-btn bordered" :aria-label="t('files.refresh')" :title="t('files.refresh')" @click="loadInfo(); scan()"><Icon name="refresh" :size="16" /></button>
            </form>
            <p class="small muted redis-hint">{{ t('redis.pattern_hint') }}</p>

            <div v-if="scanned && !keys.length && cursor === '0'" class="empty">
              <div class="empty-icon"><Icon name="key" :size="24" /></div>
              <p class="empty-lead">{{ t('redis.no_keys') }}</p>
            </div>
            <div v-if="keys.length" class="table-wrap">
              <table class="table">
                <thead>
                  <tr>
                    <th>{{ t('redis.col_key') }}</th>
                    <th>{{ t('redis.col_type') }}</th>
                    <th>{{ t('redis.col_ttl') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="k in keys" :key="k.id" :class="{ 'row-selected': key?.id === k.id }">
                    <td class="key-cell"><button type="button" class="key-btn mono" @click="openKey(k)">{{ k.name }}</button></td>
                    <td><span class="tag" :class="typeClass(k.type)">{{ k.type }}</span></td>
                    <td class="nowrap small muted">{{ ttlText(k.ttl) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div class="panel-foot">
              <button v-if="scanned && cursor !== '0'" type="button" class="btn btn-sm" :disabled="scanning" @click="scan(true)">{{ t('redis.more') }}</button>
              <span class="foot-gap"></span>
              <button type="button" class="btn btn-sm btn-danger" :disabled="!dbKeys(db)" @click="askFlush"><Icon name="trash" :size="15" />{{ t('redis.flush', { db }) }}</button>
            </div>
          </section>

          <section class="panel">
            <div class="panel-body">
              <p v-if="keyError" class="alert alert-crit pre-wrap" role="alert">{{ keyError }}</p>
              <span v-if="keyLoading" class="skel" style="height: 120px"></span>
              <div v-else-if="!key" class="empty pick">
                <div class="empty-icon"><Icon name="eye" :size="24" /></div>
                <p class="muted">{{ t('redis.pick_key') }}</p>
              </div>
              <template v-else>
                <div class="card-head">
                  <h2 class="mono key-title">{{ key.name }}</h2>
                  <button type="button" class="btn btn-sm btn-danger" @click="deleteKey"><Icon name="trash" :size="15" />{{ t('redis.delete') }}</button>
                </div>
                <dl class="kv">
                  <div><dt>{{ t('redis.col_type') }}</dt><dd><span class="tag" :class="typeClass(key.type)">{{ key.type }}</span></dd></div>
                  <div><dt>{{ t('redis.col_ttl') }}</dt><dd>{{ ttlText(key.ttl) }}</dd></div>
                  <div><dt>{{ t('redis.size') }}</dt><dd>{{ key.type === 'string' ? formatBytes(key.size) : t('redis.items', { n: key.size }, key.size) }}</dd></div>
                </dl>
                <p v-if="key.truncated" class="alert alert-warn small note">
                  {{ key.type === 'string' ? t('redis.truncated_string') : ['hash', 'list', 'set', 'zset'].includes(key.type) ? t('redis.truncated_items', { n: (key.items || []).length }, (key.items || []).length) : t('redis.unsupported_type') }}
                </p>
                <p v-if="key.binary" class="small muted note">{{ t('redis.binary') }}</p>

                <pre v-if="key.type === 'string'" class="log note">{{ key.value }}</pre>
                <div v-else-if="(key.items || []).length" class="table-wrap note value-table">
                  <table class="table">
                    <thead>
                      <tr>
                        <th v-if="key.type === 'list'">#</th>
                        <th v-if="key.type === 'hash'">{{ t('redis.field') }}</th>
                        <th>{{ key.type === 'zset' || key.type === 'set' ? t('redis.member') : t('redis.value') }}</th>
                        <th v-if="key.type === 'zset'">{{ t('redis.score') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="(it, i) in key.items" :key="i">
                        <td v-if="key.type === 'list'" class="muted small">{{ i }}</td>
                        <td v-if="key.type === 'hash'" class="mono small value-cell">{{ it.field }}</td>
                        <td class="mono small value-cell">{{ it.value }}<span v-if="it.truncated" class="muted"> …</span></td>
                        <td v-if="key.type === 'zset'" class="mono small">{{ it.score }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </template>
            </div>
          </section>
        </div>

        <Modal v-if="flushing" danger :title="t('redis.flush_title', { db })" @close="flushing = false">
          <p v-if="flushError" class="alert alert-crit pre-wrap" role="alert">{{ flushError }}</p>
          <p>{{ t('redis.flush_body', { db, n: dbKeys(db) }, dbKeys(db)) }}</p>
          <form class="stack" @submit.prevent="flush">
            <label class="field">
              <span>{{ t('redis.flush_confirm', { db }) }}</span>
              <input v-model="flushConfirm" class="input input-auto mono" inputmode="numeric" autocomplete="off" required />
            </label>
            <div class="modal-actions">
              <button type="button" class="btn btn-ghost" @click="flushing = false">{{ t('common.cancel') }}</button>
              <button class="btn btn-danger btn-solid" :disabled="flushBusy || flushConfirm.trim() !== String(db)">{{ t('redis.flush_button') }}</button>
            </div>
          </form>
        </Modal>
      </template>
    </template>
  </div>
</template>

<style scoped>
.redis-grid { display: grid; grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); gap: 16px; align-items: start; }
.redis-grid .panel-tools .search-field { flex: 1 1 140px; }
@media (max-width: 1180px) {
  .redis-grid { grid-template-columns: minmax(0, 1fr); }
}
.redis-hint { margin: 0; padding: 8px 18px 0; }
.kpi-of { font-size: 14px; font-weight: 600; }
.key-cell { max-width: 360px; }
.key-btn {
  padding: 0; border: 0; background: none; color: var(--accent); font-size: 13px; text-align: left;
  cursor: pointer; overflow-wrap: anywhere;
}
.key-btn:hover { text-decoration: underline; }
.key-title { font-size: 15px; overflow-wrap: anywhere; min-width: 0; margin: 0; }
.value-cell { overflow-wrap: anywhere; white-space: pre-wrap; max-width: 420px; }
.value-table { margin: 12px -18px -16px; }
.pick { padding: 30px 10px; }
.foot-gap { flex: 1; }
@media (max-width: 640px) {
  .redis-grid { grid-template-columns: minmax(0, 1fr); }
}
</style>
