<script setup>
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { toast } from '../toast.js'
import { daysLeft } from '../attention.js'
import { appName, logoURL } from '../logos.js'
import { SERVER_NAMES, hasRateLimit } from '../webservers.js'
import Drawer from './Drawer.vue'
import Icon from './Icon.vue'
import JobLog from './JobLog.vue'
import LogViewer from './LogViewer.vue'
import SiteOverview from './SiteOverview.vue'
import SiteSettings from './SiteSettings.vue'
import SiteAccess from './SiteAccess.vue'
import SiteSFTP from './SiteSFTP.vue'
import SiteNginx from './SiteNginx.vue'
import SiteSSL from './SiteSSL.vue'
import SiteFirewall from './SiteFirewall.vue'
import SiteScan from './SiteScan.vue'
import SiteStats from './SiteStats.vue'
import SiteGit from './SiteGit.vue'
import SiteWordPress from './SiteWordPress.vue'
import SiteBackups from './SiteBackups.vue'
import SiteHTTPS from './SiteHTTPS.vue'
import SitePageCache from './SitePageCache.vue'
import SitePointDNS from './SitePointDNS.vue'
import SiteMonitor from './SiteMonitor.vue'
import { store as cache, list as asList } from '../data.js'

// SiteDrawer holds every setting of one website, in grouped tabs. Each tab
// reuses the per-feature form (SiteSSL, SiteFirewall…). Saved sites go up
// with "saved", started jobs with "job"; the parent follows the job and
// passes it back as "job" so its log shows here too.
const props = defineProps({
  site: { type: Object, required: true },
  server: { type: String, default: 'nginx' },
  phpVersions: { type: Array, default: () => [] },
  job: { type: Object, default: null },
  busy: { type: Boolean, default: false },
  database: { type: Object, default: null },
  tab: { type: String, default: 'overview' },
})
const emit = defineEmits(['close', 'saved', 'job', 'clear-job', 'delete', 'tab', 'install-wp'])
const { t, locale } = useI18n()
const fmtDate = (v) => new Date(v).toLocaleDateString(locale.value, { year: 'numeric', month: 'short', day: 'numeric' })

const s = computed(() => props.site)
const isApp = computed(() => !!s.value.proxy_port)
const isDocker = computed(() => (s.value.app || '').startsWith('docker:'))
const serverName = computed(() => SERVER_NAMES[props.server] || 'Nginx')
const url = computed(() => `${s.value.ssl ? 'https' : 'http'}://${s.value.domain}/`)
const runtimeLogo = computed(() => (s.value.app_runtime === 'python' ? 'python' : 'nodejs'))
const days = computed(() => daysLeft(s.value.cert_expires))
// Website monitoring (api/uptime, loaded by the website list and the bell).
const monitorDown = computed(() => asList(cache['api/uptime']?.sites).some((x) => x.site_id === s.value.id && x.state === 'down'))
const sslState = computed(() => {
  if (!s.value.ssl) return 'off'
  if (s.value.ssl_error || (days.value !== null && days.value < 0)) return 'crit'
  if (days.value !== null && days.value <= 14) return 'warn'
  return 'ok'
})

const groups = computed(() =>
  [
    {
      id: 'basic',
      items: [
        { id: 'overview', icon: 'dashboard' },
        { id: 'domains', icon: 'globe', count: 1 + (s.value.aliases || []).length },
        s.value.php_version ? { id: 'php', logo: 'php' } : null,
        isApp.value && !isDocker.value ? { id: 'runtime', logo: runtimeLogo.value } : null,
      ],
    },
    {
      id: 'security',
      items: [
        { id: 'ssl', icon: 'lock', dot: sslState.value === 'off' ? '' : sslState.value },
        { id: 'waf', icon: 'shield' },
        { id: 'block', icon: 'ban' },
        { id: 'hotlink', icon: 'image' },
        { id: 'scan', icon: 'bug' },
      ],
    },
    {
      id: 'config',
      items: [
        !isApp.value ? { id: 'rewrite', icon: 'code' } : null,
        // Nginx's page cache; OpenLiteSpeed has LSCache built in (the tab says so).
        s.value.php_version && props.server !== 'apache' ? { id: 'cache', icon: 'zap' } : null,
        { id: 'webconf', logo: props.server, label: t('drawer.tab_webconf', { server: serverName.value }) },
        { id: 'redirect', icon: 'arrowRight' },
        isApp.value ? { id: 'proxy', icon: 'swap' } : null,
        { id: '404', icon: 'fileX' },
      ],
    },
    {
      id: 'ops',
      items: [
        s.value.app === 'wordpress' ? { id: 'wordpress', logo: 'wordpress' } : null,
        !isDocker.value ? { id: 'git', logo: 'git' } : null,
        { id: 'sftp', icon: 'upload' },
        { id: 'backups', icon: 'archive' },
        { id: 'monitor', icon: 'activity', dot: monitorDown.value ? 'crit' : '' },
        { id: 'stats', icon: 'bars' },
        { id: 'logs', icon: 'logs' },
        { id: 'expiry', icon: 'calendar' },
      ],
    },
  ].map((g) => ({ ...g, items: g.items.filter(Boolean).map((i) => ({ ...i, label: i.label || t('drawer.tab_' + i.id) })) })),
)
const allTabs = computed(() => groups.value.flatMap((g) => g.items))
const current = ref(allTabs.value.some((x) => x.id === props.tab) ? props.tab : 'overview')
watch(
  () => props.tab,
  (v) => {
    if (allTabs.value.some((x) => x.id === v)) current.value = v
  },
)
watch(current, (v) => emit('tab', v))
const currentTab = computed(() => allTabs.value.find((x) => x.id === current.value) || allTabs.value[0])

// Forms reset (remount) on Cancel: the per-feature forms emit "close".
const resetKey = ref(0)
const reset = () => resetKey.value++
const paneKey = computed(() => `${current.value}-${s.value.id}-${resetKey.value}`)

function saved(site) {
  const merged = { ...s.value, ...site }
  if (!site.app_status && s.value.app_status) merged.app_status = s.value.app_status
  emit('saved', merged)
  toast(t('drawer.saved'))
}

async function startJob(method, path, body) {
  try {
    const res = await api(method, path, body)
    emit('job', res.job ?? res)
  } catch (e) {
    toast(errorMessage(e), 'crit', 7000)
  }
}

async function forceHTTPS() {
  try {
    saved(await api('POST', `api/sites/${s.value.id}/force-https`, { enabled: !s.value.force_https }))
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  }
}

async function saveNote(note) {
  try {
    saved(await api('PUT', `api/sites/${s.value.id}/note`, { note }))
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  }
}

async function setRunning(run) {
  try {
    saved(await api('PUT', `api/sites/${s.value.id}/access`, { disabled: !run, redirect_url: s.value.redirect_url || '', auth_user: s.value.auth_user || '', auth_password: '' }))
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  }
}

async function appAction(action) {
  try {
    await api('POST', `api/sites/${s.value.id}/app/${action}`, {})
    toast(t('drawer.app_done'))
    emit('saved', { ...s.value, app_status: action === 'stop' ? 'inactive' : 'active' })
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  }
}

const logKinds = computed(() => {
  const k = [
    { value: 'access', label: t('webserver.log_access', { server: serverName.value }) },
    { value: 'error', label: t('webserver.log_error', { server: serverName.value }) },
  ]
  if (s.value.php_version) k.push({ value: 'php', label: t('logs.php') })
  if (s.value.app_command || isDocker.value) k.push({ value: 'app', label: t('websites.app_logs') })
  return k
})
const logURL = (k) => (k === 'app' ? `api/sites/${s.value.id}/app/logs` : `api/sites/${s.value.id}/logs?type=${k}&lines=300`)

const status = computed(() => {
  if (s.value.disabled) return { cls: 'warn', text: t('access.suspended') }
  if (s.value.redirect_url) return { cls: '', text: t('drawer.status_redirect') }
  if ((s.value.app_command || isDocker.value) && s.value.app_status && s.value.app_status !== 'active') return { cls: 'crit', text: t('drawer.status_app_down') }
  return { cls: 'ok', text: t('drawer.status_running') }
})
const palette = ['#2F6BDB', '#C2410C', '#7454E0', '#0D9488', '#B45309', '#BE185D', '#4D7C0F', '#475569']
const favColor = computed(() => (s.value.disabled ? '#94A3B8' : palette[[...s.value.domain].reduce((a, c) => (a * 31 + c.charCodeAt(0)) >>> 0, 7) % palette.length]))
const appLabel = computed(() => (s.value.app ? appName(s.value.app) : ''))
</script>

<template>
  <Drawer :label="t('drawer.label', { domain: s.domain })" @close="emit('close')">
    <template #head>
      <span class="fav lg" :style="{ background: favColor }" aria-hidden="true">{{ s.domain[0] }}</span>
      <div class="dh-text">
        <div class="row" style="gap: 10px">
          <h2 class="drawer-title" tabindex="-1" autofocus>{{ s.domain }}</h2>
          <span class="status-pill" :class="status.cls">{{ status.text }}</span>
        </div>
        <div class="dh-chips">
          <span v-if="appLabel" class="chip"><img v-if="logoURL(s.app)" :src="logoURL(s.app)" alt="" />{{ appLabel }}</span>
          <span v-if="s.php_version" class="chip"><img :src="logoURL('php')" alt="" />PHP {{ s.php_version }}</span>
          <span v-else-if="isApp && !isDocker" class="chip"><img :src="logoURL(runtimeLogo)" alt="" />{{ runtimeLogo === 'python' ? 'Python' : 'Node.js' }} · {{ s.proxy_port }}</span>
          <span v-else-if="!isApp" class="chip chip-text"><Icon name="code" :size="13" />{{ t('websites.static') }}</span>
          <span class="chip"><img v-if="logoURL(server)" :src="logoURL(server)" alt="" />{{ serverName }}</span>
          <span v-if="database" class="chip"><img :src="logoURL('mariadb')" alt="" /><span class="mono">{{ database.name }}</span></span>
        </div>
      </div>
      <div class="dh-actions">
        <a class="btn" :href="url" target="_blank" rel="noopener noreferrer"><Icon name="external" :size="15" />{{ t('websites.open_site') }}</a>
        <a v-if="s.app === 'wordpress'" class="btn dh-wp" :href="url + 'wp-admin/'" target="_blank" rel="noopener noreferrer">WP Admin</a>
        <button type="button" class="icon-btn bordered" :aria-label="t('common.close')" @click="emit('close')"><Icon name="x" /></button>
      </div>
    </template>

    <div class="drawer-main">
      <nav class="drawer-nav" :aria-label="t('drawer.sections')">
        <div v-for="g in groups" :key="g.id" role="tablist" aria-orientation="vertical" :aria-label="t('drawer.group_' + g.id)">
          <div class="vgrp" aria-hidden="true">{{ t('drawer.group_' + g.id) }}</div>
          <button
            v-for="x in g.items"
            :id="'dtab-' + x.id"
            :key="x.id"
            type="button"
            class="vtab"
            role="tab"
            :aria-selected="current === x.id"
            aria-controls="drawer-pane"
            @click="current = x.id"
          >
            <img v-if="x.logo && logoURL(x.logo)" :src="logoURL(x.logo)" alt="" />
            <Icon v-else :name="x.icon || 'file'" :size="17" :stroke="1.9" />
            <span class="truncate">{{ x.label }}</span>
            <span v-if="x.count" class="vtab-end muted small">{{ x.count }}</span>
            <span v-if="x.dot" class="vtab-end dot" :class="'dot-' + x.dot" :aria-label="t('drawer.ssl_state_' + x.dot)" role="img"></span>
          </button>
        </div>
      </nav>

      <div class="drawer-tabs-select">
        <label class="sr-only" for="drawer-tab-select">{{ t('drawer.sections') }}</label>
        <select id="drawer-tab-select" v-model="current" class="input">
          <optgroup v-for="g in groups" :key="g.id" :label="t('drawer.group_' + g.id)">
            <option v-for="x in g.items" :key="x.id" :value="x.id">{{ x.label }}</option>
          </optgroup>
        </select>
      </div>

      <section id="drawer-pane" class="drawer-pane" role="tabpanel" :aria-labelledby="'dtab-' + current">
        <div class="drawer-pane-head">
          <div>
            <h3>{{ currentTab.label }}</h3>
            <p>{{ t('drawer.desc_' + current, { server: serverName }) }}</p>
          </div>
        </div>

        <JobLog v-if="job" :job="job" @close="emit('clear-job')" />

        <SiteOverview
          v-if="current === 'overview'"
          :key="paneKey"
          :site="s"
          :server="server"
          :busy="busy"
          :database="database"
          @go="(id) => (current = id)"
          @force-https="forceHTTPS"
          @suspend="(off) => setRunning(!off)"
          @app="appAction"
          @deps="startJob('POST', `api/sites/${s.id}/app/deps`, {})"
          @delete="emit('delete', s)"
          @install-wp="emit('install-wp', s)"
          @note="saveNote"
          @job="(j) => emit('job', j)"
        />

        <section v-else-if="['domains', 'php', 'runtime', 'proxy'].includes(current)" class="card">
          <SiteSettings
            :key="paneKey"
            :site="s"
            :section="current"
            :php-versions="phpVersions"
            @saved="saved"
            @reissue="startJob('POST', `api/sites/${s.id}/ssl`, {})"
            @close="reset"
          />
          <p v-if="current === 'php'" class="muted small note">
            {{ t('drawer.php_more') }} <RouterLink :to="{ name: 'software' }">{{ t('nav.software') }}</RouterLink>
          </p>
          <SitePointDNS v-if="current === 'domains'" :key="'dns-' + paneKey" :site="s" />
        </section>

        <template v-else-if="current === 'ssl'">
          <section class="card ssl-card">
            <div class="ssl-ring" :class="'ring-' + sslState" aria-hidden="true">
              <svg width="74" height="74" viewBox="0 0 74 74">
                <circle cx="37" cy="37" r="31" class="ring-track" />
                <circle v-if="s.ssl" cx="37" cy="37" r="31" class="ring-val" :stroke-dasharray="`${(Math.max(0, Math.min(90, days ?? 0)) / 90) * 194.8} 194.8`" />
              </svg>
              <div class="ring-text">
                <template v-if="s.ssl"><strong>{{ Math.max(0, days ?? 0) }}</strong><span>{{ t('drawer.days') }}</span></template>
                <Icon v-else name="lockOpen" :size="22" />
              </div>
            </div>
            <div class="ssl-info">
              <div class="row" style="gap: 8px">
                <strong class="ssl-title">{{ s.ssl ? (s.ssl_source === 'custom' ? t('ssl.source_custom') : t('ssl.source_le')) : t('websites.https_off') }}</strong>
                <span v-if="s.ssl" class="status-pill" :class="sslState">{{ t('drawer.ssl_state_' + sslState) }}</span>
              </div>
              <div v-if="s.ssl && s.cert_expires" class="muted small">{{ t('ssl.valid_until', { date: fmtDate(s.cert_expires) }) }}</div>
              <div v-else class="muted small">{{ t('ssl.none') }}</div>
              <details v-if="s.ssl_error" class="ssl-error" open>
                <summary>{{ t('websites.ssl_error') }}</summary>
                <p class="pre-wrap">{{ s.ssl_error }}</p>
              </details>
              <p v-if="s.ssl && s.ssl_source === 'custom' && days !== null && days <= 14" class="small ssl-custom" :class="days < 0 ? 'text-crit' : 'text-warn'">{{ t(days < 0 ? 'ssl.custom_expired' : 'ssl.custom_expiring', { date: fmtDate(s.cert_expires) }) }}</p>
            </div>
          </section>
          <div class="opt" style="margin-bottom: 16px">
            <div class="opt-text">
              <div class="opt-title">{{ t('websites.force_https') }}</div>
              <div class="opt-hint">{{ s.ssl ? t('drawer.force_https_hint') : t('drawer.force_https_needs_ssl') }}</div>
            </div>
            <button type="button" class="switch" role="switch" :aria-checked="!!s.force_https" :aria-label="t('websites.force_https')" :disabled="!s.ssl" @click="forceHTTPS"></button>
          </div>
          <SiteHTTPS v-if="s.ssl" :site="s" part="hsts" :busy="busy" @saved="saved" />
          <section class="card">
            <SiteSSL :key="paneKey" :site="s" :busy="busy" hide-status @issue="(o) => startJob('POST', `api/sites/${s.id}/ssl`, o)" @saved="saved" @close="reset" />
          </section>
          <SiteHTTPS v-if="s.ssl" :site="s" part="off" :busy="busy" @saved="saved" />
        </template>

        <SitePageCache v-else-if="current === 'cache'" :key="paneKey" :site="s" :server="server" :busy="busy" @saved="saved" />

        <section v-else-if="current === 'waf'" class="card">
          <SiteFirewall :key="paneKey" :site="s" :server="server" section="waf" @saved="saved" @close="reset" />
        </section>

        <template v-else-if="current === 'block'">
          <section class="card">
            <h3 class="sub-title">{{ hasRateLimit(server) ? t('drawer.block_ips_rate') : t('drawer.block_ips') }}</h3>
            <SiteFirewall :key="paneKey" :site="s" :server="server" section="block" @saved="saved" @close="reset" />
          </section>
          <section class="card">
            <h3 class="sub-title">{{ t('drawer.block_password') }}</h3>
            <SiteAccess :key="'p' + paneKey" :site="s" section="protect" @saved="saved" @close="reset" />
          </section>
        </template>

        <section v-else-if="['hotlink', 'redirect', '404', 'expiry'].includes(current)" class="card">
          <SiteAccess :key="paneKey" :site="s" :section="current === 'expiry' ? 'suspend' : current" @saved="saved" @close="reset" />
        </section>

        <section v-else-if="current === 'scan'" class="card">
          <SiteScan :key="paneKey" :site="s" @close="reset" />
        </section>

        <section v-else-if="current === 'rewrite' || current === 'webconf'" class="card">
          <SiteNginx :key="paneKey" :site="s" :server="server" :section="current === 'rewrite' ? 'rewrite' : 'custom'" @saved="saved" @close="reset" />
        </section>

        <SiteWordPress v-else-if="current === 'wordpress'" :key="paneKey" :site="s" :busy="busy" @job="(j) => emit('job', j)" />

        <section v-else-if="current === 'git'" class="card">
          <SiteGit :key="paneKey" :site="s" @close="reset" />
        </section>

        <section v-else-if="current === 'sftp'" class="card">
          <SiteSFTP :key="paneKey" :site="s" @saved="(i) => emit('saved', { ...s, sftp: i.enabled })" @close="reset" />
        </section>

        <SiteBackups v-else-if="current === 'backups'" :key="paneKey" :site="s" :busy="busy" @job="(j) => emit('job', j)" />

        <SiteMonitor v-else-if="current === 'monitor'" :key="paneKey" :site="s" />

        <SiteStats v-else-if="current === 'stats'" :key="paneKey" :site="s" :server="server" />

        <section v-else-if="current === 'logs'" class="card">
          <LogViewer :key="paneKey" :kinds="logKinds" :url="logURL" />
        </section>
      </section>
    </div>
  </Drawer>
</template>

<style scoped>
.dh-text { display: flex; flex-direction: column; gap: 6px; min-width: 0; flex: 1 1 260px; }
.dh-chips { display: flex; gap: 6px; flex-wrap: wrap; }
.dh-actions { display: flex; gap: 8px; align-items: center; margin-left: auto; }
.sub-title { font-size: 14px; margin: 0 0 12px; }
.ssl-card { display: flex; gap: 18px; align-items: center; flex-wrap: wrap; }
.ssl-ring { position: relative; width: 74px; height: 74px; flex: none; }
.ssl-ring > svg { transform: rotate(-90deg); display: block; }
.ring-track { fill: none; stroke: var(--surface-3); stroke-width: 7; }
.ring-val { fill: none; stroke-width: 7; stroke-linecap: round; stroke: var(--ok-g); }
.ring-warn .ring-val { stroke: var(--warn-g); }
.ring-crit .ring-val { stroke: var(--crit-g); }
.ring-text { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; line-height: 1; color: var(--muted); }
.ring-text strong { font-size: 21px; font-weight: 800; color: var(--text); }
.ring-text span { font-size: 10.5px; margin-top: 2px; }
.ssl-info { flex: 1; min-width: 220px; display: flex; flex-direction: column; gap: 4px; }
.ssl-title { font-size: 15.5px; font-weight: 800; }
.ssl-error { max-width: none; }
.ssl-custom { margin: 2px 0 0; }
@media (max-width: 640px) {
  .dh-actions { width: 100%; margin-left: 0; }
  .dh-actions .btn { flex: 1; }
  .dh-wp { display: none; }
  .drawer-head .fav { display: none; }
}
</style>
