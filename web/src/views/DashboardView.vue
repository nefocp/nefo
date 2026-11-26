<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { auth } from '../auth.js'
import { errorMessage } from '../errors.js'
import { formatBytesL, formatNumber, formatUptime, relativeTime } from '../format.js'
import { server, loadServerInfo, watchStats } from '../server.js'
import { store, load, list } from '../data.js'
import { attention, critCount, refreshAttention } from '../attention.js'
import { usePageMeta } from '../page.js'
import { osLogo } from '../logos.js'
import { SERVER_NAMES } from '../webservers.js'
import { toast } from '../toast.js'
import { nudgeJobs } from '../jobs.js'
import ArcGauge from '../components/ArcGauge.vue'
import AreaChart from '../components/AreaChart.vue'
import AppLogo from '../components/AppLogo.vue'
import Icon from '../components/Icon.vue'
import PowerDialog from '../components/PowerDialog.vue'
import { usePopover } from '../composables/usePopover.js'

const { t, locale } = useI18n()
const error = ref('')

usePageMeta(() => ({ sub: t('dash.greeting', { user: auth.user?.username || '' }) + ' · ' + (critCount.value ? t('dash.greeting_attention', { n: critCount.value }, critCount.value) : t('dash.greeting_ok')) }))

const info = computed(() => server.info)
const st = computed(() => (server.stats?.supported ? server.stats : null))
const pct = (u, tot) => (tot ? (u / tot) * 100 : null)
const gb = (n) => formatBytesL(n, locale.value)
// pair writes "used / total" in the total's unit: "0,6 / 1,9 GB".
function pair(used, total) {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let d = 1
  while (total / d >= 1024 && i < units.length - 1) {
    d *= 1024
    i++
  }
  const f = (v) => formatNumber(v / d, locale.value, i >= 3 ? 1 : 0)
  return `${f(used)} / ${f(total)} ${units[i]}`
}

let stopStats = null
onMounted(async () => {
  stopStats = watchStats(5000)
  try {
    await loadServerInfo()
  } catch (e) {
    error.value = errorMessage(e)
  }
  refreshAttention(true)
  // Lists for the tiles; each may fail on its own (MariaDB not installed…).
  for (const p of ['api/databases', 'api/cron', 'api/backups', 'api/software', 'api/services', 'api/web/status']) load(p, { ttl: 15000 }).catch(() => {})
  loadHistory()
})
onUnmounted(() => stopStats?.())

// ---- hero ----
const osLine = computed(() => {
  if (!info.value) return ''
  const k = (info.value.kernel || '').match(/^(\d+\.\d+)/)
  return [info.value.os, k ? `Linux ${k[1]}` : ''].filter(Boolean).join(' · ')
})
const web = computed(() => store['api/web/status'])
const stackLine = computed(() => {
  const parts = []
  const w = web.value
  if (w?.server) parts.push(SERVER_NAMES[w.server] || w.server)
  if (list(w?.php).length) parts.push('PHP ' + list(w.php).map((p) => p.version).join(', '))
  if (store['api/software'] && list(store['api/software'].items).some((i) => i.id === 'mariadb' && i.installed)) parts.push('MariaDB')
  return parts.join(' · ')
})
const gauges = computed(() => {
  const s = st.value
  const cpus = info.value?.cpus || 1
  return [
    { key: 'load', name: t('dash.load'), percent: s ? (s.load1 / cpus) * 100 : null, inner: s ? t('dash.load_inner', { v: formatNumber(s.load1, locale.value, 2) }) : '', ok: t('dash.load_ok') },
    { key: 'cpu', name: 'CPU', percent: s ? s.cpu_percent : null, inner: t('dash.vcpu', { n: cpus }) },
    { key: 'mem', name: t('dash.memory'), percent: s ? pct(s.mem_used, s.mem_total) : null, inner: s ? pair(s.mem_used, s.mem_total) : '' },
    { key: 'disk', name: t('dash.disk'), percent: s ? pct(s.disk_used, s.disk_total) : null, inner: s ? pair(s.disk_used, s.disk_total) : '' },
  ]
})

// ---- update ----
const update = ref(null)
const checking = ref(false)
async function checkUpdate() {
  checking.value = true
  try {
    update.value = await api('GET', 'api/panel/update')
    if (!update.value.available) toast(t('settings.up_to_date'))
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  } finally {
    checking.value = false
  }
}
async function applyUpdate() {
  if (!window.confirm(t('settings.update_confirm', { v: update.value.latest }))) return
  try {
    nudgeJobs(await api('POST', 'api/panel/update', {}))
    toast(t('dash.update_started'))
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  }
}

// ---- power: restart the panel or reboot the server ----
const powerRoot = ref(null)
const powerBtn = ref(null)
const { open: powerOpen, toggle: togglePower, hide: hidePower } = usePopover(powerRoot, powerBtn)
const power = ref('')
function pickPower(mode) {
  hidePower()
  power.value = mode
}

// ---- tiles ----
const sites = computed(() => list(store['api/sites']?.sites))
const dbs = computed(() => list(store['api/databases']?.databases))
const crons = computed(() => list(store['api/cron']?.jobs))
const backups = computed(() => list(store['api/backups']?.backups))
const latestBackup = computed(() => [...backups.value].sort((a, b) => new Date(b.created_at) - new Date(a.created_at))[0])
const weekAgo = Date.now() - 7 * 86400000
const tiles = computed(() => [
  {
    key: 'sites', to: { name: 'websites' }, icon: 'globe', color: 'blue', n: store['api/sites'] ? sites.value.length : null, label: t('nav.websites'),
    hint: (() => {
      const nw = sites.value.filter((s) => new Date(s.created_at).getTime() > weekAgo).length
      const off = sites.value.filter((s) => s.disabled).length
      return off ? t('dash.tile_suspended', { n: off }) : nw ? t('dash.tile_new_week', { n: nw }) : ''
    })(),
  },
  { key: 'dbs', to: { name: 'databases' }, icon: 'database', color: 'violet', n: store['api/databases'] ? dbs.value.length : null, label: t('nav.databases'), hint: dbs.value.length ? gb(dbs.value.reduce((a, d) => a + (d.size || 0), 0)) : '' },
  { key: 'sftp', to: { name: 'websites' }, icon: 'upload', color: 'teal', n: store['api/sites'] ? sites.value.filter((s) => s.sftp).length : null, label: t('dash.tile_sftp'), hint: sites.value.length ? t('dash.tile_of_sites', { n: sites.value.length }, sites.value.length) : '' },
  { key: 'cron', to: { name: 'cron' }, icon: 'clock', color: 'orange', n: store['api/cron'] ? crons.value.length : null, label: t('nav.cron'), hint: crons.value.length ? t('dash.tile_enabled', { n: crons.value.filter((c) => c.enabled).length }) : '' },
  { key: 'backups', to: { name: 'backups' }, icon: 'archive', color: 'green', n: store['api/backups'] ? backups.value.length : null, label: t('dash.tile_backups'), hint: latestBackup.value ? t('dash.tile_latest', { when: relativeTime(latestBackup.value.created_at, locale.value) }) : '' },
  { key: 'risk', to: critCount.value ? attention.value[0]?.to : { name: 'security' }, icon: 'shieldAlert', color: critCount.value ? 'red' : 'green', n: critCount.value, label: t('dash.tile_risks'), hint: critCount.value ? t('dash.tile_view') : t('dash.tile_none'), danger: critCount.value > 0 },
])

// ---- attention card: problems, then good news ----
const goodNews = computed(() => {
  const out = []
  const b = latestBackup.value
  if (b && !list(store['api/backups']?.schedules).some((s) => s.last_error)) {
    out.push({ id: 'bk', title: t('dash.ok_backup', { target: b.target }), sub: relativeTime(b.created_at, locale.value) + ' · ' + gb(b.size) })
  }
  const https = sites.value.filter((s) => s.ssl).length
  if (sites.value.length && https === sites.value.length) out.push({ id: 'https', title: t('dash.ok_https'), sub: t('dash.ok_https_sub', { n: https }, https) })
  return out
})

// ---- history chart ----
const metric = ref('cpu')
const hours = ref(24)
const history = ref([])
const netTotal = ref(null) // { rx, tx } bytes in the range
const histLoading = ref(true)
async function loadHistory() {
  histLoading.value = true
  try {
    const res = await api('GET', `api/system/history?hours=${hours.value}`)
    history.value = list(res.points)
    netTotal.value = res.net ?? null
  } catch {
    history.value = []
    netTotal.value = null
  } finally {
    histLoading.value = false
  }
}
watch(hours, loadHistory)
function niceMax(v) {
  if (!(v > 0)) return 1
  const m = 10 ** Math.floor(Math.log10(v))
  for (const f of [1, 2, 4, 5, 8, 10]) if (f * m >= v) return f * m
  return 10 * m
}
// Network: bytes per second received (rx) and sent (tx). Samples from before
// the panel recorded traffic have no rx/tx and are left out.
const isNet = computed(() => metric.value === 'net')
const netPoints = computed(() => history.value.filter((p) => p.rx != null && p.tx != null))
const series = computed(() =>
  isNet.value
    ? netPoints.value.map((p) => ({ t: p.t, v: Number(p.rx) || 0 }))
    : history.value.map((p) => ({ t: p.t, v: Number(p[metric.value === 'load' ? 'load1' : metric.value]) || 0 })),
)
const series2 = computed(() => (isNet.value ? netPoints.value.map((p) => ({ t: p.t, v: Number(p.tx) || 0 })) : null))
// A 1024-based nice maximum, so the scale reads 4 KB/s, 2 KB/s, 0.
function niceBytes(v) {
  const unit = 1024 ** Math.max(0, Math.floor(Math.log(Math.max(v, 1)) / Math.log(1024)))
  return niceMax(v / unit) * unit
}
const chartMax = computed(() => {
  if (isNet.value) return niceBytes(Math.max(1024, ...series.value.map((p) => p.v), ...series2.value.map((p) => p.v)))
  return metric.value === 'load' ? niceMax(Math.max(1, ...series.value.map((p) => p.v))) : 100
})
const fmtRate = (v) => `${formatBytesL(v, locale.value, 1)}/s`
const fmtVal = (v) => (isNet.value ? fmtRate(v) : metric.value === 'load' ? formatNumber(v, locale.value, 2) : `${formatNumber(v, locale.value, 0)}%`)
const summary = computed(() => {
  const v = series.value.map((p) => p.v)
  if (!v.length) return null
  return { now: v.at(-1), avg: v.reduce((a, b) => a + b, 0) / v.length, max: Math.max(...v) }
})
// The latest rates, and the bytes in the range (summed by the server from
// the one-minute samples).
const netSummary = computed(() => {
  if (!isNet.value || !series.value.length) return null
  return { rx: series.value.at(-1).v, tx: series2.value.at(-1).v, rxTotal: netTotal.value?.rx ?? 0, txTotal: netTotal.value?.tx ?? 0 }
})
const metricName = computed(() => ({ cpu: 'CPU', mem: t('dash.memory'), disk: t('dash.disk'), load: t('dash.load'), net: t('dash.net') })[metric.value])
const chartLabel = computed(() => {
  if (isNet.value) return netSummary.value ? t('dash.net_chart_label', { rx: fmtRate(netSummary.value.rx), tx: fmtRate(netSummary.value.tx) }) : t('dash.net')
  return t('dash.chart_label', { metric: metricName.value, now: summary.value ? fmtVal(summary.value.now) : '', peak: summary.value ? fmtVal(summary.value.max) : '' })
})

// ---- installed software ----
const services = computed(() => list(store['api/services']?.services))
const svc = (name) => services.value.find((s) => s.name === name)
const mb = (bytes) => (bytes ? formatBytesL(bytes, locale.value, 0) : '')
const software = computed(() => {
  const items = list(store['api/software']?.items)
  const out = []
  const w = web.value
  if (w?.server) {
    const unit = { nginx: 'nginx', apache: 'apache2', openlitespeed: 'lshttpd' }[w.server]
    const s = svc(unit)
    out.push({ id: w.server, logo: w.server, name: SERVER_NAMES[w.server], sub: [w[w.server]?.version, mb(s?.memory)].filter(Boolean).join(' · '), state: s ? s.active : w[w.server]?.running ? 'active' : 'inactive', to: { name: 'services' } })
  }
  for (const it of items) {
    if (!it.installed) continue
    if (it.kind === 'php') {
      const s = svc(`php${it.version}-fpm`)
      out.push({ id: it.id, logo: 'php', name: `PHP ${it.version}`, sub: [t('dash.sw_sites', { n: it.used_by || 0 }, it.used_by || 0), mb(s?.memory)].filter(Boolean).join(' · '), state: s?.active || 'active', to: { name: 'software' } })
    }
  }
  const byId = Object.fromEntries(items.map((i) => [i.id, i]))
  const add = (id, logo, name, unit, sub) => {
    const it = byId[id]
    if (!it?.installed) return
    const s = unit ? svc(unit) : null
    out.push({ id, logo, name, sub: [it.version, sub ?? mb(s?.memory)].filter(Boolean).join(' · '), state: unit ? s?.active || 'inactive' : 'ready', to: { name: id === 'docker' ? 'docker' : 'software' } })
  }
  add('mariadb', 'mariadb', 'MariaDB', 'mariadb')
  add('redis', 'redis', 'Redis', 'redis-server')
  add('nodejs', 'nodejs', 'Node.js', '', byId.nodejs?.usage ? t('dash.sw_sites', { n: byId.nodejs.usage.sites || 0 }, byId.nodejs.usage.sites || 0) : '')
  add('python', 'python', 'Python', '', byId.python?.usage ? t('dash.sw_sites', { n: byId.python.usage.sites || 0 }, byId.python.usage.sites || 0) : '')
  add('docker', 'docker', 'Docker', '', byId.docker?.usage ? t('dash.sw_containers', { n: byId.docker.usage.containers || 0 }, byId.docker.usage.containers || 0) : '')
  add('adminer', 'adminer', 'Adminer', '')
  add('waf', 'waf', 'ModSecurity WAF', '')
  add('clamav', 'clamav', 'ClamAV', '')
  for (const id of ['redis', 'nodejs', 'docker']) {
    const it = byId[id]
    if (it && !it.installed) out.push({ id: 'add-' + id, logo: id, name: { redis: 'Redis', nodejs: 'Node.js', docker: 'Docker' }[id], sub: it.ram_mb ? t('dash.sw_not_installed_ram', { mb: it.ram_mb }) : t('dash.sw_not_installed'), add: true, to: { name: 'software' } })
  }
  return out
})
const stateText = (s) => (s === 'active' ? t('dash.sw_running') : s === 'ready' ? t('dash.sw_ready') : s === 'failed' ? t('dash.sw_failed') : t('dash.sw_stopped'))
const stateClass = (s) => (s === 'active' ? 'text-ok' : s === 'failed' ? 'text-crit' : 'muted')
</script>

<template>
  <div class="dash">
    <h1 class="sr-only">{{ t('nav.dashboard') }}</h1>
    <p v-if="error" class="alert alert-crit" role="alert">{{ error }}</p>
    <p v-if="server.stats && !server.stats.supported" class="alert alert-warn">{{ t('dashboard.unsupported') }}</p>

    <!-- server cockpit -->
    <section class="hero" :aria-label="t('dash.server_label', { host: info?.hostname || '' })">
      <div class="hero-id">
        <div class="hero-host">
          <span class="logo-tile lg hero-os">
            <img v-if="osLogo(info?.os)" :src="osLogo(info?.os)" :alt="info?.os || ''" />
            <Icon v-else name="server" :size="24" />
          </span>
          <div class="hero-host-text">
            <div v-if="info" class="hero-name">{{ info.hostname }}</div>
            <span v-else class="skel skel-ink" style="width: 160px; height: 22px"></span>
            <div class="hero-os-line">{{ osLine }}</div>
          </div>
        </div>
        <div class="hero-chips">
          <span v-if="info?.supported" class="chip-ink"><span class="live-dot" aria-hidden="true"></span>{{ t('dash.uptime', { d: formatUptime(info.uptime_seconds, t) }) }}</span>
          <span v-if="server.ip" class="chip-ink mono">{{ server.ip }}</span>
          <span v-if="info" class="chip-ink">{{ t('dash.vcpu', { n: info.cpus }) }}<template v-if="st"> · {{ gb(st.mem_total) }} RAM</template></span>
          <span v-if="stackLine" class="chip-ink">{{ stackLine }}</span>
        </div>
        <div class="hero-actions">
          <button v-if="!update?.available" type="button" class="btn btn-ink" :disabled="checking" @click="checkUpdate">
            <span v-if="checking" class="spinner" aria-hidden="true"></span><Icon v-else name="refresh" :size="15" />{{ t('dash.check_update') }}
          </button>
          <button v-else type="button" class="btn btn-ink btn-ink-accent" @click="applyUpdate"><Icon name="download" :size="15" />{{ t('dash.update_to', { v: update.latest }) }}</button>
          <RouterLink :to="{ name: 'terminal' }" class="btn btn-ink"><Icon name="terminal" :size="15" />{{ t('nav.terminal') }}</RouterLink>
          <div ref="powerRoot" class="power-wrap">
            <button ref="powerBtn" type="button" class="btn btn-ink" :aria-expanded="powerOpen" aria-haspopup="menu" @click="togglePower">
              <Icon name="power" :size="15" />{{ t('power.title') }}<Icon name="chevronDown" :size="14" />
            </button>
            <div v-if="powerOpen" class="menu power-menu" role="menu">
              <button type="button" class="menu-item" role="menuitem" @click="pickPower('panel')"><Icon name="refresh" :size="16" />{{ t('power.restart') }}</button>
              <button type="button" class="menu-item danger" role="menuitem" @click="pickPower('reboot')"><Icon name="power" :size="16" />{{ t('power.reboot') }}</button>
            </div>
          </div>
        </div>
      </div>
      <div class="hero-gauges">
        <ArcGauge v-for="g in gauges" :key="g.key" class="hero-gauge" :name="g.name" :percent="g.percent" :inner="g.inner" :ok-text="g.ok" />
      </div>
    </section>

    <PowerDialog v-if="power" :mode="power" @close="power = ''" />

    <!-- counts -->
    <div class="tiles">
      <RouterLink v-for="tile in tiles" :key="tile.key" :to="tile.to" class="tile" :class="{ danger: tile.danger }">
        <div class="tile-top">
          <span class="tile-ico" :class="'c-' + tile.color"><Icon :name="tile.icon" :size="19" :stroke="1.9" /></span>
          <span class="tile-hint" :class="{ 'text-crit': tile.danger }">{{ tile.hint }}</span>
        </div>
        <div>
          <div class="tile-num" :class="{ 'text-crit': tile.danger }">
            <template v-if="tile.n !== null">{{ formatNumber(tile.n, locale) }}</template>
            <span v-else class="skel" style="width: 36px; height: 24px"></span>
          </div>
          <div class="tile-label">{{ tile.label }}</div>
        </div>
      </RouterLink>
    </div>

    <div class="dash-row">
      <!-- history -->
      <section class="panel dash-chart">
        <div class="panel-head">
          <h2 class="panel-title"><Icon name="activity" :size="17" />{{ t('dash.history') }}</h2>
          <div class="row">
            <div class="tabs" role="tablist" :aria-label="t('dash.metric')">
              <button v-for="m in ['cpu', 'mem', 'disk', 'load', 'net']" :key="m" type="button" role="tab" :aria-selected="metric === m" @click="metric = m">
                {{ { cpu: 'CPU', mem: t('dash.memory_short'), disk: t('dash.disk_short'), load: t('dash.load_short'), net: t('dash.net') }[m] }}
              </button>
            </div>
            <label class="sr-only" for="dash-range">{{ t('dashboard.range') }}</label>
            <select id="dash-range" v-model.number="hours" class="input input-sm">
              <option v-for="h in [1, 24, 168]" :key="h" :value="h">{{ t('dashboard.range_' + h) }}</option>
            </select>
          </div>
        </div>
        <div v-if="isNet" class="chart-sum">
          <div><div class="sum-label"><span class="sum-key" aria-hidden="true"></span>{{ t('dash.net_in') }}</div><div class="sum-val">{{ netSummary ? fmtRate(netSummary.rx) : '—' }}</div></div>
          <div><div class="sum-label"><span class="sum-key second" aria-hidden="true"></span>{{ t('dash.net_out') }}</div><div class="sum-val">{{ netSummary ? fmtRate(netSummary.tx) : '—' }}</div></div>
          <div>
            <div class="sum-label">{{ t('dash.net_total', { range: t('dashboard.range_' + hours) }) }}</div>
            <div v-if="netSummary" class="sum-val sum-val-sm sum-pair">
              <span :title="t('dash.net_in')">↓ {{ formatBytesL(netSummary.rxTotal, locale, 1) }}</span>
              <span :title="t('dash.net_out')">↑ {{ formatBytesL(netSummary.txTotal, locale, 1) }}</span>
            </div>
            <div v-else class="sum-val sum-val-sm">—</div>
          </div>
        </div>
        <div v-else class="chart-sum">
          <div><div class="sum-label"><span class="sum-swatch" aria-hidden="true"></span>{{ t('dash.now') }}</div><div class="sum-val">{{ summary ? fmtVal(summary.now) : '—' }}</div></div>
          <div><div class="sum-label">{{ t('dash.avg') }}</div><div class="sum-val">{{ summary ? fmtVal(summary.avg) : '—' }}</div></div>
          <div><div class="sum-label">{{ t('dash.peak') }}</div><div class="sum-val">{{ summary ? fmtVal(summary.max) : '—' }}</div></div>
        </div>
        <div class="chart-body">
          <span v-if="histLoading && !history.length" class="skel" style="height: 180px"></span>
          <p v-else-if="series.length < 2" class="muted chart-empty">{{ isNet && history.length ? t('dash.net_empty') : t('dashboard.history_empty') }}</p>
          <AreaChart
            v-else
            :label="chartLabel"
            :points="series"
            :points2="series2"
            :names="[t('dash.net_in'), t('dash.net_out')]"
            :max="chartMax"
            :format="fmtVal"
            :locale="locale"
          />
        </div>
      </section>

      <!-- needs attention -->
      <section class="panel dash-attn">
        <div class="panel-head">
          <h2 class="panel-title"><Icon name="alert" :size="17" />{{ t('dash.attention') }}</h2>
          <span class="count-badge" :class="attention.length ? 'crit' : 'ok'">{{ attention.length }}</span>
        </div>
        <div>
          <div v-for="a in attention.slice(0, 6)" :key="a.id" class="alert-row">
            <span class="sev" :class="a.sev"><Icon :name="a.icon" :size="16" /></span>
            <div class="alert-row-text">
              <div class="alert-row-title"><span class="sr-only">{{ t('bell.sev_' + a.sev) }}: </span>{{ t(a.title[0], a.title[1] || {}, a.title[2] ?? 1) }}</div>
              <div class="alert-row-sub">{{ t(a.sub[0], a.sub[1] || {}) }}</div>
            </div>
            <RouterLink :to="a.to" class="btn btn-sm" :class="{ 'btn-primary': a.primary }">{{ t(a.action) }}</RouterLink>
          </div>
          <div v-if="attention.length > 6" class="alert-row muted small">{{ t('dash.attention_more', { n: attention.length - 6 }) }}</div>
          <div v-for="g in goodNews" :key="g.id" class="alert-row">
            <span class="sev ok"><Icon name="check" :size="16" /></span>
            <div class="alert-row-text">
              <div class="alert-row-title">{{ g.title }}</div>
              <div class="alert-row-sub">{{ g.sub }}</div>
            </div>
          </div>
          <div v-if="!attention.length && !goodNews.length" class="alert-row">
            <span class="sev ok"><Icon name="check" :size="16" /></span>
            <div class="alert-row-text">
              <div class="alert-row-title">{{ t('bell.all_good') }}</div>
              <div class="alert-row-sub">{{ t('bell.all_good_sub') }}</div>
            </div>
          </div>
        </div>
      </section>
    </div>

    <!-- installed software -->
    <section class="panel">
      <div class="panel-head">
        <h2 class="panel-title"><Icon name="layers" :size="17" />{{ t('dash.software') }}</h2>
        <RouterLink :to="{ name: 'software' }" class="btn btn-sm">{{ t('dash.software_all') }}</RouterLink>
      </div>
      <div class="sw-grid">
        <template v-if="!store['api/software'] && !store['api/web/status']">
          <span v-for="i in 4" :key="i" class="skel" style="height: 68px; border-radius: 12px"></span>
        </template>
        <RouterLink v-for="s in software" :key="s.id" :to="s.to" class="sw-tile" :class="{ add: s.add }">
          <AppLogo :name="s.logo" :dim="s.add" />
          <span class="sw-text">
            <span class="sw-name">{{ s.name }}</span>
            <span class="sw-sub">{{ s.sub }}</span>
          </span>
          <span v-if="s.add" class="btn btn-sm" aria-hidden="true">{{ t('dash.sw_install') }}</span>
          <span v-else class="sw-state" :class="stateClass(s.state)"><span class="dot" :class="s.state === 'active' ? 'dot-ok' : s.state === 'failed' ? 'dot-crit' : ''"></span>{{ stateText(s.state) }}</span>
        </RouterLink>
      </div>
    </section>
  </div>
</template>

<style scoped>
.dash { display: flex; flex-direction: column; gap: 18px; }
.dash > .alert { margin: 0; }

/* hero: the server cockpit, dark in both themes */
.hero {
  position: relative; border-radius: 18px; border: 1px solid var(--ink-line); background: var(--ink); color: var(--ink-text);
  padding: 22px 24px; display: flex; flex-wrap: wrap; gap: 22px; align-items: center;
}
.hero-id { flex: 1 1 300px; min-width: 0; display: flex; flex-direction: column; gap: 14px; }
.hero-host { display: flex; align-items: center; gap: 14px; }
.hero-os { border: 0; }
.hero-host-text { min-width: 0; }
.hero-name { font-size: 22px; font-weight: 800; color: #fff; letter-spacing: -0.02em; line-height: 1.2; overflow-wrap: anywhere; }
.hero-os-line { font-size: 13px; color: var(--ink-muted); }
.hero-chips { display: flex; gap: 8px; flex-wrap: wrap; }
.chip-ink {
  display: inline-flex; align-items: center; gap: 7px; min-height: 26px; padding: 2px 10px; border-radius: 999px;
  background: rgb(255 255 255 / 0.06); border: 1px solid rgb(255 255 255 / 0.1); font-size: 12px; color: var(--ink-text); white-space: nowrap;
}
.chip-ink.mono { font-size: 11.5px; }
.chip-ink .live-dot { width: 7px; height: 7px; box-shadow: none; }
.hero-actions { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }
.btn-ink-accent { background: var(--bright); border-color: var(--bright); color: #04201c; }
.btn-ink-accent:hover { background: var(--bright); color: #04201c; filter: brightness(1.06); }
.power-wrap { position: relative; }
.power-menu { left: 0; right: auto; }
.hero-gauges { flex: 2 1 560px; display: flex; flex-wrap: wrap; justify-content: space-around; gap: 6px; }

/* count tiles */
.tiles { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 14px; }
.tile {
  display: flex; flex-direction: column; gap: 10px; padding: 16px 18px; color: var(--text); text-decoration: none;
  background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); box-shadow: var(--shadow);
  transition: border-color 0.15s, box-shadow 0.15s;
}
.tile:hover { color: var(--text); text-decoration: none; border-color: var(--border-strong); }
.tile.danger { border-color: color-mix(in srgb, var(--crit-g) 40%, var(--border)); }
.tile-top { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.tile-ico { width: 36px; height: 36px; border-radius: 10px; display: grid; place-items: center; flex: none; }
.c-blue { background: rgb(47 107 219 / 0.12); color: #2f6bdb; }
.c-violet { background: rgb(100 64 216 / 0.12); color: #7454e0; }
.c-teal { background: rgb(13 148 136 / 0.13); color: #0d9488; }
.c-orange { background: rgb(194 87 15 / 0.12); color: #c2570f; }
.c-green { background: rgb(16 185 129 / 0.13); color: #0e9f6e; }
.c-red { background: rgb(239 68 68 / 0.12); color: #dc2626; }
.tile-hint { font-size: 12px; color: var(--muted); text-align: right; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 0; }
.tile-hint.text-crit { font-weight: 700; }
.tile-num { font-size: 26px; font-weight: 800; letter-spacing: -0.03em; line-height: 1; font-variant-numeric: tabular-nums; min-height: 26px; }
.tile-label { font-size: 13px; color: var(--muted); margin-top: 4px; }

.dash-row { display: flex; flex-wrap: wrap; gap: 18px; align-items: flex-start; }
.dash-chart { flex: 3 1 560px; }
.dash-attn { flex: 2 1 340px; }
.chart-sum { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; padding: 14px 18px 2px; }
.sum-label { font-size: 12px; color: var(--muted); display: flex; align-items: center; gap: 6px; }
.sum-swatch { width: 12px; height: 3px; border-radius: 2px; background: var(--accent); }
.sum-key { width: 14px; height: 0; border-top: 3px solid var(--s1); }
.sum-key.second { border-top-style: dashed; border-top-color: var(--s2); }
.sum-val-sm { font-size: 15px; padding-top: 3px; }
.sum-pair { display: flex; flex-wrap: wrap; column-gap: 12px; }
.sum-pair span { white-space: nowrap; }
.sum-val { font-size: 19px; font-weight: 800; letter-spacing: -0.02em; font-variant-numeric: tabular-nums; }
.chart-body { padding: 12px 18px 16px; }
.chart-empty { margin: 0; padding: 40px 0; text-align: center; }

/* installed software */
.sw-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(268px, 1fr)); gap: 12px; padding: 16px 18px 18px; }
.sw-tile {
  display: flex; align-items: center; gap: 12px; padding: 12px 14px; border: 1px solid var(--border); border-radius: 12px;
  color: var(--text); background: var(--surface); text-decoration: none; min-width: 0;
}
.sw-tile:hover { border-color: var(--accent); color: var(--text); text-decoration: none; box-shadow: 0 0 0 3px var(--accent-soft); }
.sw-tile.add { border-style: dashed; }
.sw-text { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.sw-name { font-weight: 700; }
.sw-sub { font-size: 12px; color: var(--muted); }
.sw-state { display: inline-flex; align-items: center; font-size: 12px; font-weight: 600; white-space: nowrap; }
.sw-state .dot { width: 7px; height: 7px; margin-right: 6px; box-shadow: none; }

@media (max-width: 900px) {
  .hero-gauges { flex-basis: 100%; }
}
@media (max-width: 640px) {
  .dash { gap: 14px; }
  .hero { margin: -18px -16px 0; border-radius: 0 0 24px 24px; padding: 6px 14px 18px; gap: 14px; }
  .hero-host .logo-tile { display: none; }
  .hero-name { font-size: 18px; }
  .hero-chips .chip-ink:nth-child(n + 3) { display: none; }
  .hero-gauges { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 4px; }
  .hero-gauge :deep(.arc-box) { width: 112px !important; height: 100px !important; }
  .hero-gauge { min-width: 0 !important; }
  .hero-gauge :deep(.arc-num) { font-size: 24px; }
  .hero-gauge :deep(.arc-inner) { display: none; }
  .hero-gauge :deep(.arc-val) { top: 36%; }
  .tiles { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
  .tile { padding: 12px 14px; }
  .sw-grid { grid-template-columns: minmax(0, 1fr); padding: 12px; }
  .chart-sum { padding: 12px 14px 0; }
  .chart-body { padding: 10px 14px 14px; }
  .alert-row { padding: 12px 14px; }
}
</style>
