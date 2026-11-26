<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatUptime, formatDateTime, formatBytesL } from '../format.js'
import { server, watchStats, level } from '../server.js'
import { usePageMeta } from '../page.js'
import Modal from '../components/Modal.vue'
import Icon from '../components/Icon.vue'

const { t, locale } = useI18n()

const TABS = ['processes', 'ports', 'ssh']
const tab = ref('processes')
const notice = ref('')

// ---- processes ----
const procs = ref(null) // { processes, total, cpus }
const procError = ref('')
const sort = ref('cpu')
const filter = ref('')
const auto = ref(true)
const killing = ref(null) // the process in the confirmation dialog
let timer = 0
let alive = true

async function loadProcs() {
  try {
    const r = await api('GET', `api/system/processes?sort=${sort.value}`)
    procs.value = { ...r, processes: r.processes || [] }
    procError.value = ''
  } catch (e) {
    procError.value = errorMessage(e)
  }
}

// Refresh every 5 seconds while the list is shown and the browser tab is visible.
function schedule() {
  clearTimeout(timer)
  if (alive) timer = setTimeout(tick, 5000)
}
async function tick() {
  if (tab.value === 'processes' && auto.value && !killing.value && document.visibilityState === 'visible') await loadProcs()
  schedule()
}

function setSort(s) {
  if (sort.value === s) return
  sort.value = s
  loadProcs()
}

const shownProcs = computed(() => {
  const list = procs.value?.processes ?? []
  const q = filter.value.trim().toLowerCase()
  if (!q) return list
  return list.filter((p) => String(p.pid) === q || p.user.toLowerCase().includes(q) || p.command.toLowerCase().includes(q))
})

function elapsed(s) {
  return s < 60 ? t('procs.seconds', { n: s }) : formatUptime(s, t)
}

function cannotKill(p) {
  if (p.kernel) return t('procs.kernel_hint')
  if (p.protected) return t('procs.protected_hint')
  return ''
}

// ---- kill ----
const killError = ref('')
const killBusy = ref(false)

function askKill(p) {
  notice.value = ''
  killError.value = ''
  killing.value = p
}

async function kill(signal) {
  const p = killing.value
  killBusy.value = true
  killError.value = ''
  try {
    await api('POST', `api/system/processes/${p.pid}/kill`, { signal })
    killing.value = null
    notice.value = t(signal === 'KILL' ? 'procs.killed' : 'procs.terminated', { pid: p.pid })
    await loadProcs()
  } catch (e) {
    killError.value = errorMessage(e)
  } finally {
    killBusy.value = false
  }
}

// ---- ports ----
const ports = ref(null)
const portsError = ref('')

async function loadPorts() {
  try {
    ports.value = (await api('GET', 'api/system/ports')).ports || []
    portsError.value = ''
  } catch (e) {
    portsError.value = errorMessage(e)
  }
}

const anyAddress = (a) => a === '*' || a === '0.0.0.0' || a === '::'

// ---- SSH sign-ins ----
const logins = ref(null)
const sshError = ref('')
const sshFilter = ref('all') // all, accepted, failed

async function loadLogins() {
  try {
    logins.value = (await api('GET', 'api/system/ssh-logins')).logins || []
    sshError.value = ''
  } catch (e) {
    sshError.value = errorMessage(e)
  }
}

const isFailure = (l) => l.result !== 'accepted'
const shownLogins = computed(() => {
  const list = logins.value ?? []
  if (sshFilter.value === 'accepted') return list.filter((l) => !isFailure(l))
  if (sshFilter.value === 'failed') return list.filter(isFailure)
  return list
})
const failedCount = computed(() => (logins.value ?? []).filter(isFailure).length)

function resultTag(l) {
  if (l.result === 'accepted') return { cls: 'ok', text: t('procs.ssh_accepted') }
  if (l.result === 'invalid_user') return { cls: 'crit', text: t('procs.ssh_invalid_user') }
  return { cls: 'crit', text: t('procs.ssh_failed') }
}
const cpuNow = computed(() => (server.stats?.supported ? Math.round(server.stats.cpu_percent) : null))
const memNow = computed(() => (server.stats?.supported && server.stats.mem_total ? Math.round((server.stats.mem_used / server.stats.mem_total) * 100) : null))
usePageMeta(() => ({ sub: procs.value ? t('pg.procs_sub', { n: procs.value.total }, procs.value.total) : '' }))
let stopStats = null

function refresh() {
  notice.value = ''
  if (tab.value === 'processes') loadProcs()
  else if (tab.value === 'ports') loadPorts()
  else loadLogins()
}

// The other tabs load the first time they are opened.
watch(tab, (x) => {
  if (x === 'processes') loadProcs()
  else if (x === 'ports' && !ports.value) loadPorts()
  else if (x === 'ssh' && !logins.value) loadLogins()
})

onMounted(() => {
  loadProcs()
  schedule()
  stopStats = watchStats(5000)
})
onBeforeUnmount(() => {
  stopStats?.()
  alive = false
  clearTimeout(timer)
})
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('procs.title') }}</h1></div>
    <Teleport defer to="#topbar-actions">
      <button class="btn" @click="refresh"><Icon name="refresh" :size="15" />{{ t('files.refresh') }}</button>
    </Teleport>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>

    <div class="kpis">
      <div class="kpi"><span class="kpi-ico c-blue"><Icon name="cpu" :size="20" /></span><div><div class="kpi-val">{{ procs ? procs.total : '—' }}</div><div class="kpi-label">{{ t('pg.procs_total') }}</div></div></div>
      <div class="kpi">
        <span class="kpi-ico c-teal"><Icon name="activity" :size="20" /></span>
        <div style="flex: 1; min-width: 0">
          <div class="kpi-val">{{ cpuNow === null ? '—' : cpuNow + '%' }}</div>
          <div class="kpi-label">{{ t('pg.procs_cpu', { n: procs?.cpus || server.info?.cpus || 1 }) }}</div>
          <div v-if="cpuNow !== null" class="ubar" style="width: 100%" aria-hidden="true"><span :class="'lv-' + level(cpuNow)" :style="{ width: cpuNow + '%' }"></span></div>
        </div>
      </div>
      <div class="kpi">
        <span class="kpi-ico c-violet"><Icon name="memory" :size="20" /></span>
        <div style="flex: 1; min-width: 0">
          <div class="kpi-val">{{ memNow === null ? '—' : memNow + '%' }}</div>
          <div class="kpi-label">{{ server.stats?.supported ? `${formatBytesL(server.stats.mem_used, locale)} / ${formatBytesL(server.stats.mem_total, locale)}` : t('dash.memory') }}</div>
          <div v-if="memNow !== null" class="ubar" style="width: 100%" aria-hidden="true"><span :class="'lv-' + level(memNow)" :style="{ width: memNow + '%' }"></span></div>
        </div>
      </div>
      <div class="kpi"><span class="kpi-ico" :class="failedCount ? 'c-red' : 'c-green'"><Icon name="terminal" :size="20" /></span><div><div class="kpi-val">{{ logins ? failedCount : '—' }}</div><div class="kpi-label">{{ t('pg.procs_ssh_failed') }}</div></div></div>
    </div>

    <nav class="ptabs" role="tablist" :aria-label="t('procs.tabs')">
      <button
        v-for="x in TABS"
        :id="`procs-tab-${x}`"
        :key="x"
        type="button"
        role="tab"
        :aria-selected="tab === x"
        :aria-controls="`procs-panel-${x}`"
        @click="tab = x"
      >
        <Icon :name="{ processes: 'cpu', ports: 'link', ssh: 'terminal' }[x]" :size="16" />{{ t(`procs.tab_${x}`) }}
        <span v-if="x === 'ssh' && failedCount" class="n n-crit">{{ failedCount }}</span>
        <span v-if="x === 'ports' && ports" class="n">{{ ports.length }}</span>
      </button>
    </nav>

    <!-- processes -->
    <section v-if="tab === 'processes'" id="procs-panel-processes" class="panel" role="tabpanel" aria-labelledby="procs-tab-processes">
      <div class="panel-tools">
        <label class="search-field">
          <Icon name="search" :size="15" />
          <input v-model="filter" type="search" :placeholder="t('procs.filter')" :aria-label="t('procs.filter')" />
        </label>
        <label class="check small"><input v-model="auto" type="checkbox" /><span>{{ t('procs.auto_refresh') }}</span></label>
        <span class="tools-gap"></span>
        <span v-if="procs" class="muted small">{{ t('procs.count', { shown: shownProcs.length, total: procs.total }, procs.total) }}</span>
      </div>
      <p v-if="procError" class="alert alert-crit pre-wrap" role="alert" style="margin: 12px 18px">{{ procError }}</p>
      <div v-if="!procs && !procError" class="panel-body"><span class="skel" style="height: 260px"></span></div>
      <div v-else-if="procs" class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th :aria-sort="sort === 'pid' ? 'ascending' : 'none'"><button type="button" class="th-sort" :class="{ on: sort === 'pid' }" @click="setSort('pid')">PID</button></th>
              <th>{{ t('procs.col_user') }}</th>
              <th class="num" :aria-sort="sort === 'cpu' ? 'descending' : 'none'">
                <button type="button" class="th-sort" :class="{ on: sort === 'cpu' }" :title="t('procs.cpu_hint', { n: procs.cpus })" @click="setSort('cpu')">CPU %</button>
              </th>
              <th class="num" :aria-sort="sort === 'mem' ? 'descending' : 'none'"><button type="button" class="th-sort" :class="{ on: sort === 'mem' }" @click="setSort('mem')">{{ t('procs.col_mem') }}</button></th>
              <th>{{ t('procs.col_time') }}</th>
              <th>{{ t('procs.col_command') }}</th>
              <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in shownProcs" :key="p.pid" :class="{ 'row-dim': p.kernel }">
              <td class="mono small">{{ p.pid }}</td>
              <td class="small">{{ p.user }}</td>
              <td class="num">
                <span class="mono" :class="{ hot: p.cpu >= 50 }">{{ p.cpu.toFixed(1) }}</span>
                <div class="ubar cpu-bar" aria-hidden="true"><span :class="'lv-' + level(Math.min(100, p.cpu))" :style="{ width: Math.min(100, p.cpu) + '%' }"></span></div>
              </td>
              <td class="num nowrap">
                <span class="mono">{{ p.rss_mb.toFixed(1) }} MB</span>
                <div class="muted small">{{ p.mem.toFixed(1) }}%</div>
              </td>
              <td class="nowrap small muted">{{ elapsed(p.elapsed) }}</td>
              <td class="mono small command-cell">{{ p.command }}</td>
              <td class="actions">
                <button
                  type="button"
                  class="icon-btn icon-btn-sm danger"
                  :disabled="!!cannotKill(p)"
                  :title="cannotKill(p) || t('procs.kill')"
                  :aria-label="t('pg.procs_stop', { pid: p.pid })"
                  @click="askKill(p)"
                >
                  <Icon name="stop" :size="15" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-if="procs" class="muted small panel-foot" style="margin: 0">{{ t('procs.cpu_hint', { n: procs.cpus }) }}</p>
    </section>

    <!-- ports -->
    <section v-if="tab === 'ports'" id="procs-panel-ports" class="panel" role="tabpanel" aria-labelledby="procs-tab-ports">
      <p class="muted small panel-tools" style="margin: 0">
        <span>{{ t('procs.ports_hint') }} <RouterLink :to="{ name: 'firewall' }">{{ t('nav.firewall') }}</RouterLink></span>
      </p>
      <p v-if="portsError" class="alert alert-crit pre-wrap" role="alert" style="margin: 12px 18px">{{ portsError }}</p>
      <div v-if="!ports && !portsError" class="panel-body"><span class="skel" style="height: 200px"></span></div>
      <div v-else-if="ports && !ports.length" class="empty"><p class="empty-lead">{{ t('procs.no_ports') }}</p></div>
      <div v-else-if="ports" class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>{{ t('procs.col_port') }}</th>
              <th>{{ t('procs.col_proto') }}</th>
              <th>{{ t('procs.col_address') }}</th>
              <th>{{ t('procs.col_program') }}</th>
              <th>{{ t('procs.col_reach') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(p, i) in ports" :key="i">
              <td><span class="pill mono">{{ p.port }}</span></td>
              <td class="small">{{ p.proto.toUpperCase() }}</td>
              <td class="mono small">{{ anyAddress(p.address) ? t('procs.any_address', { addr: p.address }) : p.address }}</td>
              <td>
                {{ p.process || '—' }}
                <span v-if="p.pid" class="muted small mono">({{ p.pid }})</span>
              </td>
              <td>
                <span v-if="p.local" class="status-pill ok">{{ t('procs.reach_local') }}</span>
                <span v-else class="status-pill warn">{{ t('procs.reach_network') }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- SSH sign-ins -->
    <section v-if="tab === 'ssh'" id="procs-panel-ssh" class="panel" role="tabpanel" aria-labelledby="procs-tab-ssh">
      <div class="panel-tools">
        <div class="tabs" role="group" :aria-label="t('procs.ssh_show')">
          <button v-for="f in ['all', 'accepted', 'failed']" :key="f" type="button" :aria-pressed="sshFilter === f" @click="sshFilter = f">
            {{ { all: t('procs.ssh_all'), accepted: t('procs.ssh_only_accepted'), failed: t('procs.ssh_only_failed') }[f] }}
          </button>
        </div>
        <span class="muted small ssh-hint">{{ t('procs.ssh_hint') }}</span>
      </div>
      <p v-if="failedCount" class="alert alert-warn" style="margin: 12px 18px">
        {{ t('procs.ssh_failures', { n: failedCount }, failedCount) }} <RouterLink :to="{ name: 'firewall' }">{{ t('nav.firewall') }}</RouterLink>
      </p>
      <p v-if="sshError" class="alert alert-crit pre-wrap" role="alert" style="margin: 12px 18px">{{ sshError }}</p>
      <div v-if="!logins && !sshError" class="panel-body"><span class="skel" style="height: 200px"></span></div>
      <div v-else-if="logins && !shownLogins.length" class="empty"><p class="empty-lead">{{ t('procs.ssh_empty') }}</p></div>
      <div v-else-if="logins" class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>{{ t('procs.col_when') }}</th>
              <th>{{ t('procs.col_result') }}</th>
              <th>{{ t('procs.col_user') }}</th>
              <th>{{ t('procs.col_ip') }}</th>
              <th>{{ t('procs.col_method') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(l, i) in shownLogins" :key="i">
              <td class="nowrap small">{{ formatDateTime(l.time, locale) }}</td>
              <td><span class="status-pill" :class="resultTag(l).cls">{{ resultTag(l).text }}</span></td>
              <td class="mono small">{{ l.user || '—' }}</td>
              <td class="mono small">{{ l.ip }}</td>
              <td class="small">{{ l.method || '—' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <Modal v-if="killing" danger :title="t('procs.kill_title', { pid: killing.pid })" @close="killing = null">
      <div class="stack">
        <p v-if="killError" class="alert alert-crit pre-wrap" role="alert">{{ killError }}</p>
        <dl class="kv">
          <div><dt>{{ t('procs.col_user') }}</dt><dd>{{ killing.user }}</dd></div>
          <div><dt>{{ t('procs.col_time') }}</dt><dd>{{ elapsed(killing.elapsed) }}</dd></div>
        </dl>
        <pre class="log kill-cmd">{{ killing.command }}</pre>
        <p class="small">{{ t('procs.term_hint') }}</p>
        <p class="small">{{ t('procs.kill_hint') }}</p>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="killing = null">{{ t('common.cancel') }}</button>
          <button type="button" class="btn" :disabled="killBusy" @click="kill('TERM')">{{ t('procs.send_term') }}</button>
          <button type="button" class="btn btn-danger btn-solid" :disabled="killBusy" @click="kill('KILL')">{{ t('procs.send_kill') }}</button>
        </div>
      </div>
    </Modal>
  </div>
</template>

<style scoped>
.tools-gap { flex: 1; }
.ptabs .n-crit { background: var(--crit-soft); color: var(--crit); }
.th-sort {
  padding: 0; border: 0; background: none; color: inherit; font: inherit; letter-spacing: inherit;
  text-transform: inherit; cursor: pointer;
}
.th-sort.on { color: var(--accent); }
.th-sort.on::after { content: ' ▾'; }
th[aria-sort='ascending'] .th-sort.on::after { content: ' ▴'; }
.num { text-align: right; }
.table th.num { text-align: right; }
.cpu-bar { width: 56px; margin-left: auto; }
.hot { color: var(--crit); font-weight: 700; }
.row-dim td { color: var(--muted); }
.command-cell { max-width: 520px; overflow-wrap: anywhere; }
.ssh-hint { flex: 1 1 240px; }
.kill-cmd { max-height: 140px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; margin: 0; }
.ubar .lv-warn { background: var(--warn-g); }
.ubar .lv-crit { background: var(--crit-g); }
</style>
