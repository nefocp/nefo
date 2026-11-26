<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import Icon from './Icon.vue'
import AppLogo from './AppLogo.vue'
import SoftwareRemoveModal from './SoftwareRemoveModal.vue'
import { SERVER_IDS, SERVER_NAMES, SERVER_RAM } from '../webservers.js'

// WebServerCard installs Nginx, Apache or OpenLiteSpeed, one at a time,
// through /api/web/install and /api/web/remove. Switching needs every
// website deleted first, since each one is set up for the installed server.
// Older servers answer GET /api/web/status without "server" and the other
// servers: then only Nginx is offered and it cannot be uninstalled here.
const props = defineProps({
  busy: { type: Boolean, default: false },
  supported: { type: Boolean, default: true },
  sites: { type: Number, default: 0 },
  stamp: { type: Number, default: 0 }, // changes after a job: reload
})
const emit = defineEmits(['job', 'error'])
const { t } = useI18n()

const status = ref(null)
const removing = ref('')
const NAMES = SERVER_NAMES
const RAM = SERVER_RAM

async function load() {
  try {
    status.value = await api('GET', 'api/web/status')
  } catch (e) {
    emit('error', errorMessage(e))
  }
}
onMounted(load)
watch(() => props.stamp, load)

// The servers this backend offers: those it reports in GET /api/web/status.
const offered = computed(() => SERVER_IDS.filter((id) => id === 'nginx' || (!!status.value && id in status.value)))
const hasChoice = computed(() => offered.value.length > 1)
const canRemove = computed(() => !!status.value && 'server' in status.value)
const current = computed(() => {
  const s = status.value
  if (!s) return ''
  if ('server' in s) return s.server || ''
  return s.nginx?.installed ? 'nginx' : ''
})
const servers = computed(() =>
  offered.value.map((id) => ({ id, info: status.value?.[id] || {} })),
)
const currentInfo = computed(() => status.value?.[current.value] || {})

async function install(id) {
  try {
    emit('job', await api('POST', 'api/web/install', { server: id }), 'web')
  } catch (e) {
    emit('error', errorMessage(e))
  }
}

async function remove(opts) {
  const id = removing.value
  removing.value = ''
  try {
    emit('job', await api('POST', 'api/web/remove', { confirm: opts.confirm || id }), 'web')
  } catch (e) {
    emit('error', errorMessage(e))
  }
}
</script>

<template>
  <section class="panel">
    <div class="panel-head">
      <h2 class="panel-title"><Icon name="globe" :size="17" />{{ t('software.web_title') }}</h2>
      <span v-if="current" class="status-pill" :class="currentInfo.running ? 'ok' : 'warn'">
        {{ NAMES[current] }} {{ currentInfo.version }} · {{ currentInfo.running ? t('websites.running') : t('websites.stopped') }}
      </span>
      <span v-else-if="status" class="status-pill">{{ t('software.not_installed') }}</span>
    </div>
    <p class="muted small web-hint">{{ t('software.web_hint') }}</p>
    <div v-if="status" class="ws-grid">
      <div v-for="s in servers" :key="s.id" class="ws-tile" :class="{ on: s.id === current }">
        <div class="ws-top">
          <AppLogo :name="s.id" icon="server" :dim="!!current && s.id !== current" />
          <div class="ws-name">
            <div class="ws-title">{{ NAMES[s.id] }}</div>
            <div class="small muted">
              <template v-if="s.id === current">{{ t('software.installed') }}<template v-if="s.info.version"> · <span class="mono">{{ s.info.version }}</span></template></template>
              <template v-else-if="s.info.installed">{{ t('software.web_not_panel') }}</template>
              <template v-else>{{ t('software.ram_about', { mb: RAM[s.id] }) }}</template>
            </div>
          </div>
          <span v-if="s.id === current" class="status-pill ok ws-pill">{{ t('pg.sw_in_use') }}</span>
        </div>
        <p class="muted small ws-desc">{{ t(`software.${s.id}_hint`) }}</p>
        <div class="ws-foot">
          <span class="small muted"><Icon name="memory" :size="13" /> {{ t('software.ram_about', { mb: RAM[s.id] }) }}</span>
          <button v-if="!current" class="btn btn-sm btn-primary" :disabled="busy || !supported" @click="install(s.id)">
            <Icon name="download" :size="15" />{{ t('software.install') }}
          </button>
          <button v-else-if="s.id === current && canRemove" class="btn btn-sm btn-danger" :disabled="busy || sites > 0" @click="removing = s.id">
            <Icon name="trash" :size="15" />{{ t('software.uninstall') }}
          </button>
        </div>
      </div>
    </div>
    <p v-if="current && sites > 0" class="web-blocked small">
      <Icon name="alert" :size="14" />{{ t('software.web_switch_blocked', { n: sites, name: NAMES[current] }, sites) }}
    </p>
    <p v-else-if="current && hasChoice" class="muted small web-note">{{ t('software.web_switch_hint', { name: NAMES[current] }) }}</p>

    <SoftwareRemoveModal
      v-if="removing"
      :name="NAMES[removing]"
      :body="t('software.remove_web', { name: NAMES[removing] })"
      :confirm-word="removing"
      :busy="busy"
      @confirm="remove"
      @close="removing = ''"
    />
  </section>
</template>

<style scoped>
.web-hint { margin: 0; padding: 12px 18px 0; }
.ws-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 12px; padding: 14px 18px 16px; }
.ws-tile { display: flex; flex-direction: column; gap: 8px; padding: 14px; border: 1px solid var(--border); border-radius: 12px; background: var(--surface); }
.ws-tile.on { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.ws-top { display: flex; align-items: center; gap: 12px; }
.ws-name { flex: 1; min-width: 0; }
.ws-title { font-weight: 700; }
.ws-pill { align-self: flex-start; }
.ws-desc { margin: 0; flex: 1; }
.ws-foot { display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; }
.web-blocked { display: flex; align-items: flex-start; gap: 6px; color: var(--warn); margin: 0; padding: 0 18px 16px; }
.web-blocked .icon { flex: none; margin-top: 2px; }
.web-note { margin: 0; padding: 0 18px 16px; }
</style>
