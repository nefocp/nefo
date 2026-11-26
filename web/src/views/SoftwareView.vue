<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
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
import SoftwareCard from '../components/SoftwareCard.vue'
import SoftwareRemoveModal from '../components/SoftwareRemoveModal.vue'
import WebServerCard from '../components/WebServerCard.vue'
import AppLogo from '../components/AppLogo.vue'
import { usePageMeta } from '../page.js'

// Everything optional the server can run, grouped, each with Install and
// Uninstall: a server only carries what its websites use.
const { t } = useI18n()
const items = ref([])
const supported = ref(true)
const sites = ref(0)
const loaded = ref(false)
const error = ref('')
const managePHP = ref('')
const removing = ref(null) // item whose uninstall is being confirmed
const thirdParty = ref(null) // PHP version from an extra repository, confirming the install
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
  resume(['install_software', 'remove_software', 'install_web', 'remove_web'])
})

const byId = computed(() => Object.fromEntries(items.value.map((i) => [i.id, i])))
const pick = (...ids) => ids.map((id) => byId.value[id]).filter(Boolean)
const php = computed(() => items.value.filter((i) => i.kind === 'php'))
const node = computed(() => byId.value.nodejs)

const ICONS = { mariadb: 'database', redis: 'memory', adminer: 'sliders', nodejs: 'code', python: 'code', docker: 'box', waf: 'shield', clamav: 'lock' }
const groups = computed(() => [
  { id: 'data', items: pick('mariadb', 'redis', 'adminer') },
  { id: 'runtimes', items: pick('nodejs', 'python') },
  { id: 'containers', items: pick('docker') },
  { id: 'security', items: pick('waf', 'clamav') },
])

const groupIcon = { data: 'database', runtimes: 'code', containers: 'box', security: 'shield' }
const installedCount = computed(() => items.value.filter((i) => i.installed).length)
const ramTotal = computed(() => items.value.filter((i) => i.installed).reduce((a, i) => a + (i.ram_mb || 0), 0))
usePageMeta(() => ({ sub: loaded.value ? t('pg.sw_sub', { n: installedCount.value, mb: ramTotal.value }) : '' }))

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
  if (item.kind === 'php' && item.third_party && !confirmed) {
    thirdParty.value = item
    return
  }
  thirdParty.value = null
  try {
    started(await api('POST', `api/software/${item.id}/install`, {}), item.id, item.group)
  } catch (e) {
    error.value = errorMessage(e)
  }
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
  ['docker', 'mariadb'].includes(item.kind) && item.has_data ? t(`software.delete_data_${item.kind}`, { dir: item.data_dir }) : ''

function phpNote(p) {
  if (!p.installed) return p.third_party ? t('software.from_ppa') : t('software.from_distro')
  if (p.used_by) return t('software.used_by', { n: p.used_by }, p.used_by)
  if (p.blocked === 'php_last') return t('software.php_only')
  return ''
}

function phpActions(p) {
  if (!p.installed) {
    return [{ label: t('software.install'), icon: 'download', inline: true, disabled: running.value || !supported.value, onClick: () => install(p) }]
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
          <div v-for="p in php" :key="p.id" class="php-tile" :class="{ on: p.installed }">
            <div class="php-top">
              <AppLogo name="php" size="sm" :dim="!p.installed" />
              <div class="php-name">
                <div class="php-v">PHP {{ p.version }}</div>
                <div class="small muted">{{ phpNote(p) || (p.installed ? t('software.ram_about', { mb: p.ram_mb }) : '') }}</div>
              </div>
              <span v-if="working(p.id)" class="status-pill warn"><span class="spinner" aria-hidden="true"></span>{{ t('software.working') }}</span>
              <span v-else-if="p.installed" class="status-pill ok">{{ t('software.installed') }}</span>
            </div>
            <div class="php-actions"><RowActions :actions="phpActions(p)" /></div>
          </div>
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
            <template v-if="it.installed">
              <RouterLink v-if="it.kind === 'mariadb'" class="btn btn-sm" :to="{ name: 'databases' }">{{ t('software.open_databases') }}</RouterLink>
              <RouterLink v-else-if="it.kind === 'redis'" class="btn btn-sm" :to="{ name: 'databases', query: { tab: 'redis' } }">{{ t('software.open_redis') }}</RouterLink>
              <a v-else-if="it.kind === 'adminer'" class="btn btn-sm" href="adminer/" target="_blank" rel="noopener">
                <Icon name="external" :size="15" />{{ t('software.open_adminer') }}
              </a>
              <RouterLink v-else-if="it.kind === 'docker'" class="btn btn-sm" :to="{ name: 'docker' }">{{ t('software.open_docker') }}</RouterLink>
            </template>
          </SoftwareCard>
        </div>
        <NodeVersions v-if="g.id === 'runtimes' && node && (node.installed || node.lines)" class="sw-after-grid" :stamp="stamp" @changed="load" />
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
        <p class="sw-body">{{ t('software.third_party_confirm', { name: nameOf(thirdParty) }) }}</p>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="thirdParty = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="running" @click="install(thirdParty, true)"><Icon name="download" :size="15" />{{ t('software.install') }}</button>
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
@media (max-width: 640px) {
  .sw-group-head { flex-direction: column; align-items: stretch; }
  .sw-grid { grid-template-columns: minmax(0, 1fr); }
}
</style>
