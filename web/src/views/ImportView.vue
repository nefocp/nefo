<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytesL, formatNumber } from '../format.js'
import { list, load as loadCached, invalidate } from '../data.js'
import { usePageMeta } from '../page.js'
import { useJob } from '../composables/useJob.js'
import JobLog from '../components/JobLog.vue'
import Modal from '../components/Modal.vue'
import Icon from '../components/Icon.vue'

// Import websites from cPanel (a full backup) or from another panel or
// server (a site archive and SQL dumps). The server analyses the archive
// first, as a job; the plan it returns lists what can be imported, and the
// import itself is a second job. The state lives on the server, so a
// reload picks the import up again.
const { t, locale } = useI18n()
usePageMeta(() => ({ title: t('siteimport.title'), sub: t('siteimport.sub') }))

const step = ref('source') // source, analysing, plan, importing, done
const kind = ref('cpanel')
const src = reactive({ mode: 'path', path: '', url: '' })
const dumps = ref([]) // [{ mode, path, url }]
const session = ref(null)
const error = ref('')
const busy = ref(false)
const { job, watch: watchJob, clear: clearJob } = useJob(onJobDone)

const bytes = (n) => formatBytesL(n, locale.value)
const plan = computed(() => session.value?.plan || null)

// ---- step 1: the source ----
function addDump() {
  if (dumps.value.length < 10) dumps.value.push({ mode: 'path', path: '', url: '' })
}
const source = (s) => (s.mode === 'url' ? { url: s.url.trim() } : { path: s.path.trim() })
const canAnalyse = computed(() => (src.mode === 'url' ? src.url.trim() : src.path.trim()) && dumps.value.every((d) => (d.mode === 'url' ? d.url.trim() : d.path.trim())))

async function analyse() {
  error.value = ''
  busy.value = true
  try {
    const body = { kind: kind.value, archive: source(src) }
    if (kind.value === 'archive') body.sql = dumps.value.map(source)
    const res = await api('POST', 'api/import', body)
    session.value = { id: res.id, status: 'analysing' }
    step.value = 'analysing'
    watchJob(res.job)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function onJobDone() {
  await refresh()
}

async function refresh() {
  if (!session.value?.id) return
  try {
    const s = await api('GET', `api/import/${session.value.id}`)
    session.value = s
    if (s.status === 'failed') {
      error.value = s.error || t('errors.unknown')
      step.value = 'source'
    } else if (s.status === 'ready') {
      if (selFor.value !== s.id) initSelection()
      if (s.run_error) error.value = s.run_error
      step.value = 'plan'
    } else if (s.status === 'done') {
      step.value = 'done'
      invalidate('api/sites')
      invalidate('api/databases')
    }
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// ---- the file picker ----
const picker = reactive({ open: false, for: null, path: '/root', entries: [], error: '', loading: false })
const archiveRe = /\.(tar\.gz|tgz|tar|zip)$/i
const dumpRe = /\.(sql|sql\.gz)$/i
async function browse(target, start) {
  picker.for = target
  picker.open = true
  const p = (start || '').trim()
  await openDir(p && p.startsWith('/') ? p.replace(/\/[^/]*$/, '') || '/' : '/root')
}
async function openDir(p) {
  picker.loading = true
  picker.error = ''
  try {
    const res = await api('GET', `api/files?path=${encodeURIComponent(p)}`)
    picker.path = res.path
    const re = picker.for === src ? archiveRe : dumpRe
    picker.entries = list(res.entries)
      .filter((e) => e.type === 'dir' || (e.type === 'file' && re.test(e.name)))
      .sort((a, b) => (a.type === b.type ? a.name.localeCompare(b.name) : a.type === 'dir' ? -1 : 1))
  } catch (e) {
    picker.error = errorMessage(e)
    if (p !== '/') picker.path = p
  } finally {
    picker.loading = false
  }
}
const join = (dir, name) => (dir === '/' ? '/' : dir + '/') + name
function pick(e) {
  if (e.type === 'dir') return openDir(join(picker.path, e.name))
  picker.for.path = join(picker.path, e.name)
  picker.open = false
}
const parentDir = computed(() => (picker.path === '/' ? '' : picker.path.replace(/\/[^/]+$/, '') || '/'))

// ---- step 2: the plan ----
const sel = reactive({ domains: {}, dbs: {}, certs: {}, cron: {} })
const target = reactive({ mode: 'new', domain: '', www: true, site_id: 0, php: '' })
const sites = ref([])
const selFor = ref('') // the import the choices were made for

function initSelection() {
  const p = session.value?.plan
  if (!p) return
  selFor.value = session.value.id
  sel.domains = {}
  sel.dbs = {}
  sel.certs = {}
  sel.cron = {}
  for (const d of list(p.domains)) {
    sel.domains[d.name] = !d.blocked
    sel.certs[d.name] = !!(d.certificate?.usable && d.certificate?.trusted)
  }
  for (const d of list(p.databases)) sel.dbs[d.source || d.name] = !d.blocked && p.db_ready
  if (p.site) {
    target.domain = p.site.domain || ''
    target.php = p.site.php || ''
    target.www = !target.domain.startsWith('www.')
    loadCached('api/sites', { ttl: 10000 })
      .then((r) => (sites.value = list(r.sites).filter((s) => !s.proxy_port && !(s.app || '').startsWith('docker:') && !s.app)))
      .catch(() => {})
  }
}

const chosenDomains = computed(() => list(plan.value?.domains).filter((d) => sel.domains[d.name] && !d.blocked))
const chosenDBs = computed(() => list(plan.value?.databases).filter((d) => sel.dbs[d.source || d.name] && !d.blocked))
const need = computed(() => {
  const files = plan.value?.site ? plan.value.site.size : chosenDomains.value.reduce((n, d) => n + d.size, 0)
  return { files, dbs: chosenDBs.value.reduce((n, d) => n + d.size * 2, 0) }
})
const spaceShort = computed(() => {
  const p = plan.value
  if (!p || p.free < 0) return false
  if (p.same_disk || p.free_db < 0) return need.value.files + need.value.dbs > p.free
  return need.value.files > p.free || need.value.dbs > p.free_db
})
const nothing = computed(() => (plan.value?.kind === 'cpanel' ? !chosenDomains.value.length && !chosenDBs.value.length : false))
const phpChoices = computed(() => [...list(plan.value?.php)].reverse())
const staticOK = computed(() => plan.value?.site?.app === 'static')

function cronDomainChosen(c) {
  return !!sel.domains[c.domain]
}

async function run() {
  error.value = ''
  busy.value = true
  const p = plan.value
  const body = { domains: [], databases: chosenDBs.value.map((d) => d.source || d.name), certificates: [], cron: [] }
  if (p.kind === 'cpanel') {
    body.domains = chosenDomains.value.map((d) => d.name)
    body.certificates = body.domains.filter((n) => sel.certs[n])
    body.cron = list(p.cron).map((c, i) => (sel.cron[i] && c.ok && cronDomainChosen(c) ? i : -1)).filter((i) => i >= 0)
  } else if (target.mode === 'existing') {
    body.site_id = Number(target.site_id) || 0
  } else {
    body.domain = target.domain.trim()
    body.www = target.www
    body.php = target.php
  }
  try {
    const res = await api('POST', `api/import/${session.value.id}/run`, body)
    step.value = 'importing'
    watchJob(res.job)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function stop() {
  try {
    await api('POST', `api/import/${session.value.id}/stop`)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function startOver() {
  error.value = ''
  if (session.value?.id && !['analysing', 'importing'].includes(session.value.status)) {
    await api('DELETE', `api/import/${session.value.id}`).catch(() => {})
  }
  session.value = null
  clearJob()
  step.value = 'source'
}

// ---- texts ----
const appLabel = (a) => ({ wordpress: 'WordPress', php: 'PHP', static: t('siteimport.app_static') })[a] || a
function certText(c) {
  if (!c) return t('siteimport.cert_none')
  if (c.usable) return c.trusted ? t('siteimport.cert_ok', { date: new Date(c.not_after).toLocaleDateString(locale.value) }) : t('siteimport.cert_untrusted')
  if (c.problem === 'names') return t('siteimport.cert_names', { names: list(c.missing).join(', ') })
  return t(`siteimport.cert_${c.problem}`)
}
function skippedText(s) {
  const params = { n: s.count || 0, size: bytes(s.size || 0) }
  return t(`siteimport.skipped_${s.what}`, params, s.count || 0)
}
const pwText = (d) => t(`siteimport.pw_${d.password}`)
const siteURL = (d) => `http://${d}/`

onMounted(async () => {
  // Pick up an import that is still going, or waits for its choices.
  try {
    const { imports } = await api('GET', 'api/import')
    const s = list(imports).find((x) => ['analysing', 'ready', 'importing'].includes(x.status))
    if (!s) return
    session.value = s
    if (s.status === 'ready') return refresh()
    step.value = s.status
    const j = await api('GET', `api/jobs/${s.status === 'importing' ? s.import_job_id : s.job_id}`)
    watchJob(j)
    if (j.status !== 'running') refresh()
  } catch {
    // nothing to resume
  }
})
</script>

<template>
  <div class="imp stack">
    <h1 class="sr-only">{{ t('siteimport.title') }}</h1>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>

    <!-- 1. Source -->
    <section v-if="step === 'source'" class="card stack">
      <h2 class="imp-h">{{ t('siteimport.step_source') }}</h2>
      <div class="imp-kinds" role="radiogroup" :aria-label="t('siteimport.step_source')">
        <label class="imp-kind" :class="{ on: kind === 'cpanel' }">
          <input v-model="kind" type="radio" value="cpanel" class="sr-only" />
          <span class="imp-kind-title">{{ t('siteimport.kind_cpanel') }}</span>
          <span class="muted small">{{ t('siteimport.kind_cpanel_hint') }}</span>
        </label>
        <label class="imp-kind" :class="{ on: kind === 'archive' }">
          <input v-model="kind" type="radio" value="archive" class="sr-only" />
          <span class="imp-kind-title">{{ t('siteimport.kind_archive') }}</span>
          <span class="muted small">{{ t('siteimport.kind_archive_hint') }}</span>
        </label>
      </div>

      <div class="stack">
        <div class="seg" role="group" :aria-label="t('siteimport.from')">
          <button type="button" :class="{ on: src.mode === 'path' }" :aria-pressed="src.mode === 'path'" @click="src.mode = 'path'">{{ t('siteimport.from_server') }}</button>
          <button type="button" :class="{ on: src.mode === 'url' }" :aria-pressed="src.mode === 'url'" @click="src.mode = 'url'">{{ t('siteimport.from_url') }}</button>
        </div>
        <label v-if="src.mode === 'path'" class="field">
          <span>{{ kind === 'cpanel' ? t('siteimport.path_cpanel') : t('siteimport.path_archive') }}</span>
          <div class="row imp-path">
            <input v-model="src.path" class="input mono grow" :placeholder="kind === 'cpanel' ? '/root/cpmove-user.tar.gz' : '/root/web_example.com.tar.gz'" spellcheck="false" autocapitalize="off" autocomplete="off" />
            <button type="button" class="btn" @click="browse(src, src.path)"><Icon name="folder" :size="15" />{{ t('siteimport.browse') }}</button>
          </div>
          <small class="muted">{{ t('siteimport.path_hint') }}</small>
        </label>
        <label v-else class="field">
          <span>{{ t('siteimport.url') }}</span>
          <input v-model="src.url" class="input mono" type="url" placeholder="https://old.example.com/backup.tar.gz" spellcheck="false" autocapitalize="off" autocomplete="off" />
          <small class="muted">{{ t('siteimport.url_hint') }}</small>
        </label>
      </div>

      <div v-if="kind === 'archive'" class="stack">
        <h3 class="imp-h3">{{ t('siteimport.dumps') }}</h3>
        <p class="muted small">{{ t('siteimport.dumps_hint') }}</p>
        <div v-for="(d, i) in dumps" :key="i" class="row imp-path">
          <select v-model="d.mode" class="input input-auto" :aria-label="t('siteimport.from')">
            <option value="path">{{ t('siteimport.from_server') }}</option>
            <option value="url">{{ t('siteimport.from_url') }}</option>
          </select>
          <input v-if="d.mode === 'path'" v-model="d.path" class="input mono grow" placeholder="/root/db_shop.sql.gz" spellcheck="false" :aria-label="t('siteimport.dump_path')" />
          <input v-else v-model="d.url" class="input mono grow" type="url" placeholder="https://…/db.sql.gz" spellcheck="false" :aria-label="t('siteimport.url')" />
          <button v-if="d.mode === 'path'" type="button" class="btn" @click="browse(d, d.path)"><Icon name="folder" :size="15" />{{ t('siteimport.browse') }}</button>
          <button type="button" class="icon-btn" :aria-label="t('siteimport.dump_remove')" @click="dumps.splice(i, 1)"><Icon name="x" /></button>
        </div>
        <div><button v-if="dumps.length < 10" type="button" class="btn btn-sm" @click="addDump"><Icon name="plus" :size="14" />{{ t('siteimport.dump_add') }}</button></div>
      </div>

      <ul class="muted small imp-notes">
        <li>{{ t('siteimport.note_read_only') }}</li>
        <li>{{ kind === 'cpanel' ? t('siteimport.note_cpanel_skip') : t('siteimport.note_archive') }}</li>
      </ul>
      <div class="row">
        <button class="btn btn-primary" :disabled="busy || !canAnalyse" @click="analyse"><Icon name="search" :size="15" />{{ t('siteimport.analyse') }}</button>
      </div>
    </section>

    <!-- Jobs -->
    <template v-if="job && (step === 'analysing' || step === 'importing' || (job.status === 'failed' && step !== 'done'))">
      <JobLog :job="job" @close="refresh" />
      <div v-if="job.status === 'running'" class="row">
        <button type="button" class="btn" @click="stop"><Icon name="stop" :size="15" />{{ step === 'importing' ? t('siteimport.stop_import') : t('siteimport.stop_analysis') }}</button>
        <span class="muted small">{{ step === 'importing' ? t('siteimport.stop_import_hint') : '' }}</span>
      </div>
    </template>

    <!-- 2. Plan: cPanel -->
    <template v-if="step === 'plan' && plan && plan.kind === 'cpanel'">
      <section class="card">
        <div class="card-head">
          <h2 class="imp-h">{{ t('siteimport.plan_title', { name: plan.archive.name }) }}</h2>
          <span class="muted small">{{ bytes(plan.archive.size) }}<template v-if="plan.user"> · {{ t('siteimport.cp_user', { user: plan.user }) }}</template></span>
        </div>
        <p class="muted small">{{ t('siteimport.plan_body') }}</p>
        <p v-for="n in plan.notes" :key="n.code" class="alert alert-info">{{ t(`siteimport.note_${n.code}`, { detail: n.detail || '' }) }}</p>
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th class="col-check"><span class="sr-only">{{ t('siteimport.col_import') }}</span></th>
                <th>{{ t('siteimport.col_website') }}</th>
                <th>{{ t('siteimport.col_php') }}</th>
                <th class="num">{{ t('siteimport.col_size') }}</th>
                <th>{{ t('siteimport.col_cert') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="d in plan.domains" :key="d.name" :class="{ 'imp-off': d.blocked }">
                <td class="col-check"><input v-model="sel.domains[d.name]" type="checkbox" :disabled="!!d.blocked" :aria-label="d.name" /></td>
                <td>
                  <div class="imp-name"><strong class="mono">{{ d.name }}</strong> <span class="tag">{{ t(`siteimport.type_${d.type}`) }}</span> <span class="muted small">{{ appLabel(d.app) }}</span></div>
                  <div v-if="d.aliases.length" class="muted small">{{ t('siteimport.aliases', { names: d.aliases.join(', ') }) }}</div>
                  <div class="muted small mono truncate" :title="d.docroot">{{ d.docroot }}</div>
                  <div v-if="d.blocked" class="small text-crit">{{ t(`siteimport.blocked_${d.blocked}`) }}</div>
                  <div v-if="d.nested?.length" class="small muted">{{ t('siteimport.nested', { names: d.nested.join(', ') }) }}</div>
                  <div v-if="d.taken_aliases?.length" class="small text-warn">{{ t('siteimport.taken_aliases', { names: d.taken_aliases.join(', ') }) }}</div>
                </td>
                <td class="nowrap">
                  <template v-if="d.php">PHP {{ d.php }}<span v-if="d.php_old && d.php_old !== d.php" class="muted small"> ({{ t('siteimport.php_was', { v: d.php_old }) }})</span></template>
                  <span v-else class="muted">{{ t('siteimport.app_static') }}</span>
                </td>
                <td class="num nowrap">{{ bytes(d.size) }}<div class="muted small">{{ t('siteimport.files', { n: formatNumber(d.files, locale) }, d.files) }}</div></td>
                <td>
                  <label v-if="d.certificate?.usable" class="check small"><input v-model="sel.certs[d.name]" type="checkbox" :disabled="!sel.domains[d.name]" /><span>{{ certText(d.certificate) }}</span></label>
                  <span v-else class="muted small">{{ certText(d.certificate) }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="plan.databases.length" class="card">
        <div class="card-head"><h2 class="imp-h">{{ t('siteimport.dbs_title') }}</h2></div>
        <p v-if="!plan.db_ready" class="alert alert-warn">{{ t('siteimport.db_not_running') }} <RouterLink :to="{ name: 'databases' }">{{ t('nav.databases') }}</RouterLink></p>
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th class="col-check"><span class="sr-only">{{ t('siteimport.col_import') }}</span></th>
                <th>{{ t('siteimport.col_db') }}</th>
                <th>{{ t('siteimport.col_user') }}</th>
                <th class="num">{{ t('siteimport.col_size') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="d in plan.databases" :key="d.name" :class="{ 'imp-off': d.blocked }">
                <td class="col-check"><input v-model="sel.dbs[d.name]" type="checkbox" :disabled="!!d.blocked || !plan.db_ready" :aria-label="d.name" /></td>
                <td>
                  <strong class="mono">{{ d.new_name }}</strong>
                  <span v-if="d.new_name !== d.name" class="muted small"> {{ t('siteimport.renamed', { name: d.name }) }}</span>
                  <div v-if="d.used_by.length" class="muted small">{{ t('siteimport.used_by', { names: d.used_by.join(', ') }) }}</div>
                  <div v-if="d.config_change && d.used_by.length" class="small muted">{{ t('siteimport.config_rewritten') }}</div>
                  <div v-else-if="d.config_change" class="small text-warn">{{ t('siteimport.config_manual') }}</div>
                </td>
                <td>
                  <span class="mono">{{ d.user }}</span>
                  <div class="muted small">{{ pwText(d) }}</div>
                  <div v-if="d.users.filter((u) => u !== d.user).length" class="muted small">{{ t('siteimport.other_users', { names: d.users.filter((u) => u !== d.user).join(', ') }) }}</div>
                </td>
                <td class="num nowrap">{{ bytes(d.size) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="plan.cron.length" class="card stack">
        <h2 class="imp-h">{{ t('siteimport.cron_title') }}</h2>
        <p class="muted small">{{ t('siteimport.cron_body') }}</p>
        <label v-for="(c, i) in plan.cron" :key="i" class="check imp-cron" :class="{ 'imp-off': !c.ok }">
          <input v-model="sel.cron[i]" type="checkbox" :disabled="!c.ok || !cronDomainChosen(c)" />
          <span class="stack-tight">
            <code class="mono">{{ c.schedule }} {{ c.new_command || c.command }}</code>
            <span class="muted small">{{ c.domain ? t('siteimport.cron_as', { domain: c.domain }) : '' }}<template v-if="c.notes.includes('old_home')"> · <span class="text-warn">{{ t('siteimport.cron_old_home') }}</span></template><template v-if="!c.ok"> · {{ t('siteimport.cron_invalid') }}</template></span>
          </span>
        </label>
      </section>

      <section class="card stack">
        <h2 class="imp-h">{{ t('siteimport.skipped_title') }}</h2>
        <ul class="imp-notes">
          <li v-for="s in plan.skipped" :key="s.what">{{ skippedText(s) }}</li>
          <li>{{ t('siteimport.skipped_always') }}</li>
        </ul>
      </section>
    </template>

    <!-- 2. Plan: site archive -->
    <template v-if="step === 'plan' && plan && plan.kind === 'archive'">
      <section class="card stack">
        <div class="card-head">
          <h2 class="imp-h">{{ t('siteimport.plan_title', { name: plan.archive.name }) }}</h2>
          <span class="muted small">{{ bytes(plan.archive.size) }}</span>
        </div>
        <p>{{ t('siteimport.archive_found', { files: formatNumber(plan.site.files, locale), size: bytes(plan.site.size), app: appLabel(plan.site.app) }) }}</p>
        <p v-if="plan.site.root" class="muted small">{{ t('siteimport.archive_root', { root: plan.site.root }) }}</p>
        <p v-if="plan.site.old_root" class="muted small">{{ t('siteimport.archive_old_root', { root: plan.site.old_root }) }}</p>
        <p v-for="n in plan.notes" :key="n.code" class="alert alert-warn">{{ t(`siteimport.note_${n.code}`, { detail: n.detail || '' }) }}</p>

        <div class="seg" role="group" :aria-label="t('siteimport.target')">
          <button type="button" :class="{ on: target.mode === 'new' }" :aria-pressed="target.mode === 'new'" @click="target.mode = 'new'">{{ t('siteimport.target_new') }}</button>
          <button type="button" :class="{ on: target.mode === 'existing' }" :aria-pressed="target.mode === 'existing'" @click="target.mode = 'existing'">{{ t('siteimport.target_existing') }}</button>
        </div>
        <div v-if="target.mode === 'new'" class="grid-2">
          <label class="field">
            <span>{{ t('websites.domain') }}</span>
            <input v-model="target.domain" class="input mono" placeholder="example.com" spellcheck="false" autocapitalize="off" autocomplete="off" maxlength="253" />
            <label class="check small"><input v-model="target.www" type="checkbox" /><span>{{ t('websites.add_www', { name: `www.${target.domain.trim() || 'example.com'}` }) }}</span></label>
          </label>
          <label class="field">
            <span>{{ t('siteimport.col_php') }}</span>
            <select v-model="target.php" class="input">
              <option v-for="v in phpChoices" :key="v" :value="v">PHP {{ v }}</option>
              <option v-if="staticOK" value="">{{ t('websites.type_static') }}</option>
            </select>
          </label>
        </div>
        <label v-else class="field">
          <span>{{ t('siteimport.target_site') }}</span>
          <select v-model="target.site_id" class="input">
            <option :value="0" disabled>{{ t('siteimport.target_pick') }}</option>
            <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.domain }}{{ s.php_version ? ' · PHP ' + s.php_version : '' }}</option>
          </select>
          <small class="muted">{{ t('siteimport.target_existing_hint') }}</small>
        </label>
      </section>
      <section v-if="plan.databases.length" class="card stack">
        <h2 class="imp-h">{{ t('siteimport.dbs_title') }}</h2>
        <p v-if="!plan.db_ready" class="alert alert-warn">{{ t('siteimport.db_not_running') }} <RouterLink :to="{ name: 'databases' }">{{ t('nav.databases') }}</RouterLink></p>
        <label v-for="d in plan.databases" :key="d.source" class="check">
          <input v-model="sel.dbs[d.source]" type="checkbox" :disabled="!!d.blocked || !plan.db_ready" />
          <span>
            <span class="mono">{{ d.source.split('/').pop() }}</span> → <strong class="mono">{{ d.new_name }}</strong>
            <span class="muted small"> · {{ bytes(d.size) }}</span>
            <span v-if="d.used_by.length" class="muted small"> · {{ t('siteimport.config_rewritten') }}</span>
          </span>
        </label>
      </section>
    </template>

    <template v-if="step === 'plan' && plan">
      <section class="card imp-confirm">
        <p class="small" :class="spaceShort ? 'text-crit' : 'muted'">
          {{ t('siteimport.space', { need: bytes(need.files + need.dbs), free: plan.free >= 0 ? bytes(plan.free) : '?' }) }}
          <template v-if="spaceShort"> · {{ t('siteimport.space_short') }}</template>
        </p>
        <div class="row">
          <button type="button" class="btn btn-ghost" @click="startOver">{{ t('siteimport.start_over') }}</button>
          <button class="btn btn-primary" :disabled="busy || nothing || spaceShort || (plan.kind === 'archive' && (target.mode === 'new' ? !target.domain.trim() : !target.site_id))" @click="run">
            <Icon name="download" :size="15" />
            <template v-if="plan.kind === 'cpanel'">{{ t('siteimport.run_cpanel', { what: [t('siteimport.n_sites', { n: chosenDomains.length }, chosenDomains.length), t('siteimport.n_dbs', { n: chosenDBs.length }, chosenDBs.length)].join(', ') }) }}</template>
            <template v-else>{{ t('siteimport.run_archive') }}</template>
          </button>
        </div>
        <p class="muted small">{{ t('siteimport.run_hint') }}</p>
      </section>
    </template>

    <!-- 3. Done -->
    <template v-if="step === 'done' && session?.result">
      <section class="card stack">
        <h2 class="imp-h"><Icon name="checkCircle" :size="18" class="text-ok" /> {{ t('siteimport.done_title') }}</h2>
        <ul class="list">
          <li v-for="s in session.result.sites" :key="s.id" class="list-row">
            <div class="list-main">
              <div class="list-title mono">{{ s.domain }}</div>
              <div class="list-meta muted small">{{ appLabel(s.app) }}</div>
            </div>
            <div class="list-side row">
              <a class="btn btn-sm" :href="siteURL(s.domain)" target="_blank" rel="noopener noreferrer"><Icon name="external" :size="14" />{{ t('websites.open_site') }}</a>
              <RouterLink class="btn btn-sm" :to="{ name: 'websites', query: { site: String(s.id) } }">{{ t('siteimport.manage') }}</RouterLink>
            </div>
          </li>
        </ul>
        <div v-if="session.result.databases.length" class="stack-tight">
          <h3 class="imp-h3">{{ t('siteimport.dbs_title') }}</h3>
          <p v-for="d in session.result.databases" :key="d.id" class="small">
            <strong class="mono">{{ d.name }}</strong> · <span class="mono">{{ d.user }}</span> · {{ pwText(d) }}
            <span v-if="d.manual" class="text-warn"> · {{ t('siteimport.config_manual') }}</span>
          </p>
        </div>
        <div v-if="session.result.warnings.length" class="alert alert-warn">
          <p>{{ t('siteimport.warnings') }}</p>
          <ul><li v-for="(w, i) in session.result.warnings" :key="i" class="pre-wrap">{{ w }}</li></ul>
        </div>
        <div>
          <h3 class="imp-h3">{{ t('siteimport.next_title') }}</h3>
          <ul class="imp-notes">
            <li>{{ t('siteimport.next_dns') }}</li>
            <li>{{ t('siteimport.next_https') }}</li>
            <li>{{ t('siteimport.next_php') }}</li>
            <li>{{ t('siteimport.next_mail') }}</li>
          </ul>
        </div>
        <details v-if="job" class="small"><summary>{{ t('siteimport.show_log') }}</summary><JobLog :job="job" embedded /></details>
        <div class="row"><button type="button" class="btn" @click="startOver">{{ t('siteimport.another') }}</button></div>
      </section>
    </template>

    <Modal v-if="picker.open" :title="t('siteimport.picker_title')" wide @close="picker.open = false">
      <div class="stack">
        <div class="row imp-path">
          <button type="button" class="btn btn-sm" :disabled="!parentDir" @click="openDir(parentDir)"><Icon name="up" :size="14" />{{ t('siteimport.picker_up') }}</button>
          <code class="mono truncate grow">{{ picker.path }}</code>
        </div>
        <p v-if="picker.error" class="alert alert-crit">{{ picker.error }}</p>
        <p v-if="picker.loading" class="muted"><span class="spinner" aria-hidden="true"></span></p>
        <ul v-else class="imp-picker">
          <li v-for="e in picker.entries" :key="e.name">
            <button type="button" class="imp-entry" @click="pick(e)">
              <Icon :name="e.type === 'dir' ? 'folder' : 'archive'" :size="16" />
              <span class="truncate grow">{{ e.name }}</span>
              <span v-if="e.type === 'file'" class="muted small">{{ bytes(e.size) }}</span>
            </button>
          </li>
          <li v-if="!picker.entries.length" class="muted small">{{ t('siteimport.picker_empty') }}</li>
        </ul>
      </div>
    </Modal>
  </div>
</template>

<style scoped>
.imp { max-width: 1080px; }
.imp-h { display: flex; align-items: center; gap: 8px; margin: 0; font-size: 16px; }
.imp-h3 { margin: 0; font-size: 14px; }
.imp-kinds { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 12px; }
.imp-kind {
  display: flex; flex-direction: column; gap: 4px; padding: 14px 16px; border: 1px solid var(--border);
  border-radius: 12px; background: var(--surface); cursor: pointer;
}
.imp-kind.on { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-ring); }
.imp-kind:has(input:focus-visible) { outline: 2px solid var(--accent); outline-offset: 2px; }
.imp-kind-title { font-weight: 700; }
.imp-path { gap: 8px; flex-wrap: nowrap; align-items: center; }
.imp-notes { margin: 0; padding-left: 18px; }
.imp-notes li { margin: 3px 0; }
.imp-off { opacity: 0.6; }
.imp-name { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.imp-cron { align-items: flex-start; }
.imp-cron code { word-break: break-all; }
.imp-confirm .row { justify-content: flex-end; gap: 8px; }
.imp-picker { list-style: none; margin: 0; padding: 0; max-height: 55vh; overflow-y: auto; border: 1px solid var(--border); border-radius: 10px; }
.imp-picker li + li { border-top: 1px solid var(--border); }
.imp-entry { display: flex; align-items: center; gap: 10px; width: 100%; padding: 9px 12px; border: 0; background: none; color: var(--text); font: inherit; text-align: left; cursor: pointer; }
.imp-entry:hover, .imp-entry:focus-visible { background: var(--hover); }
@media (max-width: 640px) {
  .imp-path { flex-wrap: wrap; }
}
</style>
