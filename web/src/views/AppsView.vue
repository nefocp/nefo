<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { load as loadData, put, list } from '../data.js'
import { toast } from '../toast.js'
import { useJob } from '../composables/useJob.js'
import { usePageMeta } from '../page.js'
import Icon from '../components/Icon.vue'
import AppLogo from '../components/AppLogo.vue'
import JobLog from '../components/JobLog.vue'
import AppCard from '../components/apps/AppCard.vue'
import AppInstalled from '../components/apps/AppInstalled.vue'
import AppInstallDrawer from '../components/apps/AppInstallDrawer.vue'
import SecretField from '../components/apps/SecretField.vue'
import { CATEGORIES, tx } from '../components/apps/appstore.js'

// The app store: the catalog (Docker apps with their own website, PHP apps
// for an empty website, services for this server only) and the installed
// apps. Installs run as jobs and show in the job pill like everywhere else.
const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()

const data = ref(null) // GET /api/apps
const sites = ref([])
const error = ref('')
const installing = ref(null) // the app whose install drawer is open
const phpAccount = ref(null) // a PHP app's account, shown once after the install starts

const apps = computed(() => list(data.value?.apps))
// The catalog's answer lists the installed apps without their memory use
// (docker stats is slow); the Installed tab asks for it separately.
const live = ref(null)
const installed = computed(() => list(live.value ?? data.value?.installed))
const dockerReady = computed(() => !!data.value?.docker_installed)

async function loadLive() {
  try {
    live.value = list((await api('GET', 'api/apps/installed')).installed)
  } catch {
    // the catalog's list is shown
  }
}

async function load(force = true) {
  try {
    const [d, s] = await Promise.all([loadData('api/apps', { force }), loadData('api/sites', { force })])
    data.value = d
    live.value = null
    sites.value = list(s?.sites)
    error.value = ''
    if (tab.value === 'installed') loadLive()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// ---- jobs ----
const appById = computed(() => Object.fromEntries(apps.value.map((a) => [a.id, a])))
const { job, running: jobRunning, watch: watchJob, resume, clear: clearJob } = useJob(async (j) => {
  await load()
  if (j.status !== 'succeeded') return
  if (j.kind.startsWith('install_')) {
    const id = j.kind.slice('install_'.length)
    toast(t('apps.installed_toast', { app: appById.value[id]?.name || id }))
    tab.value = 'installed'
  } else if (j.kind === 'update_app') toast(t('apps.updated_toast'))
  else if (j.kind === 'backup') toast(t('apps.backup_toast'))
  else if (j.kind === 'uninstall_app') toast(t('apps.uninstalled_toast'))
})
function started(res) {
  const app = installing.value
  installing.value = null
  if (res.admin_password) phpAccount.value = { app: app.name, url: res.admin_url, user: res.admin_user, password: res.admin_password }
  else phpAccount.value = null
  if (res.job) watchJob(res.job)
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

// ---- tabs and filters ----
const tab = ref(route.query.tab === 'installed' ? 'installed' : 'catalog')
watch(tab, (v) => {
  if ((route.query.tab || 'catalog') !== v) router.replace({ query: { ...route.query, tab: v === 'catalog' ? undefined : v } })
  if (v === 'installed' && data.value) loadLive()
})
// The menu's "App store" link (or Back) changes the query of this same page.
watch(() => route.query.tab, (v) => (tab.value = v === 'installed' ? 'installed' : 'catalog'))
const q = ref('')
const cat = ref('all')
const fold = (s) => String(s || '').normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/đ/g, 'd').toLowerCase()
const cats = computed(() => {
  const n = {}
  for (const a of apps.value) n[a.category] = (n[a.category] || 0) + 1
  return CATEGORIES.filter((c) => n[c]).map((c) => ({ id: c, n: n[c] }))
})
const blockedOf = (a) => (a.kind !== 'php' && !dockerReady.value) || !!a.check?.refusals?.length
const shown = computed(() => {
  const f = fold(q.value.trim())
  const out = apps.value.filter((a) => (cat.value === 'all' || a.category === cat.value) &&
    (!f || fold(`${a.name} ${tx(a.description, locale.value)} ${a.category} ${t('apps.cat_' + a.category)}`).includes(f)))
  // what can be installed here first
  return [...out.filter((a) => !blockedOf(a)), ...out.filter(blockedOf)]
})
const installsOf = (a) => installed.value.filter((x) => x.app === a.id).length

// ---- header ----
const sourceText = computed(() => (data.value?.catalog?.source === 'remote' ? t('apps.src_remote') : t('apps.src_embedded')))
usePageMeta(() => ({ sub: data.value ? t('apps.sub', { n: apps.value.length, m: installed.value.length }, apps.value.length) : '' }))
const refreshing = ref(false)
async function refreshCatalog() {
  refreshing.value = true
  try {
    await api('POST', 'api/apps/catalog/refresh', {})
    await load()
    toast(t('apps.catalog_refreshed'))
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  } finally {
    refreshing.value = false
  }
}

// ?install=<id> (from the command palette) opens that app's drawer.
function openFromRoute() {
  const id = route.query.install
  if (!id || !data.value) return
  const a = appById.value[id]
  router.replace({ query: { ...route.query, install: undefined } })
  if (a) {
    tab.value = 'catalog'
    installing.value = a
  }
}
watch(() => route.query.install, openFromRoute)

onMounted(async () => {
  await load(false)
  openFromRoute()
  resume([...apps.value.map((a) => 'install_' + a.id), 'update_app', 'backup', 'uninstall_app'])
  // The cached answer may be old: ask again for the live state.
  load()
})

function onInstalledChanged() {
  load()
}
// keep the shared cache up to date for the palette and other pages
watch(data, (d) => d && put('api/apps', d))
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('apps.title') }}</h1></div>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />

    <section v-if="phpAccount" class="panel as-account" aria-live="polite">
      <div class="panel-head">
        <h2 class="panel-title"><Icon name="key" :size="16" />{{ t('apps.creds_title', { app: phpAccount.app }) }}</h2>
        <button type="button" class="icon-btn" :aria-label="t('common.close')" @click="phpAccount = null"><Icon name="x" /></button>
      </div>
      <div class="panel-body">
        <p class="muted small as-account-note">{{ t('apps.creds_body') }}</p>
        <div class="as-account-grid">
          <SecretField :label="t('apps.admin_url')" :value="phpAccount.url" link />
          <SecretField :label="t('apps.admin_user')" :value="phpAccount.user" />
          <SecretField :label="t('apps.admin_password')" :value="phpAccount.password" secret />
        </div>
      </div>
    </section>

    <template v-if="!data && !error">
      <span class="skel" style="height: 84px; border-radius: 14px"></span>
      <div class="as-grid"><span v-for="i in 8" :key="i" class="skel" style="height: 196px; border-radius: 14px"></span></div>
    </template>

    <template v-if="data">
      <section class="svc-head">
        <AppLogo size="lg" icon="package" />
        <div class="svc-text">
          <h2 class="svc-title">{{ t('apps.title') }}</h2>
          <div class="svc-meta">
            <span><Icon name="package" :size="14" />{{ t('apps.n_apps', { n: apps.length }, apps.length) }}</span>
            <span><Icon name="checkCircle" :size="14" />{{ t('apps.n_installed', { n: installed.length }) }}</span>
            <span><Icon name="layers" :size="14" />{{ t('apps.catalog_version', { v: data.catalog?.version }) }} · {{ sourceText }}</span>
          </div>
        </div>
        <div v-if="data.catalog?.remote_enabled" class="svc-actions">
          <button type="button" class="btn" :disabled="refreshing" @click="refreshCatalog"><Icon name="refresh" :size="15" />{{ t('apps.catalog_refresh') }}</button>
        </div>
      </section>

      <div class="as-tabbar">
        <nav class="ptabs" role="tablist" :aria-label="t('apps.tabs')">
          <button id="apps-tab-catalog" type="button" role="tab" :aria-selected="tab === 'catalog'" aria-controls="apps-panel-catalog" @click="tab = 'catalog'">
            <Icon name="package" :size="16" />{{ t('apps.tab_catalog') }}<span class="n">{{ apps.length }}</span>
          </button>
          <button id="apps-tab-installed" type="button" role="tab" :aria-selected="tab === 'installed'" aria-controls="apps-panel-installed" @click="tab = 'installed'">
            <Icon name="checkCircle" :size="16" />{{ t('apps.tab_installed') }}<span class="n">{{ installed.length }}</span>
          </button>
        </nav>
        <label v-if="tab === 'catalog'" class="search-field as-search">
          <Icon name="search" :size="15" />
          <input v-model="q" type="search" :placeholder="t('apps.search')" :aria-label="t('apps.search')" />
        </label>
      </div>

      <div v-if="tab === 'catalog'" id="apps-panel-catalog" class="pg" role="tabpanel" aria-labelledby="apps-tab-catalog">
        <div v-if="!dockerReady" class="alert alert-warn as-docker">
          <AppLogo name="docker" size="sm" />
          <span>{{ t('apps.docker_missing_long') }}</span>
          <RouterLink class="btn btn-sm" :to="{ name: 'software' }"><Icon name="download" :size="15" />{{ t('apps.install_docker') }}</RouterLink>
        </div>

        <div class="as-tools">
          <div class="as-cats" role="group" :aria-label="t('apps.categories')">
            <button type="button" class="as-cat" :aria-pressed="cat === 'all'" @click="cat = 'all'">{{ t('apps.cat_all') }} <span class="n">{{ apps.length }}</span></button>
            <button v-for="c in cats" :key="c.id" type="button" class="as-cat" :aria-pressed="cat === c.id" @click="cat = c.id">
              {{ t(`apps.cat_${c.id}`) }} <span class="n">{{ c.n }}</span>
            </button>
          </div>
        </div>

        <div v-if="!shown.length" class="card empty">
          <div class="empty-icon"><Icon name="search" :size="26" /></div>
          <p class="empty-lead">{{ t('apps.none_found') }}</p>
          <button type="button" class="btn" @click="q = ''; cat = 'all'">{{ t('apps.clear_filter') }}</button>
        </div>
        <div v-else class="as-grid">
          <AppCard
            v-for="a in shown"
            :key="a.id"
            :app="a"
            :installs="installsOf(a)"
            :docker-ready="dockerReady"
            @open="installing = a"
            @manage="tab = 'installed'"
          />
        </div>
      </div>

      <div v-else id="apps-panel-installed" role="tabpanel" aria-labelledby="apps-tab-installed">
        <AppInstalled :installed="installed" :apps="apps" :sites="sites" :busy="jobRunning" @job="watchJob" @changed="onInstalledChanged" @browse="tab = 'catalog'" />
      </div>
    </template>

    <AppInstallDrawer
      v-if="installing"
      :key="installing.id"
      :app="installing"
      :sites="sites"
      :docker-ready="dockerReady"
      :busy="jobRunning"
      @close="installing = null"
      @started="started"
    />
  </div>
</template>

<style scoped>
.as-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(268px, 1fr)); gap: 14px; }
.as-tabbar { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; border-bottom: 1px solid var(--border); }
.as-tabbar .ptabs { border-bottom: 0; flex: 1 1 auto; }
.as-search { flex: 0 1 300px; background: var(--surface); margin-bottom: 4px; }
.as-tools { display: flex; }
.as-cats { flex: 1 1 auto; display: flex; flex-wrap: wrap; gap: 6px; min-width: 0; }
.as-cat {
  display: inline-flex; align-items: center; gap: 6px; min-height: 32px; padding: 0 12px; border-radius: 999px;
  border: 1px solid var(--border); background: var(--surface); color: var(--text-2); font: 600 12.5px var(--font); cursor: pointer; white-space: nowrap;
}
.as-cat:hover { background: var(--hover); color: var(--text); }
.as-cat .n { font-size: 11.5px; color: var(--muted); font-weight: 600; }
/* the chosen chip is the text colour on the page colour: ink in light, light in dark */
.as-cat[aria-pressed='true'] { background: var(--text); border-color: var(--text); color: var(--bg); }
.as-cat[aria-pressed='true'] .n { color: color-mix(in srgb, var(--bg) 70%, transparent); }
.as-docker { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.as-docker span { flex: 1 1 260px; }
.as-account-note { margin: 0 0 12px; }
.as-account-grid { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
@media (max-width: 640px) {
  .as-tabbar { border-bottom: 0; }
  .as-tabbar .ptabs { flex: 1 1 100%; border-bottom: 1px solid var(--border); }
  .as-search { flex: 1 1 100%; margin-bottom: 0; }
  .as-cats { flex-wrap: nowrap; overflow-x: auto; scrollbar-width: none; margin: 0 -16px; padding: 0 16px 2px; }
  .as-cats::-webkit-scrollbar { display: none; }
  .as-cat { min-height: 44px; }
  .as-grid { grid-template-columns: 1fr; }
}
</style>
