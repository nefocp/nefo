<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { useJob } from '../composables/useJob.js'
import { refreshFeatures } from '../features.js'
import Modal from '../components/Modal.vue'
import Icon from '../components/Icon.vue'
import JobLog from '../components/JobLog.vue'
import RowActions from '../components/RowActions.vue'
import PHPManage from '../components/PHPManage.vue'
import NodeVersions from '../components/NodeVersions.vue'
import JavaVersions from '../components/JavaVersions.vue'
import SoftwareCard from '../components/SoftwareCard.vue'
import DbEngineCard from '../components/DbEngineCard.vue'
import SoftwareRemoveModal from '../components/SoftwareRemoveModal.vue'
import WebServerCard from '../components/WebServerCard.vue'
import AppLogo from '../components/AppLogo.vue'
import { usePageMeta } from '../page.js'

// Everything optional the server can run, grouped, each with Install and
// Uninstall: a server only carries what its websites use.
const { t } = useI18n()
const route = useRoute()
const items = ref([])
const supported = ref(true)
const sites = ref(0)
const loaded = ref(false)
const error = ref('')
const managePHP = ref('')
const removing = ref(null) // item whose uninstall is being confirmed
const thirdParty = ref(null) // PHP version from an extra repository, confirming the install
const pgInstall = ref(null) // PostgreSQL's install dialog: { item, versions, choice, error }
const jobItem = ref('') // what the running job installs or removes ("web" for the web server)
const jobGroup = ref('') // the section that shows its log; "" puts it at the top
const stamp = ref(0) // changes after every job, so the web server and Node.js cards reload

const { job, running, watch: watchJob, resume, clear } = useJob(async () => {
  stamp.value++
  await Promise.all([load(), refreshFeatures(true)])
})

async function load() {
  try {
    const res = await api('GET', 'api/software')
    items.value = res.items || []
    supported.value = res.supported
    sites.value = res.sites ?? 0
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loaded.value = true
  }
}

onMounted(async () => {
  await load()
  // ?php=8.3 opens that version's settings (from the security check).
  const v = route.query.php
  if (typeof v === 'string' && items.value.some((i) => i.kind === 'php' && i.installed && i.version === v)) managePHP.value = v
  resume(['install_software', 'remove_software', 'update_software', 'install_web', 'remove_web', 'upgrade_db', 'switch_db'])
})

const byId = computed(() => Object.fromEntries(items.value.map((i) => [i.id, i])))
const pick = (...ids) => ids.map((id) => byId.value[id]).filter(Boolean)
const php = computed(() => items.value.filter((i) => i.kind === 'php'))
// PHP 5.6 to 7.3 ("legacy"): for old websites only, folded away under the
// others. "eol" (no security updates on this server) can mark newer ones too.
const phpCurrent = computed(() => php.value.filter((p) => !p.legacy))
const phpOld = computed(() => php.value.filter((p) => p.legacy))
const phpOldInstalled = computed(() => phpOld.value.filter((p) => p.installed).length)
// Composer and WP-CLI, in the Tools group.
const tools = computed(() => pick('composer', 'wpcli'))
const node = computed(() => byId.value.nodejs)
const java = computed(() => byId.value.java)
// The ionCube Loader, in the PHP group: a card with what it does for each
// installed PHP version.
const ioncube = computed(() => byId.value.ioncube)
// MariaDB's series from MariaDB's repository (mariadb-11.4…), offered as
// versions on the database server card.
const mariaSeries = computed(() => items.value.filter((i) => i.kind === 'mariadb' && i.id !== 'mariadb'))

const ICONS = { mariadb: 'database', mysql: 'database', phpmyadmin: 'sliders', postgresql: 'database', redis: 'memory', memcached: 'memory', adminer: 'sliders', nodejs: 'code', python: 'code', java: 'code', docker: 'box', waf: 'shield', clamav: 'lock', ioncube: 'key' }
const groups = computed(() => [
  // MariaDB or MySQL: one card (DbEngineCard) before these.
  { id: 'data', items: pick('postgresql', 'redis', 'memcached', 'adminer', 'phpmyadmin') },
  { id: 'runtimes', items: pick('nodejs', 'python', 'java') },
  { id: 'containers', items: pick('docker') },
  { id: 'security', items: pick('waf', 'clamav') },
])

const groupIcon = { data: 'database', runtimes: 'code', containers: 'box', security: 'shield' }
// An installed MariaDB series is the MariaDB item too: counted once.
const installedItems = computed(() => items.value.filter((i) => i.installed && !(i.kind === 'mariadb' && i.id !== 'mariadb')))
const installedCount = computed(() => installedItems.value.length)
const ramTotal = computed(() => installedItems.value.reduce((a, i) => a + (i.ram_mb || 0), 0))
usePageMeta(() => ({ sub: loaded.value ? t('pg.sw_sub', { n: installedCount.value, mb: ramTotal.value }, installedCount.value) : '' }))

const nameOf = (item) => (item.kind === 'php' ? `PHP ${item.version}` : t(`software.${item.kind}`))

function started(snapshot, id, group) {
  jobItem.value = id
  jobGroup.value = group
  watchJob(snapshot)
  nextTick(() => document.getElementById('sw-job')?.scrollIntoView({ behavior: 'smooth', block: 'nearest' }))
}

function closeJob() {
  clear()
  jobItem.value = jobGroup.value = ''
}

async function install(item, confirmed = false) {
  error.value = ''
  // An old PHP version is installed only after the risk was shown (the
  // server refuses it without accept_eol).
  if (item.kind === 'php' && (item.third_party || item.legacy) && !confirmed) {
    thirdParty.value = item
    return
  }
  // PostgreSQL: the distribution's version or one from apt.postgresql.org.
  if (item.kind === 'postgresql' && !confirmed) {
    pgInstall.value = { item, versions: null, choice: '', error: '' }
    try {
      pgInstall.value.versions = await api('GET', 'api/postgres/versions')
    } catch (e) {
      pgInstall.value.error = errorMessage(e)
    }
    return
  }
  thirdParty.value = null
  try {
    started(await api('POST', `api/software/${item.id}/install`, item.legacy ? { accept_eol: true } : {}), item.id, item.group)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// Installing again loads an installed extension into every PHP version.
async function reinstall(item) {
  error.value = ''
  try {
    started(await api('POST', `api/software/${item.id}/install`, {}), item.id, item.group)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function installPostgres() {
  const p = pgInstall.value
  p.error = ''
  try {
    started(await api('POST', 'api/software/postgresql/install', p.choice ? { version: p.choice } : {}), p.item.id, p.item.group)
    pgInstall.value = null
  } catch (e) {
    p.error = errorMessage(e)
  }
}

// Update installs a tool's latest official release.
async function update(item) {
  error.value = ''
  try {
    started(await api('POST', `api/software/${item.id}/update`, {}), item.id, item.group)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// What a tool's card says about where it comes from and what uses it.
function toolNotes(it) {
  if (!it.installed) return it.kind === 'wpcli' ? [t('software.wpcli_auto')] : []
  const notes = []
  if (it.kind === 'composer') {
    notes.push(t(`software.composer_source_${it.source === 'official' ? 'official' : 'apt'}`))
    const apps = it.usage?.apps || 0
    const deploys = it.usage?.deploys || 0
    if (apps) notes.push(t('software.composer_apps', { n: apps }, apps))
    if (deploys) notes.push(t('software.composer_deploys', { n: deploys }, deploys))
  } else {
    notes.push(it.version ? t('software.wpcli_source') : t('software.wpcli_unknown'))
  }
  return notes
}

async function remove(opts) {
  const item = removing.value
  removing.value = null
  error.value = ''
  try {
    started(await api('POST', `api/software/${item.id}/remove`, opts), item.id, item.group)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

function webJob(snapshot) {
  error.value = ''
  started(snapshot, 'web', 'web')
}

const working = (id) => running.value && jobItem.value === id

// What an uninstall does, in the confirmation dialog.
const removeBody = (item) => t(`software.remove_${item.kind}`, { name: nameOf(item), dir: item.data_dir || '' })
const dataLabel = (item) =>
  ['docker', 'mariadb', 'mysql', 'postgresql'].includes(item.kind) && item.has_data ? t(`software.delete_data_${item.kind}`, { dir: item.data_dir }) : ''

// What the ionCube Loader does for one installed PHP version.
const ioncubeState = (s) => t(`software.ioncube_${s.state}`, { v: s.version, loader: ioncube.value?.version || '' })

function phpNote(p) {
  if (!p.installed && p.not_offered) return t(`errors.${p.not_offered}`, { detail: p.not_offered_detail || '' })
  if (!p.installed) return p.third_party ? t('software.from_ppa') : t('software.from_distro')
  if (p.used_by) return t('software.used_by', { n: p.used_by }, p.used_by)
  if (p.blocked === 'php_last') return t('software.php_only')
  if (p.blocked === 'php_last_supported') return t('software.php_last_supported')
  return ''
}

function phpActions(p) {
  if (!p.installed) {
    return [{ label: t('software.install'), icon: 'download', inline: true, disabled: running.value || !supported.value || !!p.not_offered, onClick: () => install(p) }]
  }
  return [
    { label: t('php.manage'), icon: 'settings', inline: true, onClick: () => (managePHP.value = p.version) },
    { label: t('software.uninstall'), icon: 'trash', danger: true, disabled: running.value || !p.removable, onClick: () => (removing.value = p) },
  ]
}
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('software.title') }}</h1></div>
    <Teleport defer to="#topbar-actions">
      <RouterLink v-if="supported" class="btn" :to="{ name: 'setup' }"><Icon name="rocket" :size="15" />{{ t('software.setup_again') }}</RouterLink>
    </Teleport>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="!supported" class="alert alert-warn">{{ t('websites.unsupported') }}</p>
    <JobLog v-if="job && !jobGroup" id="sw-job" :job="job" @close="closeJob" />

    <template v-if="!loaded">
      <div class="kpis"><span v-for="i in 3" :key="i" class="skel" style="height: 72px; border-radius: 14px"></span></div>
      <span class="skel" style="height: 220px; border-radius: 14px"></span>
    </template>

    <template v-if="loaded">
      <div class="kpis">
        <div class="kpi"><span class="kpi-ico c-teal"><Icon name="layers" :size="20" /></span><div><div class="kpi-val">{{ installedCount }}</div><div class="kpi-label">{{ t('pg.sw_installed') }}</div></div></div>
        <div class="kpi"><span class="kpi-ico c-violet"><Icon name="memory" :size="20" /></span><div><div class="kpi-val">{{ t('pg.sw_ram', { mb: ramTotal }) }}</div><div class="kpi-label">{{ t('pg.sw_ram_label') }}</div></div></div>
        <div class="kpi"><span class="kpi-ico c-blue"><Icon name="globe" :size="20" /></span><div><div class="kpi-val">{{ sites }}</div><div class="kpi-label">{{ t('nav.websites') }}</div></div></div>
      </div>

      <!-- web server -->
      <section class="sw-group" aria-labelledby="sw-g-web">
        <JobLog v-if="job && jobGroup === 'web'" id="sw-job" :job="job" @close="closeJob" />
        <WebServerCard :busy="running" :supported="supported" :sites="sites" :stamp="stamp" @job="webJob" @error="(m) => (error = m)" />
      </section>

      <!-- PHP -->
      <section class="sw-group" aria-labelledby="sw-g-php">
        <div class="sw-group-head">
          <h2 id="sw-g-php"><AppLogo name="php" size="xs" />{{ t('software.php_title') }}</h2>
          <p class="muted small">{{ t('software.php_hint') }}</p>
        </div>
        <JobLog v-if="job && jobGroup === 'php'" id="sw-job" :job="job" @close="closeJob" />
        <div class="php-grid">
          <div v-for="p in phpCurrent" :key="p.id" class="php-tile" :class="{ on: p.installed }">
            <div class="php-top">
              <AppLogo name="php" size="sm" :dim="!p.installed" />
              <div class="php-name">
                <div class="php-v">PHP {{ p.version }}</div>
                <div class="small muted">{{ phpNote(p) || (p.installed ? t('software.ram_about', { mb: p.ram_mb }) : '') }}</div>
              </div>
              <span v-if="working(p.id)" class="status-pill warn"><span class="spinner" aria-hidden="true"></span>{{ t('software.working') }}</span>
              <span v-else-if="p.installed" class="status-pill ok">{{ t('software.installed') }}</span>
            </div>
            <template v-if="p.eol">
              <span class="tag tag-crit eol-tag">{{ t('software.eol_badge') }}</span>
              <p class="small muted eol-line">{{ t('software.eol_since', { date: p.security_until }) }}</p>
            </template>
            <div class="php-actions"><RowActions :actions="phpActions(p)" /></div>
          </div>
        </div>
        <!-- PHP 5.6 to 7.3: folded, for old websites only -->
        <details v-if="phpOld.length" class="php-old">
          <summary>
            {{ t('software.php_old_title') }}
            <span class="tag tag-crit">{{ t('software.eol_badge') }}</span>
            <span v-if="phpOldInstalled" class="muted small">{{ t('software.php_old_installed', { n: phpOldInstalled }) }}</span>
          </summary>
          <p class="muted small php-old-hint">{{ t('software.php_old_hint') }}</p>
          <div class="php-grid">
            <div v-for="p in phpOld" :key="p.id" class="php-tile eol" :class="{ on: p.installed }">
              <div class="php-top">
                <AppLogo name="php" size="sm" :dim="!p.installed" />
                <div class="php-name">
                  <div class="php-v">PHP {{ p.version }}</div>
                  <div class="small muted">{{ phpNote(p) || (p.installed ? t('software.ram_about', { mb: p.ram_mb }) : '') }}</div>
                </div>
                <span v-if="working(p.id)" class="status-pill warn"><span class="spinner" aria-hidden="true"></span>{{ t('software.working') }}</span>
                <span v-else-if="p.installed" class="status-pill ok">{{ t('software.installed') }}</span>
              </div>
              <span class="tag tag-crit eol-tag">{{ t('software.eol_badge') }}</span>
              <div class="php-actions"><RowActions :actions="phpActions(p)" /></div>
            </div>
          </div>
        </details>
        <div v-if="ioncube" class="sw-grid">
          <SoftwareCard
            :item="ioncube"
            :icon="ICONS.ioncube"
            :busy="running"
            :working="working('ioncube')"
            :supported="supported"
            @install="install"
            @remove="(i) => (removing = i)"
          >
            <button v-if="ioncube.installed && ioncube.updatable" class="btn btn-sm" :disabled="running || !supported" @click="update(ioncube)">
              <Icon name="refresh" :size="15" />{{ t('software.update') }}
            </button>
            <button v-else-if="ioncube.installed && (ioncube.php || []).some((s) => s.state === 'off')" class="btn btn-sm" :disabled="running || !supported" @click="reinstall(ioncube)">
              <Icon name="download" :size="15" />{{ t('software.ioncube_load_all') }}
            </button>
            <template #extra>
              <ul v-if="(ioncube.php || []).length" class="tool-notes small">
                <li v-for="s in ioncube.php" :key="s.version" :class="s.state === 'enabled' ? '' : 'muted'">
                  <strong>PHP {{ s.version }}</strong>: {{ ioncubeState(s) }}
                </li>
              </ul>
              <p v-else class="muted small sw-body">{{ t('software.ioncube_no_php') }}</p>
              <p v-if="!ioncube.installed && ioncube.not_offered" class="muted small sw-body">{{ t(`errors.${ioncube.not_offered}`, { detail: ioncube.not_offered_detail || '' }) }}</p>
            </template>
          </SoftwareCard>
        </div>
      </section>

      <!-- the other groups -->
      <section v-for="g in groups" :key="g.id" class="sw-group" :aria-labelledby="`sw-g-${g.id}`">
        <div class="sw-group-head">
          <h2 :id="`sw-g-${g.id}`"><Icon :name="groupIcon[g.id]" :size="17" class="g-ico" />{{ t(`software.group_${g.id}`) }}</h2>
          <p class="muted small">{{ t(`software.group_${g.id}_hint`) }}</p>
        </div>
        <JobLog v-if="job && jobGroup === g.id" id="sw-job" :job="job" @close="closeJob" />
        <div class="sw-grid">
          <DbEngineCard
            v-if="g.id === 'data' && byId.mariadb"
            :mariadb="byId.mariadb"
            :mysql="byId.mysql || null"
            :series="mariaSeries"
            :busy="running"
            :working="working('mariadb') || working('mysql') || mariaSeries.some((s) => working(s.id))"
            :supported="supported"
            :stamp="stamp"
            @install="install"
            @remove="(i) => (removing = i)"
            @job="(snap) => started(snap, 'mariadb', 'data')"
            @error="(m) => (error = m)"
          />
          <SoftwareCard
            v-for="it in g.items"
            :key="it.id"
            :item="it"
            :icon="ICONS[it.kind]"
            :busy="running"
            :working="working(it.id)"
            :supported="supported"
            @install="install"
            @remove="(i) => (removing = i)"
          >
            <template v-if="it.kind === 'postgresql' && it.installed" #extra>
              <p class="muted small sw-src">{{ it.third_party ? t('pgv.source_pgdg') : t('pgv.source_distribution') }}</p>
            </template>
            <template v-if="it.installed">
              <RouterLink v-if="it.kind === 'mariadb'" class="btn btn-sm" :to="{ name: 'databases' }">{{ t('software.open_databases') }}</RouterLink>
              <RouterLink v-else-if="it.kind === 'postgresql'" class="btn btn-sm" :to="{ name: 'databases', query: { tab: 'postgres' } }">{{ t('software.open_postgresql') }}</RouterLink>
              <RouterLink v-else-if="it.kind === 'redis'" class="btn btn-sm" :to="{ name: 'databases', query: { tab: 'redis' } }">{{ t('software.open_redis') }}</RouterLink>
              <RouterLink v-else-if="it.kind === 'memcached'" class="btn btn-sm" :to="{ name: 'databases', query: { tab: 'memcached' } }">{{ t('software.open_memcached') }}</RouterLink>
              <a v-else-if="it.kind === 'adminer'" class="btn btn-sm" href="adminer/" target="_blank" rel="noopener">
                <Icon name="external" :size="15" />{{ t('software.open_adminer') }}
              </a>
              <a v-else-if="it.kind === 'phpmyadmin'" class="btn btn-sm" href="phpmyadmin/" target="_blank" rel="noopener">
                <Icon name="external" :size="15" />{{ t('software.open_phpmyadmin') }}
              </a>
              <RouterLink v-else-if="it.kind === 'docker'" class="btn btn-sm" :to="{ name: 'docker' }">{{ t('software.open_docker') }}</RouterLink>
            </template>
          </SoftwareCard>
        </div>
        <NodeVersions v-if="g.id === 'runtimes' && node && (node.installed || node.lines)" class="sw-after-grid" :stamp="stamp" @changed="load" />
        <JavaVersions v-if="g.id === 'runtimes' && java && java.installed" class="sw-after-grid" :stamp="stamp" @changed="load" />
      </section>

      <!-- tools: Composer and WP-CLI -->
      <section v-if="tools.length" class="sw-group" aria-labelledby="sw-g-tools">
        <div class="sw-group-head">
          <h2 id="sw-g-tools"><Icon name="terminal" :size="17" class="g-ico" />{{ t('software.group_tools') }}</h2>
          <p class="muted small">{{ t('software.group_tools_hint') }}</p>
        </div>
        <JobLog v-if="job && jobGroup === 'tools'" id="sw-job" :job="job" @close="closeJob" />
        <div class="sw-grid">
          <SoftwareCard
            v-for="it in tools"
            :key="it.id"
            :item="it"
            :icon="it.kind === 'wpcli' ? 'wordpress' : 'package'"
            :busy="running"
            :working="working(it.id)"
            :supported="supported"
            @install="install"
            @remove="(i) => (removing = i)"
          >
            <button v-if="it.installed && it.updatable" class="btn btn-sm" :disabled="running || !supported" @click="update(it)">
              <Icon name="refresh" :size="15" />{{ t('software.update') }}
            </button>
            <template #extra>
              <ul v-if="toolNotes(it).length" class="tool-notes muted small">
                <li v-for="n in toolNotes(it)" :key="n">{{ n }}</li>
              </ul>
            </template>
          </SoftwareCard>
        </div>
      </section>
    </template>

    <SoftwareRemoveModal
      v-if="removing"
      :name="nameOf(removing)"
      :body="removeBody(removing)"
      :data-label="dataLabel(removing)"
      :data-word="removing.id"
      :busy="running"
      @confirm="remove"
      @close="removing = null"
    />

    <Modal v-if="thirdParty" :title="t('software.install_title', { name: nameOf(thirdParty) })" @close="thirdParty = null">
      <div class="stack">
        <p v-if="thirdParty.legacy" class="alert alert-warn sw-body" role="alert">
          <strong>{{ t('software.eol_badge') }}.</strong> {{ t('software.eol_confirm', { v: thirdParty.version }) }}
        </p>
        <p class="sw-body">{{ t('software.third_party_confirm', { name: nameOf(thirdParty) }) }}</p>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="thirdParty = null">{{ t('common.cancel') }}</button>
          <button class="btn" :class="thirdParty.legacy ? 'btn-danger' : 'btn-primary'" :disabled="running" @click="install(thirdParty, true)">
            <Icon name="download" :size="15" />{{ thirdParty.legacy ? t('software.install_anyway') : t('software.install') }}
          </button>
        </div>
      </div>
    </Modal>

    <Modal v-if="pgInstall" :title="t('software.install_title', { name: 'PostgreSQL' })" @close="pgInstall = null">
      <div class="stack">
        <p v-if="pgInstall.error" class="alert alert-crit pre-wrap" role="alert">{{ pgInstall.error }}</p>
        <span v-if="!pgInstall.versions && !pgInstall.error" class="skel" style="height: 120px; border-radius: 10px"></span>
        <template v-if="pgInstall.versions">
          <fieldset class="pgv-choices">
            <legend class="pgv-legend">{{ t('pgv.choose') }}</legend>
            <label class="pgv-choice" :class="{ on: pgInstall.choice === '' }">
              <input v-model="pgInstall.choice" type="radio" value="" />
              <span>
                <strong>{{ pgInstall.versions.distribution ? t('pgv.option_distribution', { v: pgInstall.versions.distribution }) : t('pgv.option_distribution_unknown') }}</strong>
                <span class="tag tag-ok">{{ t('pgv.recommended') }}</span>
                <span class="small muted pgv-sub">{{ t('pgv.option_distribution_hint', { release: pgInstall.versions.release }) }}</span>
              </span>
            </label>
            <label v-for="v in pgInstall.versions.pgdg" :key="v" class="pgv-choice" :class="{ on: pgInstall.choice === v }">
              <input v-model="pgInstall.choice" type="radio" :value="v" />
              <span>
                <strong>{{ t('pgv.option_pgdg', { v }) }}</strong>
                <span class="small muted pgv-sub">{{ Number(v) > Number(pgInstall.versions.distribution || 0) ? t('pgv.option_newer') : t('pgv.option_older') }}</span>
              </span>
            </label>
          </fieldset>
          <p v-if="pgInstall.versions.pgdg_unavailable" class="alert alert-warn small">
            {{ t(`errors.${pgInstall.versions.pgdg_unavailable}`, { detail: pgInstall.versions.pgdg_detail || '' }) }}
          </p>
          <p class="small muted sw-body">{{ pgInstall.choice ? t('pgv.pgdg_note') : t('pgv.distribution_note') }}</p>
        </template>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="pgInstall = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="running || !pgInstall.versions" @click="installPostgres">
            <Icon name="download" :size="15" />{{ t('software.install') }}
          </button>
        </div>
      </div>
    </Modal>

    <Modal v-if="managePHP" :title="t('php.title', { v: managePHP })" wide @close="managePHP = ''">
      <PHPManage :version="managePHP" @close="managePHP = ''" />
    </Modal>
  </div>
</template>

<style scoped>
.sw-group { display: flex; flex-direction: column; gap: 12px; }
.sw-group-head { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 14px; }
.sw-group-head h2 { margin: 0; font-size: 16px; display: inline-flex; align-items: center; gap: 9px; }
.sw-group-head p { margin: 0; flex: 1; min-width: 240px; }
.g-ico { color: var(--accent); }
.sw-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 14px; }
.php-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(230px, 1fr)); gap: 12px; }
.php-tile {
  display: flex; flex-direction: column; gap: 10px; padding: 12px 14px; border: 1px dashed var(--border-strong); border-radius: 12px; background: var(--surface);
}
.php-tile.on { border-style: solid; border-color: var(--border); box-shadow: var(--shadow); }
.php-top { display: flex; align-items: center; gap: 10px; }
.php-name { flex: 1; min-width: 0; }
.php-v { font-weight: 700; }
.php-actions { display: flex; justify-content: flex-end; }
.sw-after-grid { margin: 0; }
.sw-body { margin: 0; line-height: 1.55; }
.php-old { display: flex; flex-direction: column; gap: 10px; }
.php-old > summary { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; }
.php-old[open] > summary { margin-bottom: 10px; }
.php-old-hint { margin: 0 0 10px; }
.php-tile.eol.on { border-color: var(--crit-soft); }
/* longer languages wrap the badge instead of cutting it */
.eol-tag { align-self: flex-start; white-space: normal; border-radius: 9px; max-width: 100%; }
.eol-line { margin: -4px 0 0; }
.tool-notes { margin: 0; padding-left: 18px; display: flex; flex-direction: column; gap: 3px; }
.sw-src { margin: 0; }
.pgv-choices { border: 0; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
.pgv-legend { font-size: 13px; font-weight: 600; color: var(--text-2); margin-bottom: 6px; padding: 0; }
.pgv-choice { display: flex; gap: 10px; align-items: flex-start; padding: 10px 12px; border: 1px solid var(--border); border-radius: 10px; cursor: pointer; }
.pgv-choice.on { border-color: var(--accent); background: var(--accent-soft); }
.pgv-choice input { margin-top: 3px; }
.pgv-choice strong { margin-right: 6px; }
.pgv-sub { display: block; margin-top: 2px; }
@media (max-width: 640px) {
  .sw-group-head { flex-direction: column; align-items: stretch; }
  .sw-grid { grid-template-columns: minmax(0, 1fr); }
}
</style>
