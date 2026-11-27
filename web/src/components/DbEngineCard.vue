<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytesL, formatDateTime } from '../format.js'
import Icon from './Icon.vue'
import AppLogo from './AppLogo.vue'
import Modal from './Modal.vue'

// DbEngineCard is the Software page's database server: a server has
// MariaDB or MySQL 8.4, never both. While neither is installed it offers the
// choice (MariaDB recommended, the distribution's or an LTS series from
// MariaDB's repository); where Oracle has no MySQL packages for the server,
// MySQL's item says why (not_offered). Once one is installed, the card shows
// it with the other one as the alternative: Switch moves the databases over
// (GET/POST api/db/switch), Upgrade takes MariaDB to a newer series
// (api/db/upgrade), and the dumps both keep are listed (api/db/engine).
const props = defineProps({
  mariadb: { type: Object, required: true },
  mysql: { type: Object, default: null },
  series: { type: Array, default: () => [] }, // mariadb-11.4… items
  busy: { type: Boolean, default: false },
  working: { type: Boolean, default: false },
  supported: { type: Boolean, default: true },
  stamp: { type: Number, default: 0 }, // changes after every job
})
const emit = defineEmits(['install', 'remove', 'job', 'error'])
const { t, te, locale } = useI18n()

const reasonText = (code, detail) => (te(`errors.${code}`) ? t(`errors.${code}`, { detail: detail ?? '' }) : code)

// ---- not installed: the choice ----
const installed = computed(() => (props.mysql?.installed ? props.mysql : props.mariadb.installed ? props.mariadb : null))
const mysqlOffered = computed(() => !!props.mysql && !props.mysql.not_offered)
const seriesOffered = computed(() => props.series.filter((s) => !s.not_offered))
const choice = ref('mariadb')
const mariaVersion = ref('mariadb') // "mariadb" (the distribution's) or a series' ID
const chosen = computed(() => {
  if (choice.value === 'mysql' && mysqlOffered.value) return props.mysql
  return seriesOffered.value.find((s) => s.id === mariaVersion.value) || props.mariadb
})
const chosenName = computed(() => (chosen.value.kind === 'mysql' ? t('software.mysql') : chosen.value.id === 'mariadb' ? t('software.mariadb') : `MariaDB ${chosen.value.version}`))
const whyNot = computed(() => (props.mysql?.not_offered ? reasonText(props.mysql.not_offered, props.mysql.not_offered_detail) : ''))

// ---- installed: the engine's details ----
const engine = ref(null) // GET api/db/engine
const isMaria = computed(() => installed.value?.kind === 'mariadb')
const otherKind = computed(() => (isMaria.value ? 'mysql' : 'mariadb'))
const otherName = computed(() => (isMaria.value ? t('software.mysql') : t('software.mariadb')))
const sourceText = computed(() => {
  const src = engine.value?.source
  return src && te(`software.db_source_${src}`) ? t(`software.db_source_${src}`) : ''
})
const upgrades = computed(() => engine.value?.upgrades || [])
const upgradesOffered = computed(() => upgrades.value.filter((u) => !u.blocked))
const switchBlocked = computed(() => {
  const b = engine.value?.switch_blocked
  return b ? reasonText(b.code, b.detail) : ''
})
const dumps = computed(() => engine.value?.dumps || [])
const blockedText = computed(() => {
  const b = installed.value?.blocked
  return b ? reasonText(b, installed.value.blocked_detail) : ''
})
const ram = computed(() => t('software.ram_about', { mb: installed.value?.ram_mb || 0 }))

async function loadEngine() {
  if (!installed.value) {
    engine.value = null
    return
  }
  try {
    engine.value = await api('GET', 'api/db/engine')
  } catch (e) {
    emit('error', errorMessage(e))
  }
}
onMounted(loadEngine)
watch(() => [props.stamp, installed.value?.id], loadEngine)

// ---- upgrade ----
const upgrading = ref(false)
const upSeries = ref('')
const upPlan = ref(null)
const upBlocked = ref('')
const upError = ref('')
async function loadUpgradePlan() {
  upPlan.value = null
  upBlocked.value = upError.value = ''
  if (!upSeries.value) return
  try {
    const res = await api('GET', `api/db/upgrade?series=${encodeURIComponent(upSeries.value)}`)
    upPlan.value = res.plan
    if (res.blocked) upBlocked.value = reasonText(res.blocked.code, res.blocked.detail)
  } catch (e) {
    upError.value = errorMessage(e)
  }
}
function openUpgrade() {
  upSeries.value = upgradesOffered.value[upgradesOffered.value.length - 1]?.series || ''
  upgrading.value = true
  loadUpgradePlan()
}
async function startUpgrade() {
  upError.value = ''
  try {
    const snap = await api('POST', 'api/db/upgrade', { series: upSeries.value })
    upgrading.value = false
    emit('job', snap)
  } catch (e) {
    upError.value = errorMessage(e)
  }
}

// ---- switch ----
const switching = ref(false)
const swPlan = ref(null)
const swError = ref('')
const typed = ref('')
const swProblems = computed(() => (swPlan.value?.problems || []).map((p) => reasonText(p.code, p.detail)))
const swOK = computed(() => !!swPlan.value && !swProblems.value.length && typed.value.trim() === otherKind.value)
const HOW = { password: 'ok', hash: 'ok', socket: 'ok', kept: 'info', new: 'warn', skip: 'crit' }
async function openSwitch() {
  swPlan.value = null
  swError.value = typed.value = ''
  switching.value = true
  try {
    swPlan.value = await api('GET', `api/db/switch?to=${otherKind.value}`)
  } catch (e) {
    swError.value = errorMessage(e)
  }
}
async function startSwitch() {
  swError.value = ''
  try {
    const snap = await api('POST', 'api/db/switch', { to: otherKind.value, confirm: typed.value.trim() })
    switching.value = false
    emit('job', snap)
  } catch (e) {
    swError.value = errorMessage(e)
  }
}
const mb = (n) => formatBytesL(Math.max(n, 0) * 1048576, locale.value)
const bytes = (n) => formatBytesL(n, locale.value)
const engineLabel = (e) => (e === 'mysql' ? 'MySQL' : 'MariaDB')

// ---- dumps ----
async function deleteDump(d) {
  if (!window.confirm(t('software.db_dump_delete_confirm', { path: d.path }))) return
  try {
    await api('DELETE', `api/db/dumps/${encodeURIComponent(d.name)}`)
    await loadEngine()
  } catch (e) {
    emit('error', errorMessage(e))
  }
}
const resultTag = { done: 'tag-ok', rolled_back: 'tag-warn', failed: 'tag-crit' }
</script>

<template>
  <!-- installed: the engine, its upgrade, the other engine, the dumps -->
  <section v-if="installed" class="dbe" :aria-busy="working">
    <div class="dbe-main">
      <div class="dbe-head">
        <AppLogo :name="installed.kind" icon="database" />
        <div class="dbe-name">
          <h3>{{ t('software.db_engine') }} · {{ isMaria ? t('software.mariadb') : t('software.mysql') }}</h3>
          <div class="muted small">
            <span v-if="installed.version" class="mono">{{ installed.version }}</span>
            <span v-if="sourceText"> · {{ sourceText }}</span>
          </div>
        </div>
        <span v-if="working" class="status-pill warn"><span class="spinner" aria-hidden="true"></span>{{ t('software.working') }}</span>
        <RouterLink v-else-if="installed.stopped" class="status-pill warn" :to="{ name: 'services' }" :title="t('software.stopped_hint')">{{ t('pg.svc_state_inactive') }}</RouterLink>
        <span v-else class="status-pill ok">{{ t('software.installed') }}</span>
      </div>
      <p class="muted small dbe-desc">{{ t(`software.${installed.kind}_hint`) }}</p>
      <div class="dbe-meta">
        <span><Icon name="memory" :size="13" />{{ ram }}</span>
        <span v-if="installed.usage?.databases"><Icon name="database" :size="13" />{{ t('software.usage_databases', { n: installed.usage.databases }, installed.usage.databases) }}</span>
      </div>
      <p v-if="blockedText" class="dbe-blocked small"><Icon name="alert" :size="14" />{{ blockedText }}</p>
      <div class="dbe-actions">
        <RouterLink class="btn btn-sm" :to="{ name: 'databases' }">{{ t('software.open_databases') }}</RouterLink>
        <button v-if="upgradesOffered.length" class="btn btn-sm" :disabled="busy || !supported" @click="openUpgrade">
          <Icon name="up" :size="15" />{{ t('software.db_upgrade') }}
        </button>
        <span class="dbe-gap"></span>
        <button v-if="installed.removable || installed.blocked" class="btn btn-sm btn-ghost dbe-remove" :disabled="busy || !installed.removable" @click="emit('remove', installed)">
          <Icon name="trash" :size="15" />{{ t('software.uninstall') }}
        </button>
      </div>
    </div>

    <div class="dbe-alt">
      <AppLogo :name="otherKind" icon="database" size="sm" dim />
      <div class="dbe-alt-text">
        <div class="dbe-alt-title">{{ t('software.db_alt_title', { name: otherName }) }}</div>
        <p class="muted small">{{ t('software.db_alt_body', { from: isMaria ? 'MariaDB' : 'MySQL', to: isMaria ? 'MySQL' : 'MariaDB' }) }}</p>
        <p v-if="switchBlocked" class="small dbe-why"><Icon name="info" :size="14" />{{ switchBlocked }}</p>
      </div>
      <button class="btn btn-sm" :disabled="busy || !supported || !!switchBlocked || !engine" @click="openSwitch">
        <Icon name="swap" :size="15" />{{ t('software.db_switch_to', { name: isMaria ? 'MySQL' : 'MariaDB' }) }}
      </button>
    </div>

    <div v-if="dumps.length" class="dbe-dumps">
      <div class="dbe-dumps-head">
        <Icon name="archive" :size="15" /><strong>{{ t('software.db_dumps_title') }}</strong>
        <span class="muted small">{{ t('software.db_dumps_hint') }}</span>
      </div>
      <ul class="dbe-dump-list">
        <li v-for="d in dumps" :key="d.name">
          <div class="dbe-dump-what">
            <span>{{ d.from }} → {{ d.to }}</span>
            <span v-if="d.result" class="tag" :class="resultTag[d.result]">{{ t(`software.db_dump_${d.result}`) }}</span>
          </div>
          <div class="muted small dbe-dump-meta">
            {{ formatDateTime(d.created, locale) }} · {{ t('software.db_dump_dbs', { n: d.databases.length }, d.databases.length) }} · {{ bytes(d.size) }}
            · <span class="mono">{{ d.path }}</span>
          </div>
          <button type="button" class="icon-btn" :aria-label="t('software.db_dump_delete')" :title="t('software.db_dump_delete')" :disabled="busy" @click="deleteDump(d)">
            <Icon name="trash" :size="15" />
          </button>
        </li>
      </ul>
    </div>
  </section>

  <!-- not installed: the choice -->
  <section v-else class="db-card" :aria-busy="working">
    <div class="db-head">
      <AppLogo :name="choice === 'mysql' ? 'mysql' : 'mariadb'" icon="database" dim />
      <div class="db-name">
        <h3>{{ t('software.db_engine') }}</h3>
        <div class="muted small">{{ t('software.db_engine_sub') }}</div>
      </div>
      <span v-if="working" class="status-pill warn"><span class="spinner" aria-hidden="true"></span>{{ t('software.working') }}</span>
      <span v-else class="status-pill">{{ t('software.not_installed') }}</span>
    </div>
    <div class="db-opts" role="radiogroup" :aria-label="t('software.db_engine')">
      <label class="db-opt" :class="{ on: choice === 'mariadb' }">
        <input v-model="choice" type="radio" name="db-engine" value="mariadb" />
        <span class="db-opt-text">
          <span class="db-opt-title">{{ t('software.mariadb') }} <span class="tag tag-ok">{{ t('software.db_recommended') }}</span></span>
          <span class="muted small">{{ t('software.db_choice_mariadb', { mb: mariadb.ram_mb }) }}</span>
        </span>
      </label>
      <div v-if="seriesOffered.length && choice === 'mariadb'" class="db-versions" role="radiogroup" :aria-label="t('software.db_version')">
        <label class="db-ver" :class="{ on: mariaVersion === 'mariadb' }">
          <input v-model="mariaVersion" type="radio" name="db-maria-version" value="mariadb" />
          <span>
            <span class="db-ver-title">{{ mariadb.version ? t('software.db_version_distro', { v: mariadb.version }) : t('software.db_version_distro_any') }}</span>
            <span class="muted small">{{ t('software.db_version_distro_hint') }}</span>
          </span>
        </label>
        <label v-for="s in seriesOffered" :key="s.id" class="db-ver" :class="{ on: mariaVersion === s.id }">
          <input v-model="mariaVersion" type="radio" name="db-maria-version" :value="s.id" />
          <span>
            <span class="db-ver-title">{{ t('software.db_version_series', { v: s.version }) }}</span>
            <span class="muted small">{{ t('software.db_version_series_hint', { date: s.security_until }) }}</span>
          </span>
        </label>
      </div>
      <label v-if="mysqlOffered" class="db-opt" :class="{ on: choice === 'mysql' }">
        <input v-model="choice" type="radio" name="db-engine" value="mysql" />
        <span class="db-opt-text">
          <span class="db-opt-title">{{ t('software.mysql') }}</span>
          <span class="muted small">{{ t('software.db_choice_mysql', { mb: mysql.ram_mb }) }}</span>
        </span>
      </label>
    </div>
    <p v-if="whyNot" class="muted small db-note"><Icon name="info" :size="14" />{{ whyNot }}</p>
    <p class="muted small db-note">{{ t('software.db_switch_note') }}</p>
    <div class="db-actions">
      <button class="btn btn-primary btn-sm" :disabled="busy || !supported" @click="emit('install', chosen)">
        <Icon name="download" :size="15" />{{ t('software.install_engine', { name: chosenName }) }}
      </button>
    </div>
  </section>

  <!-- upgrade MariaDB -->
  <Modal v-if="upgrading" :title="t('software.db_upgrade_title')" wide @close="upgrading = false">
    <form class="stack" @submit.prevent="startUpgrade">
      <p class="dbe-body">{{ t('software.db_upgrade_intro', { from: engine?.version || '' }) }}</p>
      <div class="db-opts" role="radiogroup" :aria-label="t('software.db_version')">
        <label v-for="u in upgrades" :key="u.series" class="db-opt" :class="{ on: upSeries === u.series, off: !!u.blocked }">
          <input v-model="upSeries" type="radio" name="db-upgrade" :value="u.series" :disabled="!!u.blocked" @change="loadUpgradePlan" />
          <span class="db-opt-text">
            <span class="db-opt-title">{{ t('software.db_version_series', { v: u.series }) }}</span>
            <span class="muted small">{{ u.blocked ? reasonText(u.blocked.code, u.blocked.detail) : t('software.db_version_series_hint', { date: u.until }) }}</span>
          </span>
        </label>
      </div>
      <template v-if="upPlan">
        <ol class="dbe-steps">
          <li>{{ t('software.db_upgrade_step1', { n: upPlan.databases.length, dir: upPlan.dumps_dir }) }}</li>
          <li>{{ t('software.db_upgrade_step2') }}</li>
          <li>{{ t('software.db_upgrade_step3', { to: upPlan.to }) }}</li>
          <li>{{ t('software.db_upgrade_step4', { from: upPlan.from }) }}</li>
        </ol>
        <p class="muted small">{{ t('software.db_disk', { need: mb(upPlan.need_mb), free: upPlan.free_mb >= 0 ? mb(upPlan.free_mb) : '?' }) }}</p>
        <p class="alert alert-warn dbe-body" role="note">{{ t('software.db_upgrade_no_back', { to: upPlan.to }) }}</p>
      </template>
      <p v-if="upBlocked" class="alert alert-crit dbe-body" role="alert">{{ upBlocked }}</p>
      <p v-if="upError" class="alert alert-crit pre-wrap dbe-body" role="alert">{{ upError }}</p>
      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="upgrading = false">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="busy || !upPlan || !!upBlocked">
          <Icon name="up" :size="15" />{{ t('software.db_upgrade_start', { v: upSeries }) }}
        </button>
      </div>
    </form>
  </Modal>

  <!-- switch MariaDB <-> MySQL -->
  <Modal v-if="switching" :title="t('software.db_switch_title', { name: otherName })" wide @close="switching = false">
    <form class="stack" @submit.prevent="swOK && startSwitch()">
      <span v-if="!swPlan && !swError" class="skel" style="height: 160px; border-radius: 12px"></span>
      <template v-if="swPlan">
        <p class="dbe-body">{{ t('software.db_switch_intro', { from: `${engineLabel(swPlan.from)} ${swPlan.from_version}`, to: `${engineLabel(swPlan.to)} ${swPlan.to_version}`.trim() }) }}</p>
        <div class="dbe-cols">
          <div>
            <h4 class="dbe-sub">{{ t('software.db_switch_dbs', { n: swPlan.databases.length }, swPlan.databases.length) }}</h4>
            <p v-if="!swPlan.databases.length" class="muted small">{{ t('software.db_switch_no_dbs', { name: engineLabel(swPlan.to) }) }}</p>
            <ul v-else class="dbe-list">
              <li v-for="d in swPlan.databases" :key="d.name"><span class="mono">{{ d.name }}</span><span class="muted small">{{ bytes(d.size) }}</span></li>
            </ul>
          </div>
          <div>
            <h4 class="dbe-sub">{{ t('software.db_switch_accounts', { n: swPlan.accounts.length }, swPlan.accounts.length) }}</h4>
            <p v-if="!swPlan.accounts.length" class="muted small">{{ t('software.db_switch_no_accounts') }}</p>
            <ul v-else class="dbe-list">
              <li v-for="a in swPlan.accounts" :key="`${a.user}@${a.host}`">
                <span class="mono">{{ a.user }}@{{ a.host }}</span>
                <span class="tag" :class="`tag-${HOW[a.how] || 'info'}`" :title="a.why || ''">{{ t(`software.db_how_${a.how}`, { name: engineLabel(swPlan.to) }) }}</span>
              </li>
            </ul>
          </div>
        </div>
        <ul class="dbe-notes small">
          <li>{{ t('software.db_switch_note_offline') }}</li>
          <li>{{ t('software.db_switch_note_kept') }}</li>
          <li v-if="swPlan.remote">{{ t('software.db_switch_note_remote', { n: swPlan.remote }, swPlan.remote) }}</li>
          <li v-if="swPlan.slow_log">{{ t('software.db_switch_note_slow') }}</li>
          <li>{{ t('software.db_switch_note_back', { from: engineLabel(swPlan.from), dir: swPlan.dumps_dir }) }}</li>
        </ul>
        <p class="muted small">{{ t('software.db_disk', { need: mb(swPlan.need_mb), free: swPlan.free_mb >= 0 ? mb(swPlan.free_mb) : '?' }) }}</p>
        <div v-if="swProblems.length" class="alert alert-warn dbe-body" role="alert">
          <strong>{{ t('software.db_switch_problems') }}</strong>
          <ul class="dbe-problems"><li v-for="p in swProblems" :key="p">{{ p }}</li></ul>
        </div>
        <label v-else class="field">
          <span>{{ t('software.type_to_confirm', { word: otherKind }) }}</span>
          <input v-model="typed" class="input mono" autocomplete="off" spellcheck="false" autocapitalize="off" required />
        </label>
      </template>
      <p v-if="swError" class="alert alert-crit pre-wrap dbe-body" role="alert">{{ swError }}</p>
      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="switching = false">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="busy || !swOK">
          <Icon name="swap" :size="15" />{{ t('software.db_switch_to', { name: otherKind === 'mysql' ? 'MySQL' : 'MariaDB' }) }}
        </button>
      </div>
    </form>
  </Modal>
</template>

<style scoped>
/* the installed engine: full width, with the alternative and the dumps */
.dbe {
  grid-column: 1 / -1; display: flex; flex-direction: column; min-width: 0;
  background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); box-shadow: var(--shadow);
}
.dbe-main { display: flex; flex-direction: column; gap: 10px; padding: 16px; }
.dbe-head { display: flex; align-items: center; gap: 12px; }
.dbe-name { flex: 1; min-width: 0; }
.dbe-name h3 { margin: 0; font-size: 15px; }
.dbe-desc { margin: 0; }
.dbe-meta { display: flex; flex-wrap: wrap; gap: 4px 14px; color: var(--muted); font-size: 12.5px; }
.dbe-meta span { display: inline-flex; align-items: center; gap: 5px; }
.dbe-blocked { display: flex; align-items: flex-start; gap: 6px; margin: 0; color: var(--warn); }
.dbe-blocked .icon, .dbe-why .icon { flex: none; margin-top: 2px; }
.dbe-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.dbe-gap { flex: 1; }
.dbe-remove { color: var(--crit); }
.dbe-alt {
  display: flex; align-items: center; gap: 12px; padding: 12px 16px; flex-wrap: wrap;
  border-top: 1px solid var(--border); background: color-mix(in srgb, var(--accent) 4%, var(--surface));
}
.dbe-alt-text { flex: 1 1 260px; min-width: 0; }
.dbe-alt-title { font-weight: 600; }
.dbe-alt-text p { margin: 2px 0 0; }
.dbe-why { display: flex; gap: 6px; color: var(--warn); }
.dbe-dumps { padding: 12px 16px 14px; border-top: 1px solid var(--border); display: flex; flex-direction: column; gap: 8px; }
.dbe-dumps-head { display: flex; align-items: baseline; flex-wrap: wrap; gap: 4px 8px; }
.dbe-dumps-head .icon { align-self: center; color: var(--accent); }
.dbe-dump-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
.dbe-dump-list li {
  display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 2px 10px; align-items: center;
  padding: 8px 10px; border: 1px solid var(--border); border-radius: 10px;
}
.dbe-dump-what { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; font-weight: 600; }
.dbe-dump-meta { grid-column: 1; overflow-wrap: anywhere; }
.dbe-dump-list .icon-btn { grid-row: 1 / span 2; grid-column: 2; color: var(--crit); }
/* the modals */
.dbe-body { margin: 0; line-height: 1.55; }
.dbe-steps { margin: 0; padding-left: 20px; display: flex; flex-direction: column; gap: 6px; line-height: 1.5; }
.dbe-cols { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 14px; }
.dbe-sub { margin: 0 0 6px; font-size: 13.5px; }
.dbe-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; max-height: 220px; overflow-y: auto; }
.dbe-list li { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 5px 8px; border: 1px solid var(--border); border-radius: 8px; }
.dbe-list .mono { overflow-wrap: anywhere; min-width: 0; }
.dbe-list .tag { flex: none; }
.dbe-notes { margin: 0; padding-left: 18px; display: flex; flex-direction: column; gap: 4px; color: var(--text-2); line-height: 1.5; }
.dbe-problems { margin: 6px 0 0; padding-left: 18px; }
/* the choice */
.db-card {
  display: flex; flex-direction: column; gap: 10px; padding: 16px; min-width: 0;
  background: var(--surface); border: 1px dashed var(--border-strong); border-radius: var(--radius);
}
.db-head { display: flex; align-items: center; gap: 12px; }
.db-name { flex: 1; min-width: 0; }
.db-name h3 { margin: 0; font-size: 15px; }
.db-opts { display: flex; flex-direction: column; gap: 8px; }
.db-opt {
  display: flex; align-items: flex-start; gap: 10px; padding: 10px 12px; cursor: pointer;
  border: 1px solid var(--border); border-radius: 10px;
}
.db-opt.on { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent) inset; }
.db-opt.off { cursor: not-allowed; opacity: 0.7; }
.db-opt input { margin-top: 3px; flex: none; }
.db-opt-text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.db-opt-title { font-weight: 600; display: inline-flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.db-versions { display: flex; flex-direction: column; gap: 6px; margin: -2px 0 0 26px; }
.db-ver { display: flex; align-items: flex-start; gap: 8px; padding: 6px 8px; border-radius: 8px; border: 1px solid transparent; cursor: pointer; }
.db-ver.on { background: var(--accent-soft); border-color: color-mix(in srgb, var(--accent) 30%, transparent); }
.db-ver input { margin-top: 3px; flex: none; }
.db-ver > span { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
.db-ver-title { font-weight: 600; font-size: 13px; }
.db-note { display: flex; align-items: flex-start; gap: 6px; margin: 0; }
.db-note .icon { flex: none; margin-top: 2px; }
.db-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-top: auto; padding-top: 4px; }
</style>
