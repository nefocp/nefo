<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatDateTime, formatNumber, relativeTime } from '../format.js'
import { load, put, list } from '../data.js'
import { usePageMeta } from '../page.js'
import Icon from '../components/Icon.vue'

// The security check (internal/seccheck): findings with their severity, a
// one-line explanation and the page that fixes them; the checks that
// passed; a run on demand and the daily run's settings.
const { t, te, locale } = useI18n()
const PATH = 'api/security/check'

const data = ref(null)
const error = ref('')
const running = ref(false)
const tab = ref('open') // open | accepted
const showPassed = ref(false)

async function refresh() {
  try {
    data.value = await load(PATH, { force: true })
  } catch (e) {
    error.value = errorMessage(e)
  }
}
onMounted(refresh)

function keep(d) {
  data.value = d
  put(PATH, d) // the dashboard and the bell read it too
}

async function run() {
  error.value = ''
  running.value = true
  try {
    keep(await api('POST', PATH, {}))
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    running.value = false
  }
}

async function saveSettings(patch) {
  error.value = ''
  try {
    keep(await api('PUT', 'api/security/check/settings', { ...data.value.settings, ...patch }))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function accept(f, accepted) {
  error.value = ''
  try {
    keep(await api('POST', 'api/security/check/accept', { id: f.id, accepted }))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const report = computed(() => data.value?.report || null)
const counts = computed(() => data.value?.counts || { critical: 0, warning: 0, info: 0, accepted: 0 })
const findings = computed(() => list(report.value?.findings))
const shown = computed(() => findings.value.filter((f) => (tab.value === 'accepted' ? f.accepted : !f.accepted)))
const passed = computed(() => list(report.value?.passed))
const skipped = computed(() => list(report.value?.skipped))
const partial = computed(() => list(report.value?.partial))
const openCount = computed(() => counts.value.critical + counts.value.warning + counts.value.info)

const sevClass = { critical: 'crit', warning: 'warn', info: 'info' }
const sevIcon = { critical: 'shieldAlert', warning: 'alert', info: 'info' }

// The texts' placeholders: the finding's params and extra, with a few
// values put into words.
function params(f) {
  const p = { ...(f.params || {}), ...(f.extra || {}) }
  if (p.exposure) p.exposure = t('seccheck.exposure.' + p.exposure)
  if (p.checked) p.checked = formatDateTime(p.checked, locale.value)
  if (f.check === 'wp_updates') p.core = p.core || '—'
  if (p.n) p.n = Number(p.n)
  return p
}
const title = (f) => {
  const p = params(f)
  return t(`seccheck.c.${f.check}.title`, p, p.n ?? 1)
}
function text(f) {
  const base = `seccheck.c.${f.check}.`
  let key = base + 'text'
  if (f.params?.custom === 'true' && te(base + 'text_custom')) key = base + 'text_custom'
  if (f.check === 'redis_no_password') key = base + (f.params?.where === 'public' ? 'text_public' : 'text_local')
  if (f.check === 'php_display_errors' && f.params?.scope !== 'all') key = base + 'text_pools'
  return t(key, params(f))
}
const fixLabel = (f) => t('seccheck.fix_go', { page: te('nav.' + navName(f.fix.page)) ? t('nav.' + navName(f.fix.page)) : f.fix.page })
const navName = (p) => ({ security: 'account', 'security-check': 'security_check' })[p] || p
const fixTo = (f) => ({ name: f.fix.page, query: f.fix.query || {} })

const names = (keys) => keys.map((k) => (te('seccheck.name.' + k) ? t('seccheck.name.' + k) : k)).join(', ')
const tookText = computed(() => (report.value ? t('seccheck.took', { s: formatNumber((report.value.millis || 0) / 1000, locale.value, 1) }) : ''))

const kpis = computed(() => [
  { key: 'critical', icon: 'shieldAlert', color: counts.value.critical ? 'c-red' : 'c-gray', n: counts.value.critical, label: t('seccheck.k_critical') },
  { key: 'warning', icon: 'alert', color: counts.value.warning ? 'c-orange' : 'c-gray', n: counts.value.warning, label: t('seccheck.k_warning') },
  { key: 'info', icon: 'info', color: counts.value.info ? 'c-blue' : 'c-gray', n: counts.value.info, label: t('seccheck.k_info') },
  { key: 'passed', icon: 'checkCircle', color: 'c-green', n: passed.value.length, label: t('seccheck.k_passed') },
])
const state = computed(() => (counts.value.critical ? 'crit' : counts.value.warning ? 'warn' : 'ok'))

usePageMeta(() => ({ sub: report.value ? t('seccheck.last_run', { when: relativeTime(report.value.checked_at, locale.value) }) : t('seccheck.subtitle') }))
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('seccheck.title') }}</h1></div>
    <Teleport defer to="#topbar-actions">
      <button v-if="data" class="btn btn-primary" :disabled="running" @click="run">
        <span v-if="running" class="spinner" aria-hidden="true"></span><Icon v-else name="shieldCheck" :size="15" />{{ running ? t('seccheck.running') : t('seccheck.run') }}
      </button>
    </Teleport>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>

    <template v-if="!data && !error">
      <span class="skel" style="height: 96px; border-radius: 14px"></span>
      <div class="kpis"><span v-for="i in 4" :key="i" class="skel" style="height: 72px; border-radius: 14px"></span></div>
    </template>

    <template v-if="data">
      <!-- summary -->
      <section class="svc-head sc-head" :class="report ? 'sc-' + state : ''">
        <span class="sc-badge" :class="report ? state : 'none'" aria-hidden="true">
          <Icon :name="!report ? 'shield' : state === 'ok' ? 'shieldCheck' : 'shieldAlert'" :size="28" :stroke="1.8" />
        </span>
        <div class="svc-text">
          <h2 class="svc-title">
            {{ t('seccheck.title') }}
            <span v-if="report" class="status-pill" :class="state">
              {{ state === 'ok' ? t('seccheck.none_open') : t('seccheck.problems', { n: counts.critical + counts.warning }, counts.critical + counts.warning) }}
            </span>
          </h2>
          <div class="svc-meta">
            <span v-if="report"><Icon name="clock" :size="14" />{{ t('seccheck.last_run', { when: relativeTime(report.checked_at, locale) }) }} · {{ tookText }}</span>
            <span v-if="partial.length" class="text-warn" :title="t('seccheck.partial', { list: names(partial) })"><Icon name="alert" :size="14" />{{ t('seccheck.partial_short') }}</span>
            <span v-else>{{ t('seccheck.never') }}</span>
          </div>
        </div>
        <div class="sc-settings">
          <label class="sc-switch">
            <button type="button" class="switch switch-sm" role="switch" :aria-checked="data.settings.daily" @click="saveSettings({ daily: !data.settings.daily })"></button>
            <span>{{ t('seccheck.daily') }}</span>
          </label>
          <label class="sc-switch" :title="t('seccheck.notify_hint')">
            <button type="button" class="switch switch-sm" role="switch" :aria-checked="data.settings.notify" :disabled="!data.settings.daily" @click="saveSettings({ notify: !data.settings.notify })"></button>
            <span>{{ t('seccheck.notify') }}</span>
          </label>
        </div>
      </section>

      <!-- never run -->
      <section v-if="!report" class="card empty">
        <div class="empty-icon"><Icon name="shieldCheck" :size="26" /></div>
        <p class="empty-lead">{{ t('seccheck.empty_lead') }}</p>
        <p class="muted">{{ t('seccheck.empty_text') }}</p>
        <button class="btn btn-primary" :disabled="running" @click="run">
          <span v-if="running" class="spinner" aria-hidden="true"></span><Icon v-else name="shieldCheck" :size="15" />{{ running ? t('seccheck.running') : t('seccheck.run') }}
        </button>
        <p class="muted small sc-cost">{{ t('seccheck.cost') }}</p>
      </section>

      <template v-else>
        <div class="kpis sc-kpis">
          <div v-for="k in kpis" :key="k.key" class="kpi">
            <span class="kpi-ico" :class="k.color"><Icon :name="k.icon" :size="20" /></span>
            <div>
              <div class="kpi-val">{{ formatNumber(k.n, locale) }}</div>
              <div class="kpi-label">{{ k.label }}</div>
            </div>
          </div>
        </div>

        <section class="panel">
          <div class="panel-tools">
            <div class="tabs" role="group" :aria-label="t('seccheck.title')">
              <button type="button" :aria-pressed="tab === 'open'" @click="tab = 'open'">{{ t('seccheck.tab_open') }} <span class="sc-n">{{ openCount }}</span></button>
              <button type="button" :aria-pressed="tab === 'accepted'" @click="tab = 'accepted'">{{ t('seccheck.tab_accepted') }} <span class="sc-n">{{ counts.accepted }}</span></button>
            </div>
            <span v-if="tab === 'accepted'" class="muted small">{{ t('seccheck.accepted_note') }}</span>
          </div>

          <div v-if="!shown.length" class="empty sc-empty">
            <template v-if="tab === 'open'">
              <div class="empty-icon sc-ok-icon"><Icon name="checkCircle" :size="26" /></div>
              <p class="empty-lead">{{ t('seccheck.none_open') }}</p>
              <p class="muted">{{ t('seccheck.none_open_sub') }}</p>
            </template>
            <p v-else class="muted">{{ t('seccheck.none_accepted') }}</p>
          </div>

          <ul v-else class="sc-list">
            <li v-for="f in shown" :key="f.id" class="sc-item" :class="'sc-' + sevClass[f.severity]">
              <span class="sev" :class="sevClass[f.severity]"><Icon :name="sevIcon[f.severity]" :size="16" /></span>
              <div class="sc-body">
                <div class="sc-title">
                  <span class="tag" :class="'tag-' + (f.severity === 'info' ? 'info' : sevClass[f.severity])">{{ t('seccheck.sev_' + f.severity) }}</span>
                  <span>{{ title(f) }}</span>
                </div>
                <div class="sc-text">{{ text(f) }}</div>
                <div v-if="f.check === 'wp_updates'" class="muted small">{{ t('seccheck.wp_hint') }}</div>
              </div>
              <div class="sc-actions">
                <RouterLink v-if="f.fix && !f.accepted" :to="fixTo(f)" class="btn btn-sm" :class="{ 'btn-primary': f.severity === 'critical' }">
                  {{ fixLabel(f) }}<Icon name="arrowRight" :size="14" />
                </RouterLink>
                <button v-if="!f.accepted" type="button" class="btn btn-sm btn-ghost" :title="t('seccheck.accept_hint')" @click="accept(f, true)">{{ t('seccheck.accept') }}</button>
                <button v-else type="button" class="btn btn-sm" @click="accept(f, false)"><Icon name="eye" :size="14" />{{ t('seccheck.restore') }}</button>
              </div>
            </li>
          </ul>
        </section>

        <section class="panel">
          <button type="button" class="panel-head sc-passed-head" :aria-expanded="showPassed" @click="showPassed = !showPassed">
            <h2 class="panel-title"><Icon name="checkCircle" :size="17" />{{ t('seccheck.passed_title') }}<span class="count-badge ok">{{ passed.length }}</span></h2>
            <Icon :name="showPassed ? 'chevronDown' : 'chevronRight'" :size="16" />
          </button>
          <ul v-if="showPassed" class="sc-passed">
            <li v-for="k in passed" :key="k"><Icon name="check" :size="15" />{{ te('seccheck.ok.' + k) ? t('seccheck.ok.' + k) : k }}</li>
          </ul>
          <div class="panel-body sc-foot">
            <p v-if="skipped.length" class="muted small">{{ t('seccheck.skipped', { list: names(skipped) }) }}</p>
            <p v-if="partial.length" class="small text-warn">{{ t('seccheck.partial', { list: names(partial) }) }}</p>
            <p class="muted small">{{ t('seccheck.cost') }}</p>
          </div>
        </section>
      </template>
    </template>
  </div>
</template>

<style scoped>
.sc-head { position: relative; overflow: hidden; }
.sc-head::before { content: ''; position: absolute; inset: 0 auto 0 0; width: 4px; background: var(--border); }
.sc-head.sc-crit::before { background: var(--crit-g); }
.sc-head.sc-warn::before { background: var(--warn-g, var(--warn)); }
.sc-head.sc-ok::before { background: var(--ok-g); }
.sc-badge { width: 56px; height: 56px; border-radius: 16px; display: grid; place-items: center; flex: none; }
.sc-badge.crit { background: var(--crit-soft); color: var(--crit); }
.sc-badge.warn { background: var(--warn-soft); color: var(--warn); }
.sc-badge.ok { background: var(--ok-soft); color: var(--ok); }
.sc-badge.none { background: var(--accent-soft); color: var(--accent); }
.sc-settings { display: flex; flex-direction: column; gap: 8px; }
.sc-switch { display: flex; align-items: center; gap: 9px; font-size: 13px; font-weight: 500; cursor: pointer; }
.sc-cost { margin-top: 14px !important; }
.sc-n { font-size: 11.5px; color: var(--muted); font-weight: 700; margin-left: 2px; }
.sc-list { list-style: none; margin: 0; padding: 0; }
.sc-item { display: flex; gap: 12px; align-items: flex-start; padding: 14px 18px; border-bottom: 1px solid var(--border); position: relative; }
.sc-item:last-child { border-bottom: 0; }
.sc-item::before { content: ''; position: absolute; left: 0; top: 10px; bottom: 10px; width: 3px; border-radius: 0 3px 3px 0; }
.sc-item.sc-crit::before { background: var(--crit-g); }
.sc-item.sc-warn::before { background: var(--warn-g, var(--warn)); }
.sc-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.sc-title { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; font-weight: 700; font-size: 14px; overflow-wrap: anywhere; }
.sc-text { color: var(--text-2); font-size: 13px; line-height: 1.5; overflow-wrap: anywhere; }
.sc-actions { display: flex; gap: 6px; flex-wrap: wrap; justify-content: flex-end; flex: none; }
.sc-empty { padding: 36px 20px; }
.sc-ok-icon { background: var(--ok-soft); color: var(--ok); }
.sc-passed-head { width: 100%; border: 0; background: none; font: inherit; color: inherit; cursor: pointer; text-align: left; }
.sc-passed-head .panel-title { display: flex; align-items: center; gap: 8px; }
.sc-passed { list-style: none; margin: 0; padding: 4px 18px 8px; display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 6px 18px; }
.sc-passed li { display: flex; align-items: center; gap: 8px; font-size: 13px; padding: 6px 0; color: var(--text); }
.sc-passed li :deep(svg) { color: var(--ok); flex: none; }
.sc-foot { display: flex; flex-direction: column; gap: 6px; padding-top: 10px; }
.sc-foot p { margin: 0; }
@media (max-width: 640px) {
  .sc-item { flex-wrap: wrap; padding: 12px 14px; }
  .sc-actions { width: 100%; justify-content: flex-start; padding-left: 42px; }
  .sc-settings { width: 100%; }
}
</style>
