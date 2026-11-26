<script setup>
import { ref, computed, watch, onMounted, defineAsyncComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import RowActions from '../components/RowActions.vue'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import JobLog from '../components/JobLog.vue'
import LogViewer from '../components/LogViewer.vue'
import Modal from '../components/Modal.vue'
import Icon from '../components/Icon.vue'
import DockerCompose from '../components/DockerCompose.vue'
import DockerImages from '../components/DockerImages.vue'
import { useJob } from '../composables/useJob.js'
import AppLogo from '../components/AppLogo.vue'
import { logoURL } from '../logos.js'
import { usePageMeta } from '../page.js'

// Loaded on demand: the terminal library is large.
const DockerTerminal = defineAsyncComponent(() => import('../components/DockerTerminal.vue'))

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const data = ref(null)
const error = ref('')
const logs = ref(null)
const shell = ref(null) // the container whose terminal is open
const version = ref(0) // bumped when a job has finished, so the tabs reload
const { job, running: jobRunning, watch: watchJob, resume, clear: clearJob } = useJob(async () => {
  version.value++
  await load()
})

const TABS = ['containers', 'compose', 'images']
const tab = ref(TABS.includes(route.query.tab) ? route.query.tab : 'containers')
watch(tab, (v) => router.replace({ query: { ...route.query, tab: v === 'containers' ? undefined : v } }))
const compose = ref(null)
const images = ref(null)

async function load() {
  try {
    data.value = await api('GET', 'api/docker')
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function act(c, action) {
  error.value = ''
  if (action === 'remove' && !window.confirm(t('docker.remove_confirm', { name: c.name }))) return
  try {
    await api('POST', `api/docker/containers/${c.name}/${action}`, {})
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const showRun = ref(false)
const form = ref({ image: '', name: '', host_port: '', container_port: '', env: '' })

async function run() {
  error.value = ''
  const f = form.value
  try {
    watchJob(
      await api('POST', 'api/docker/run', {
        image: f.image.trim(),
        name: f.name.trim(),
        host_port: Number(f.host_port) || 0,
        container_port: Number(f.container_port) || 0,
        env: f.env.split('\n').map((l) => l.trim()).filter(Boolean),
      }),
    )
    showRun.value = false
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const containers = computed(() => data.value?.containers || [])
const runningCount = computed(() => containers.value.filter((c) => c.state === 'running').length)
// logoOf guesses an app logo from the image name ("louislam/uptime-kuma:1" → uptime-kuma).
function logoOf(c) {
  const base = String(c.image || '').split('@')[0].split(':')[0].split('/').pop()
  return logoURL(base) ? base : logoURL(c.name) ? c.name : 'docker'
}
const stateText = (s) => (['running', 'exited', 'created', 'paused', 'restarting', 'dead'].includes(s) ? t('pg.dk_st_' + s) : s)
usePageMeta(() => ({ sub: data.value?.installed ? t('pg.dk_running', { n: String(runningCount.value), total: containers.value.length }, containers.value.length) : t('docker.subtitle') }))

onMounted(async () => {
  await load()
  resume(['docker_run', 'image_prune', 'compose_create', 'compose_save', 'compose_start', 'compose_stop',
    'compose_restart', 'compose_down', 'compose_update', 'compose_delete'])
})
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">Docker</h1></div>
    <Teleport defer to="#topbar-actions">
      <template v-if="data?.installed">
        <button v-if="tab === 'containers'" class="btn btn-primary" @click="showRun = true"><Icon name="plus" :size="15" />{{ t('docker.run') }}</button>
        <button v-else-if="tab === 'compose'" class="btn btn-primary" @click="compose?.create()"><Icon name="plus" :size="15" />{{ t('docker.compose_new') }}</button>
        <button v-else class="btn" :disabled="jobRunning" @click="images?.prune()"><Icon name="trash" :size="15" />{{ t('docker.prune') }}</button>
      </template>
    </Teleport>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />
    <Modal v-if="logs" wide :title="t('logs.title', { name: logs })" @close="logs = null"><LogViewer :key="logs" :url="() => `api/docker/containers/${logs}/logs`" /></Modal>
    <Modal v-if="shell" wide :title="t('docker.terminal_title', { name: shell })" @close="shell = null"><DockerTerminal :key="shell" :container="shell" /></Modal>

    <span v-if="!data && !error" class="skel" style="height: 260px; border-radius: 14px"></span>

    <div v-if="data && !data.installed" class="card empty">
      <AppLogo name="docker" size="lg" style="margin: 0 auto 14px" />
      <p class="empty-lead">{{ t('docker.not_installed') }}</p>
      <RouterLink class="btn btn-primary" :to="{ name: 'software' }"><Icon name="download" />{{ t('nav.software') }}</RouterLink>
    </div>

    <template v-else-if="data">
      <section class="svc-head">
        <AppLogo name="docker" size="lg" />
        <div class="svc-text">
          <h2 class="svc-title">Docker <span class="status-pill ok">{{ t('pg.svc_state_active') }}</span></h2>
          <div class="svc-meta">
            <span><Icon name="box" :size="14" />{{ t('pg.dk_running', { n: String(runningCount), total: containers.length }, containers.length) }}</span>
            <span><Icon name="layers" :size="14" />{{ t('pg.dk_images', { n: (data.images || []).length }, (data.images || []).length) }}</span>
            <span><Icon name="lock" :size="14" />{{ t('pg.dk_local') }}</span>
          </div>
        </div>
      </section>

      <nav class="ptabs" role="tablist" :aria-label="t('docker.tabs')">
        <button
          v-for="x in TABS"
          :id="`docker-tab-${x}`"
          :key="x"
          type="button"
          role="tab"
          :aria-selected="tab === x"
          :aria-controls="`docker-panel-${x}`"
          @click="tab = x"
        >
          <Icon :name="{ containers: 'box', compose: 'layers', images: 'archive' }[x]" :size="16" />{{ t(`docker.tab_${x}`) }}
          <span v-if="x === 'containers'" class="n">{{ containers.length }}</span>
          <span v-else-if="x === 'images'" class="n">{{ (data.images || []).length }}</span>
        </button>
      </nav>

      <section v-if="tab === 'containers'" id="docker-panel-containers" class="panel" role="tabpanel" aria-labelledby="docker-tab-containers">
        <div v-if="!containers.length" class="empty">
          <div class="empty-icon"><Icon name="box" :size="26" /></div>
          <p class="empty-lead">{{ t('docker.no_containers') }}</p>
          <button class="btn btn-primary" @click="showRun = true"><Icon name="plus" />{{ t('docker.run') }}</button>
        </div>
        <div v-else class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>{{ t('docker.name') }}</th>
                <th>{{ t('pg.dk_state') }}</th>
                <th>{{ t('pg.dk_ports') }}</th>
                <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="c in containers" :key="c.id">
                <td>
                  <div class="name-ico">
                    <AppLogo :name="logoOf(c)" size="sm" icon="box" />
                    <div class="name-ico-text">
                      <div class="dk-name">{{ c.name }}</div>
                      <div class="name-ico-sub mono">{{ c.image }}</div>
                    </div>
                  </div>
                </td>
                <td class="small">
                  <span class="status-pill" :class="c.state === 'running' ? 'ok' : c.state === 'exited' || c.state === 'dead' ? 'crit' : 'warn'">{{ stateText(c.state) }}</span>
                  <div class="name-ico-sub">{{ c.status }}</div>
                </td>
                <td class="mono small dk-ports">{{ c.ports || '—' }}</td>
                <td class="actions">
                  <RowActions :actions="[
                    c.state !== 'running'
                      ? { label: t('services.start'), icon: 'play', inline: true, onClick: () => act(c, 'start') }
                      : { label: t('services.stop'), icon: 'stop', inline: true, onClick: () => act(c, 'stop') },
                    { label: t('docker.terminal'), icon: 'terminal', hidden: c.state !== 'running', onClick: () => (shell = c.name) },
                    { label: t('services.restart'), icon: 'refresh', onClick: () => act(c, 'restart') },
                    { label: t('logs.button'), icon: 'logs', onClick: () => (logs = c.name) },
                    { label: t('websites.delete'), icon: 'trash', danger: true, onClick: () => act(c, 'remove') },
                  ]" />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <div v-else-if="tab === 'compose'" id="docker-panel-compose" role="tabpanel" aria-labelledby="docker-tab-compose">
        <DockerCompose ref="compose" :version="version" :busy="jobRunning" @job="watchJob" />
      </div>

      <div v-else id="docker-panel-images" role="tabpanel" aria-labelledby="docker-tab-images">
        <DockerImages ref="images" :version="version" :busy="jobRunning" @job="watchJob" @changed="load" />
      </div>

      <Modal v-if="showRun" :title="t('docker.run_title')" @close="showRun = false">
        <form class="stack" @submit.prevent="run">
          <label class="field">
            <span>{{ t('docker.image') }}</span>
            <input v-model="form.image" class="input mono" placeholder="nginx:alpine" required />
          </label>
          <label class="field">
            <span>{{ t('docker.name') }}</span>
            <input v-model="form.name" class="input mono" placeholder="my-app" required />
          </label>
          <div class="row">
            <label class="field grow">
              <span>{{ t('docker.host_port') }}</span>
              <input v-model="form.host_port" class="input" type="number" min="1024" max="65535" />
            </label>
            <label class="field grow">
              <span>{{ t('docker.container_port') }}</span>
              <input v-model="form.container_port" class="input" type="number" min="1" max="65535" />
            </label>
          </div>
          <label class="field">
            <span>{{ t('docker.env') }}</span>
            <textarea v-model="form.env" class="input mono" rows="3" placeholder="TZ=UTC"></textarea>
          </label>
          <p class="muted small" style="margin: 0">{{ t('docker.port_hint') }}</p>
          <div class="modal-actions">
            <button type="button" class="btn btn-ghost" @click="showRun = false">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="jobRunning"><Icon name="play" />{{ t('docker.start') }}</button>
          </div>
        </form>
      </Modal>
    </template>
  </div>
</template>

<style scoped>
.dk-name { font-weight: 700; }
.dk-ports { max-width: 260px; overflow-wrap: anywhere; }
</style>
