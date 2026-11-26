<script setup>
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import JobLog from '../components/JobLog.vue'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { useJob } from '../composables/useJob.js'
import { usePageMeta } from '../page.js'
import { put, list } from '../data.js'
import { server } from '../server.js'
import { formatBytesL, relativeTime } from '../format.js'

// The server's own upkeep: OS updates, swap, time and host name, TCP BBR
// and disk cleanup. Calm on purpose: plain status text, colour only for
// what needs the administrator (security updates, a restart).
const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()

const TABS = [
  { id: 'updates', icon: 'download' },
  { id: 'swap', icon: 'memory' },
  { id: 'time', icon: 'clock' },
  { id: 'network', icon: 'activity' },
  { id: 'cleanup', icon: 'trash' },
]
const tab = computed(() => (TABS.some((x) => x.id === route.query.tab) ? route.query.tab : 'updates'))
const setTab = (id) => router.replace({ query: { ...route.query, tab: id === 'updates' ? undefined : id } })

const d = reactive({ updates: null, swap: null, time: null, network: null, cleanup: null })
const zones = ref([])
const supported = ref(true)
const error = ref('')
const notice = ref('')
const busy = ref('') // the quick change under way
const confirm = ref(null) // { title, body, extra, items, label, danger, go }
const form = reactive({ size: 0, swappiness: 10, zone: '', host: '' })
let cleaning = null // { id, bytes } of the cleanup job, for "freed"

const KINDS = ['os_updates_check', 'os_upgrade', 'os_upgrade_security', 'auto_updates', 'swap_create', 'swap_remove', 'system_cleanup']
const { job, running, watch: watchJob, resume, clear: clearJob } = useJob(async (j) => {
  await Promise.all([loadUpdates(), loadTab(tab.value)])
  if (j.kind === 'system_cleanup' && j.status === 'succeeded' && cleaning) {
    const now = list(d.cleanup?.items).find((i) => i.id === cleaning.id)
    notice.value = t('system.clean_freed', { size: size(Math.max(0, cleaning.bytes - (now?.bytes || 0))) })
  }
  cleaning = null
})

// ---- loading ----
async function loadUpdates() {
  try {
    const u = await api('GET', 'api/system/updates')
    d.updates = u
    supported.value = u.supported
    put('api/system/updates', u) // the bell and the dashboard read it
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const loaders = {
  updates: () => {},
  swap: async () => {
    const first = !d.swap
    d.swap = await api('GET', 'api/system/swap')
    if (first) {
      if (d.swap.swappiness >= 0) form.swappiness = d.swap.swappiness
      const mem = d.swap.mem_total_kb / 1024
      const want = mem <= 1536 ? 1024 : 2048
      form.size = swapSizes.value.includes(want) ? want : swapSizes.value.at(-1) || 0
    }
  },
  time: async () => {
    setTime(await api('GET', 'api/system/time'))
    if (!zones.value.length) zones.value = (await api('GET', 'api/system/timezones')).timezones || []
  },
  network: async () => (d.network = await api('GET', 'api/system/network')),
  cleanup: async () => (d.cleanup = await api('GET', 'api/system/cleanup')),
}

async function loadTab(id) {
  if (!supported.value) return
  try {
    await loaders[id]()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

watch(tab, (id) => {
  error.value = notice.value = ''
  loadTab(id)
})

onMounted(async () => {
  await loadUpdates()
  loadTab(tab.value)
  resume(KINDS)
})

// ---- actions ----
function clearMessages() {
  error.value = ''
  notice.value = ''
}

async function startJob(method, path, body) {
  clearMessages()
  confirm.value = null
  try {
    watchJob(await api(method, path, body))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// change makes a quick change; an answer that is a job is followed instead.
async function change(key, method, path, body, okText, after) {
  clearMessages()
  busy.value = key
  try {
    const res = await api(method, path, body)
    if (res?.id && res?.status) {
      watchJob(res)
      return
    }
    await after?.(res)
    if (okText) notice.value = okText
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = ''
  }
}

const size = (b) => formatBytesL(b, locale.value)
const names = (arr) => {
  try {
    return new Intl.ListFormat(locale.value, { style: 'long', type: 'conjunction' }).format(arr)
  } catch {
    return arr.join(', ')
  }
}

// ---- updates ----
const u = computed(() => d.updates)
const pkgs = computed(() => [...list(u.value?.packages)].sort((a, b) => b.security - a.security || a.name.localeCompare(b.name)))
const security = computed(() => u.value?.security || 0)
const checkedText = computed(() => {
  if (!u.value?.checked_at) return t('system.upd_never')
  return t('system.upd_checked', { when: relativeTime(u.value.checked_at, locale.value) })
})
const rebootWhy = computed(() => {
  const r = u.value?.reboot
  if (!r?.required) return ''
  if (list(r.packages).length) return t('system.reboot_pkgs', { names: r.packages.join(', ') })
  return r.kernel ? t('system.reboot_kernel', { kernel: r.kernel }) : ''
})
const autoHint = computed(() => {
  const a = u.value?.auto || {}
  if (a.enabled) return t('system.auto_on')
  return a.installed ? t('system.auto_off') : t('system.auto_install')
})

function askInstall(securityOnly) {
  const n = securityOnly ? security.value : pkgs.value.length
  const chosen = securityOnly ? pkgs.value.filter((p) => p.security) : pkgs.value
  const restarts = restartsOf(chosen)
  confirm.value = {
    title: securityOnly ? t('system.upd_install_security') : t('system.upd_install_all'),
    body: t(securityOnly ? 'system.upd_confirm_security' : 'system.upd_confirm_all', { n }, n),
    extra: [restarts.length ? t('system.upd_confirm_restarts', { names: names(restarts) }) : '', t('system.upd_confirm_keep')].filter(Boolean).join(' '),
    label: t('system.upd_install'),
    go: () => startJob('POST', 'api/system/updates/install', { security: securityOnly }),
  }
}

// restartsOf follows the server's list (systools.restartsFor) for a part of the updates.
function restartsOf(chosen) {
  const all = u.value?.restarts || []
  const has = (re) => chosen.some((p) => re.test(p.name))
  const tests = { Nginx: /^nginx/, Apache: /^apache2(-|$)/, OpenLiteSpeed: /^openlitespeed$/, PHP: /^(php\d+\.\d+|lsphp\d+)(-|$)/, MariaDB: /^(mariadb|mysql)-server/, Redis: /^redis-server$/, Docker: /^(docker-ce|docker\.io|containerd)/ }
  return all.filter((s) => tests[s] && has(tests[s]))
}

function toggleAuto() {
  change('auto', 'PUT', 'api/system/updates/auto', { enabled: !u.value.auto.enabled }, null, loadUpdates)
}

// ---- swap ----
const sw = computed(() => d.swap)
const swapSizes = computed(() => list(sw.value?.sizes).filter((mb) => sw.value.disk_free_mb < 0 || mb + 1024 <= sw.value.disk_free_mb))
const sizeLabel = (mb) => (mb >= 1024 ? `${mb / 1024} GB` : `${mb} MB`)
const kb = (n) => size(n * 1024)

function createSwap() {
  startJob('POST', 'api/system/swap', { size_mb: Number(form.size) })
}

function askRemoveSwap() {
  confirm.value = {
    title: t('system.swap_remove'),
    body: t('system.swap_remove_confirm', { path: sw.value.file }),
    label: t('system.swap_remove'),
    danger: true,
    go: () => startJob('DELETE', 'api/system/swap'),
  }
}

function saveSwappiness() {
  const n = Number(form.swappiness)
  change('swappiness', 'PUT', 'api/system/swap/swappiness', { value: n }, t('system.swappiness_saved', { n }), loaders.swap)
}

// ---- time and host name ----
const tm = computed(() => d.time)
const loadedAt = ref(0)
const tick = ref(Date.now())
const timer = setInterval(() => (tick.value = Date.now()), 1000)
onBeforeUnmount(() => clearInterval(timer))

function setTime(ti) {
  d.time = ti
  loadedAt.value = Date.now()
  form.zone = ti.timezone
  form.host = ti.hostname
}

// The server's clock, running on from its last answer, in its own time zone.
const clock = computed(() => {
  const ti = tm.value
  if (!ti?.time) return '—'
  const at = new Date(new Date(ti.time).getTime() + (tick.value - loadedAt.value))
  try {
    return at.toLocaleString(locale.value, { timeZone: ti.timezone || undefined, dateStyle: 'medium', timeStyle: 'medium' })
  } catch {
    return at.toLocaleString(locale.value)
  }
})
const syncText = computed(() => {
  const ti = tm.value
  if (!ti?.ntp) return t('system.time_ntp_off')
  return ti.synced ? t('system.time_synced') : t('system.time_not_synced')
})

function saveZone() {
  change('zone', 'PUT', 'api/system/timezone', { timezone: form.zone }, t('system.zone_saved', { zone: form.zone }), setTime)
}

function toggleNTP() {
  change('ntp', 'PUT', 'api/system/ntp', { enabled: !tm.value.ntp }, null, setTime)
}

function saveHost() {
  change('host', 'PUT', 'api/system/hostname', { hostname: form.host.trim() }, '', (ti) => {
    setTime(ti)
    if (server.info) server.info.hostname = ti.hostname // the sidebar and the reboot dialog
    notice.value = t('system.host_saved', { name: ti.hostname })
  })
}

// ---- network ----
const net = computed(() => d.network)
const bbrHint = computed(() => {
  const n = net.value
  if (!n.bbr_available) return t('system.bbr_none')
  if (n.bbr && !n.panel) return t('system.bbr_external')
  return t('system.bbr_hint')
})

function toggleBBR() {
  const on = !net.value.bbr
  change('bbr', 'PUT', 'api/system/network/bbr', { enabled: on }, on ? t('system.bbr_on_done') : t('system.bbr_off_done'), (n) => (d.network = n))
}

// ---- cleanup ----
const items = computed(() => list(d.cleanup?.items))
const reclaimable = computed(() => items.value.reduce((a, i) => a + (i.bytes || 0), 0))

function cleanSub(it) {
  switch (it.id) {
    case 'journal':
      return t('system.clean_journal_sub', { total: size(it.total || 0) })
    case 'autoremove': {
      const n = list(it.packages).length
      return n ? t('system.clean_autoremove_sub', { n }, n) : t('system.clean_autoremove_none')
    }
    case 'apt':
    case 'logs':
      return t(`system.clean_${it.id}_sub`) + (it.files ? ' · ' + t('system.clean_files', { n: it.files }, it.files) : '')
  }
  return t(`system.clean_${it.id}_sub`)
}

function askClean(it) {
  const pk = list(it.packages)
  confirm.value = {
    title: t(`system.clean_${it.id}`),
    body: it.id === 'autoremove' ? t('system.clean_confirm_autoremove', { size: size(it.bytes) }) : t('system.clean_confirm', { name: t(`system.clean_${it.id}`), size: size(it.bytes) }),
    items: pk.map((p) => (p.bytes ? `${p.name} (${size(p.bytes)})` : p.name)),
    label: t('system.clean_button'),
    danger: it.id === 'autoremove',
    go: () => {
      cleaning = { id: it.id, bytes: it.bytes }
      startJob('POST', `api/system/cleanup/${it.id}`, { packages: pk.map((p) => p.name) })
    },
  }
}

usePageMeta(() => ({ sub: [server.info?.os, server.info?.kernel].filter(Boolean).join(' · ') }))
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('system.title') }}</h1></div>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />

    <nav class="ptabs" role="tablist" :aria-label="t('system.title')">
      <button v-for="x in TABS" :key="x.id" type="button" role="tab" :aria-selected="tab === x.id" @click="setTab(x.id)">
        <Icon :name="x.icon" :size="16" />{{ t('system.tab_' + x.id) }}
        <span v-if="x.id === 'updates' && pkgs.length" class="n">{{ pkgs.length }}</span>
      </button>
    </nav>

    <p v-if="!supported" class="alert alert-warn">{{ t('system.unsupported') }}</p>
    <span v-else-if="!d[tab] && !error" class="skel" style="height: 220px; border-radius: 14px"></span>

    <!-- updates -->
    <template v-else-if="tab === 'updates' && u">
      <div v-if="u.reboot.required" class="alert alert-warn sys-alert" role="status">
        <span class="sys-alert-text">{{ t('system.reboot_required') }} <span v-if="rebootWhy" class="small">{{ rebootWhy }}</span></span>
        <RouterLink :to="{ name: 'settings' }" class="btn btn-sm">{{ t('system.reboot_link') }}</RouterLink>
      </div>

      <section class="panel">
        <div class="panel-head">
          <h2 class="panel-title"><Icon name="download" :size="17" />{{ t('system.upd_title') }}</h2>
          <span v-if="security" class="status-pill crit">{{ t('system.upd_security_n', { n: security }, security) }}</span>
        </div>
        <div class="panel-tools">
          <span class="muted small">{{ checkedText }}</span>
          <span class="grow"></span>
          <button class="btn btn-sm" :disabled="running" @click="startJob('POST', 'api/system/updates/check', {})"><Icon name="refresh" :size="14" />{{ t('system.upd_check') }}</button>
          <button v-if="security && security < pkgs.length" class="btn btn-sm" :disabled="running" @click="askInstall(true)">{{ t('system.upd_install_security') }}</button>
          <button v-if="pkgs.length" class="btn btn-sm btn-primary" :disabled="running" @click="askInstall(false)"><Icon name="download" :size="14" />{{ t('system.upd_install_all') }}</button>
        </div>
        <p v-if="list(u.restarts).length" class="muted small sys-hint">{{ t('system.upd_restarts', { names: names(u.restarts) }) }}</p>
        <div v-if="pkgs.length" class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>{{ t('system.upd_col_package') }}</th>
                <th>{{ t('system.upd_col_version') }}</th>
                <th>{{ t('system.upd_col_source') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in pkgs" :key="p.name">
                <td><span class="mono small">{{ p.name }}</span> <span v-if="p.security" class="tag tag-crit">{{ t('system.upd_security') }}</span></td>
                <td class="mono small">{{ p.current || t('system.upd_new') }} → {{ p.version }}</td>
                <td class="small muted">{{ p.origin }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="sys-empty muted">{{ u.checked_at ? t('system.upd_none') : t('system.upd_never_hint') }}</p>
        <p v-if="list(u.held).length" class="panel-foot muted small" style="margin: 0">{{ t('system.upd_held', { names: u.held.join(', ') }) }}</p>
      </section>

      <div class="opt">
        <div class="opt-text">
          <div class="opt-title">{{ t('system.auto_title') }}</div>
          <div class="opt-hint">{{ autoHint }}</div>
        </div>
        <span v-if="busy === 'auto'" class="spinner" aria-hidden="true"></span>
        <button type="button" class="switch" role="switch" :aria-checked="!!u.auto.enabled" :aria-label="t('system.auto_title')" :disabled="running || !!busy" @click="toggleAuto"></button>
      </div>
    </template>

    <!-- swap -->
    <template v-else-if="tab === 'swap' && sw">
      <section class="panel">
        <div class="panel-head"><h2 class="panel-title"><Icon name="memory" :size="17" />{{ t('system.swap_title') }}</h2></div>
        <div class="panel-body stack">
          <p class="sys-text">
            {{ t('system.swap_mem', { total: kb(sw.mem_total_kb), avail: kb(sw.mem_available_kb) }) }}<br />
            {{ sw.total_kb ? t('system.swap_total', { total: kb(sw.total_kb), used: kb(sw.total_kb - sw.free_kb) }) : t('system.swap_none') }}
          </p>
          <div v-if="list(sw.devices).length" class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>{{ t('system.swap_col_path') }}</th>
                  <th>{{ t('system.swap_col_type') }}</th>
                  <th>{{ t('system.swap_col_size') }}</th>
                  <th>{{ t('system.swap_col_used') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="dev in sw.devices" :key="dev.path">
                  <td><span class="mono small">{{ dev.path }}</span> <span v-if="dev.panel" class="tag">{{ t('system.swap_panel') }}</span></td>
                  <td class="small">{{ dev.type === 'partition' ? t('system.swap_type_partition') : t('system.swap_type_file') }}</td>
                  <td class="num">{{ kb(dev.size_kb) }}</td>
                  <td class="num">{{ kb(dev.used_kb) }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <p v-if="sw.foreign" class="muted small" style="margin: 0">{{ t('system.swap_foreign', { path: sw.foreign }) }}</p>
          <div v-else-if="sw.file_exists || sw.file_active">
            <button class="btn btn-danger" :disabled="running" @click="askRemoveSwap"><Icon name="trash" :size="15" />{{ t('system.swap_remove') }}</button>
          </div>
          <form v-else class="sys-form" @submit.prevent="createSwap">
            <label class="field">
              <span>{{ t('system.swap_size') }}</span>
              <select v-model.number="form.size" class="input input-auto" :disabled="!swapSizes.length">
                <option v-for="mb in swapSizes" :key="mb" :value="mb">{{ sizeLabel(mb) }}</option>
              </select>
            </label>
            <button class="btn btn-primary" :disabled="running || !swapSizes.length || !form.size"><Icon name="plus" :size="15" />{{ t('system.swap_create') }}</button>
            <span v-if="sw.disk_free_mb >= 0" class="muted small">{{ t('system.swap_free_disk', { free: size(sw.disk_free_mb * 1048576) }) }}</span>
          </form>
          <p v-if="!sw.total_kb" class="muted small" style="margin: 0">{{ t('system.swap_hint') }}</p>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head"><h2 class="panel-title"><Icon name="sliders" :size="17" />{{ t('system.swappiness') }}</h2></div>
        <form class="panel-body stack" @submit.prevent="saveSwappiness">
          <div class="sys-form">
            <label class="field">
              <span>vm.swappiness</span>
              <input v-model.number="form.swappiness" class="input mono sys-num" type="number" min="0" max="100" required />
            </label>
            <button class="btn" :disabled="busy === 'swappiness' || form.swappiness === sw.swappiness">{{ t('system.save') }}</button>
          </div>
          <p class="muted small" style="margin: 0">{{ t('system.swappiness_hint') }}</p>
        </form>
      </section>
    </template>

    <!-- time and host name -->
    <template v-else-if="tab === 'time' && tm">
      <section class="panel">
        <div class="panel-head"><h2 class="panel-title"><Icon name="clock" :size="17" />{{ t('system.time_title') }}</h2></div>
        <div class="panel-body stack">
          <p v-if="!tm.available" class="alert alert-warn" style="margin: 0">{{ t('system.time_unavailable') }}</p>
          <dl class="kv">
            <div><dt>{{ t('system.time_now') }}</dt><dd>{{ clock }}</dd></div>
            <div><dt>{{ t('system.time_zone') }}</dt><dd class="mono">{{ tm.timezone || '—' }}</dd></div>
            <div><dt>{{ t('system.time_sync') }}</dt><dd>{{ syncText }}</dd></div>
          </dl>
          <form class="sys-form" @submit.prevent="saveZone">
            <label class="field sys-grow">
              <span>{{ t('system.time_zone') }}</span>
              <select v-model="form.zone" class="input" :disabled="!zones.length">
                <option v-if="!zones.includes(form.zone)" :value="form.zone">{{ form.zone }}</option>
                <option v-for="z in zones" :key="z" :value="z">{{ z }}</option>
              </select>
            </label>
            <button class="btn" :disabled="busy === 'zone' || !form.zone || form.zone === tm.timezone">{{ t('system.save') }}</button>
          </form>
          <div class="opt">
            <div class="opt-text">
              <div class="opt-title">{{ t('system.ntp_title') }}</div>
              <div class="opt-hint">{{ tm.can_ntp || tm.ntp ? t('system.ntp_hint') : t('system.ntp_missing') }}</div>
            </div>
            <button type="button" class="switch" role="switch" :aria-checked="!!tm.ntp" :aria-label="t('system.ntp_title')" :disabled="!!busy || (!tm.ntp && !tm.can_ntp)" @click="toggleNTP"></button>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head"><h2 class="panel-title"><Icon name="server" :size="17" />{{ t('system.host_title') }}</h2></div>
        <form class="panel-body stack form-narrow" @submit.prevent="saveHost">
          <label class="field">
            <span>{{ t('system.host_label') }}</span>
            <input v-model="form.host" class="input mono" maxlength="64" autocomplete="off" spellcheck="false" required />
            <small class="muted">{{ t('system.host_hint') }}</small>
          </label>
          <div><button class="btn btn-primary" :disabled="busy === 'host' || !form.host.trim() || form.host.trim().toLowerCase() === tm.hostname">{{ t('system.save') }}</button></div>
        </form>
      </section>
    </template>

    <!-- network -->
    <section v-else-if="tab === 'network' && net" class="panel">
      <div class="panel-head"><h2 class="panel-title"><Icon name="activity" :size="17" />{{ t('system.net_title') }}</h2></div>
      <div class="panel-body stack">
        <dl class="kv">
          <div><dt>{{ t('system.net_current') }}</dt><dd class="mono">{{ net.congestion || '—' }}</dd></div>
          <div><dt>{{ t('system.net_qdisc') }}</dt><dd class="mono">{{ net.qdisc || '—' }}</dd></div>
          <div><dt>{{ t('system.net_available') }}</dt><dd class="mono">{{ list(net.available).join(' ') || '—' }}</dd></div>
        </dl>
        <div class="opt">
          <div class="opt-text">
            <div class="opt-title">{{ t('system.bbr_title') }}</div>
            <div class="opt-hint">{{ bbrHint }}</div>
          </div>
          <span v-if="busy === 'bbr'" class="spinner" aria-hidden="true"></span>
          <button type="button" class="switch" role="switch" :aria-checked="!!net.bbr" :aria-label="t('system.bbr_title')"
            :disabled="!!busy || !net.bbr_available || (net.bbr && !net.panel)" @click="toggleBBR"></button>
        </div>
      </div>
    </section>

    <!-- cleanup -->
    <section v-else-if="tab === 'cleanup' && d.cleanup" class="panel">
      <div class="panel-head">
        <h2 class="panel-title"><Icon name="trash" :size="17" />{{ t('system.clean_title') }}</h2>
        <span class="muted small">{{ reclaimable ? t('system.clean_total', { size: size(reclaimable) }) : t('system.clean_nothing') }}</span>
      </div>
      <div v-for="it in items" :key="it.id" class="sys-row">
        <div class="sys-row-text">
          <div class="alert-row-title">{{ t(`system.clean_${it.id}`) }}</div>
          <div class="alert-row-sub">{{ cleanSub(it) }}</div>
          <div v-if="it.error" class="small muted pre-wrap">{{ t('system.clean_error', { error: it.error }) }}</div>
        </div>
        <span class="num sys-size">{{ size(it.bytes) }}</span>
        <button class="btn btn-sm" :disabled="running || !it.bytes" @click="askClean(it)">{{ t('system.clean_button') }}</button>
      </div>
    </section>

    <Modal v-if="confirm" :title="confirm.title" :danger="confirm.danger" @close="confirm = null">
      <div class="stack">
        <p class="sys-text">{{ confirm.body }}</p>
        <ul v-if="list(confirm.items).length" class="sys-list mono small">
          <li v-for="x in confirm.items" :key="x">{{ x }}</li>
        </ul>
        <p v-if="confirm.extra" class="muted small" style="margin: 0">{{ confirm.extra }}</p>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="confirm = null">{{ t('common.cancel') }}</button>
          <button type="button" class="btn" :class="confirm.danger ? 'btn-danger' : 'btn-primary'" :disabled="running" @click="confirm.go()">{{ confirm.label }}</button>
        </div>
      </div>
    </Modal>
  </div>
</template>

<style scoped>
.sys-alert { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.sys-alert-text { flex: 1 1 260px; }
.sys-hint { margin: 0; padding: 12px 18px 0; }
.sys-empty { margin: 0; padding: 18px; }
.sys-text { margin: 0; line-height: 1.6; }
.sys-form { display: flex; align-items: flex-end; gap: 10px; flex-wrap: wrap; }
.sys-grow { flex: 1 1 260px; max-width: 420px; }
.sys-num { width: 110px; }
.sys-row { display: flex; align-items: center; gap: 14px; padding: 14px 18px; border-top: 1px solid var(--border); flex-wrap: wrap; }
.sys-row-text { flex: 1 1 260px; min-width: 0; }
.sys-size { min-width: 72px; text-align: right; font-weight: 600; }
.sys-list { margin: 0; padding: 10px 14px 10px 28px; max-height: 220px; overflow: auto; border: 1px solid var(--border); border-radius: 10px; background: var(--surface-2); }
.num { font-variant-numeric: tabular-nums; }
</style>
