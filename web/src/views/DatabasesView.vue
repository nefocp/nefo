<script setup>
import { ref, computed, watch, onMounted, defineAsyncComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import Icon from '../components/Icon.vue'
import AppLogo from '../components/AppLogo.vue'
import RowActions from '../components/RowActions.vue'
import Modal from '../components/Modal.vue'
import { api, ApiError } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytesL, formatNumber } from '../format.js'
import { features } from '../features.js'
import { put, list } from '../data.js'
import { usePageMeta } from '../page.js'
import { toast } from '../toast.js'
import JobLog from '../components/JobLog.vue'
import CopyField from '../components/CopyField.vue'
import DbRemoteAccess from '../components/DbRemoteAccess.vue'
import DbSlowLog from '../components/DbSlowLog.vue'
import { useJob } from '../composables/useJob.js'

// Databases: MariaDB (the list, logins, import and export, remote access),
// its slow query log, and Redis when it is installed, as tabs.
const RedisView = defineAsyncComponent(() => import('./RedisView.vue'))

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()

const status = ref(null)
const databases = ref([])
const sites = ref([])
const loading = ref(true)
const error = ref('')
const notice = ref('')
const creds = ref(null) // credentials being shown

const { job, running: jobRunning, watch: watchJob, resume, clear: clearJob } = useJob(() => load())

async function load() {
  try {
    const [st, dbs, s] = await Promise.all([api('GET', 'api/db/status'), api('GET', 'api/databases').catch(() => ({ databases: [] })), api('GET', 'api/sites')])
    status.value = st
    databases.value = list(dbs.databases)
    sites.value = list(s.sites)
    put('api/databases', { databases: databases.value })
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

function clearMessages() {
  error.value = ''
  notice.value = ''
}

// ---- tabs: ?tab=redis|slow ----
const tabs = computed(() => [
  { id: 'list', label: t('pg.db_tab_list'), logo: 'mariadb', n: status.value?.installed ? databases.value.length : null },
  features.redis || route.query.tab === 'redis' ? { id: 'redis', label: 'Redis', logo: 'redis' } : null,
  status.value?.installed ? { id: 'slow', label: t('pg.db_tab_slow'), icon: 'clock' } : null,
].filter(Boolean))
const tab = computed(() => (tabs.value.some((x) => x.id === route.query.tab) ? route.query.tab : 'list'))
function setTab(id) {
  router.replace({ query: { ...route.query, tab: id === 'list' ? undefined : id } })
}

usePageMeta(() => ({
  sub: status.value?.installed
    ? [`MariaDB ${status.value.version || ''}`.trim(), t('pg.db_count', { n: databases.value.length }, databases.value.length), features.redis ? 'Redis' : ''].filter(Boolean).join(' · ')
    : t('databases.subtitle'),
}))

async function install() {
  clearMessages()
  try {
    watchJob(await api('POST', 'api/db/install', {}))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function restart() {
  clearMessages()
  if (!window.confirm(t('pg.db_restart_confirm'))) return
  try {
    const res = await api('POST', 'api/db/restart', {})
    watchJob(res.job ?? res)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// ---- create ----
const showForm = ref(false)
const form = ref({ name: '', password: '', site_id: 0 })
const formError = ref('')
const busy = ref(false)

async function create() {
  formError.value = ''
  busy.value = true
  try {
    const c = await api('POST', 'api/databases', {
      name: form.value.name.trim().toLowerCase(),
      password: form.value.password,
      site_id: Number(form.value.site_id) || 0,
    })
    creds.value = c
    showForm.value = false
    form.value = { name: '', password: '', site_id: 0 }
    toast(t('pg.db_created', { name: c.name }))
    await load()
  } catch (e) {
    formError.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

// ---- credentials and password ----
async function showCredentials(d) {
  clearMessages()
  if (creds.value?.id === d.id) {
    creds.value = null
    return
  }
  try {
    creds.value = await api('GET', `api/databases/${d.id}/credentials`)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// A new password is random unless the user types one, checked like the
// panel does (validate.DBPassword).
const pwFor = ref(null) // the database getting a new password
const pwMode = ref('random') // random or own
const pwValue = ref('')
const pwError = ref('')
const dbPasswordRe = /^[A-Za-z0-9!#%*+,.:=?@^_~-]{12,64}$/

function newPassword(d) {
  clearMessages()
  pwFor.value = d
  pwMode.value = 'random'
  pwValue.value = ''
  pwError.value = ''
}

async function setPassword() {
  const d = pwFor.value
  const own = pwMode.value === 'own'
  pwError.value = ''
  if (own && !dbPasswordRe.test(pwValue.value)) {
    pwError.value = t('errors.invalid_db_password')
    return
  }
  busy.value = true
  try {
    creds.value = await api('POST', `api/databases/${d.id}/password`, { password: own ? pwValue.value : '' })
    notice.value = t('databases.password_changed', { name: d.name })
    pwFor.value = null
  } catch (e) {
    pwError.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

// ---- delete ----
const deleting = ref(null)
const confirmText = ref('')

function askDelete(d) {
  clearMessages()
  deleting.value = d
  confirmText.value = ''
}

async function remove() {
  const d = deleting.value
  busy.value = true
  try {
    const res = await api('DELETE', `api/databases/${d.id}`, { confirm: confirmText.value.trim() })
    // MariaDB restarts when the last database with remote access goes.
    if (res?.job) watchJob(res.job)
    notice.value = t('databases.deleted', { name: d.name })
    deleting.value = null
    if (creds.value?.id === d.id) creds.value = null
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

// ---- import ----
const importing = ref(null)
const importFile = ref(null)
const importReset = ref(false)
const importBusy = ref(false)
const importError = ref('')

function openImport(d) {
  clearMessages()
  importing.value = d
  importReset.value = false
  importError.value = ''
}

async function runImport() {
  const file = importFile.value?.files?.[0]
  if (!file) return
  importBusy.value = true
  importError.value = ''
  const body = new FormData()
  body.append('file', file, file.name)
  try {
    const res = await fetch(`api/databases/${importing.value.id}/import?reset=${importReset.value ? 1 : 0}`, {
      method: 'POST', body, headers: { 'X-Requested-With': 'panel' }, credentials: 'same-origin',
    })
    if (!res.ok) {
      const data = await res.json().catch(() => null)
      throw new ApiError(res.status, data?.error || 'unknown', data)
    }
    notice.value = t('databases.imported', { name: importing.value.name })
    importing.value = null
    await load()
  } catch (e) {
    importError.value = errorMessage(e)
  } finally {
    importBusy.value = false
  }
}

// ---- remote access ----
const remote = ref(null)
function openRemote(d) {
  clearMessages()
  remote.value = d
}
function remoteSaved({ job: j, hosts }) {
  const d = databases.value.find((x) => x.id === remote.value?.id)
  if (d) d.remote_hosts = hosts
  if (j) watchJob(j)
}

// ---- list ----
const q = ref('')
const filtered = computed(() => {
  const f = q.value.trim().toLowerCase()
  return databases.value.filter((d) => !f || [d.name, d.username, d.site_domain].join(' ').toLowerCase().includes(f))
})
const totalSize = computed(() => databases.value.reduce((a, d) => a + (d.size || 0), 0))
const maxSize = computed(() => Math.max(1, ...databases.value.map((d) => d.size || 0)))
const remoteCount = computed(() => databases.value.filter((d) => list(d.remote_hosts).length).length)
const formatDate = (v) => new Date(v).toLocaleDateString(locale.value, { year: 'numeric', month: 'short', day: 'numeric' })
const ready = computed(() => status.value?.installed)
const adminer = ref(false)
api('GET', 'api/software').then((r) => (adminer.value = list(r.items).some((i) => i.id === 'adminer' && i.installed))).catch(() => {})

function actions(d) {
  return [
    { label: t('databases.show'), icon: 'key', inline: true, onClick: () => showCredentials(d) },
    { label: t('databases.export'), icon: 'download', href: `api/databases/${d.id}/export` },
    { label: t('databases.import'), icon: 'upload', onClick: () => openImport(d) },
    { label: t('databases.change_password'), icon: 'refresh', onClick: () => newPassword(d) },
    { label: t('databases.remote'), icon: 'globe', disabled: jobRunning.value, onClick: () => openRemote(d) },
    { label: t('websites.delete'), icon: 'trash', danger: true, onClick: () => askDelete(d) },
  ]
}

watch(
  () => route.query.tab,
  () => clearMessages(),
)

onMounted(async () => {
  await load()
  resume(['install_db', 'db_restart'])
})
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('databases.title') }}</h1></div>
    <Teleport defer to="#topbar-actions">
      <template v-if="ready && tab === 'list'">
        <button class="btn btn-primary" :disabled="jobRunning" @click="showForm = true; clearMessages()"><Icon name="plus" :size="15" />{{ t('databases.add') }}</button>
      </template>
    </Teleport>

    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />

    <template v-if="loading">
      <span class="skel" style="height: 84px; border-radius: 14px"></span>
      <span class="skel" style="height: 260px; border-radius: 14px"></span>
    </template>

    <template v-else-if="status">
      <!-- MariaDB header -->
      <section class="svc-head">
        <AppLogo name="mariadb" size="lg" />
        <div class="svc-text">
          <h2 class="svc-title">
            MariaDB <span v-if="status.version" class="muted" style="font-weight: 600">{{ status.version }}</span>
            <span v-if="status.installed" class="status-pill" :class="status.running ? 'ok' : 'crit'">{{ status.running ? t('pg.svc_state_active') : t('pg.svc_state_inactive') }}</span>
            <span v-else class="status-pill">{{ t('pg.not_installed') }}</span>
          </h2>
          <div v-if="status.installed" class="svc-meta">
            <span><Icon name="database" :size="14" />{{ t('pg.db_count', { n: databases.length }, databases.length) }}</span>
            <span><Icon name="disk" :size="14" />{{ formatBytesL(totalSize, locale) }}</span>
            <span v-if="remoteCount"><Icon name="globe" :size="14" />{{ t('pg.db_remote_n', { n: remoteCount }) }}</span>
            <span v-else><Icon name="lock" :size="14" />{{ t('pg.db_local_only') }}</span>
          </div>
          <div v-else class="svc-meta">{{ t('databases.subtitle') }}</div>
        </div>
        <div v-if="status.installed" class="svc-actions">
          <a v-if="adminer" class="btn" href="adminer/" target="_blank" rel="noopener"><AppLogo name="adminer" size="xs" />{{ t('databases.open_adminer') }}</a>
          <button type="button" class="btn" :disabled="jobRunning" @click="restart"><Icon name="refresh" :size="15" />{{ t('pg.restart') }}</button>
        </div>
      </section>

      <nav v-if="tabs.length > 1" class="ptabs" role="tablist" :aria-label="t('pg.db_tabs')">
        <button v-for="x in tabs" :key="x.id" type="button" role="tab" :aria-selected="tab === x.id" @click="setTab(x.id)">
          <AppLogo v-if="x.logo" :name="x.logo" size="xs" />
          <Icon v-else :name="x.icon" :size="16" />
          {{ x.label }}<span v-if="x.n !== null && x.n !== undefined" class="n">{{ x.n }}</span>
        </button>
      </nav>

      <!-- MariaDB databases -->
      <template v-if="tab === 'list'">
        <div v-if="!status.supported" class="card empty">
          <div class="empty-icon"><Icon name="database" :size="26" /></div>
          <p class="empty-lead">{{ t('databases.unsupported') }}</p>
        </div>

        <section v-else-if="!status.installed" class="card empty">
          <AppLogo name="mariadb" size="lg" style="margin: 0 auto 14px" />
          <p class="empty-lead">{{ t('databases.install_title') }}</p>
          <p class="muted">{{ t('databases.install_body') }}</p>
          <button class="btn btn-primary" :disabled="jobRunning" @click="install"><Icon name="download" />{{ t('databases.install') }}</button>
        </section>

        <template v-else>
          <section v-if="creds" class="card creds-card">
            <div class="card-head">
              <h2><Icon name="key" :size="17" class="accent-ico" />{{ t('databases.credentials_title', { name: creds.name }) }}</h2>
              <button class="icon-btn" :aria-label="t('common.close')" @click="creds = null"><Icon name="x" /></button>
            </div>
            <div class="creds">
              <CopyField :label="t('databases.col_name')" :value="creds.name" />
              <CopyField :label="t('databases.col_user')" :value="creds.username" />
              <CopyField :label="t('databases.password')" :value="creds.password" />
              <CopyField :label="t('databases.host')" :value="creds.host" />
            </div>
            <p v-if="adminer" class="note">
              <a class="btn btn-sm" href="adminer/" target="_blank" rel="noopener"><Icon name="external" :size="15" />{{ t('databases.open_adminer') }}</a>
            </p>
          </section>

          <section class="panel">
            <div class="panel-tools">
              <label class="search-field">
                <Icon name="search" :size="15" />
                <input v-model="q" type="search" :placeholder="t('pg.db_filter')" :aria-label="t('pg.db_filter')" />
              </label>
              <span class="grow"></span>
              <span class="muted small">{{ t('pg.shown_of', { n: filtered.length, total: databases.length }) }}</span>
            </div>
            <div v-if="!databases.length" class="empty">
              <div class="empty-icon"><Icon name="database" :size="26" /></div>
              <p class="empty-lead">{{ t('databases.empty') }}</p>
              <p class="muted">{{ t('pg.db_empty_hint') }}</p>
              <button class="btn btn-primary" @click="showForm = true"><Icon name="plus" />{{ t('databases.add') }}</button>
            </div>
            <p v-else-if="!filtered.length" class="muted panel-body" style="margin: 0">{{ t('pg.no_match') }}</p>
            <div v-else class="table-wrap">
              <table class="table">
                <thead>
                  <tr>
                    <th>{{ t('databases.col_name') }}</th>
                    <th>{{ t('databases.col_site') }}</th>
                    <th>{{ t('databases.col_size') }}</th>
                    <th>{{ t('pg.db_access') }}</th>
                    <th>{{ t('databases.col_created') }}</th>
                    <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="d in filtered" :key="d.id" :class="{ 'row-selected': creds?.id === d.id }">
                    <td>
                      <div class="name-ico">
                        <span class="tile"><Icon name="database" :size="16" /></span>
                        <div class="name-ico-text">
                          <div class="mono db-name">{{ d.name }}</div>
                          <div class="name-ico-sub">{{ t('pg.db_user', { user: d.username }) }}</div>
                          <div v-if="d.missing" class="small text-crit">{{ t('databases.missing') }}</div>
                        </div>
                      </div>
                    </td>
                    <td>
                      <RouterLink v-if="d.site_id" :to="{ name: 'websites', query: { site: String(d.site_id) } }">{{ d.site_domain }}</RouterLink>
                      <span v-else class="muted">—</span>
                    </td>
                    <td class="nowrap">
                      {{ formatBytesL(d.size, locale) }}
                      <div class="ubar" aria-hidden="true"><span :style="{ width: Math.max(2, ((d.size || 0) / maxSize) * 100) + '%' }"></span></div>
                    </td>
                    <td>
                      <span v-if="list(d.remote_hosts).length" class="status-pill warn" :title="d.remote_hosts.join(', ')">{{ t('databases.remote_tag', { n: d.remote_hosts.length }) }}</span>
                      <span v-else class="status-pill ok">{{ t('pg.db_local') }}</span>
                    </td>
                    <td class="nowrap muted">{{ formatDate(d.created_at) }}</td>
                    <td class="actions"><RowActions :actions="actions(d)" /></td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </template>
      </template>

      <RedisView v-else-if="tab === 'redis'" embedded />
      <DbSlowLog v-else-if="tab === 'slow'" />
    </template>

    <Modal v-if="showForm" :title="t('databases.new_title')" @close="showForm = false">
      <p v-if="formError" class="alert alert-crit pre-wrap" role="alert">{{ formError }}</p>
      <form class="stack" @submit.prevent="create">
        <label class="field">
          <span>{{ t('databases.col_name') }}</span>
          <input v-model="form.name" class="input mono" maxlength="32" autocomplete="off" spellcheck="false" required />
          <small class="muted">{{ t('databases.name_hint') }}</small>
        </label>
        <label class="field">
          <span>{{ t('databases.password') }}</span>
          <input v-model="form.password" class="input" autocomplete="new-password" spellcheck="false" maxlength="64" />
          <small class="muted">{{ t('databases.password_hint') }}</small>
        </label>
        <label class="field">
          <span>{{ t('databases.col_site') }}</span>
          <select v-model="form.site_id" class="input">
            <option :value="0">{{ t('databases.site_none') }}</option>
            <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.domain }}</option>
          </select>
        </label>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="showForm = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="busy">{{ t('databases.create') }}</button>
        </div>
      </form>
    </Modal>

    <Modal v-if="deleting" danger :title="t('databases.delete_title', { name: deleting.name })" @close="deleting = null">
      <p>{{ t('databases.delete_body') }}</p>
      <form class="stack" @submit.prevent="remove">
        <label class="field">
          <span>{{ t('websites.delete_confirm', { domain: deleting.name }) }}</span>
          <input v-model="confirmText" class="input mono" autocomplete="off" spellcheck="false" required />
        </label>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="deleting = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-danger btn-solid" :disabled="busy || confirmText.trim() !== deleting.name"><Icon name="trash" />{{ t('websites.delete') }}</button>
        </div>
      </form>
    </Modal>

    <Modal v-if="pwFor" :title="t('databases.password_title', { name: pwFor.name })" @close="pwFor = null">
      <p v-if="pwError" class="alert alert-crit pre-wrap" role="alert">{{ pwError }}</p>
      <p>{{ t('databases.change_password_body') }}</p>
      <form class="stack" @submit.prevent="setPassword">
        <label class="check"><input v-model="pwMode" type="radio" value="random" /><span>{{ t('databases.password_random') }}</span></label>
        <label class="check"><input v-model="pwMode" type="radio" value="own" /><span>{{ t('databases.password_own') }}</span></label>
        <label v-if="pwMode === 'own'" class="field">
          <span>{{ t('databases.password') }}</span>
          <input v-model="pwValue" class="input mono" autocomplete="new-password" spellcheck="false" autocapitalize="off" maxlength="64" required />
          <small class="muted">{{ t('databases.password_rules') }}</small>
        </label>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="pwFor = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="busy || (pwMode === 'own' && !pwValue)"><Icon name="refresh" />{{ t('databases.change_password') }}</button>
        </div>
      </form>
    </Modal>

    <DbRemoteAccess v-if="remote" :db="remote" @close="remote = null" @saved="remoteSaved" />

    <Modal v-if="importing" :title="t('databases.import_title', { name: importing.name })" @close="importing = null">
      <p v-if="importError" class="alert alert-crit pre-wrap" role="alert">{{ importError }}</p>
      <form class="stack" @submit.prevent="runImport">
        <label class="field">
          <span>{{ t('databases.import_file') }}</span>
          <input ref="importFile" type="file" accept=".sql,.gz" required />
        </label>
        <label class="check"><input v-model="importReset" type="checkbox" /><span>{{ t('databases.import_reset') }}</span></label>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="importing = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="importBusy"><Icon name="upload" />{{ importBusy ? t('databases.importing') : t('databases.import') }}</button>
        </div>
      </form>
    </Modal>
  </div>
</template>

<style scoped>
.db-name { font-weight: 500; }
.creds-card .card-head h2 { display: flex; align-items: center; gap: 9px; }
.accent-ico { color: var(--accent); }
</style>
