<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { useJob } from '../composables/useJob.js'
import { refreshFeatures } from '../features.js'
import { usePageMeta } from '../page.js'
import { formatBytesL } from '../format.js'
import { SERVER_NAMES } from '../webservers.js'
import Icon from '../components/Icon.vue'
import AppLogo from '../components/AppLogo.vue'
import JobLog from '../components/JobLog.vue'

// The first setup of a server, on one page: a web server, PHP versions, a
// database server and extras, installed one after another as one job
// (GET/POST /api/setup). Parts that are installed already are skipped, so
// after a failure the same choice can simply be installed again.
const { t, te, locale } = useI18n()
const router = useRouter()

const view = ref(null)
const loaded = ref(false)
const error = ref('')
const web = ref('')
const php = ref([])
const db = ref('')
const extras = ref([])
const plan = ref(null) // the steps of the job started here
const outcome = ref('') // done | failed, once the job has finished
let initialized = false

const { job, running, watch: watchJob, resume, clear } = useJob(async (j) => {
  outcome.value = j.status === 'succeeded' ? 'done' : 'failed'
  await Promise.all([load(), refreshFeatures(true)])
})

async function load() {
  try {
    const v = await api('GET', 'api/setup')
    view.value = v
    // The choice starts as the server's recommendation, and then stays the
    // administrator's: a retry installs the same choice again.
    if (!initialized) {
      initialized = true
      const d = v.defaults || {}
      web.value = d.web || ''
      php.value = [...(d.php || [])]
      db.value = d.db || ''
      extras.value = [...(d.extras || [])]
    }
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loaded.value = true
  }
}

onMounted(async () => {
  await load()
  resume(['setup'])
})

usePageMeta(() => ({ title: t('setup.title') }))

const parts = computed(() => view.value?.parts || [])
const byId = computed(() => Object.fromEntries(parts.value.map((p) => [p.id, p])))
const group = (g) => parts.value.filter((p) => p.group === g)
// Once a web server is installed, the others can't be: show only it.
const webParts = computed(() => {
  const all = group('web')
  const installed = all.find((p) => p.installed)
  return installed ? [installed] : all
})
const phpParts = computed(() => group('php'))
const dbParts = computed(() => group('db'))
const extraGroups = computed(() =>
  ['data', 'runtimes', 'security'].map((id) => ({ id, items: group(id) })).filter((g) => g.items.length),
)

const managedWeb = computed(() => view.value?.web_server || '')
const distro = computed(() => view.value?.distro_php || '')
// A web server that will be installed brings the distribution's PHP.
const webComing = computed(() => !!web.value && !byId.value[web.value]?.installed)
const hasWeb = computed(() => !!web.value || !!managedWeb.value)

// Names of parts the Software page may not have a translation for yet.
const NAMES = { mariadb: 'MariaDB', mysql: 'MySQL', postgresql: 'PostgreSQL', redis: 'Redis', memcached: 'Memcached', nodejs: 'Node.js', python: 'Python', docker: 'Docker', adminer: 'Adminer', clamav: 'ClamAV' }
const nameOf = (id) => {
  if (SERVER_NAMES[id]) return SERVER_NAMES[id]
  if (id.startsWith('php')) return 'PHP ' + id.slice(3)
  if (te(`software.${id}`)) return t(`software.${id}`)
  return NAMES[id] || plan.value?.steps?.find((s) => s.id === id)?.name || id
}
const reason = (p) => {
  const b = p.blocked
  if (!b) return ''
  return te(`errors.${b.code}`) ? t(`errors.${b.code}`, { detail: b.detail || '' }) : b.code
}
const disabled = (p) => running.value || p.installed || (!!p.blocked && !p.installed) || (p.needs_web && !hasWeb.value)
const phpFixed = (p) => p.installed || (webComing.value && p.version === distro.value)
const phpOn = (p) => phpFixed(p) || php.value.includes(p.version)

function togglePHP(p) {
  if (phpFixed(p) || running.value) return
  php.value = php.value.includes(p.version) ? php.value.filter((v) => v !== p.version) : [...php.value, p.version]
}
function toggleExtra(p) {
  if (disabled(p)) return
  extras.value = extras.value.includes(p.id) ? extras.value.filter((x) => x !== p.id) : [...extras.value, p.id]
}

// The parts of the choice, as the plan counts them.
const chosen = computed(() => {
  const out = []
  const add = (id) => byId.value[id] && out.push(byId.value[id])
  if (web.value) add(web.value)
  for (const v of php.value) if (!(webComing.value && v === distro.value)) add('php' + v)
  if (db.value) add(db.value)
  for (const id of extras.value) if (byId.value[id] && (!byId.value[id].needs_web || hasWeb.value)) add(id)
  return out
})
const toInstall = computed(() => chosen.value.filter((p) => !p.installed))
const ramSum = computed(() => toInstall.value.reduce((a, p) => a + (p.ram_mb || 0), 0))
const diskSum = computed(() => toInstall.value.reduce((a, p) => a + (p.disk_mb || 0), 0))
const ramWarn = computed(() => view.value?.ram_budget_mb != null && ramSum.value > view.value.ram_budget_mb)
const diskWarn = computed(() => view.value?.disk_budget_mb != null && diskSum.value > view.value.disk_budget_mb)
const thirdParty = computed(() => toInstall.value.filter((p) => p.group === 'php' && p.third_party).map((p) => p.version))

const mb = (n) => formatBytesL(Math.max(n, 0) * 1048576, locale.value)
const cost = (p) => {
  const ram = !p.ram_mb ? t('setup.ram_idle') : p.ram_max ? t('setup.ram_max', { mb: p.ram_mb }) : t('setup.ram', { mb: p.ram_mb })
  return `${ram} · ${t('setup.disk', { size: mb(p.disk_mb || 0) })}`
}

const serverLine = computed(() => {
  const s = view.value?.server
  if (!s || !s.mem_total_mb) return ''
  return t('setup.server_line', {
    cpus: t('setup.cpus', { n: s.cpus }, s.cpus),
    ram: mb(s.mem_total_mb),
    used: mb(s.mem_used_mb),
    disk: mb(s.disk_free_mb),
  })
})

async function install() {
  error.value = ''
  outcome.value = ''
  const body = {
    web: web.value,
    php: php.value,
    db: db.value,
    extras: extras.value.filter((id) => byId.value[id] && (!byId.value[id].needs_web || hasWeb.value)),
  }
  try {
    const res = await api('POST', 'api/setup', body)
    plan.value = res.plan || null
    watchJob(res)
    nextTick(() => document.getElementById('setup-job')?.scrollIntoView({ behavior: 'smooth', block: 'nearest' }))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function skip() {
  error.value = ''
  try {
    await api('POST', 'api/setup/skip', {})
    await refreshFeatures(true)
    router.push({ name: 'dashboard' })
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// After the job: what is installed now, and where it stopped.
const steps = computed(() => (plan.value?.steps || []).map((s) => ({ ...s, now: !!byId.value[s.id]?.installed })))
const stoppedAt = computed(() => steps.value.find((s) => !s.now))
const doneList = computed(() => steps.value.filter((s) => s.now).map((s) => nameOf(s.id)).join(', '))

function closeJob() {
  clear()
  outcome.value = ''
}
</script>

<template>
  <div class="pg setup">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('setup.title') }}</h1></div>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>

    <template v-if="!loaded">
      <span class="skel" style="height: 120px; border-radius: 14px"></span>
      <span class="skel" style="height: 260px; border-radius: 14px"></span>
    </template>

    <div v-else-if="view && !view.supported" class="card empty">
      <div class="empty-icon"><Icon name="server" :size="26" /></div>
      <p class="empty-lead">{{ t('websites.unsupported') }}</p>
    </div>

    <template v-else-if="view">
      <p class="muted setup-lead">{{ t('setup.lead') }}</p>

      <!-- web server -->
      <section class="panel" aria-labelledby="setup-web">
        <div class="panel-head">
          <h2 id="setup-web" class="panel-title"><Icon name="globe" :size="17" />{{ t('setup.web') }}</h2>
          <span class="muted small">{{ t('setup.web_hint') }}</span>
        </div>
        <div class="panel-body opt-grid" role="radiogroup" :aria-labelledby="'setup-web'">
          <label v-for="p in webParts" :key="p.id" class="opt" :class="{ on: web === p.id, off: running || (!!p.blocked && !p.installed) }">
            <input v-model="web" type="radio" name="setup-web" :value="p.id" :disabled="running || (!!p.blocked && !p.installed)" />
            <AppLogo :name="p.id" size="sm" icon="server" />
            <span class="opt-text">
              <span class="opt-name">
                {{ nameOf(p.id) }}
                <span v-if="p.installed" class="tag tag-ok">{{ t('software.installed') }}</span>
                <span v-else-if="p.id === 'nginx'" class="tag tag-info">{{ t('setup.recommended') }}</span>
              </span>
              <span class="muted small">{{ t(`setup.${p.id}`) }}</span>
              <span v-if="p.blocked && !p.installed" class="opt-why small">{{ reason(p) }}</span>
              <span v-else-if="!p.installed" class="opt-cost"><Icon name="memory" :size="13" />{{ cost(p) }}</span>
            </span>
          </label>
          <label v-if="!managedWeb" class="opt" :class="{ on: web === '', off: running }">
            <input v-model="web" type="radio" name="setup-web" value="" :disabled="running" />
            <span class="logo-tile sm opt-none"><Icon name="x" :size="16" /></span>
            <span class="opt-text">
              <span class="opt-name">{{ t('setup.none') }}</span>
              <span class="muted small">{{ t('setup.web_none') }}</span>
            </span>
          </label>
        </div>
      </section>

      <!-- PHP -->
      <section v-if="phpParts.length" class="panel" aria-labelledby="setup-php">
        <div class="panel-head">
          <h2 id="setup-php" class="panel-title"><AppLogo name="php" size="xs" />{{ t('setup.php') }}</h2>
          <span class="muted small">{{ t('setup.php_hint', { mb: phpParts[0].ram_mb }) }}</span>
        </div>
        <div class="panel-body stack-tight">
          <div class="php-grid" role="group" :aria-labelledby="'setup-php'">
            <label v-for="p in phpParts" :key="p.id" class="opt opt-php" :class="{ on: phpOn(p), off: running || phpFixed(p) }">
              <input type="checkbox" :checked="phpOn(p)" :disabled="running || phpFixed(p)" @change="togglePHP(p)" />
              <span class="opt-text">
                <span class="opt-name">PHP {{ p.version }}</span>
                <span v-if="p.installed" class="small muted">{{ t('software.installed') }}</span>
                <span v-else-if="webComing && p.version === distro" class="small muted">{{ t('setup.php_with_web', { server: nameOf(web) }) }}</span>
                <span v-else-if="p.eol" class="small eol">{{ t('setup.eol') }}</span>
              </span>
            </label>
          </div>
          <p v-if="thirdParty.length && view.php_repo" class="muted small setup-note">
            <Icon name="info" :size="14" />{{ t('setup.php_repo', { versions: thirdParty.join(', '), repo: view.php_repo }, thirdParty.length) }}
          </p>
        </div>
      </section>

      <!-- database server -->
      <section v-if="dbParts.length" class="panel" aria-labelledby="setup-db">
        <div class="panel-head">
          <h2 id="setup-db" class="panel-title"><Icon name="database" :size="17" />{{ t('setup.db') }}</h2>
          <span class="muted small">{{ t('setup.db_hint') }}</span>
        </div>
        <div class="panel-body opt-grid" role="radiogroup" :aria-labelledby="'setup-db'">
          <label v-for="p in dbParts" :key="p.id" class="opt" :class="{ on: db === p.id, off: running || (!!p.blocked && !p.installed) }">
            <input v-model="db" type="radio" name="setup-db" :value="p.id" :disabled="running || (!!p.blocked && !p.installed)" />
            <AppLogo :name="p.id" size="sm" icon="database" />
            <span class="opt-text">
              <span class="opt-name">
                {{ nameOf(p.id) }}
                <span v-if="p.installed" class="tag tag-ok">{{ t('software.installed') }}</span>
                <span v-else-if="p.id === 'mariadb'" class="tag tag-info">{{ t('setup.recommended') }}</span>
              </span>
              <span class="muted small">{{ te(`setup.${p.id}`) ? t(`setup.${p.id}`) : '' }}</span>
              <span v-if="p.blocked && !p.installed" class="opt-why small">{{ reason(p) }}</span>
              <span v-else-if="!p.installed" class="opt-cost"><Icon name="memory" :size="13" />{{ cost(p) }}</span>
            </span>
          </label>
          <label v-if="!dbParts.some((p) => p.installed)" class="opt" :class="{ on: db === '', off: running }">
            <input v-model="db" type="radio" name="setup-db" value="" :disabled="running" />
            <span class="logo-tile sm opt-none"><Icon name="x" :size="16" /></span>
            <span class="opt-text">
              <span class="opt-name">{{ t('setup.none') }}</span>
              <span class="muted small">{{ t('setup.db_none') }}</span>
            </span>
          </label>
        </div>
      </section>

      <!-- extras -->
      <section v-if="extraGroups.length" class="panel" aria-labelledby="setup-extras">
        <div class="panel-head">
          <h2 id="setup-extras" class="panel-title"><Icon name="layers" :size="17" />{{ t('setup.extras') }}</h2>
          <span class="muted small">{{ t('setup.extras_hint') }}</span>
        </div>
        <div class="panel-body stack">
          <div v-for="g in extraGroups" :key="g.id" class="stack-tight">
            <h3 class="setup-sub">{{ t(`setup.group_${g.id}`) }}</h3>
            <div class="opt-grid">
              <label v-for="p in g.items" :key="p.id" class="opt" :class="{ on: p.installed || (extras.includes(p.id) && !disabled(p)), off: disabled(p) }">
                <input type="checkbox" :checked="p.installed || (extras.includes(p.id) && !disabled(p))" :disabled="disabled(p)" @change="toggleExtra(p)" />
                <AppLogo :name="p.id" size="sm" :icon="p.group === 'security' ? 'shield' : p.group === 'runtimes' ? 'code' : 'database'" />
                <span class="opt-text">
                  <span class="opt-name">
                    {{ nameOf(p.id) }}
                    <span v-if="p.installed" class="tag tag-ok">{{ t('software.installed') }}</span>
                  </span>
                  <span class="muted small">{{ te(`setup.${p.id}`) ? t(`setup.${p.id}`) : '' }}</span>
                  <span v-if="p.blocked && !p.installed" class="opt-why small">{{ reason(p) }}</span>
                  <span v-else-if="p.needs_web && !hasWeb && !p.installed" class="opt-why small">{{ t('setup.needs_web') }}</span>
                  <span v-else-if="!p.installed" class="opt-cost"><Icon name="memory" :size="13" />{{ cost(p) }}</span>
                </span>
              </label>
            </div>
          </div>
        </div>
      </section>

      <JobLog v-if="job" id="setup-job" :job="job" @close="closeJob" />

      <p v-if="outcome === 'done'" class="alert alert-ok setup-outcome" role="status">
        <strong>{{ t('setup.done_title') }}.</strong>
        <template v-if="doneList"> {{ t('setup.done_body', { list: doneList }) }}</template>
        <span class="row">
          <RouterLink class="btn btn-sm btn-primary" :to="{ name: 'websites' }"><Icon name="plus" :size="15" />{{ t('setup.add_site') }}</RouterLink>
          <RouterLink class="btn btn-sm" :to="{ name: 'dashboard' }">{{ t('setup.dashboard') }}</RouterLink>
        </span>
      </p>
      <div v-else-if="outcome === 'failed'" class="alert alert-crit setup-outcome" role="alert">
        <template v-if="stoppedAt">
          <strong>{{ t('setup.failed_title', { step: nameOf(stoppedAt.id) }) }}.</strong>
          {{ doneList ? t('setup.failed_installed', { list: doneList }) : t('setup.failed_none') }}
          {{ t('setup.failed_retry') }}
        </template>
        <template v-else>{{ t('setup.failed_generic') }}</template>
      </div>

      <!-- summary and actions -->
      <section class="panel setup-foot">
        <div class="foot-text">
          <p v-if="serverLine" class="small muted foot-line"><Icon name="server" :size="14" />{{ serverLine }}</p>
          <p class="foot-line">
            <Icon name="memory" :size="15" />
            <template v-if="!chosen.length">{{ t('setup.nothing_chosen') }}</template>
            <template v-else-if="!toInstall.length">{{ t('setup.nothing_new') }}</template>
            <template v-else>{{ t('setup.choice_line', { ram: mb(ramSum), disk: mb(diskSum) }) }}</template>
          </p>
          <p v-if="ramWarn" class="foot-warn small"><Icon name="alert" :size="14" />{{ t('setup.warn_ram', { free: mb(view.ram_budget_mb) }) }}</p>
          <p v-if="diskWarn" class="foot-warn small"><Icon name="alert" :size="14" />{{ t('setup.warn_disk', { free: mb(view.server.disk_free_mb) }) }}</p>
        </div>
        <div class="foot-actions">
          <button v-if="view.state !== 'done'" type="button" class="btn btn-ghost" :disabled="running" :title="t('setup.skip_hint')" @click="skip">{{ t('setup.skip') }}</button>
          <button type="button" class="btn btn-primary" :disabled="running || !chosen.length" @click="install">
            <span v-if="running" class="spinner" aria-hidden="true"></span>
            <Icon v-else name="download" :size="15" />{{ toInstall.length || !chosen.length ? t('setup.install') : t('setup.finish') }}
          </button>
        </div>
      </section>
      <p v-if="view.state !== 'done'" class="muted small setup-skip-hint">{{ t('setup.skip_hint') }}</p>
    </template>
  </div>
</template>

<style scoped>
.setup-lead { margin: 0; max-width: 70ch; line-height: 1.55; }
.setup .panel-head { justify-content: flex-start; gap: 6px 14px; }
.setup .panel-head .small { flex: 1; min-width: 220px; }
.opt-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 10px; }
.php-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 8px; }
.opt {
  display: flex; align-items: flex-start; gap: 10px; min-width: 0; padding: 12px 14px; cursor: pointer;
  border: 1px solid var(--border); border-radius: 12px; background: var(--surface);
}
.opt:hover { border-color: var(--border-strong); }
.opt.on { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-ring); }
.opt.off { cursor: default; }
.opt.off:not(.on) { opacity: 0.62; }
.opt:has(input:focus-visible) { outline: 2px solid var(--accent); outline-offset: 2px; }
.opt input { flex: none; width: 16px; height: 16px; margin: 4px 0 0; accent-color: var(--accent); }
.opt-php { padding: 10px 12px; align-items: center; }
.opt-php input { margin: 0; }
.opt-text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.opt-name { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; font-weight: 700; }
.opt-cost { display: inline-flex; align-items: center; gap: 4px; margin-top: 2px; color: var(--muted); font-size: 12px; }
.opt-why { color: var(--warn); }
.opt-none { color: var(--muted); }
.eol { color: var(--warn); font-weight: 600; }
.setup-sub { margin: 0; font-size: 13px; font-weight: 700; color: var(--text-2); }
.setup-note { display: flex; align-items: flex-start; gap: 6px; margin: 4px 0 0; }
.setup-note .icon { flex: none; margin-top: 2px; }
.setup-outcome { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; margin: 0; }
.setup-outcome .row { margin-left: auto; }
.setup-foot { display: flex; align-items: center; justify-content: space-between; gap: 14px; flex-wrap: wrap; padding: 14px 18px; }
.foot-text { display: flex; flex-direction: column; gap: 6px; min-width: 0; flex: 1 1 340px; }
.foot-line { display: flex; align-items: flex-start; gap: 7px; margin: 0; }
.foot-line .icon, .foot-warn .icon { flex: none; margin-top: 2px; }
.foot-warn { display: flex; align-items: flex-start; gap: 7px; margin: 0; color: var(--warn); }
.foot-actions { display: flex; align-items: center; gap: 8px; }
.setup-skip-hint { margin: -6px 0 0; text-align: right; }
@media (max-width: 640px) {
  .opt-grid { grid-template-columns: minmax(0, 1fr); }
  .php-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .foot-actions { width: 100%; }
  .foot-actions .btn { flex: 1; justify-content: center; }
  .setup-skip-hint { text-align: left; }
}
</style>
