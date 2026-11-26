<script setup>
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount, defineAsyncComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { store, load as loadCached, put, list } from '../data.js'
import { features } from '../features.js'
import { daysLeft } from '../attention.js'
import { usePageMeta } from '../page.js'
import { formatNumber } from '../format.js'
import { appName, logoURL } from '../logos.js'
import { toast } from '../toast.js'
import JobLog from '../components/JobLog.vue'
import CopyField from '../components/CopyField.vue'
import Modal from '../components/Modal.vue'
import Icon from '../components/Icon.vue'
import AppLogo from '../components/AppLogo.vue'
import Sparkline from '../components/Sparkline.vue'
import { useJob } from '../composables/useJob.js'
import { SERVER_IDS, SERVER_NAMES, SERVER_RAM } from '../webservers.js'

// The drawer holds every per-site form: load it when first opened.
const SiteDrawer = defineAsyncComponent(() => import('../components/SiteDrawer.vue'))

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()

const status = ref(null)
const dbStatus = ref(null)
const sites = ref([])
const loading = ref(true)
const loadError = ref('')
const notice = ref('')
const actionError = ref('')
const warnings = ref([])

const { job, running: jobRunning, watch: watchJob, resume, clear: clearJob } = useJob((j) => {
  if (j.kind === 'backup') loadCached('api/backups', { force: true }).catch(() => {})
  return load()
})

async function load() {
  try {
    const [st, db, res] = await Promise.all([api('GET', 'api/web/status'), api('GET', 'api/db/status'), api('GET', 'api/sites')])
    status.value = st
    dbStatus.value = db
    sites.value = list(res.sites)
    put('api/sites', res)
    loadError.value = ''
    if (!form.value.php && !form.value.touchedType) form.value.php = latestPHP.value
  } catch (e) {
    loadError.value = errorMessage(e)
  } finally {
    loading.value = false
  }
  loadCached('api/backups', { ttl: 30000 }).catch(() => {})
  loadCached('api/databases', { ttl: 60000 }).catch(() => {})
  loadStats()
}

const phpList = computed(() => list(status.value?.php))
const latestPHP = computed(() => phpList.value.at(-1)?.version ?? '')

// ---- web server: Nginx, Apache or OpenLiteSpeed, chosen at install ----
const serverNames = SERVER_NAMES
// A backend from before the choice sends no "server": it runs Nginx.
const serverName = computed(() => serverNames[status.value?.server] || (status.value?.ready ? 'Nginx' : ''))
const web = computed(() => (status.value?.server && status.value[status.value.server]) || status.value?.nginx || {})
const installServer = ref('nginx')
const serverOptions = computed(() =>
  SERVER_IDS.filter((id) => id === 'nginx' || (status.value && id in status.value)).map((id) => ({
    id,
    name: serverNames[id],
    hint: t(`webserver.${id}_hint`),
    ram: SERVER_RAM[id],
  })),
)
watch(status, (st) => {
  const present = SERVER_IDS.filter((id) => st?.[id]?.installed)
  if (present.length === 1) installServer.value = present[0]
})

usePageMeta(() => ({
  sub: status.value?.ready
    ? [t('ws.count', { n: sites.value.length }, sites.value.length), [serverName.value, web.value.version].filter(Boolean).join(' '), phpList.value.length ? 'PHP ' + phpList.value.map((p) => p.version).join(', ') : ''].filter(Boolean).join(' · ')
    : serverName.value ? t('webserver.subtitle', { server: serverName.value }) : t('webserver.subtitle_none'),
}))

function clearMessages() {
  notice.value = ''
  actionError.value = ''
  warnings.value = []
}

async function startJob(method, path, body) {
  clearMessages()
  try {
    const res = await api(method, path, body)
    watchJob(res.job ?? res)
    return res
  } catch (e) {
    actionError.value = errorMessage(e)
    return null
  }
}

// ---- filters, sort, pages ----
const typeOf = (s) => {
  if ((s.app || '').startsWith('docker:')) return 'docker'
  if (s.proxy_port) return s.app_runtime === 'python' ? 'python' : 'node'
  if (s.php_version) return 'php'
  return 'static'
}
const kind = ref('all')
const q = ref('')
const sort = ref('name')
const page = ref(0)
const PAGE = 20
const kinds = computed(() => {
  const n = (k) => sites.value.filter((s) => typeOf(s) === k).length
  return [
    { id: 'all', label: t('ws.all'), n: sites.value.length },
    { id: 'php', label: 'PHP', logo: 'php', n: n('php') },
    { id: 'node', label: 'Node.js', logo: 'nodejs', n: n('node') },
    { id: 'python', label: 'Python', logo: 'python', n: n('python'), hide: !n('python') && !features.python },
    { id: 'docker', label: 'Docker', logo: 'docker', n: n('docker'), hide: !n('docker') && !features.docker },
    { id: 'static', label: t('ws.static'), icon: 'code', n: n('static') },
  ].filter((k) => !k.hide)
})
const fold = (s) => String(s || '').normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase()
const filtered = computed(() => {
  const f = fold(q.value.trim())
  let out = sites.value.filter((s) => (kind.value === 'all' || typeOf(s) === kind.value) && (!f || fold([s.domain, ...list(s.aliases), s.app, s.root, s.note].join(' ')).includes(f)))
  const by = {
    name: (a, b) => a.domain.localeCompare(b.domain),
    new: (a, b) => new Date(b.created_at) - new Date(a.created_at),
    traffic: (a, b) => (stats[b.id]?.total ?? -1) - (stats[a.id]?.total ?? -1),
    ssl: (a, b) => (a.ssl ? daysLeft(a.cert_expires) ?? 9999 : 99999) - (b.ssl ? daysLeft(b.cert_expires) ?? 9999 : 99999),
  }[sort.value]
  return [...out].sort(by)
})
const pages = computed(() => Math.max(1, Math.ceil(filtered.value.length / PAGE)))
const shownSites = computed(() => filtered.value.slice(page.value * PAGE, page.value * PAGE + PAGE))
watch([kind, q, sort], () => (page.value = 0))
watch(pages, (n) => {
  if (page.value >= n) page.value = n - 1
})

// ---- per-site traffic, from the access logs (7 days, two sites at a time) ----
// Up to 40 sites are all read (for the total and the sort); beyond that only
// the sites on the page, so a big server does not parse every log at once.
const stats = reactive({}) // id -> { loading, total, daily: [numbers], none }
let statsSeq = 0
const statsQueue = []
let workers = 0
function loadStats(force = true) {
  const want = sites.value.length <= 40 ? sites.value : shownSites.value
  for (const s of want) {
    if (!force && stats[s.id]) continue
    if (stats[s.id]?.loading) continue
    stats[s.id] = { ...(stats[s.id] || {}), loading: true }
    statsQueue.push(s)
  }
  while (workers < 2 && statsQueue.length) worker(statsSeq)
}
async function worker(seq) {
  workers++
  while (statsQueue.length && seq === statsSeq) {
    const s = statsQueue.shift()
    try {
      const r = await api('GET', `api/sites/${s.id}/stats?days=7`)
      stats[s.id] = { loading: false, total: r.totals?.requests ?? 0, daily: list(r.daily).map((d) => d.requests || 0), none: r.files === 0 }
    } catch {
      stats[s.id] = { loading: false, total: null, daily: [], none: true }
    }
  }
  workers--
}
watch(shownSites, () => loadStats(false))
const traffic7 = computed(() => {
  const v = sites.value.map((s) => stats[s.id])
  if (!v.length) return '—'
  if (v.some((x) => x?.loading)) return null
  if (v.some((x) => !x)) return '—'
  return formatNumber(v.reduce((a, x) => a + (x.total || 0), 0), locale.value)
})

// ---- backups per site ----
const backupInfo = (s) => {
  const b = list(store['api/backups']?.backups).filter((x) => x.kind === 'site' && x.target === s.domain)
  const sc = list(store['api/backups']?.schedules).find((x) => x.kind === 'site' && x.target === s.domain)
  return { n: b.length, sc }
}
const hh = (h) => String(h).padStart(2, '0') + ':00'

// ---- KPIs ----
const kpis = computed(() => {
  const all = sites.value.length
  const https = sites.value.filter((s) => s.ssl).length
  const soon = sites.value.filter((s) => s.ssl && (s.ssl_error || (daysLeft(s.cert_expires) ?? 99) <= 14)).length
  const waf = sites.value.filter((s) => s.waf).length
  const sched = sites.value.filter((s) => backupInfo(s).sc).length
  return [
    { key: 'https', icon: 'lock', color: 'green', value: `${https} / ${all}`, label: soon ? t('ws.kpi_https_soon', { n: soon }, soon) : t('ws.kpi_https') },
    { key: 'traffic', icon: 'activity', color: 'blue', value: traffic7.value, label: t('ws.kpi_traffic') },
    { key: 'waf', icon: 'shield', color: 'red', value: `${waf} / ${all}`, label: t('ws.kpi_waf') },
    { key: 'backup', icon: 'archive', color: 'orange', value: store['api/backups'] ? `${sched} / ${all}` : null, label: t('ws.kpi_backup') },
  ]
})

// ---- selection and bulk actions ----
const selected = ref(new Set())
const allOnPage = computed(() => shownSites.value.length > 0 && shownSites.value.every((s) => selected.value.has(s.id)))
function toggleAll() {
  const next = new Set(selected.value)
  if (allOnPage.value) shownSites.value.forEach((s) => next.delete(s.id))
  else shownSites.value.forEach((s) => next.add(s.id))
  selected.value = next
}
function toggleOne(id) {
  const next = new Set(selected.value)
  next.has(id) ? next.delete(id) : next.add(id)
  selected.value = next
}
watch(sites, (v) => {
  const ids = new Set(v.map((s) => s.id))
  selected.value = new Set([...selected.value].filter((id) => ids.has(id)))
})
const bulk = ref('')
const bulkBusy = ref(false)
async function runBulk() {
  const run = bulk.value === 'resume'
  if (!bulk.value) return
  const targets = sites.value.filter((s) => selected.value.has(s.id) && s.disabled === run)
  if (!run && targets.length && !window.confirm(t('ws.bulk_suspend_confirm', { n: targets.length }, targets.length))) return
  bulkBusy.value = true
  clearMessages()
  let ok = 0
  for (const s of targets) {
    try {
      replaceSite(await api('PUT', `api/sites/${s.id}/access`, { disabled: !run, redirect_url: s.redirect_url || '', auth_user: s.auth_user || '', auth_password: '' }))
      ok++
    } catch (e) {
      actionError.value = `${s.domain}: ${errorMessage(e)}`
    }
  }
  bulkBusy.value = false
  bulk.value = ''
  toast(t('ws.bulk_done', { n: ok }, ok))
}

// ---- run switch (suspend / resume) ----
const switching = ref(0)
async function setRunning(s, run) {
  if (!run && !window.confirm(t('ws.suspend_confirm', { domain: s.domain }))) return
  switching.value = s.id
  clearMessages()
  try {
    replaceSite(await api('PUT', `api/sites/${s.id}/access`, { disabled: !run, redirect_url: s.redirect_url || '', auth_user: s.auth_user || '', auth_password: '' }))
    toast(run ? t('ws.resumed', { domain: s.domain }) : t('ws.suspended', { domain: s.domain }))
  } catch (e) {
    actionError.value = errorMessage(e)
  } finally {
    switching.value = 0
  }
}

function replaceSite(updated) {
  sites.value = sites.value.map((s) => {
    if (s.id !== updated.id) return s
    const m = { ...s, ...updated }
    if (!updated.app_status && s.app_status) m.app_status = s.app_status
    return m
  })
  put('api/sites', { sites: sites.value })
}

// ---- new site ----
const showForm = ref(false)
const form = ref({ domain: '', www: false, wwwTouched: false, php: '', touchedType: false, port: '', command: '' })
const formError = ref('')
const creating = ref(false)
const cleanDomain = computed(() => form.value.domain.trim().toLowerCase().replace(/^https?:\/\//, '').replace(/\/.*$/, '').replace(/\.$/, ''))

watch(cleanDomain, (d) => {
  if (!form.value.wwwTouched) form.value.www = d.split('.').length === 2 && !d.startsWith('www.')
})

function openForm() {
  clearMessages()
  formError.value = ''
  showForm.value = true
}

async function createSite() {
  formError.value = ''
  creating.value = true
  const domain = cleanDomain.value
  const isApp = form.value.php === 'app'
  try {
    const site = await api('POST', 'api/sites', {
      domain,
      aliases: form.value.www ? [`www.${domain}`] : [],
      php_version: isApp ? '' : form.value.php,
      proxy_port: isApp ? Number(form.value.port) || 0 : 0,
      app_command: isApp ? form.value.command.trim() : '',
    })
    notice.value = t('websites.created', { domain: site.domain, path: `${site.root}/${isApp ? 'app' : 'public'}` })
    showForm.value = false
    form.value = { domain: '', www: false, wwwTouched: false, php: latestPHP.value, touchedType: false, port: '', command: '' }
    await load()
  } catch (e) {
    formError.value = errorMessage(e)
  } finally {
    creating.value = false
  }
}

// ---- WordPress install ----
const wpSite = ref(null)
const wp = ref({ title: '', admin_user: '', admin_email: '', admin_password: '' })
const wpError = ref('')
const wpResult = ref(null)

function openWordPress(site) {
  clearMessages()
  wpError.value = ''
  wpSite.value = site
  wp.value = { title: site.domain, admin_user: '', admin_email: '', admin_password: '' }
}

async function installWordPress() {
  wpError.value = ''
  try {
    const res = await api('POST', `api/sites/${wpSite.value.id}/wordpress`, wp.value)
    wpResult.value = res
    wpSite.value = null
    watchJob(res.job)
  } catch (e) {
    wpError.value = errorMessage(e)
  }
}

// ---- site drawer: ?site=<id>&tab=<tab> ----
const drawerSite = computed(() => sites.value.find((s) => String(s.id) === String(route.query.site)) || null)
const drawerTab = computed(() => (typeof route.query.tab === 'string' ? route.query.tab : 'overview'))
function openDrawer(s, tab = 'overview') {
  clearMessages()
  router.push({ query: { ...route.query, site: String(s.id), tab: tab === 'overview' ? undefined : tab } })
}
function closeDrawer() {
  const { site, tab, ...rest } = route.query
  router.push({ query: rest })
}
function onDrawerTab(tab) {
  if (tab !== drawerTab.value) router.replace({ query: { ...route.query, tab: tab === 'overview' ? undefined : tab } })
}
const siteDB = (s) => list(store['api/databases']?.databases).find((d) => d.site_id === s.id) || null

// ---- delete ----
const deleting = ref(null)
const confirmText = ref('')
const deleteDBs = ref(true)
const deleteBusy = ref(false)
const deleteError = ref('')

function askDelete(site) {
  clearMessages()
  deleteError.value = ''
  deleting.value = site
  confirmText.value = ''
  deleteDBs.value = true
}

async function deleteSite() {
  const site = deleting.value
  deleteBusy.value = true
  deleteError.value = ''
  try {
    const res = await api('DELETE', `api/sites/${site.id}`, { confirm: confirmText.value.trim(), delete_databases: deleteDBs.value })
    notice.value = t('websites.deleted', { domain: site.domain })
    warnings.value = list(res?.warnings)
    deleting.value = null
    if (drawerSite.value?.id === site.id) closeDrawer()
    await load()
  } catch (e) {
    deleteError.value = errorMessage(e)
  } finally {
    deleteBusy.value = false
  }
}

// ---- display helpers ----
const siteURL = (s) => `${s.ssl ? 'https' : 'http'}://${s.domain}/`
const palette = ['#2F6BDB', '#C2410C', '#7454E0', '#0D9488', '#B45309', '#BE185D', '#4D7C0F', '#475569']
const favColor = (s) => (s.disabled ? '#94A3B8' : palette[[...s.domain].reduce((a, c) => (a * 31 + c.charCodeAt(0)) >>> 0, 7) % palette.length])
const hasService = (s) => !!s.app_command || (s.app || '').startsWith('docker:')
const fmtDate = (v) => new Date(v).toLocaleDateString(locale.value, { year: 'numeric', month: 'short', day: 'numeric' })
function subLine(s) {
  if (s.expires_at && daysLeft(s.expires_at) !== null && daysLeft(s.expires_at) <= 7) return { text: t('access.expires_on', { date: fmtDate(new Date(new Date(s.expires_at).getTime() - 86400000)) }), cls: 'text-crit' }
  if (s.note) return { text: s.note, cls: 'site-note' }
  if (s.redirect_url) return { text: '→ ' + s.redirect_url, cls: 'muted' }
  const a = list(s.aliases)
  if (a.length) return { text: a.length === 1 ? a[0] : t('ws.aliases_n', { first: a[0], n: a.length - 1 }), cls: 'muted' }
  return { text: s.root, cls: 'muted mono-sm' }
}
function appCell(s) {
  const ty = typeOf(s)
  if (s.app && ty === 'php') return { logo: s.app, name: appName(s.app), extra: s.php_version ? 'PHP ' + s.php_version : '', pill: true }
  if (ty === 'php') return { logo: 'php', name: 'PHP ' + s.php_version }
  if (ty === 'docker') return { logo: logoURL(s.app) ? s.app : 'docker', name: appName(s.app), extra: 'Docker' }
  if (ty === 'python') return { logo: 'python', name: s.app ? appName(s.app) : 'Python', extra: t('ws.port', { port: s.proxy_port }) }
  if (ty === 'node') return { logo: s.app && logoURL(s.app) ? s.app : 'nodejs', name: s.app ? appName(s.app) : s.app_runtime?.startsWith('node:') ? 'Node.js ' + s.app_runtime.slice(5) : 'Node.js', extra: t('ws.port', { port: s.proxy_port }) }
  return { icon: 'code', name: t('ws.static_html') }
}
function sslCell(s) {
  if (!s.ssl) return null
  const d = daysLeft(s.cert_expires)
  if (s.ssl_error) return { cls: 'text-crit', icon: 'shieldAlert', text: d !== null && d >= 0 ? t('ws.ssl_days_failed', { n: d }, d) : t('ws.ssl_failed') }
  if (d !== null && d < 0) return { cls: 'text-crit', icon: 'shieldAlert', text: t('ws.ssl_expired') }
  if (d !== null && d <= 14) return { cls: 'text-warn', icon: 'shieldAlert', text: t('ws.ssl_days', { n: d }, d) }
  return { cls: 'text-ok', icon: 'shieldCheck', text: t('ws.ssl_days', { n: d ?? '?' }, d ?? 2), own: s.ssl_source === 'custom' }
}


watch(
  () => route.query.new,
  (v) => {
    if (v && status.value?.ready) {
      openForm()
      const { new: _, ...rest } = route.query
      router.replace({ query: rest })
    }
  },
)

onMounted(async () => {
  await load()
  resume()
  if (route.query.new && status.value?.ready) {
    openForm()
    const { new: _, ...rest } = route.query
    router.replace({ query: rest })
  }
})
onBeforeUnmount(() => {
  statsSeq++
  statsQueue.length = 0
})
</script>

<template>
  <div class="ws">
    <h1 class="sr-only">{{ t('websites.title') }}</h1>
    <Teleport defer to="#topbar-actions">
      <template v-if="status?.ready">
        <RouterLink :to="{ name: 'apps' }" class="btn ws-top-store"><Icon name="package" :size="15" />{{ t('ws.from_store') }}</RouterLink>
        <button type="button" class="btn btn-primary" :disabled="jobRunning" @click="openForm"><Icon name="plus" :size="15" />{{ t('websites.add') }}</button>
      </template>
    </Teleport>

    <p v-if="loadError" class="alert alert-crit" role="alert">{{ loadError }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <p v-if="actionError" class="alert alert-crit pre-wrap" role="alert">{{ actionError }}</p>
    <div v-if="warnings.length" class="alert alert-warn" role="status">
      {{ t('websites.delete_warnings') }}
      <ul class="pre-wrap"><li v-for="w in warnings" :key="w">{{ w }}</li></ul>
    </div>

    <section v-if="wpResult" class="card">
      <div class="card-head">
        <h2>{{ t('websites.wp_credentials_title') }}</h2>
        <button class="icon-btn" :aria-label="t('common.close')" @click="wpResult = null"><Icon name="x" /></button>
      </div>
      <p class="muted">{{ t('websites.wp_credentials_body') }}</p>
      <div class="creds">
        <CopyField :label="t('websites.wp_admin_url')" :value="wpResult.admin_url" />
        <CopyField :label="t('websites.wp_admin_user')" :value="wpResult.admin_user" />
        <CopyField :label="t('websites.wp_admin_password')" :value="wpResult.admin_password" />
      </div>
    </section>

    <JobLog v-if="job && !drawerSite" :job="job" @close="clearJob" />

    <!-- loading -->
    <template v-if="loading">
      <div class="kpis">
        <span v-for="i in 4" :key="i" class="skel" style="height: 72px; border-radius: 14px"></span>
      </div>
      <span class="skel" style="height: 320px; border-radius: 14px"></span>
    </template>

    <template v-else-if="status">
      <div v-if="!status.supported" class="card empty">
        <div class="empty-icon"><Icon name="globe" :size="26" /></div>
        <p class="empty-lead">{{ t('websites.unsupported') }}</p>
      </div>

      <div v-else-if="!status.ready" class="card empty">
        <div class="empty-icon"><Icon name="server" :size="26" /></div>
        <p class="empty-lead">{{ t('websites.install_title') }}</p>
        <p class="muted">{{ t('webserver.install_body', { php: status.default_php || '?' }) }}</p>
        <div class="server-choice" role="radiogroup" :aria-label="t('webserver.choose')">
          <label v-for="o in serverOptions" :key="o.id" class="server-option" :class="{ on: installServer === o.id }">
            <input v-model="installServer" type="radio" name="web-server" :value="o.id" class="sr-only" />
            <AppLogo :name="o.id" size="sm" icon="server" />
            <span class="server-text">
              <span class="server-name">{{ o.name }}</span>
              <span class="muted small">{{ o.hint }}</span>
              <span class="server-ram small"><Icon name="memory" :size="13" />{{ t('software.ram_about', { mb: o.ram }) }}</span>
            </span>
            <Icon v-if="installServer === o.id" name="checkCircle" :size="18" class="server-check" />
          </label>
        </div>
        <button class="btn btn-primary" :disabled="jobRunning" @click="startJob('POST', 'api/web/install', { server: installServer })">
          <Icon name="download" />{{ t('webserver.install', { server: serverNames[installServer] }) }}
        </button>
      </div>

      <template v-else>
        <p v-if="!web.running" class="alert alert-crit">{{ t('ws.server_down', { server: serverName }) }} <RouterLink :to="{ name: 'services' }">{{ t('nav.services') }}</RouterLink></p>

        <div class="kpis">
          <div v-for="k in kpis" :key="k.key" class="kpi">
            <span class="kpi-ico" :class="'c-' + k.color"><Icon :name="k.icon" :size="20" :stroke="1.9" /></span>
            <div>
              <div class="kpi-val">
                <template v-if="k.value !== null">{{ k.value }}</template>
                <span v-else class="skel" style="width: 60px; height: 20px"></span>
              </div>
              <div class="kpi-label">{{ k.label }}</div>
            </div>
          </div>
        </div>

        <div class="ws-tools">
          <div class="seg" role="tablist" :aria-label="t('ws.types')">
            <button v-for="k in kinds" :key="k.id" type="button" role="tab" :aria-selected="kind === k.id" @click="kind = k.id">
              <span v-if="k.logo" class="seg-logo"><img :src="logoURL(k.logo)" alt="" /></span>
              <Icon v-else-if="k.icon" :name="k.icon" :size="16" />
              {{ k.label }} <span class="n">{{ k.n }}</span>
            </button>
          </div>
          <div class="ws-tools-gap"></div>
          <label class="sr-only" for="ws-sort">{{ t('ws.sort') }}</label>
          <select id="ws-sort" v-model="sort" class="input input-auto ws-sort">
            <option value="name">{{ t('ws.sort_name') }}</option>
            <option value="new">{{ t('ws.sort_new') }}</option>
            <option value="traffic">{{ t('ws.sort_traffic') }}</option>
            <option value="ssl">{{ t('ws.sort_ssl') }}</option>
          </select>
          <label class="search-field ws-search">
            <Icon name="search" :size="15" />
            <input v-model="q" type="search" :placeholder="t('ws.filter_ph')" :aria-label="t('ws.filter')" />
          </label>
        </div>

        <section class="panel">
          <div v-if="!sites.length" class="empty">
            <div class="empty-icon"><Icon name="globe" :size="26" /></div>
            <p class="empty-lead">{{ t('websites.empty') }}</p>
            <p class="muted">{{ t('ws.empty_hint') }}</p>
            <div class="row" style="justify-content: center">
              <button class="btn btn-primary" @click="openForm"><Icon name="plus" />{{ t('websites.add') }}</button>
              <RouterLink :to="{ name: 'apps' }" class="btn"><Icon name="package" :size="16" />{{ t('ws.from_store') }}</RouterLink>
            </div>
          </div>
          <div v-else-if="!filtered.length" class="empty">
            <div class="empty-icon"><Icon name="search" :size="24" /></div>
            <p class="empty-lead">{{ t('ws.no_match') }}</p>
            <button class="btn" @click="q = ''; kind = 'all'">{{ t('ws.clear_filter') }}</button>
          </div>
          <div v-else class="table-wrap">
            <table class="table ws-table">
              <thead>
                <tr>
                  <th class="col-check"><input type="checkbox" :checked="allOnPage" :aria-label="t('ws.select_all')" @change="toggleAll" /></th>
                  <th>{{ t('ws.col_site') }}</th>
                  <th>{{ t('ws.col_app') }}</th>
                  <th>{{ t('ws.col_running') }}</th>
                  <th>SSL</th>
                  <th>{{ t('ws.col_traffic') }}</th>
                  <th>{{ t('ws.col_backups') }}</th>
                  <th class="actions">{{ t('ws.col_actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="s in shownSites" :key="s.id" :class="{ 'row-selected': selected.has(s.id), 'row-off': s.disabled }">
                  <td class="col-check"><input type="checkbox" :checked="selected.has(s.id)" :aria-label="t('ws.select', { domain: s.domain })" @change="toggleOne(s.id)" /></td>
                  <td>
                    <div class="site-cell">
                      <span class="fav" :style="{ background: favColor(s) }" aria-hidden="true">{{ s.domain[0] }}</span>
                      <div class="site-cell-text">
                        <button type="button" class="domain-btn" :title="s.domain" @click="openDrawer(s)">{{ s.domain }}</button>
                        <div class="site-sub truncate" :class="subLine(s).cls">{{ subLine(s).text }}</div>
                      </div>
                    </div>
                  </td>
                  <td class="ws-wrap">
                    <span class="app-cell" :class="{ dim: s.disabled }">
                      <AppLogo v-if="appCell(s).logo" :name="appCell(s).logo" size="xs" />
                      <span v-else class="logo-tile xs"><Icon :name="appCell(s).icon" :size="13" /></span>
                      <span>{{ appCell(s).name }}</span>
                      <span v-if="appCell(s).pill" class="pill">{{ appCell(s).extra }}</span>
                      <span v-else-if="appCell(s).extra" class="muted small app-extra">{{ appCell(s).extra }}</span>
                    </span>
                  </td>
                  <td class="ws-wrap">
                    <div class="run-cell">
                      <button
                        type="button"
                        class="switch switch-ok switch-sm"
                        role="switch"
                        :aria-checked="!s.disabled"
                        :aria-label="t('ws.running_label', { domain: s.domain })"
                        :disabled="switching === s.id"
                        @click="setRunning(s, s.disabled)"
                      ></button>
                      <span :class="s.disabled ? 'muted' : 'text-ok'" class="run-text">{{ s.disabled ? t('access.suspended') : hasService(s) && s.app_status && s.app_status !== 'active' ? t('ws.app_down') : t('ws.running') }}</span>
                    </div>
                  </td>
                  <td class="ws-wrap">
                    <button v-if="sslCell(s)" type="button" class="ssl-cell" :class="sslCell(s).cls" @click="openDrawer(s, 'ssl')">
                      <Icon :name="sslCell(s).icon" :size="15" :stroke="2.2" /><span>{{ sslCell(s).text }}<span v-if="sslCell(s).own" class="muted ssl-own">{{ t('ssl.own_short') }}</span></span>
                    </button>
                    <button v-else type="button" class="link-btn small" @click="openDrawer(s, 'ssl')">+ {{ t('websites.https_enable') }}</button>
                  </td>
                  <td>
                    <div class="traffic-cell">
                      <template v-if="stats[s.id] && !stats[s.id].loading">
                        <template v-if="!stats[s.id].none">
                          <Sparkline :values="stats[s.id].daily" :width="56" />
                          <span class="num traffic-n">{{ formatNumber(stats[s.id].total, locale) }}</span>
                        </template>
                        <span v-else class="muted">—</span>
                      </template>
                      <span v-else class="skel" style="width: 96px; height: 18px"></span>
                    </div>
                  </td>
                  <td class="nowrap">
                    <template v-if="backupInfo(s).n || backupInfo(s).sc">
                      <span>{{ t('ws.backups_n', { n: backupInfo(s).n }, backupInfo(s).n) }}</span>
                      <span v-if="backupInfo(s).sc" class="muted small bk-time">{{ hh(backupInfo(s).sc.hour) }}</span>
                    </template>
                    <span v-else class="muted">{{ t('ws.backups_none') }}</span>
                  </td>
                  <td class="actions">
                    <div class="act">
                      <a v-if="!s.disabled" class="icon-btn icon-btn-sm" :href="siteURL(s)" target="_blank" rel="noopener noreferrer" :aria-label="t('ws.open', { domain: s.domain })" :title="t('websites.open_site')"><Icon name="external" :size="16" :stroke="1.9" /></a>
                      <RouterLink class="icon-btn icon-btn-sm" :to="{ name: 'files', query: { path: s.root } }" :aria-label="t('ws.files', { domain: s.domain })" :title="t('nav.files')"><Icon name="folder" :size="16" :stroke="1.9" /></RouterLink>
                      <button type="button" class="icon-btn icon-btn-sm" :aria-label="t('ws.stats', { domain: s.domain })" :title="t('drawer.tab_stats')" @click="openDrawer(s, 'stats')"><Icon name="bars" :size="16" :stroke="1.9" /></button>
                      <button type="button" class="btn btn-sm act-settings" @click="openDrawer(s)">{{ t('ws.settings') }}</button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="sites.length && filtered.length" class="panel-foot">
            <span class="muted">{{ t('ws.selected', { n: selected.size }) }}</span>
            <label class="sr-only" for="ws-bulk">{{ t('ws.bulk') }}</label>
            <select id="ws-bulk" v-model="bulk" class="input input-sm" :disabled="!selected.size || bulkBusy">
              <option value="">{{ t('ws.bulk') }}</option>
              <option value="suspend">{{ t('ws.bulk_suspend') }}</option>
              <option value="resume">{{ t('ws.bulk_resume') }}</option>
            </select>
            <button type="button" class="btn btn-sm" :disabled="!bulk || !selected.size || bulkBusy" @click="runBulk">{{ t('ws.apply') }}</button>
            <span class="ws-tools-gap"></span>
            <span class="muted num">{{ t('ws.range', { from: page * PAGE + 1, to: Math.min(filtered.length, (page + 1) * PAGE), n: filtered.length }) }}</span>
            <button type="button" class="icon-btn icon-btn-sm bordered" :aria-label="t('ws.prev')" :disabled="page === 0" @click="page--"><Icon name="chevronLeft" :size="16" /></button>
            <button type="button" class="icon-btn icon-btn-sm bordered" :aria-label="t('ws.next')" :disabled="page >= pages - 1" @click="page++"><Icon name="chevronRight" :size="16" /></button>
          </div>
        </section>
        <p v-if="sites.some((s) => !s.ssl)" class="muted small">{{ t('websites.ssl_note') }}</p>
      </template>
    </template>

    <!-- site drawer -->
    <SiteDrawer
      v-if="drawerSite && status?.ready"
      :key="drawerSite.id"
      :site="drawerSite"
      :server="status.server || 'nginx'"
      :php-versions="phpList.map((p) => p.version)"
      :job="job"
      :busy="jobRunning"
      :database="siteDB(drawerSite)"
      :tab="drawerTab"
      @close="closeDrawer"
      @tab="onDrawerTab"
      @saved="replaceSite"
      @job="(j) => { clearMessages(); watchJob(j) }"
      @clear-job="clearJob"
      @delete="askDelete"
      @install-wp="openWordPress"
    />

    <!-- new site -->
    <Modal v-if="showForm" :title="t('websites.new_title')" @close="showForm = false">
      <p v-if="formError" class="alert alert-crit pre-wrap" role="alert">{{ formError }}</p>
      <form class="stack" @submit.prevent="createSite">
        <label class="field">
          <span>{{ t('websites.domain') }}</span>
          <input v-model="form.domain" class="input" placeholder="example.com" autocomplete="off" autocapitalize="off" spellcheck="false" maxlength="253" required />
          <small class="muted">{{ t('websites.domain_hint') }}</small>
        </label>
        <label v-if="cleanDomain && !cleanDomain.startsWith('www.')" class="check">
          <input v-model="form.www" type="checkbox" @change="form.wwwTouched = true" />
          <span>{{ t('websites.add_www', { name: `www.${cleanDomain}` }) }}</span>
        </label>
        <label class="field">
          <span>{{ t('websites.type') }}</span>
          <select v-model="form.php" class="input" @change="form.touchedType = true">
            <option v-for="p in [...phpList].reverse()" :key="p.version" :value="p.version">{{ t('websites.type_php', { v: p.version }) }}</option>
            <option value="">{{ t('websites.type_static') }}</option>
            <option value="app">{{ t('websites.type_app') }}</option>
          </select>
        </label>
        <template v-if="form.php === 'app'">
          <label class="field">
            <span>{{ t('websites.app_port') }}</span>
            <input v-model="form.port" class="input input-auto" type="number" min="1024" max="65535" placeholder="3000" required />
          </label>
          <label class="field">
            <span>{{ t('websites.app_command') }}</span>
            <input v-model="form.command" class="input mono" placeholder="node server.js" maxlength="1000" />
            <small class="muted">{{ t('websites.app_hint') }}</small>
          </label>
        </template>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="showForm = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="creating || jobRunning">{{ creating ? t('websites.creating') : t('websites.create') }}</button>
        </div>
      </form>
    </Modal>

    <!-- WordPress -->
    <Modal v-if="wpSite" :title="t('websites.wp_title', { domain: wpSite.domain })" @close="wpSite = null">
      <p v-if="!dbStatus?.running" class="alert alert-warn">
        {{ t('websites.wp_needs_db') }} <RouterLink :to="{ name: 'databases' }">{{ t('nav.databases') }}</RouterLink>
      </p>
      <p v-if="!wpSite.ssl" class="muted small">{{ t('websites.wp_ssl_hint') }}</p>
      <p v-if="wpError" class="alert alert-crit pre-wrap" role="alert">{{ wpError }}</p>
      <form class="stack" @submit.prevent="installWordPress">
        <label class="field"><span>{{ t('websites.wp_site_title') }}</span><input v-model="wp.title" class="input" maxlength="200" required /></label>
        <label class="field">
          <span>{{ t('websites.wp_admin_user') }}</span>
          <input v-model="wp.admin_user" class="input" maxlength="60" autocomplete="off" spellcheck="false" required />
          <small class="muted">{{ t('websites.wp_user_hint') }}</small>
        </label>
        <label class="field"><span>{{ t('websites.wp_admin_email') }}</span><input v-model="wp.admin_email" class="input" type="email" maxlength="254" required /></label>
        <label class="field">
          <span>{{ t('websites.wp_admin_password') }}</span>
          <input v-model="wp.admin_password" class="input" autocomplete="new-password" minlength="12" maxlength="128" />
          <small class="muted">{{ t('websites.wp_password_hint') }}</small>
        </label>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="wpSite = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="jobRunning || !dbStatus?.running"><Icon name="wordpress" />{{ t('websites.wp_start') }}</button>
        </div>
      </form>
    </Modal>

    <!-- delete -->
    <Modal v-if="deleting" danger :title="t('websites.delete_title', { domain: deleting.domain })" @close="deleting = null">
      <p>{{ t('websites.delete_body') }}</p>
      <p v-if="deleteError" class="alert alert-crit pre-wrap" role="alert">{{ deleteError }}</p>
      <form class="stack" @submit.prevent="deleteSite">
        <label class="check"><input v-model="deleteDBs" type="checkbox" /><span>{{ t('websites.delete_dbs') }}</span></label>
        <label class="field">
          <span>{{ t('websites.delete_confirm', { domain: deleting.domain }) }}</span>
          <input v-model="confirmText" class="input" autocomplete="off" autocapitalize="off" spellcheck="false" required />
        </label>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="deleting = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-danger btn-solid" :disabled="deleteBusy || confirmText.trim().toLowerCase() !== deleting.domain"><Icon name="trash" />{{ t('websites.delete') }}</button>
        </div>
      </form>
    </Modal>
  </div>
</template>

<style scoped>
.ws { display: flex; flex-direction: column; gap: 16px; }
.ws > .alert, .ws > .card { margin: 0; }

.ws-tools { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.ws-tools-gap { flex: 1; }
.ws-sort { min-height: 36px; }
.ws-search { width: 240px; max-width: 100%; }

.ws-table th, .ws-table td { padding-left: 11px; padding-right: 11px; }
.ws-table th:first-child, .ws-table td:first-child { padding-left: 18px; padding-right: 4px; }
.ws-table th:last-child, .ws-table td:last-child { padding-right: 18px; }
.ws-table td { white-space: nowrap; padding-top: 12px; padding-bottom: 12px; }
.site-cell { display: flex; align-items: center; gap: 12px; min-width: 0; }
.site-cell-text { min-width: 0; max-width: 210px; }
.ws-table th { white-space: normal; line-height: 1.3; }
.domain-btn { padding: 0; border: 0; background: none; font: 700 13.5px var(--font); color: var(--text); cursor: pointer; text-align: left; }
.domain-btn:hover { color: var(--accent); text-decoration: underline; }
.row-off .domain-btn { color: var(--text-2); }
.site-sub { font-size: 12px; max-width: 200px; }
.site-sub.text-crit { font-weight: 600; }
.site-sub.site-note { color: var(--text-2); }
.mono-sm { font-family: var(--mono); font-size: 11.5px; }
.app-cell { display: inline-flex; align-items: center; gap: 8px; }
.app-cell.dim { opacity: 0.7; }
.app-cell .pill { min-height: 22px; margin-left: 2px; }
.run-cell { display: flex; align-items: center; gap: 8px; }
.run-text { font-size: 13px; font-weight: 600; }
.ssl-cell { display: inline-flex; align-items: center; gap: 6px; padding: 0; border: 0; background: none; font: 600 13px var(--font); cursor: pointer; }
.ssl-cell:hover { text-decoration: underline; }
.ssl-own { font-weight: 500; }
.traffic-cell { display: flex; align-items: center; gap: 10px; }
.traffic-n { font-weight: 600; }
.act { display: inline-flex; gap: 2px; align-items: center; }
.act-settings { margin-left: 6px; }

/* The table must fit its card, or the actions column (with Settings) ends
   up out of view. A domain longer than its column ends in "…" instead of
   running into the next one, and the running and SSL states may go on two
   lines. Up to about 1500 px (with the sidebar) the cells are tighter, and
   the application's version or port, "own cert" and the backup time go on
   a second line. Narrower screens still scroll the table inside its card. */
.domain-btn { max-width: 100%; overflow: hidden; text-overflow: ellipsis; vertical-align: top; }
.app-cell > * { white-space: nowrap; }
.app-extra::before { content: '· '; }
.ssl-own::before, .bk-time::before { content: ' · '; }
@media (min-width: 641px) {
  .ws-table td.ws-wrap { white-space: normal; }
  .ssl-cell, .ws-wrap .link-btn { text-align: left; }
}
@media (min-width: 641px) and (max-width: 1500px) {
  .ws-table th, .ws-table td { padding-left: 8px; padding-right: 8px; }
  .site-cell { gap: 10px; }
  .site-cell-text { max-width: 180px; }
  .site-sub { max-width: 150px; }
  .app-cell { display: inline-grid; grid-template-columns: auto auto; column-gap: 8px; row-gap: 3px; justify-items: start; }
  .app-cell > :nth-child(3) { grid-column: 2; margin-left: 0; }
  .app-extra::before, .ssl-own::before, .bk-time::before { content: none; }
  .ssl-own, .bk-time { display: block; }
  .traffic-cell { gap: 8px; }
  .traffic-cell svg { width: 40px; }
}
@media (min-width: 641px) and (max-width: 1300px) {
  .traffic-cell svg { display: none; }
}

.server-choice { display: grid; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); gap: 10px; max-width: 780px; margin: 16px auto; text-align: left; }
.server-option { position: relative; display: flex; gap: 12px; align-items: flex-start; padding: 14px; border: 1px solid var(--border); border-radius: 12px; background: var(--surface); cursor: pointer; }
.server-option:hover { border-color: var(--border-strong); }
.server-option.on { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-ring); }
.server-option:has(input:focus-visible) { outline: 2px solid var(--accent); outline-offset: 2px; }
.server-text { display: flex; flex-direction: column; gap: 2px; }
.server-name { font-weight: 700; }
.server-ram { display: inline-flex; align-items: center; gap: 4px; margin-top: 4px; color: var(--muted); }
.server-check { position: absolute; top: 10px; right: 10px; color: var(--accent); }

@media (max-width: 640px) {
  .ws-search { width: 100%; }
  .ws-sort { flex: 1; }
  .ws-tools-gap { display: none; }
  .seg { overflow-x: auto; flex-wrap: nowrap; }
  .ws-top-store { display: none; }
  .panel-foot .ws-tools-gap { display: block; }
}
</style>
