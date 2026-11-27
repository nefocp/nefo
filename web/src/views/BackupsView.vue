<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import RowActions from '../components/RowActions.vue'
import Modal from '../components/Modal.vue'
import BackupStorage from '../components/BackupStorage.vue'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytes, relativeTime } from '../format.js'
import { useRoute, useRouter } from 'vue-router'
import { usePageMeta } from '../page.js'
import Icon from '../components/Icon.vue'
import JobLog from '../components/JobLog.vue'
import AppLogo from '../components/AppLogo.vue'
import { useJob } from '../composables/useJob.js'
import { put, list } from '../data.js'
import { logoSrc } from '../components/apps/appstore.js'
import { appName } from '../logos.js'

const { t, locale } = useI18n()

const data = ref(null)
const error = ref('')
const notice = ref('')
const { job, running: jobRunning, watch: watchJob, resume, clear: clearJob } = useJob(() => load())

async function load() {
  try {
    const first = !data.value
    data.value = await api('GET', 'api/backups')
    error.value = ''
    if (first) {
      const tg = data.value.targets || {}
      const kind = (tg.site || []).length ? 'site' : (tg.database || []).length ? 'database' : 'folder'
      now.value.kind = sched.value.kind = kind
    }
    if (!now.value.target) pickDefaultTarget(now.value)
    if (!sched.value.target) pickDefaultTarget(sched.value)
  } catch (e) {
    error.value = errorMessage(e)
  }
  nameApps()
}

function clearMessages() {
  error.value = ''
  notice.value = ''
}

// Folders are typed in (with earlier ones suggested); sites and databases are picked.
function pickDefaultTarget(form) {
  form.target = form.kind === 'folder' ? '' : data.value?.targets[form.kind]?.[0] ?? ''
}

const hasStorage = computed(() => (data.value?.remotes ?? []).length > 0)
const kindLabel = (kind) => t(`backups.kind_${kind}`)
const remoteLabel = (r) => t(r === 'sftp' ? 'backups.remote_sftp' : r === 'gdrive' ? 'backups.remote_gdrive' : 'backups.remote_s3')
const isEncrypted = (b, r) => (b.encrypted || []).includes(r)
// A backup kept here can be uploaded to the remote storages that lack it.
const missingRemote = (b) => (data.value?.remotes ?? []).some((r) => !(b.remotes || []).includes(r))

// ---- back up now ----
const now = ref({ kind: 'site', target: '', remote: false })

async function backupNow() {
  clearMessages()
  try {
    watchJob(await api('POST', 'api/backups', { ...now.value, target: now.value.target.trim() }))
    showNow.value = false
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// ---- actions on a backup ----
const restoring = ref(null)
const confirmText = ref('')
const restorePassphrase = ref('')
// A backup kept only in remote storage, encrypted there: the restore may
// need the passphrase it was uploaded with.
const needsPassphrase = (b) => !b.local && (b.encrypted || []).length > 0

async function startJob(path, body) {
  clearMessages()
  try {
    watchJob(await api('POST', path, body))
    return true
  } catch (e) {
    error.value = errorMessage(e)
    return false
  }
}

function askRestore(b) {
  clearMessages()
  restoring.value = b
  confirmText.value = ''
  restorePassphrase.value = ''
}

const restoreBody = (b) =>
  b.kind === 'site' ? t(b.exists === false ? 'backups.restore_site_new' : 'backups.restore_site_body')
    : b.kind === 'folder' ? t('backups.restore_folder_body', { folder: b.target })
      : b.kind === 'app' ? t('backups.restore_app_body') : t(b.exists === false ? 'backups.restore_db_new' : 'backups.restore_db_body')

// A backup found on a storage and added to the list: restore it now.
async function restoreFound(id) {
  await load()
  setTab('list')
  const b = backups.value.find((x) => x.id === id)
  if (b) askRestore(b)
}

// App backups and schedules (kind "app") name the app's instance. A backup
// also records its app, so it keeps its name and logo after the app is
// uninstalled; older ones are found through the installed instance, or the
// catalog (a service app's instance is named after its app). The app store
// says the app's name, logo and website.
const appStore = ref(null)
function appOf(b) {
  const inst = list(appStore.value?.installed).find((a) => a.name === b.target)
  const apps = list(appStore.value?.apps)
  const id = b.app || inst?.app || (apps.some((a) => a.id === b.target) ? b.target : '')
  const cat = apps.find((a) => a.id === id)
  return { name: cat?.name || (id ? appName(id) : ''), logo: logoSrc(cat?.logo || id), where: inst?.domain || '', known: !!cat }
}
// An app to back up, in the forms: its name and website.
function targetLabel(kind, x) {
  if (kind !== 'app') return x
  const a = appOf({ target: x })
  return !a.name ? x : a.where ? `${a.name} · ${a.where}` : a.name
}

async function restore() {
  const b = restoring.value
  const body = { confirm: confirmText.value.trim() }
  if (needsPassphrase(b) && restorePassphrase.value) body.passphrase = restorePassphrase.value
  if (await startJob(`api/backups/${b.id}/restore`, body)) {
    restoring.value = null
    restorePassphrase.value = ''
  }
}

async function remove(b) {
  clearMessages()
  if (!window.confirm(t('backups.delete_confirm', { name: b.name }))) return
  try {
    await api('DELETE', `api/backups/${b.id}`)
    notice.value = t('backups.deleted', { name: b.name })
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// ---- schedules ----
const showSched = ref(false)
const sched = ref({ kind: 'site', target: '', frequency: 'daily', hour: 3, weekday: 0, keep: 7, remote: false })

async function addSchedule() {
  clearMessages()
  try {
    await api('POST', 'api/backups/schedules', {
      ...sched.value, target: sched.value.target.trim(),
      hour: Number(sched.value.hour), weekday: Number(sched.value.weekday), keep: Number(sched.value.keep),
    })
    showSched.value = false
    notice.value = t('backups.schedule_added')
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function removeSchedule(sc) {
  clearMessages()
  if (!window.confirm(t('backups.schedule_delete_confirm', { target: sc.target }))) return
  try {
    await api('DELETE', `api/backups/schedules/${sc.id}`)
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// ---- display helpers ----
const weekdays = computed(() =>
  [0, 1, 2, 3, 4, 5, 6].map((d) => new Date(2026, 10, 1 + d).toLocaleDateString(locale.value, { weekday: 'long' })),
)
const hh = (h) => String(h).padStart(2, '0') + ':00'
const describe = (sc) =>
  sc.frequency === 'daily'
    ? t('backups.every_day', { time: hh(sc.hour) })
    : t('backups.every_week', { day: weekdays.value[sc.weekday], time: hh(sc.hour) })
const formatDate = (v) => new Date(v).toLocaleString(locale.value, { dateStyle: 'medium', timeStyle: 'short' })
// ---- page layout: tabs, filters, KPIs ----
const route = useRoute()
const router = useRouter()
const tab = computed(() => (['schedules', 'storage'].includes(route.query.tab) ? route.query.tab : 'list'))
const setTab = (id) => router.replace({ query: { ...route.query, tab: id === 'list' ? undefined : id } })
const kind = ref('all')
const q = ref('')
const backups = computed(() => data.value?.backups || [])
const schedules = computed(() => data.value?.schedules || [])
const shown = computed(() => {
  const f = q.value.trim().toLowerCase()
  return backups.value.filter((b) => (kind.value === 'all' || b.kind === kind.value) && (!f || `${b.target} ${b.name}`.toLowerCase().includes(f)))
})
const kindIcon = (k) => ({ site: 'globe', database: 'database', folder: 'folder', app: 'package' })[k] || 'archive'
const kinds = computed(() => ['all', 'site', 'database', 'folder', ...(backups.value.some((b) => b.kind === 'app') ? ['app'] : [])])
// What the forms can back up: apps once some are installed.
const formKinds = computed(() => ['site', 'database', 'folder', ...((data.value?.targets?.app || []).length ? ['app'] : [])])
const showNow = ref(false)
function openNow() {
  clearMessages()
  if (!now.value.target) pickDefaultTarget(now.value)
  showNow.value = true
}
function openSched() {
  clearMessages()
  if (!sched.value.target) pickDefaultTarget(sched.value)
  showSched.value = true
}
const kpis = computed(() => {
  const latest = [...backups.value].sort((a, b) => new Date(b.created_at) - new Date(a.created_at))[0]
  const failed = schedules.value.filter((s) => s.last_error).length
  const remotes = data.value?.remotes || []
  return [
    { key: 'n', icon: 'archive', color: 'green', value: `${backups.value.length} · ${formatBytes(backups.value.reduce((a, b) => a + (b.size || 0), 0))}`, label: t('pg.bk_kpi_count') },
    { key: 'last', icon: 'clock', color: 'blue', value: latest ? relativeTime(latest.created_at, locale.value) : '—', label: latest ? t('pg.bk_kpi_last', { target: latest.target }) : t('pg.bk_kpi_none') },
    { key: 'sched', icon: 'calendar', color: failed ? 'red' : 'orange', value: String(schedules.value.length), label: failed ? t('pg.bk_kpi_sched_failed', { n: failed }) : t('pg.bk_kpi_sched') },
    { key: 'remote', icon: 'cloud', color: remotes.length ? 'teal' : 'gray', value: remotes.length ? remotes.map(remoteLabel).join(' + ') : t('pg.bk_kpi_local'), label: t('backups.storage_title') },
  ]
})
usePageMeta(() => ({ sub: data.value ? [t('pg.bk_sub', { n: backups.value.length }, backups.value.length), t('pg.bk_sub_schedules', { s: schedules.value.length }, schedules.value.length)].join(' · ') : '' }))

const utcOffset = computed(() => {
  const m = (data.value?.server_utc_offset ?? 0) / 60
  const sign = m < 0 ? '-' : '+'
  const a = Math.abs(m)
  return `UTC${sign}${String(Math.floor(a / 60)).padStart(2, '0')}:${String(a % 60).padStart(2, '0')}`
})

onMounted(async () => {
  await load()
  resume(['backup', 'restore', 'upload_backup'])
})
// The app store's list names the apps of app backups, schedules and the
// forms (asked after each load of the backups while some app is not named yet).
let askingApps = false
function nameApps() {
  const d = data.value
  const unnamed = [...(d?.backups || []), ...(d?.schedules || [])].some((b) => b.kind === 'app' && !appOf(b).known) ||
    (d?.targets?.app || []).some((x) => !appOf({ target: x }).known)
  if (askingApps || !unnamed) return
  askingApps = true
  api('GET', 'api/apps')
    .then((d) => {
      appStore.value = d
      put('api/apps', d)
    })
    .catch(() => {})
    .finally(() => (askingApps = false))
}
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('backups.title') }}</h1></div>
    <Teleport defer to="#topbar-actions">
      <template v-if="data">
        <button class="btn bk-top-sec" @click="openSched"><Icon name="clock" :size="15" />{{ t('backups.schedule_add') }}</button>
        <button class="btn btn-primary" :disabled="jobRunning" @click="openNow"><Icon name="archive" :size="15" />{{ t('backups.now') }}</button>
      </template>
    </Teleport>

    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />

    <template v-if="!data && !error">
      <div class="kpis"><span v-for="i in 4" :key="i" class="skel" style="height: 72px; border-radius: 14px"></span></div>
      <span class="skel" style="height: 260px; border-radius: 14px"></span>
    </template>

    <template v-if="data">
      <datalist id="backup-folders">
        <option v-for="f in data.targets.folder || []" :key="f" :value="f" />
      </datalist>

      <div class="kpis">
        <div v-for="k in kpis" :key="k.key" class="kpi">
          <span class="kpi-ico" :class="'c-' + k.color"><Icon :name="k.icon" :size="20" /></span>
          <div style="min-width: 0">
            <div class="kpi-val">{{ k.value }}</div>
            <div class="kpi-label">{{ k.label }}</div>
          </div>
        </div>
      </div>

      <nav class="ptabs" role="tablist" :aria-label="t('backups.title')">
        <button type="button" role="tab" :aria-selected="tab === 'list'" @click="setTab('list')"><Icon name="archive" :size="16" />{{ t('backups.list_title') }}<span class="n">{{ backups.length }}</span></button>
        <button type="button" role="tab" :aria-selected="tab === 'schedules'" @click="setTab('schedules')"><Icon name="clock" :size="16" />{{ t('backups.schedules_title') }}<span class="n">{{ schedules.length }}</span></button>
        <button type="button" role="tab" :aria-selected="tab === 'storage'" @click="setTab('storage')"><Icon name="cloud" :size="16" />{{ t('backups.storage_title') }}</button>
      </nav>

      <!-- backups -->
      <section v-if="tab === 'list'" class="panel">
        <div class="panel-tools">
          <div class="tabs" role="group" :aria-label="t('pg.bk_kind')">
            <button v-for="k in kinds" :key="k" type="button" :aria-pressed="kind === k" @click="kind = k">
              {{ k === 'all' ? t('ws.all') : kindLabel(k) }}
            </button>
          </div>
          <span class="tools-gap"></span>
          <label class="search-field">
            <Icon name="search" :size="15" />
            <input v-model="q" type="search" :placeholder="t('pg.bk_filter')" :aria-label="t('pg.bk_filter')" />
          </label>
        </div>
        <div v-if="!backups.length" class="empty">
          <div class="empty-icon"><Icon name="archive" :size="26" /></div>
          <p class="empty-lead">{{ t('backups.empty') }}</p>
          <p class="muted">{{ t('backups.now_hint') }}</p>
          <button class="btn btn-primary" :disabled="jobRunning" @click="openNow"><Icon name="archive" />{{ t('backups.now') }}</button>
        </div>
        <p v-else-if="!shown.length" class="muted panel-body" style="margin: 0">{{ t('pg.no_match') }}</p>
        <div v-else class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>{{ t('backups.col_target') }}</th>
                <th>{{ t('backups.col_created') }}</th>
                <th>{{ t('backups.col_size') }}</th>
                <th>{{ t('backups.col_where') }}</th>
                <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="b in shown" :key="b.id">
                <td>
                  <div v-if="b.kind === 'app'" class="name-ico">
                    <AppLogo :src="appOf(b).logo" size="sm" />
                    <div class="name-ico-text">
                      <div class="bk-target">{{ appOf(b).name || b.target }} <span class="tag bk-kind">{{ kindLabel('app') }}</span></div>
                      <div class="name-ico-sub bk-file"><template v-if="appOf(b).where">{{ appOf(b).where }} · </template><span class="mono">{{ b.name }}</span></div>
                    </div>
                  </div>
                  <div v-else class="name-ico">
                    <span class="tile" :class="'k-' + b.kind"><Icon :name="kindIcon(b.kind)" :size="16" /></span>
                    <div class="name-ico-text">
                      <div :class="{ mono: b.kind === 'folder' }" class="bk-target">{{ b.target }}</div>
                      <div class="name-ico-sub mono bk-file">{{ b.name }}</div>
                    </div>
                  </div>
                </td>
                <td class="nowrap">
                  {{ relativeTime(b.created_at, locale) }}
                  <div class="name-ico-sub">{{ formatDate(b.created_at) }}<template v-if="b.scheduled"> · {{ t('backups.auto') }}</template></div>
                </td>
                <td class="nowrap">{{ formatBytes(b.size) }}</td>
                <td class="nowrap">
                  <span v-if="b.local" class="status-pill ok">{{ t('backups.local') }}</span>
                  <span v-for="r in b.remotes || []" :key="r" class="tag tag-info" :title="isEncrypted(b, r) ? t('backups.encrypted_copy') : t('backups.remote')"><Icon :name="isEncrypted(b, r) ? 'lock' : 'cloud'" :size="12" />{{ remoteLabel(r) }}</span>
                </td>
                <td class="actions">
                  <RowActions :actions="[
                    { label: t('backups.restore'), icon: 'refresh', inline: true, disabled: jobRunning, onClick: () => askRestore(b) },
                    b.local ? { label: t('files.download'), icon: 'download', href: `api/backups/${b.id}/download` } : null,
                    b.local && missingRemote(b) ? { label: t('backups.upload'), icon: 'upload', disabled: jobRunning, onClick: () => startJob(`api/backups/${b.id}/upload`, {}) } : null,
                    { label: t('websites.delete'), icon: 'trash', danger: true, onClick: () => remove(b) },
                  ].filter(Boolean)" />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <!-- schedules -->
      <section v-else-if="tab === 'schedules'" class="panel">
        <div class="panel-head">
          <h2 class="panel-title"><Icon name="clock" :size="17" />{{ t('backups.schedules_title') }}</h2>
          <button class="btn btn-sm" @click="openSched"><Icon name="plus" :size="14" />{{ t('backups.schedule_add') }}</button>
        </div>
        <div v-if="!schedules.length" class="empty">
          <div class="empty-icon"><Icon name="clock" :size="26" /></div>
          <p class="empty-lead">{{ t('backups.no_schedules') }}</p>
          <button class="btn btn-primary" @click="openSched"><Icon name="plus" />{{ t('backups.schedule_add') }}</button>
        </div>
        <div v-else class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>{{ t('backups.col_target') }}</th>
                <th>{{ t('pg.bk_when') }}</th>
                <th>{{ t('pg.bk_keep') }}</th>
                <th>{{ t('pg.bk_last') }}</th>
                <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="sc in schedules" :key="sc.id">
                <td>
                  <div v-if="sc.kind === 'app'" class="name-ico">
                    <AppLogo :src="appOf(sc).logo" size="sm" />
                    <div class="name-ico-text">
                      <div>{{ appOf(sc).name || sc.target }}</div>
                      <div class="name-ico-sub">{{ kindLabel('app') }}<template v-if="appOf(sc).where"> · {{ appOf(sc).where }}</template></div>
                    </div>
                  </div>
                  <div v-else class="name-ico">
                    <span class="tile" :class="'k-' + sc.kind"><Icon :name="kindIcon(sc.kind)" :size="16" /></span>
                    <div class="name-ico-text">
                      <div :class="{ mono: sc.kind === 'folder' }">{{ sc.target }}</div>
                      <div class="name-ico-sub">{{ kindLabel(sc.kind) }}</div>
                    </div>
                  </div>
                </td>
                <td>{{ describe(sc) }}<div v-if="sc.remote" class="name-ico-sub"><Icon name="cloud" :size="12" /> {{ t('backups.remote') }}</div></td>
                <td class="nowrap">{{ t('backups.keeps', { n: sc.keep }) }}</td>
                <td class="small">
                  <template v-if="sc.last_run_at">
                    <span class="status-pill" :class="sc.last_error ? 'crit' : 'ok'">{{ sc.last_error ? t('jobs.failed') : t('pg.bk_ok') }}</span>
                    <div class="name-ico-sub">{{ formatDate(sc.last_run_at) }}</div>
                    <details v-if="sc.last_error" class="ssl-error"><summary>{{ t('backups.last_failed') }}</summary><p class="pre-wrap">{{ sc.last_error }}</p></details>
                  </template>
                  <span v-else class="status-pill">{{ t('backups.never_ran') }}</span>
                </td>
                <td class="actions"><button class="btn btn-sm btn-danger" @click="removeSchedule(sc)"><Icon name="trash" :size="14" />{{ t('websites.delete') }}</button></td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="muted small panel-foot" style="margin: 0">{{ t('backups.server_time', { offset: utcOffset }) }}</p>
      </section>

      <!-- remote storage -->
      <BackupStorage v-else :s3="data.storage" :sftp="data.sftp" :gdrive="data.gdrive" :gdrive-default-folder="data.gdrive_default_folder || ''"
        :gdrive-builtin="!!data.gdrive_builtin_client" @changed="load" @restore="restoreFound" />

      <!-- back up now -->
      <Modal v-if="showNow" :title="t('backups.now_title')" @close="showNow = false">
        <form class="stack" @submit.prevent="backupNow">
          <div class="tabs bk-kinds" role="radiogroup" :aria-label="t('pg.bk_kind')">
            <button v-for="k in formKinds" :key="k" type="button" role="radio" :aria-checked="now.kind === k" :aria-pressed="now.kind === k" @click="now.kind = k; pickDefaultTarget(now)">
              <Icon :name="kindIcon(k)" :size="15" />{{ kindLabel(k) }}
            </button>
          </div>
          <label class="field">
            <span>{{ now.kind === 'folder' ? t('backups.folder_path') : t('backups.col_target') }}</span>
            <input v-if="now.kind === 'folder'" v-model="now.target" class="input mono" list="backup-folders" placeholder="/etc/nginx" spellcheck="false" autocapitalize="off" required />
            <select v-else v-model="now.target" class="input" required>
              <option v-for="x in data.targets[now.kind] || []" :key="x" :value="x">{{ targetLabel(now.kind, x) }}</option>
            </select>
            <small class="muted">{{ now.kind === 'folder' ? t('backups.folder_hint') : t('backups.now_hint') }}</small>
          </label>
          <label v-if="hasStorage" class="check"><input v-model="now.remote" type="checkbox" /><span>{{ t('backups.upload_too') }}</span></label>
          <div class="modal-actions">
            <button type="button" class="btn btn-ghost" @click="showNow = false">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="jobRunning || !now.target.trim()"><Icon name="archive" />{{ t('backups.now') }}</button>
          </div>
        </form>
      </Modal>

      <!-- new schedule -->
      <Modal v-if="showSched" :title="t('backups.schedule_add')" @close="showSched = false">
        <form class="stack" @submit.prevent="addSchedule">
          <div class="tabs bk-kinds" role="radiogroup" :aria-label="t('pg.bk_kind')">
            <button v-for="k in formKinds" :key="k" type="button" role="radio" :aria-checked="sched.kind === k" :aria-pressed="sched.kind === k" @click="sched.kind = k; pickDefaultTarget(sched)">
              <Icon :name="kindIcon(k)" :size="15" />{{ kindLabel(k) }}
            </button>
          </div>
          <label class="field">
            <span>{{ sched.kind === 'folder' ? t('backups.folder_path') : t('backups.col_target') }}</span>
            <input v-if="sched.kind === 'folder'" v-model="sched.target" class="input mono" list="backup-folders" placeholder="/etc/nginx" spellcheck="false" autocapitalize="off" required />
            <select v-else v-model="sched.target" class="input" required>
              <option v-for="x in data.targets[sched.kind] || []" :key="x" :value="x">{{ targetLabel(sched.kind, x) }}</option>
            </select>
          </label>
          <div class="row">
            <label class="field grow">
              <span>{{ t('pg.bk_when') }}</span>
              <select v-model="sched.frequency" class="input">
                <option value="daily">{{ t('backups.daily') }}</option>
                <option value="weekly">{{ t('backups.weekly') }}</option>
              </select>
            </label>
            <label v-if="sched.frequency === 'weekly'" class="field grow">
              <span>{{ t('pg.bk_day') }}</span>
              <select v-model="sched.weekday" class="input">
                <option v-for="(d, i) in weekdays" :key="i" :value="i">{{ d }}</option>
              </select>
            </label>
            <label class="field grow">
              <span>{{ t('pg.bk_hour') }}</span>
              <select v-model="sched.hour" class="input">
                <option v-for="h in 24" :key="h" :value="h - 1">{{ hh(h - 1) }}</option>
              </select>
            </label>
          </div>
          <small class="muted">{{ t('backups.server_time', { offset: utcOffset }) }}</small>
          <label class="field">
            <span>{{ t('backups.keep') }}</span>
            <input v-model="sched.keep" class="input input-auto" type="number" min="1" max="365" required />
          </label>
          <label v-if="hasStorage" class="check"><input v-model="sched.remote" type="checkbox" /><span>{{ t('backups.upload_too') }}</span></label>
          <div class="modal-actions">
            <button type="button" class="btn btn-ghost" @click="showSched = false">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="!sched.target.trim()">{{ t('backups.schedule_save') }}</button>
          </div>
        </form>
      </Modal>

      <!-- restore confirmation -->
      <Modal v-if="restoring" danger :title="t('backups.restore_title', { name: restoring.name })" @close="restoring = null">
        <p>{{ restoreBody(restoring) }}</p>
        <p v-if="restoring.kind === 'app' && restoring.exists === false" class="alert alert-warn">{{ t('backups.restore_app_missing') }}</p>
        <p v-else-if="restoring.kind === 'app'" class="alert alert-info">{{ t('backups.restore_app_passwords') }}</p>
        <form class="stack" @submit.prevent="restore">
          <label v-if="needsPassphrase(restoring)" class="field">
            <span>{{ t('backups.restore_passphrase') }}</span>
            <input v-model="restorePassphrase" class="input mono" type="password" autocomplete="off" spellcheck="false" />
            <small class="muted">{{ t('backups.restore_passphrase_hint') }}</small>
          </label>
          <label class="field">
            <span>{{ t('websites.delete_confirm', { domain: restoring.target }) }}</span>
            <input v-model="confirmText" class="input" autocomplete="off" spellcheck="false" required />
          </label>
          <div class="modal-actions">
            <button type="button" class="btn btn-ghost" @click="restoring = null">{{ t('common.cancel') }}</button>
            <button class="btn btn-danger btn-solid" :disabled="jobRunning || confirmText.trim() !== restoring.target"><Icon name="refresh" />{{ t('backups.restore') }}</button>
          </div>
        </form>
      </Modal>
    </template>
  </div>
</template>

<style scoped>
.tools-gap { flex: 1; }
.tile.k-site { background: rgb(47 107 219 / 0.12); color: #2f6bdb; }
.tile.k-database { background: rgb(116 84 224 / 0.12); color: #7454e0; }
.tile.k-folder { background: rgb(217 154 11 / 0.14); color: #b7800a; }
.bk-target { font-weight: 600; overflow-wrap: anywhere; }
.bk-kind { min-height: 20px; font-size: 11px; margin-left: 4px; vertical-align: 1px; }
.bk-file { overflow-wrap: anywhere; font-size: 11.5px; }
.bk-kinds { display: flex; width: 100%; }
.bk-kinds button { flex: 1; justify-content: center; }
.panel-foot.muted { border-top: 1px solid var(--border); }
@media (max-width: 640px) {
  .bk-top-sec { display: none; }
}
</style>
