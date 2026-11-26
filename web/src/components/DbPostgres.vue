<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from './Icon.vue'
import AppLogo from './AppLogo.vue'
import RowActions from './RowActions.vue'
import Modal from './Modal.vue'
import JobLog from './JobLog.vue'
import CopyField from './CopyField.vue'
import { api, ApiError } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytesL } from '../format.js'
import { list } from '../data.js'
import { toast } from '../toast.js'
import { useJob } from '../composables/useJob.js'

// DbPostgres is the PostgreSQL tab of the Databases page: databases, each
// owned by a role of the same name, with their logins, import and export.
// Installing and removing PostgreSQL is on the Software page.
defineProps({ adminer: { type: Boolean, default: false } })
const { t, locale } = useI18n()

const status = ref(null)
const databases = ref([])
const sites = ref([])
const loading = ref(true)
const error = ref('')
const notice = ref('')
const creds = ref(null)
const busy = ref(false)

const { job, running: jobRunning, watch: watchJob, resume, clear: clearJob } = useJob(() => load())

async function load() {
  try {
    const [st, dbs, s] = await Promise.all([
      api('GET', 'api/postgres/status'),
      api('GET', 'api/postgres/databases').catch(() => ({ databases: [] })),
      api('GET', 'api/sites'),
    ])
    status.value = st
    databases.value = list(dbs.databases)
    sites.value = list(s.sites)
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

const ready = computed(() => status.value?.installed && status.value?.running)
const totalSize = computed(() => databases.value.reduce((a, d) => a + (d.size || 0), 0))
const maxSize = computed(() => Math.max(1, ...databases.value.map((d) => d.size || 0)))
const formatDate = (v) => new Date(v).toLocaleDateString(locale.value, { year: 'numeric', month: 'short', day: 'numeric' })
const adminerURL = (d) => `adminer/?${new URLSearchParams({ pgsql: 'localhost', username: d.username, db: d.name, ns: 'public' })}`

async function restart() {
  clearMessages()
  if (!window.confirm(t('postgres.restart_confirm'))) return
  try {
    watchJob(await api('POST', 'api/postgres/restart', {}))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// ---- create ----
const showForm = ref(false)
const form = ref({ name: '', password: '', site_id: 0 })
const formError = ref('')

function openForm() {
  clearMessages()
  formError.value = ''
  showForm.value = true
}

async function create() {
  formError.value = ''
  busy.value = true
  try {
    const c = await api('POST', 'api/postgres/databases', {
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
    creds.value = await api('GET', `api/postgres/databases/${d.id}/credentials`)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const pwFor = ref(null)
const pwMode = ref('random')
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
    creds.value = await api('POST', `api/postgres/databases/${d.id}/password`, { password: own ? pwValue.value : '' })
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
    await api('DELETE', `api/postgres/databases/${d.id}`, { confirm: confirmText.value.trim() })
    notice.value = t('databases.deleted', { name: d.name })
    deleting.value = null
    if (creds.value?.id === d.id) creds.value = null
    await load()
  } catch (e) {
    error.value = errorMessage(e)
    deleting.value = null
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
    const res = await fetch(`api/postgres/databases/${importing.value.id}/import?reset=${importReset.value ? 1 : 0}`, {
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

// ---- list ----
const q = ref('')
const filtered = computed(() => {
  const f = q.value.trim().toLowerCase()
  return databases.value.filter((d) => !f || [d.name, d.username, d.site_domain].join(' ').toLowerCase().includes(f))
})

function actions(d, adminer) {
  return [
    { label: t('databases.show'), icon: 'key', inline: true, onClick: () => showCredentials(d) },
    adminer ? { label: t('databases.open_adminer'), icon: 'external', href: adminerURL(d) } : null,
    { label: t('postgres.export'), icon: 'download', href: `api/postgres/databases/${d.id}/export` },
    { label: t('databases.import'), icon: 'upload', onClick: () => openImport(d) },
    { label: t('databases.change_password'), icon: 'refresh', onClick: () => newPassword(d) },
    { label: t('websites.delete'), icon: 'trash', danger: true, onClick: () => askDelete(d) },
  ].filter(Boolean)
}

onMounted(async () => {
  await load()
  resume(['pg_restart'])
})
</script>

<template>
  <div class="pg">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />

    <span v-if="loading" class="skel" style="height: 220px; border-radius: 14px"></span>

    <template v-else-if="status">
      <div v-if="!status.installed" class="card empty">
        <AppLogo name="postgresql" size="lg" style="margin: 0 auto 14px" />
        <p class="empty-lead">{{ t('postgres.not_installed') }}</p>
        <p class="muted">{{ t('postgres.not_installed_hint') }}</p>
        <RouterLink class="btn btn-primary" :to="{ name: 'software' }">{{ t('nav.software') }}</RouterLink>
      </div>

      <template v-else>
        <section class="svc-head">
          <AppLogo name="postgresql" size="lg" :dim="!status.running" />
          <div class="svc-text">
            <h2 class="svc-title">
              PostgreSQL <span v-if="status.version" class="muted" style="font-weight: 600">{{ status.version }}</span>
              <span class="status-pill" :class="status.running ? 'ok' : 'crit'">{{ status.running ? t('pg.svc_state_active') : t('pg.svc_state_inactive') }}</span>
            </h2>
            <div class="svc-meta">
              <span><Icon name="database" :size="14" />{{ t('pg.db_count', { n: databases.length }, databases.length) }}</span>
              <span><Icon name="disk" :size="14" />{{ formatBytesL(totalSize, locale) }}</span>
              <span><Icon name="lock" :size="14" />{{ t('postgres.local_only', { port: status.port || 5432 }) }}</span>
            </div>
          </div>
          <div class="svc-actions">
            <a v-if="adminer" class="btn" href="adminer/?pgsql=localhost" target="_blank" rel="noopener"><AppLogo name="adminer" size="xs" />{{ t('databases.open_adminer') }}</a>
            <button type="button" class="btn" :disabled="jobRunning" @click="restart"><Icon name="refresh" :size="15" />{{ t('pg.restart') }}</button>
            <button v-if="ready" type="button" class="btn btn-primary" :disabled="jobRunning" @click="openForm"><Icon name="plus" :size="15" />{{ t('databases.add') }}</button>
          </div>
        </section>

        <div v-if="!status.running" class="card empty">
          <AppLogo name="postgresql" size="lg" dim style="margin: 0 auto 14px" />
          <p class="empty-lead">{{ t('postgres.not_running') }}</p>
          <RouterLink class="btn" :to="{ name: 'services' }">{{ t('nav.services') }}</RouterLink>
        </div>

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
            <CopyField :label="t('postgres.port')" :value="String(creds.port)" />
          </div>
          <p class="muted small">{{ t('postgres.connect_hint', { port: creds.port }) }}</p>
          <p v-if="adminer" class="note">
            <a class="btn btn-sm" :href="adminerURL(creds)" target="_blank" rel="noopener"><Icon name="external" :size="15" />{{ t('databases.open_adminer') }}</a>
          </p>
        </section>

        <section v-if="status.running" class="panel">
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
            <p class="muted">{{ t('postgres.empty_hint') }}</p>
            <button class="btn btn-primary" @click="openForm"><Icon name="plus" />{{ t('databases.add') }}</button>
          </div>
          <p v-else-if="!filtered.length" class="muted panel-body" style="margin: 0">{{ t('pg.no_match') }}</p>
          <div v-else class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>{{ t('databases.col_name') }}</th>
                  <th>{{ t('databases.col_site') }}</th>
                  <th>{{ t('databases.col_size') }}</th>
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
                        <div class="name-ico-sub">{{ t('postgres.owner', { owner: d.owner || d.username }) }}</div>
                        <div v-if="d.missing" class="small text-crit">{{ t('postgres.missing') }}</div>
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
                  <td class="nowrap muted">{{ formatDate(d.created_at) }}</td>
                  <td class="actions"><RowActions :actions="actions(d, adminer)" /></td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </template>
    </template>

    <Modal v-if="showForm" :title="t('postgres.new_title')" @close="showForm = false">
      <p v-if="formError" class="alert alert-crit pre-wrap" role="alert">{{ formError }}</p>
      <form class="stack" @submit.prevent="create">
        <label class="field">
          <span>{{ t('databases.col_name') }}</span>
          <input v-model="form.name" class="input mono" maxlength="32" autocomplete="off" spellcheck="false" required />
          <small class="muted">{{ t('postgres.name_hint') }}</small>
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
      <p>{{ t('postgres.delete_body') }}</p>
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
      <p>{{ t('postgres.change_password_body') }}</p>
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

    <Modal v-if="importing" :title="t('databases.import_title', { name: importing.name })" @close="importing = null">
      <p v-if="importError" class="alert alert-crit pre-wrap" role="alert">{{ importError }}</p>
      <form class="stack" @submit.prevent="runImport">
        <p class="muted small" style="margin: 0">{{ t('postgres.import_hint') }}</p>
        <label class="field">
          <span>{{ t('databases.import_file') }}</span>
          <input ref="importFile" type="file" accept=".sql,.gz" required />
        </label>
        <label class="check"><input v-model="importReset" type="checkbox" /><span>{{ t('postgres.import_reset') }}</span></label>
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
