<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytes, formatUptime } from '../format.js'
import LogViewer from '../components/LogViewer.vue'
import Modal from '../components/Modal.vue'
import RowActions from '../components/RowActions.vue'
import Icon from '../components/Icon.vue'
import AppLogo from '../components/AppLogo.vue'
import { usePageMeta } from '../page.js'
import { BRAND } from '../brand.js'

const { t } = useI18n()
const services = ref([])
const error = ref('')
const notice = ref('')
const busy = ref('')
const logs = ref(null)
const loaded = ref(false)

async function load() {
  try {
    services.value = (await api('GET', 'api/services')).services || []
    loaded.value = true
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function act(s, action) {
  error.value = ''
  notice.value = ''
  if ((action === 'stop' || s.name === 'nefo') && !window.confirm(t(`services.confirm_${s.name === 'nefo' ? 'panel' : 'stop'}`, { name: s.name }))) return
  busy.value = s.name + action
  try {
    await api('POST', `api/services/${s.name}/${action}`, {})
    notice.value = t('services.done', { name: s.name, action: t(`services.${action}`) })
    if (s.name === 'nefo') {
      setTimeout(() => window.location.reload(), 4000)
      return
    }
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = ''
  }
}

const icons = { start: 'play', stop: 'stop', restart: 'refresh', reload: 'refresh' }
function serviceActions(svc) {
  const acts = svc.actions || []
  const main = svc.active === 'active' ? (acts.includes('restart') ? 'restart' : null) : acts.includes('start') ? 'start' : null
  return [
    ...(svc.actions || []).map((a) => ({
      label: t(`services.${a}`), icon: icons[a], inline: a === main, danger: a === 'stop',
      disabled: !!busy.value || (a === 'start' && svc.active === 'active') || (a === 'stop' && svc.active !== 'active'),
      onClick: () => act(svc, a),
    })),
    { label: t('logs.button'), icon: 'logs', onClick: () => (logs.value = svc.name) },
  ]
}

const uptime = (since) => formatUptime(Math.max(0, (Date.now() - new Date(since).getTime()) / 1000), t)
const stateCls = (s) => (s.active === 'active' ? 'ok' : s.active === 'failed' ? 'crit' : 'warn')
const stateText = (s) => (['active', 'inactive', 'failed', 'activating', 'deactivating'].includes(s.active) ? t('pg.svc_state_' + s.active) : s.active)
// meta gives a service's display name and logo or icon.
function meta(s) {
  const n = s.name
  const php = n.match(/^php([0-9.]+)-fpm$/)
  if (php) return { name: `PHP ${php[1]} FPM`, logo: 'php' }
  return (
    {
      nefo: { name: BRAND.name, icon: 'dashboard' },
      nginx: { name: 'Nginx', logo: 'nginx' },
      apache2: { name: 'Apache', logo: 'apache' },
      lshttpd: { name: 'OpenLiteSpeed', logo: 'openlitespeed' },
      mariadb: { name: 'MariaDB', logo: 'mariadb' },
      mysql: { name: 'MySQL', logo: 'mysql' },
      postgresql: { name: 'PostgreSQL', logo: 'postgresql' },
      memcached: { name: 'Memcached', logo: 'memcached' },
      'redis-server': { name: 'Redis', logo: 'redis' },
      fail2ban: { name: 'fail2ban', icon: 'shieldCheck' },
      ssh: { name: 'SSH', icon: 'terminal' },
      cron: { name: 'Cron', icon: 'clock' },
      ufw: { name: t('pg.svc_ufw'), icon: 'shield' },
    }[n] || { name: n, icon: 'server' }
  )
}
const running = computed(() => services.value.filter((s) => s.active === 'active').length)
const failed = computed(() => services.value.filter((s) => s.active === 'failed').length)
const totalMem = computed(() => services.value.reduce((a, s) => a + (s.memory || 0), 0))
const maxMem = computed(() => Math.max(1, ...services.value.map((s) => s.memory || 0)))
usePageMeta(() => ({ sub: loaded.value ? t('pg.svc_sub', { n: running.value, total: services.value.length }) : '' }))

onMounted(load)
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('services.title') }}</h1></div>
    <Teleport defer to="#topbar-actions">
      <button class="btn" @click="load"><Icon name="refresh" :size="15" />{{ t('files.refresh') }}</button>
    </Teleport>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <Modal v-if="logs" wide :title="t('logs.title', { name: logs })" @close="logs = null"><LogViewer :key="logs" :url="() => `api/services/${logs}/logs`" /></Modal>

    <div class="kpis">
      <div class="kpi"><span class="kpi-ico c-green"><Icon name="play" :size="20" /></span><div><div class="kpi-val">{{ loaded ? `${running} / ${services.length}` : '' }}<span v-if="!loaded" class="skel" style="width: 50px; height: 20px"></span></div><div class="kpi-label">{{ t('pg.svc_running') }}</div></div></div>
      <div class="kpi"><span class="kpi-ico" :class="failed ? 'c-red' : 'c-gray'"><Icon name="alert" :size="20" /></span><div><div class="kpi-val">{{ loaded ? failed : '' }}</div><div class="kpi-label">{{ t('pg.svc_failed') }}</div></div></div>
      <div class="kpi"><span class="kpi-ico c-violet"><Icon name="memory" :size="20" /></span><div><div class="kpi-val">{{ loaded ? formatBytes(totalMem) : '' }}</div><div class="kpi-label">{{ t('pg.svc_memory') }}</div></div></div>
    </div>

    <section class="panel">
      <div v-if="!loaded" class="panel-body"><span class="skel" style="height: 260px"></span></div>
      <div v-else class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>{{ t('services.col_name') }}</th>
              <th>{{ t('services.col_state') }}</th>
              <th>{{ t('services.col_memory') }}</th>
              <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in services" :key="s.name">
              <td>
                <div class="name-ico">
                  <AppLogo v-if="meta(s).logo" :name="meta(s).logo" size="sm" />
                  <span v-else class="tile"><Icon :name="meta(s).icon" :size="16" /></span>
                  <div class="name-ico-text">
                    <div class="svc-name">{{ meta(s).name }} <span class="mono muted svc-unit">{{ s.name }}</span></div>
                    <div class="name-ico-sub svc-desc">{{ s.description }}</div>
                  </div>
                </div>
              </td>
              <td class="nowrap">
                <span class="status-pill" :class="stateCls(s)">{{ stateText(s) }}</span>
                <div class="name-ico-sub">
                  <template v-if="s.since">{{ t('services.up_for', { time: uptime(s.since) }) }}</template>
                  <template v-else>{{ s.sub }}</template>
                  <template v-if="!s.enabled"> · {{ t('services.not_enabled') }}</template>
                </div>
              </td>
              <td class="nowrap">
                {{ s.memory ? formatBytes(s.memory) : '—' }}
                <div v-if="s.memory" class="ubar" aria-hidden="true"><span :style="{ width: Math.max(2, (s.memory / maxMem) * 100) + '%' }"></span></div>
              </td>
              <td class="actions"><RowActions :actions="serviceActions(s)" /></td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<style scoped>
.svc-name { font-weight: 700; }
.svc-unit { font-size: 11.5px; font-weight: 400; margin-left: 4px; }
.svc-desc { max-width: 360px; }
</style>
