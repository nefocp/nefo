<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { useJob } from '../composables/useJob.js'
import Icon from '../components/Icon.vue'
import JobLog from '../components/JobLog.vue'
import RowActions from '../components/RowActions.vue'
import WpPasswordModal from '../components/WpPasswordModal.vue'
import WpStagingModal from '../components/WpStagingModal.vue'
import WpPushModal from '../components/WpPushModal.vue'
import AppLogo from '../components/AppLogo.vue'
import { usePageMeta } from '../page.js'

// The WordPress toolkit: every WordPress site with its version, available
// updates and settings. The list comes at once; each site's status is
// checked with wp-cli on the server and fills in when it is ready.

const { t } = useI18n()
const sites = ref([])
const supported = ref(true)
const wpCli = ref(null)
const loaded = ref(false)
const error = ref('')
const notice = ref('')
const statuses = reactive({}) // site id → { loading, data, error }
const pwSite = ref(null)
const stagingSite = ref(null)
const pushSite = ref(null) // a staging copy, pushed to pushSite.staging_of
const { job, running: jobRunning, watch: watchJob, resume, clear: clearJob } = useJob(onJobDone)

// load shows the list at once, then checks the sites.
async function load() {
  let res
  try {
    res = await api('GET', 'api/wordpress')
    sites.value = res.sites || []
    supported.value = res.supported
    wpCli.value = res.wp_cli
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
    return
  } finally {
    loaded.value = true
  }
  if (res.supported) await loadStatuses(res.sites || [])
}

// loadStatuses checks three sites at a time: each check starts several PHP
// processes on the server.
async function loadStatuses(list) {
  for (const s of list) statuses[s.id] = { loading: true, data: statuses[s.id]?.data ?? null, error: '' }
  const queue = [...list]
  const worker = async () => {
    while (queue.length) await loadStatus(queue.shift())
  }
  await Promise.all([worker(), worker(), worker()])
}

async function loadStatus(s) {
  const prev = statuses[s.id]
  statuses[s.id] = { loading: true, data: prev?.data ?? null, error: '' }
  try {
    statuses[s.id] = { loading: false, data: await api('GET', `api/wordpress/${s.id}`), error: '' }
  } catch (e) {
    statuses[s.id] = { loading: false, data: null, error: errorMessage(e) }
  }
}

function onJobDone(j) {
  if (j.status === 'succeeded') {
    notice.value = t({ wp_staging: 'wp.staging_done', wp_push: 'wp.push_done' }[j.kind] || 'wp.update_done', { domain: j.target })
  }
  return load()
}

onMounted(() => {
  load()
  resume(['wp_update', 'wp_staging', 'wp_push'])
})

function clearMessages() {
  error.value = ''
  notice.value = ''
}

const stat = (s) => statuses[s.id] || { loading: true, data: null, error: '' }

async function update(s) {
  clearMessages()
  if (!window.confirm(t('wp.update_confirm', { domain: s.domain }))) return
  try {
    const res = await api('POST', `api/wordpress/${s.id}/update`)
    watchJob(res.job)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// toggle changes maintenance, debug or auto-updates.
async function toggle(s, what, on) {
  clearMessages()
  if (what === 'maintenance' && on && !window.confirm(t('wp.maintenance_confirm', { domain: s.domain }))) return
  const path = { maintenance: 'maintenance', debug: 'debug', auto_updates: 'auto-updates' }[what]
  try {
    await api('PUT', `api/wordpress/${s.id}/${path}`, { on })
    notice.value = t(`wp.done_${what}_${on ? 'on' : 'off'}`, { domain: s.domain })
  } catch (e) {
    error.value = errorMessage(e)
  }
  await loadStatus(s)
}

function openStaging(s) {
  clearMessages()
  stagingSite.value = s
}

function stagingStarted(j) {
  stagingSite.value = null
  watchJob(j)
}

function pushStarted(j) {
  pushSite.value = null
  watchJob(j)
}

function actions(s) {
  const d = stat(s).data
  return [
    { label: t('wp.update'), icon: 'download', inline: true, disabled: jobRunning.value, onClick: () => update(s) },
    { label: 'WP Admin', icon: 'external', href: `${s.url}/wp-admin/` },
    d?.maintenance
      ? { label: t('wp.maintenance_turn_off'), icon: 'play', onClick: () => toggle(s, 'maintenance', false) }
      : { label: t('wp.maintenance_turn_on'), icon: 'stop', disabled: !d, onClick: () => toggle(s, 'maintenance', true) },
    d?.debug
      ? { label: t('wp.debug_turn_off'), icon: 'code', onClick: () => toggle(s, 'debug', false) }
      : { label: t('wp.debug_turn_on'), icon: 'code', disabled: !d, onClick: () => toggle(s, 'debug', true) },
    d?.auto_updates
      ? { label: t('wp.auto_turn_off'), icon: 'refresh', onClick: () => toggle(s, 'auto_updates', false) }
      : { label: t('wp.auto_turn_on'), icon: 'refresh', disabled: !d, onClick: () => toggle(s, 'auto_updates', true) },
    { label: t('wp.reset_password'), icon: 'key', onClick: () => { clearMessages(); pwSite.value = s } },
    { label: t('wp.clone'), icon: 'copy', disabled: jobRunning.value, onClick: () => openStaging(s) },
    s.staging_of ? { label: t('wp.push'), icon: 'upload', disabled: jobRunning.value, onClick: () => { clearMessages(); pushSite.value = s } } : null,
    { label: t('wp.recheck'), icon: 'refresh', disabled: stat(s).loading, onClick: () => loadStatus(s) },
  ].filter(Boolean)
}

const typeLabel = (u) => t(`wp.type_${u.type}`)
const needUpdate = computed(() => sites.value.filter((s) => { const d = stat(s).data; return d && !d.error && (d.core_update || (d.updates || []).length) }).length)
const maintenanceOn = computed(() => sites.value.filter((s) => stat(s).data?.maintenance).length)
usePageMeta(() => ({ sub: loaded.value ? t('pg.wp_sub', { n: sites.value.length }, sites.value.length) : '' }))
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('wp.title') }}</h1></div>
    <Teleport defer to="#topbar-actions">
      <button v-if="sites.length" class="btn" :disabled="sites.some((s) => stat(s).loading)" @click="clearMessages(); load()"><Icon name="refresh" :size="15" />{{ t('wp.recheck_all') }}</button>
      <RouterLink class="btn btn-primary" :to="{ name: 'apps', query: { install: 'wordpress' } }"><Icon name="plus" :size="15" />{{ t('pg.wp_new') }}</RouterLink>
    </Teleport>

    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />

    <span v-if="!loaded" class="skel" style="height: 240px; border-radius: 14px"></span>

    <template v-if="loaded">
      <div v-if="!supported" class="card empty">
        <AppLogo name="wordpress" size="lg" style="margin: 0 auto 14px" />
        <p class="empty-lead">{{ t('websites.unsupported') }}</p>
      </div>

      <template v-else>
        <div v-if="sites.length" class="kpis">
          <div class="kpi"><AppLogo name="wordpress" /><div><div class="kpi-val">{{ sites.length }}</div><div class="kpi-label">{{ t('pg.wp_sites') }}</div></div></div>
          <div class="kpi"><span class="kpi-ico" :class="needUpdate ? 'c-orange' : 'c-green'"><Icon name="download" :size="20" /></span><div><div class="kpi-val">{{ needUpdate }}</div><div class="kpi-label">{{ t('pg.wp_need_update') }}</div></div></div>
          <div class="kpi"><span class="kpi-ico" :class="maintenanceOn ? 'c-red' : 'c-gray'"><Icon name="stop" :size="20" /></span><div><div class="kpi-val">{{ maintenanceOn }}</div><div class="kpi-label">{{ t('pg.wp_maint') }}</div></div></div>
          <div v-if="wpCli" class="kpi"><span class="kpi-ico c-blue"><Icon name="terminal" :size="20" /></span><div><div class="kpi-val">wp-cli {{ wpCli.version }}</div><div class="kpi-label">{{ t('pg.wp_cli') }}</div></div></div>
        </div>

        <section class="panel">
          <div v-if="!sites.length" class="empty">
            <AppLogo name="wordpress" size="lg" style="margin: 0 auto 14px" />
            <p class="empty-lead">{{ t('wp.empty') }}</p>
            <p class="muted">{{ t('wp.empty_hint') }}</p>
            <RouterLink class="btn btn-primary" :to="{ name: 'apps', query: { install: 'wordpress' } }"><Icon name="plus" />{{ t('pg.wp_new') }}</RouterLink>
          </div>

          <div v-else class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>{{ t('ws.col_site') }}</th>
                  <th>{{ t('pg.wp_version') }}</th>
                  <th>{{ t('pg.wp_addons') }}</th>
                  <th>{{ t('pg.wp_modes') }}</th>
                  <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="s in sites" :key="s.id" :aria-busy="stat(s).loading">
                  <td>
                    <div class="name-ico">
                      <AppLogo name="wordpress" size="sm" />
                      <div class="name-ico-text">
                        <RouterLink class="wp-domain" :to="{ name: 'websites', query: { site: String(s.id), tab: 'wordpress' } }">{{ s.domain }}</RouterLink>
                        <div class="name-ico-sub">
                          PHP {{ s.php_version }}
                          <span v-if="s.staging_of"> · {{ t('wp.staging_of', { domain: s.staging_of.domain }) }}</span>
                          <span v-if="s.disabled" class="status-pill crit wp-off">{{ t('access.suspended') }}</span>
                        </div>
                      </div>
                    </div>
                  </td>
                  <td class="nowrap">
                    <template v-if="stat(s).loading && !stat(s).data"><span class="spinner" aria-hidden="true"></span> <span class="muted small">{{ t('wp.checking') }}</span></template>
                    <template v-else-if="stat(s).data && !stat(s).data.error">
                      <div class="wp-v">{{ stat(s).data.version }}</div>
                      <span v-if="stat(s).data.core_update" class="status-pill warn">{{ t('wp.core_update', { version: stat(s).data.core_update }) }}</span>
                      <span v-else-if="!(stat(s).data.updates || []).length" class="status-pill ok">{{ t('wp.up_to_date') }}</span>
                    </template>
                    <span v-else class="status-pill crit">{{ t('pg.wp_check_failed') }}</span>
                  </td>
                  <td class="small">
                    <template v-if="stat(s).data && !stat(s).data.error">
                      <div>{{ t('wp.plugins', { n: stat(s).data.plugins }) }} <span v-if="stat(s).data.plugin_updates" class="tag tag-warn">{{ t('wp.to_update', { n: stat(s).data.plugin_updates }) }}</span></div>
                      <div>{{ t('wp.themes', { n: stat(s).data.themes }) }} <span v-if="stat(s).data.theme_updates" class="tag tag-warn">{{ t('wp.to_update', { n: stat(s).data.theme_updates }) }}</span></div>
                      <details v-if="(stat(s).data.updates || []).length" class="wp-updates">
                        <summary>{{ t('wp.show_updates', { n: stat(s).data.updates.length }, stat(s).data.updates.length) }}</summary>
                        <ul>
                          <li v-for="u in stat(s).data.updates" :key="u.type + u.name">
                            <span class="muted">{{ typeLabel(u) }}</span> {{ u.title || u.name }}
                            <span class="mono small">{{ u.version }} → {{ u.new_version }}</span>
                          </li>
                        </ul>
                      </details>
                    </template>
                    <span v-else class="muted">—</span>
                  </td>
                  <td>
                    <div class="wp-flags">
                      <span v-if="stat(s).data?.maintenance" class="status-pill crit">{{ t('wp.maintenance') }}</span>
                      <span v-if="stat(s).data?.debug" class="status-pill warn">{{ t('wp.debug') }}</span>
                      <span v-if="stat(s).data?.auto_updates" class="status-pill ok">{{ t('wp.auto_updates') }}</span>
                      <span v-if="stat(s).data && !stat(s).data.maintenance && !stat(s).data.debug && !stat(s).data.auto_updates" class="muted small">—</span>
                    </div>
                    <p v-if="stat(s).error || stat(s).data?.error" class="small text-crit pre-wrap wp-error">
                      {{ stat(s).error || t('wp.check_failed', { detail: stat(s).data.error }) }}
                    </p>
                  </td>
                  <td class="actions"><RowActions :actions="actions(s)" /></td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-if="sites.length && wpCli" class="muted small panel-foot" style="margin: 0">{{ t('wp.note', { version: wpCli.version }) }}</p>
        </section>
      </template>
    </template>

    <WpPasswordModal v-if="pwSite" :site="pwSite" @close="pwSite = null" />
    <WpStagingModal v-if="stagingSite" :site="stagingSite" @close="stagingSite = null" @started="stagingStarted" />
    <WpPushModal v-if="pushSite" :site="pushSite" :live="pushSite.staging_of" @close="pushSite = null" @started="pushStarted" />
  </div>
</template>

<style scoped>
.wp-domain { font-weight: 700; color: var(--text); }
.wp-v { font-weight: 700; margin-bottom: 3px; }
.wp-off { margin-left: 6px; min-height: 18px; font-size: 11px; }
.wp-flags { display: flex; flex-wrap: wrap; gap: 4px; }
.wp-updates { margin-top: 6px; font-size: 12.5px; }
.wp-updates summary { cursor: pointer; color: var(--muted); }
.wp-updates ul { margin: 6px 0 0; padding-left: 18px; }
.wp-updates li { margin: 2px 0; }
.wp-error { margin: 6px 0 0; max-width: 360px; }
</style>
