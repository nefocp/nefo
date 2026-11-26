<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { toast } from '../toast.js'
import { load as loadCached } from '../data.js'
import { relativeTime, formatNumber } from '../format.js'
import LineChart from './LineChart.vue'

// SiteMonitor is the Monitoring tab of a website: the panel asks for the
// website every few minutes, through DNS like a visitor, and sends an alert
// when it stops answering (internal/uptime). The numbers come back with
// every save, and again every minute while the tab is open.
const props = defineProps({ site: { type: Object, required: true } })
const { t, locale } = useI18n()

const INTERVALS = [1, 2, 5, 10]
const data = ref(null)
const error = ref('')
const saving = ref(false)
const form = reactive({ interval: 5, path: '/', keyword: '', strict_4xx: false })
const KEYS = ['interval', 'path', 'keyword', 'strict_4xx']

function fill(d, keepForm = false) {
  data.value = d
  if (!keepForm) for (const k of KEYS) form[k] = d.settings[k]
}

async function load() {
  try {
    // A refresh keeps what is being typed.
    fill(await api('GET', `api/sites/${props.site.id}/monitor`), !!data.value && changed.value)
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

let timer = 0
onMounted(() => {
  load()
  timer = setInterval(() => {
    if (data.value?.settings.enabled && !saving.value) load()
  }, 60000)
})
onBeforeUnmount(() => clearInterval(timer))

const on = computed(() => !!data.value?.settings.enabled)
const changed = computed(() => !!data.value && KEYS.some((k) => form[k] !== data.value.settings[k]))

async function save(enabled) {
  saving.value = true
  try {
    const body = { enabled, interval: Number(form.interval), path: form.path.trim() || '/', keyword: form.keyword.trim(), strict_4xx: form.strict_4xx }
    fill(await api('PUT', `api/sites/${props.site.id}/monitor`, body))
    loadCached('api/uptime', { force: true }).catch(() => {}) // the website list and the bell
    toast(t('drawer.saved'))
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  } finally {
    saving.value = false
  }
}

// ---- where it stands ----
const st = computed(() => data.value?.status || null)
const state = computed(() => {
  if (!on.value) return null
  if (data.value.paused) return { cls: 'warn', text: t('uptime.paused') }
  if (!st.value || st.value.state === 'pending') return { cls: '', text: t('uptime.pending') }
  if (st.value.state === 'down') return { cls: 'crit', text: t('uptime.down') }
  return { cls: 'ok', text: t('uptime.up') }
})
const fmtTime = (v) => new Date(v).toLocaleString(locale.value, { dateStyle: 'medium', timeStyle: 'short' })
const ms = (v) => `${formatNumber(v, locale.value)} ms`
const reason = (c) => c.error || (c.code ? `HTTP ${c.code}` : '')
const last = computed(() => st.value?.last || null)
const lastLine = computed(() => {
  const c = last.value
  if (!c) return ''
  return [t('uptime.last_check', { when: relativeTime(c.t, locale.value) }), c.ok ? (c.code ? `HTTP ${c.code}` : '') : reason(c), c.ok ? ms(c.ms) : ''].filter(Boolean).join(' · ')
})

function pct(s) {
  if (!s?.checks) return '—'
  const p = (s.up / s.checks) * 100
  return `${formatNumber(p, locale.value, p === 100 ? 0 : 2)}%`
}
const facts = computed(() => {
  const d = data.value
  if (!d) return []
  const out = [
    { key: 'day', label: t('uptime.uptime_day'), value: pct(d.day) },
    { key: 'week', label: t('uptime.uptime_week'), value: pct(d.week) },
    { key: 'avg', label: t('uptime.avg'), value: d.day?.up ? ms(Math.round(d.day.avg_ms)) : '—' },
  ]
  if (st.value?.cert_expires) out.push({ key: 'cert', label: t('uptime.cert'), value: new Date(st.value.cert_expires).toLocaleDateString(locale.value, { dateStyle: 'medium' }) })
  return out
})

// ---- response times, 24 hours ----
const points = computed(() => (data.value?.points || []).map((p) => ({ t: p.t, v: p.v })))
function niceMax(v) {
  if (!(v > 0)) return 100
  const m = 10 ** Math.floor(Math.log10(v))
  for (const f of [1, 2, 4, 6, 8, 10]) if (f * m >= v) return Math.max(100, f * m)
  return 10 * m
}
const chartMax = computed(() => niceMax(Math.max(0, ...points.value.map((p) => p.v))))
const failures = computed(() => data.value?.failures || [])
</script>

<template>
  <p v-if="error" class="alert alert-crit" role="alert">{{ error }}</p>
  <div v-else-if="!data" class="muted">…</div>

  <template v-else>
    <div class="opt mon-opt">
      <div class="opt-text">
        <div class="opt-title">{{ t('uptime.switch') }}</div>
        <div class="opt-hint">{{ on ? t('uptime.on_hint', { n: data.settings.interval }, data.settings.interval) : t('uptime.off_hint') }}</div>
      </div>
      <button type="button" class="switch" role="switch" :aria-checked="on" :aria-label="t('uptime.switch')" :disabled="saving" @click="save(!on)"></button>
    </div>

    <template v-if="on">
      <section class="card stack">
        <div class="row mon-state">
          <span class="status-pill" :class="state.cls">{{ state.text }}</span>
          <span v-if="st?.state === 'down' && st.since" class="small text-crit">{{ t('uptime.down_since', { when: fmtTime(st.since) }) }}</span>
          <span v-if="lastLine" class="small muted">{{ lastLine }}</span>
        </div>
        <p v-if="data.paused" class="muted small mon-text">{{ t('uptime.paused_hint') }}</p>
        <dl class="kv">
          <div v-for="f in facts" :key="f.key">
            <dt>{{ f.label }}</dt>
            <dd class="num">{{ f.value }}</dd>
          </div>
        </dl>
        <LineChart v-if="points.length > 1" :title="t('uptime.chart')" :points="points" :max="chartMax" unit=" ms" :locale="locale" />
      </section>

      <section v-if="failures.length" class="card">
        <h3 class="mon-title">{{ t('uptime.failures') }}</h3>
        <div class="table-wrap">
          <table class="table small">
            <thead>
              <tr>
                <th>{{ t('uptime.col_time') }}</th>
                <th>{{ t('uptime.col_problem') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="f in failures" :key="f.t">
                <td class="nowrap">{{ fmtTime(f.t) }}</td>
                <td class="mon-err">{{ reason(f) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="card">
        <form class="stack" @submit.prevent="save(true)">
          <div class="mon-grid">
            <label class="field">
              <span>{{ t('uptime.interval') }}</span>
              <select v-model.number="form.interval" class="input">
                <option v-for="n in INTERVALS" :key="n" :value="n">{{ t('uptime.every', { n }, n) }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ t('uptime.path') }}</span>
              <input v-model="form.path" class="input mono" maxlength="300" spellcheck="false" autocomplete="off" placeholder="/" />
            </label>
          </div>
          <label class="field">
            <span>{{ t('uptime.keyword') }}</span>
            <input v-model="form.keyword" class="input" maxlength="200" autocomplete="off" />
            <small class="muted">{{ t('uptime.keyword_hint') }}</small>
          </label>
          <label class="check">
            <input v-model="form.strict_4xx" type="checkbox" />
            <span>{{ t('uptime.strict_4xx') }}</span>
          </label>
          <div class="row">
            <button class="btn btn-primary" :disabled="saving || !changed">{{ t('uptime.save') }}</button>
          </div>
        </form>
      </section>
    </template>

    <p class="muted small mon-text">
      {{ t('uptime.hint') }} <RouterLink :to="{ name: 'settings', query: { tab: 'alerts' } }">{{ t('uptime.alerts_link') }}</RouterLink>
    </p>
  </template>
</template>

<style scoped>
.mon-opt { margin-bottom: 16px; }
.mon-state { gap: 10px; flex-wrap: wrap; }
.mon-text { margin: 0; }
.mon-title { font-size: 14px; margin: 0 0 12px; }
.mon-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 14px; }
.mon-err { overflow-wrap: anywhere; }
.num { font-variant-numeric: tabular-nums; }
</style>
