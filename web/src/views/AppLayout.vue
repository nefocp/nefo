<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { features, refreshFeatures } from '../features.js'
import { server, loadServerInfo, watchStats, level } from '../server.js'
import { store, load, list } from '../data.js'
import { refreshAttention, critCount } from '../attention.js'
import { startJobsPolling, stopJobsPolling } from '../jobs.js'
import { pageMeta } from '../page.js'
import { BRAND, setDocumentTitle } from '../brand.js'
import { isDark, toggleTheme } from '../theme.js'
import { osLogo } from '../logos.js'
import Icon from '../components/Icon.vue'
import BrandMark from '../components/BrandMark.vue'
import CommandPalette from '../components/CommandPalette.vue'
import JobPill from '../components/JobPill.vue'
import NotificationsMenu from '../components/NotificationsMenu.vue'
import AccountMenu from '../components/AccountMenu.vue'

const { t } = useI18n()
const route = useRoute()

const groups = [
  { items: [{ name: 'dashboard', label: 'nav.dashboard', icon: 'dashboard' }] },
  {
    title: 'nav.group_hosting',
    items: [
      { name: 'websites', label: 'nav.websites', icon: 'globe', count: 'sites' },
      { name: 'apps', label: 'nav.apps', icon: 'package' },
      { name: 'wordpress', label: 'nav.wordpress', icon: 'wordpress', feature: 'wordpress_sites' },
      { name: 'databases', label: 'nav.databases', icon: 'database', count: 'databases' },
      { name: 'files', label: 'nav.files', icon: 'folder' },
      { name: 'backups', label: 'nav.backups', icon: 'archive' },
    ],
  },
  {
    title: 'nav.group_server',
    items: [
      { name: 'docker', label: 'nav.docker', icon: 'box', feature: 'docker' },
      { name: 'terminal', label: 'nav.terminal', icon: 'terminal' },
      { name: 'services', label: 'nav.services', icon: 'server' },
      { name: 'processes', label: 'nav.processes', icon: 'cpu' },
      { name: 'daemons', label: 'nav.daemons', icon: 'activity' },
      { name: 'cron', label: 'nav.cron', icon: 'clock' },
      { name: 'software', label: 'nav.software', icon: 'layers' },
      { name: 'system', label: 'nav.system', icon: 'gear' },
    ],
  },
  {
    title: 'nav.group_security',
    items: [
      { name: 'firewall', label: 'nav.firewall', icon: 'shield' },
      { name: 'security', label: 'nav.account', icon: 'lock', crit: true },
      { name: 'settings', label: 'nav.settings', icon: 'settings' },
    ],
  },
]

// Items with a feature only show while that software is installed (or
// while their page is open). Websites, Databases and Software always show:
// they offer the installs.
const shown = (item) => !item.feature || features[item.feature] || route.name === item.name
const visibleGroups = computed(() => groups.map((g) => ({ ...g, items: g.items.filter(shown) })).filter((g) => g.items.length))
const allItems = groups.flatMap((g) => g.items)

const counts = computed(() => ({
  sites: store['api/sites'] ? list(store['api/sites'].sites).length : null,
  databases: store['api/databases'] ? list(store['api/databases'].databases).length : null,
}))
const isActive = (item) => (item.name === 'dashboard' ? route.name === 'dashboard' : route.name === item.name)

// ---- title ----
const navTitle = computed(() => {
  const item = allItems.find((i) => i.name === route.name)
  return item ? t(item.label) : route.name === 'notfound' ? t('notfound.title') : ''
})
const title = computed(() => pageMeta.title || navTitle.value)
watch(title, (v) => setDocumentTitle(v), { immediate: true })

// ---- sidebar on phones ----
const open = ref(false)
const palette = ref(false)
const sideEl = ref(null)
const mainEl = ref(null)
watch(
  () => route.fullPath,
  () => {
    open.value = false
    palette.value = false
    refreshFeatures()
    load('api/sites', { ttl: 60000 }).catch(() => {})
  },
)

// ---- live meters ----
let stopStats = null
const cpu = computed(() => (server.stats?.supported ? Math.round(server.stats.cpu_percent) : null))
const mem = computed(() => (server.stats?.supported && server.stats.mem_total ? Math.round((server.stats.mem_used / server.stats.mem_total) * 100) : null))

// ---- command palette ----
const pages = computed(() => visibleGroups.value.flatMap((g) => g.items).map((i) => ({ name: i.name, label: t(i.label), icon: i.icon })))
function onKey(e) {
  if ((e.ctrlKey || e.metaKey) && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault()
    palette.value = !palette.value
  }
}

onMounted(() => {
  refreshFeatures(true)
  loadServerInfo().catch(() => {})
  stopStats = watchStats(10000)
  startJobsPolling()
  refreshAttention()
  load('api/databases', { ttl: 60000 }).catch(() => {})
  document.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => {
  stopStats?.()
  stopJobsPolling()
  document.removeEventListener('keydown', onKey)
})

const shortVersion = computed(() => {
  const v = server.info?.panel_version || ''
  const m = v.match(/^v?(\d+\.\d+)/)
  return m ? m[1] : v
})
</script>

<template>
  <a class="skip-link" href="#main" @click.prevent="mainEl?.focus()">{{ t('nav.skip') }}</a>
  <div class="shell" :data-route="route.name">
    <aside id="sidebar" ref="sideEl" class="side" :class="{ open }" :aria-label="t('nav.main')">
      <div class="side-brand">
        <BrandMark />
        <span class="brand-name">{{ BRAND.name }}</span>
        <span v-if="shortVersion" class="brand-ver" :title="server.info?.panel_version">{{ shortVersion }}</span>
        <button type="button" class="icon-btn side-close" :aria-label="t('common.close')" @click="open = false"><Icon name="x" /></button>
      </div>

      <div class="side-server" :title="server.info?.os || ''">
        <span class="os-tile">
          <img v-if="osLogo(server.info?.os)" :src="osLogo(server.info?.os)" :alt="server.info?.os || ''" />
          <Icon v-else name="server" :size="15" />
        </span>
        <span class="side-server-text">
          <span class="side-host truncate">{{ server.info?.hostname || '…' }}</span>
          <span class="side-ip truncate">{{ server.ip || ' ' }}</span>
        </span>
        <span class="live-dot" :class="{ off: !server.online }" role="img" :aria-label="server.online ? t('nav.online') : t('nav.offline')"></span>
      </div>

      <nav class="side-nav" :aria-label="t('nav.main')">
        <template v-for="(g, gi) in visibleGroups" :key="gi">
          <div v-if="g.title" class="nav-group">{{ t(g.title) }}</div>
          <RouterLink
            v-for="item in g.items"
            :key="item.name"
            :to="{ name: item.name }"
            class="nav-item"
            :class="{ active: isActive(item) }"
            :aria-current="isActive(item) ? 'page' : undefined"
          >
            <Icon :name="item.icon" :stroke="1.8" />
            <span>{{ t(item.label) }}</span>
            <span v-if="item.count && counts[item.count] !== null" class="count">{{ counts[item.count] }}</span>
            <span v-if="item.crit && critCount" class="count-crit" :aria-label="t('nav.crit_n', { n: critCount }, critCount)">{{ critCount }}</span>
          </RouterLink>
        </template>
      </nav>

      <div class="mini" :aria-label="t('nav.resources')" role="group">
        <div class="mini-row"><span>CPU</span><strong>{{ cpu === null ? '—' : cpu + '%' }}</strong></div>
        <div class="mini-bar" aria-hidden="true"><span :class="cpu !== null && level(cpu)" :style="{ width: (cpu ?? 0) + '%' }"></span></div>
        <div class="mini-row"><span>RAM</span><strong>{{ mem === null ? '—' : mem + '%' }}</strong></div>
        <div class="mini-bar" aria-hidden="true"><span :class="mem !== null && level(mem)" :style="{ width: (mem ?? 0) + '%' }"></span></div>
      </div>
    </aside>
    <div v-if="open" class="side-backdrop" @click="open = false"></div>

    <div class="main">
      <header class="topbar">
        <button type="button" class="icon-btn menu-toggle" :aria-label="t('nav.menu')" aria-controls="sidebar" :aria-expanded="open" @click="open = !open">
          <Icon name="menu" />
        </button>
        <div class="top-title">
          <span class="top-title-main truncate">{{ title }}</span>
          <span v-if="pageMeta.sub" class="top-title-sub truncate">{{ pageMeta.sub }}</span>
        </div>
        <button type="button" class="search-btn" :aria-label="t('palette.open_label')" @click="palette = true">
          <Icon name="search" :size="16" />
          <span class="search-label truncate">{{ t('palette.button') }}</span>
          <kbd>Ctrl K</kbd>
        </button>
        <div class="top-spacer"></div>
        <div id="topbar-actions" class="top-actions"></div>
        <JobPill />
        <NotificationsMenu />
        <button type="button" class="icon-btn theme-toggle" :aria-label="isDark ? t('nav.theme_light') : t('nav.theme_dark')" :title="isDark ? t('nav.theme_light') : t('nav.theme_dark')" @click="toggleTheme">
          <Icon :name="isDark ? 'sun' : 'moon'" :size="19" :stroke="1.8" />
        </button>
        <AccountMenu />
      </header>

      <main id="main" ref="mainEl" class="content" tabindex="-1">
        <RouterView />
      </main>
    </div>
  </div>
  <CommandPalette v-if="palette" :pages="pages" @close="palette = false" />
</template>
