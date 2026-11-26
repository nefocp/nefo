<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytes } from '../format.js'
import LineChart from './LineChart.vue'
import Icon from './Icon.vue'
import { SERVER_NAMES } from '../webservers.js'

// SiteStats shows a website's traffic, computed by the server from its
// access logs (Nginx, Apache or OpenLiteSpeed). It is the content of a modal.
const props = defineProps({ site: { type: Object, required: true }, server: { type: String, default: 'nginx' } })
const serverName = computed(() => SERVER_NAMES[props.server] || 'Nginx')
const { t, locale } = useI18n()

const ranges = [1, 7, 14, 30]
const days = ref(7)
const report = ref(null)
const loading = ref(false)
const error = ref('')
let seq = 0 // answers to an older period are dropped

async function load() {
  const n = ++seq
  loading.value = true
  error.value = ''
  try {
    const r = await api('GET', `api/sites/${props.site.id}/stats?days=${days.value}`)
    if (n === seq) report.value = r
  } catch (e) {
    if (n === seq) error.value = errorMessage(e)
  } finally {
    if (n === seq) loading.value = false
  }
}
onMounted(load)
watch(days, load)

// ---- number formats ----
const compact = (n) => new Intl.NumberFormat(locale.value, { notation: 'compact', maximumFractionDigits: 1 }).format(n)
const full = (n) => Number(n).toLocaleString(locale.value)
const share = (n, of) => (of > 0 ? (n / of) * 100 : 0)
const pct = (v) => `${v.toFixed(v > 0 && v < 10 ? 1 : 0)}%`
const fmtDate = (v) => new Date(v).toLocaleString(locale.value, { dateStyle: 'medium', timeStyle: 'short' })

const totals = computed(() => report.value?.totals)
const noLogs = computed(() => report.value && report.value.files === 0)
const noRequests = computed(() => report.value && report.value.files > 0 && totals.value.requests === 0)

const tiles = computed(() => {
  const c = totals.value
  if (!c) return []
  const avg = Math.round(c.requests / days.value)
  return [
    {
      key: 'requests',
      icon: 'activity',
      label: t('stats.requests'),
      value: compact(c.requests),
      detail: days.value === 1 ? t('stats.since_midnight') : t('stats.per_day', { n: compact(avg) }),
    },
    { key: 'visitors', icon: 'globe', label: t('stats.visitors'), value: compact(c.visitors), detail: t('stats.visitors_hint') },
    {
      key: 'bandwidth',
      icon: 'download',
      label: t('stats.bandwidth'),
      value: formatBytes(c.bytes),
      detail: c.requests ? t('stats.per_request', { size: formatBytes(c.bytes / c.requests) }) : '',
    },
    {
      key: 'errors',
      icon: 'alert',
      label: t('stats.error_rate'),
      value: pct(share(c.status_4xx + c.status_5xx, c.requests)),
      detail: t('stats.errors_detail', { c4: full(c.status_4xx), c5: full(c.status_5xx) }),
    },
    {
      key: 'bots',
      icon: 'cpu',
      label: t('stats.bot_share'),
      value: pct(share(c.bots, c.requests)),
      detail: t('stats.bots_detail', { n: compact(c.bots) }, c.bots),
    },
  ]
})

// ---- charts ----
// Bucket times are the server's midnights (or hours). Without their UTC
// offset, the browser reads them as its own local time, so the chart shows
// the server's dates whatever the viewer's time zone.
const wall = (iso) => String(iso).slice(0, 19)
const hourly = computed(() => days.value === 1 && (report.value?.hourly?.length ?? 0) > 1)
const buckets = computed(() => (hourly.value ? report.value.hourly : report.value?.daily) ?? [])
const series = (key) => buckets.value.map((b) => ({ t: wall(b.t), v: b[key] }))

// A round top for the scale whose half is a whole number: 2, 4, 6, 8, 10, 20...
function niceMax(v) {
  if (!(v > 0)) return 2
  const m = 10 ** Math.floor(Math.log10(v))
  for (const f of [1, 2, 4, 6, 8, 10]) if (f * m >= v) return Math.max(2, f * m)
  return 10 * m
}
const maxOf = (key) => niceMax(Math.max(0, ...buckets.value.map((b) => b[key])))

// ---- top lists ----
const agentName = (name) => (name === '' ? t('stats.no_agent') : name === 'Other' ? t('stats.other') : name === 'Other bot' ? t('stats.other_bot') : name)

const tables = computed(() => {
  const r = report.value
  if (!r) return []
  return [
    { key: 'paths', title: t('stats.top_pages'), col: t('stats.col_page'), items: r.paths || [], mono: true, bytes: true },
    { key: 'ips', title: t('stats.top_ips'), col: t('stats.col_ip'), items: r.ips || [], mono: true, bytes: true },
    { key: 'referrers', title: t('stats.top_referrers'), col: t('stats.col_referrer'), items: r.referrers || [], empty: t('stats.no_referrers') },
    { key: 'browsers', title: t('stats.browsers'), col: t('stats.col_browser'), items: r.browsers || [], agent: true },
    { key: 'bots', title: t('stats.bots'), col: t('stats.col_bot'), items: r.bots || [], agent: true },
  ]
})
</script>

<template>
  <div class="site-stats stack">
    <div class="row stats-bar">
      <div class="row" role="group" :aria-label="t('stats.range')">
        <button
          v-for="d in ranges"
          :key="d"
          type="button"
          class="btn btn-sm"
          :class="{ 'btn-primary': days === d }"
          :aria-pressed="days === d"
          @click="days = d"
        >
          {{ t('stats.range_' + d) }}
        </button>
      </div>
      <span v-if="loading" class="muted small" role="status">{{ t('stats.loading') }}</span>
    </div>

    <p v-if="error" class="alert alert-crit" role="alert">{{ error }}</p>
    <div v-if="!report && !error" class="muted">…</div>

    <div v-else-if="noLogs" class="empty">
      <div class="empty-icon"><Icon name="activity" /></div>
      <p class="empty-lead">{{ t('stats.no_logs') }}</p>
      <p class="muted">{{ t('webserver.stats_no_logs_hint', { server: serverName }) }}</p>
    </div>

    <template v-else-if="report">
      <div class="stats-body stack" :class="{ stale: loading }">
        <p v-if="report.truncated" class="alert alert-warn">{{ t('stats.truncated', { n: full(report.lines) }, report.lines) }}</p>
        <p v-if="report.logs_since" class="alert alert-warn">{{ t('stats.logs_since', { date: fmtDate(report.logs_since) }) }}</p>

        <div v-if="noRequests" class="empty">
          <div class="empty-icon"><Icon name="activity" /></div>
          <p class="empty-lead">{{ t('stats.no_requests') }}</p>
          <p class="muted">{{ t('stats.no_requests_hint') }}</p>
        </div>

        <template v-else>
          <div class="stats-tiles">
            <div v-for="tile in tiles" :key="tile.key" class="card stat">
              <div class="stat-top">
                <div class="stat-label">{{ tile.label }}</div>
                <span class="stat-icon"><Icon :name="tile.icon" /></span>
              </div>
              <div class="stat-value">{{ tile.value }}</div>
              <div class="stat-detail">{{ tile.detail || ' ' }}</div>
            </div>
          </div>

          <section class="card">
            <div class="grid-charts">
              <LineChart
                :title="hourly ? t('stats.chart_requests_hour') : t('stats.chart_requests_day')"
                :points="series('requests')"
                :max="maxOf('requests')"
                unit=""
                :locale="locale"
              />
              <LineChart
                :title="hourly ? t('stats.chart_visitors_hour') : t('stats.chart_visitors_day')"
                :points="series('visitors')"
                :max="maxOf('visitors')"
                unit=""
                :locale="locale"
              />
            </div>
          </section>

          <div class="grid-2 stats-tables">
            <section v-for="tb in tables" :key="tb.key" class="card">
              <h3 class="stats-title">{{ tb.title }}</h3>
              <p v-if="!tb.items.length" class="muted small">{{ tb.empty || t('stats.none') }}</p>
              <div v-else class="table-wrap">
                <table class="table small">
                  <thead>
                    <tr>
                      <th>{{ tb.col }}</th>
                      <th class="num">{{ t('stats.requests') }}</th>
                      <th v-if="tb.bytes" class="num">{{ t('stats.bandwidth') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="it in tb.items" :key="it.name">
                      <td class="name-col">
                        <div :class="{ mono: tb.mono }" class="item-name" :title="it.name">{{ tb.agent ? agentName(it.name) : it.name }}</div>
                        <div class="bar" aria-hidden="true">
                          <div class="bar-fill" :style="{ width: `${Math.max(share(it.requests, totals.requests), 0.5)}%` }"></div>
                        </div>
                      </td>
                      <td class="num">
                        {{ full(it.requests) }}
                        <span class="muted">{{ pct(share(it.requests, totals.requests)) }}</span>
                      </td>
                      <td v-if="tb.bytes" class="num">{{ formatBytes(it.bytes) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </section>
          </div>
        </template>

        <p class="muted small">
          {{ t('webserver.stats_source', { server: serverName, time: fmtDate(report.to) }) }}
          <template v-if="report.malformed"> {{ t('stats.malformed', { n: full(report.malformed) }, report.malformed) }}</template>
          <template v-if="report.approximate"> {{ t('stats.approximate') }}</template>
        </p>
      </div>
    </template>
  </div>
</template>

<style scoped>
.stats-bar { justify-content: space-between; }
.stats-body { transition: opacity 0.2s; }
.stats-body.stale { opacity: 0.55; }
.stats-tiles { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; }
.stats-tiles .card { margin-bottom: 0; padding: 16px; }
.stats-tiles .stat-value { font-size: 24px; margin: 4px 0 0; }
.stats-tiles .stat-detail { margin-top: 6px; }
.site-stats .card { margin-bottom: 0; }
.stats-tables { margin-bottom: 0; }
.stats-title { font-size: 14px; margin: 0 0 12px; }
.name-col { max-width: 260px; }
.item-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.num { text-align: right !important; white-space: nowrap; font-variant-numeric: tabular-nums; }
.num .muted { margin-left: 4px; font-size: 11.5px; }
.bar { height: 4px; margin-top: 5px; border-radius: 999px; background: var(--accent-soft); overflow: hidden; }
.bar-fill { height: 100%; border-radius: 999px; background: var(--accent); }
</style>
