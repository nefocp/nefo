<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { api } from '../../api.js'
import { errorMessage } from '../../errors.js'
import { toast } from '../../toast.js'
import { appName } from '../../logos.js'
import AppLogo from '../AppLogo.vue'
import Icon from '../Icon.vue'
import Modal from '../Modal.vue'
import LogViewer from '../LogViewer.vue'
import RowActions from '../RowActions.vue'
import AppCredentials from './AppCredentials.vue'
import { logoSrc, mb, siteURL } from './appstore.js'

// AppInstalled lists the installed apps with their state and what can be
// done with each. Jobs it starts (update, backup, uninstall) go to "job".
const props = defineProps({
  installed: { type: Array, required: true },
  apps: { type: Array, default: () => [] }, // the catalog
  sites: { type: Array, default: () => [] },
  busy: { type: Boolean, default: false },
})
const emit = defineEmits(['job', 'changed', 'browse'])
const { t, locale } = useI18n()
const router = useRouter()

const byId = computed(() => Object.fromEntries(props.apps.map((a) => [a.id, a])))
const siteOf = (a) => props.sites.find((s) => s.id === a.site_id)
const nameOf = (a) => byId.value[a.app]?.name || appName(a.app)
const logoOf = (a) => logoSrc(byId.value[a.app]?.logo || a.app)
const isInstance = (a) => a.kind !== 'php' && !a.legacy

// ---- filter ----
const q = ref('')
const show = ref('all') // all, site, service
const rows = computed(() => {
  const f = q.value.trim().toLowerCase()
  return props.installed.filter((a) => {
    if (show.value === 'service' && a.kind !== 'service') return false
    if (show.value === 'site' && a.kind === 'service') return false
    return !f || `${nameOf(a)} ${a.domain || ''} ${a.name}`.toLowerCase().includes(f)
  })
})
const counts = computed(() => ({
  all: props.installed.length,
  site: props.installed.filter((a) => a.kind !== 'service').length,
  service: props.installed.filter((a) => a.kind === 'service').length,
}))

// ---- state ----
function status(a) {
  const p = pending.value[a.name]
  if (p) return { cls: 'warn', text: t({ start: 'apps.st_starting', stop: 'apps.st_stopping', restart: 'apps.st_restarting' }[p]) }
  if (a.state === 'installing') return { cls: 'warn', text: t('apps.st_installing') }
  if (a.state === 'failed') return { cls: 'crit', text: t('apps.st_failed') }
  if (a.kind === 'php') {
    const s = siteOf(a)
    return s?.disabled ? { cls: 'warn', text: t('apps.st_suspended') } : { cls: 'ok', text: t('apps.st_site') }
  }
  return {
    running: { cls: 'ok', text: t('apps.st_running') },
    stopped: { cls: 'crit', text: t('apps.st_stopped') },
    partial: { cls: 'warn', text: t('apps.st_partial') },
  }[a.status] || { cls: 'crit', text: t('apps.st_missing') }
}
const where = (a) => (a.domain ? a.domain : a.port ? `127.0.0.1:${a.port}` : '')

// ---- actions ----
const error = ref('')
const creds = ref(null) // the row whose sign-in details are open
const logs = ref(null)
const updating = ref(null)
const removing = ref(null)
const confirmText = ref('')
const sending = ref(false)

const base = (a) => `api/apps/installed/${encodeURIComponent(a.name)}`
// Starting waits for the app's database to be healthy: the row says what is
// going on meanwhile.
const pending = ref({}) // instance -> start, stop or restart
async function power(a, action) {
  error.value = ''
  pending.value = { ...pending.value, [a.name]: action }
  try {
    if (isInstance(a)) await api('POST', `${base(a)}/${action}`, {})
    else await api('POST', `api/sites/${a.site_id}/app/${action}`, {})
    toast(t(`apps.done_${action}`, { app: nameOf(a) }))
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    const { [a.name]: _, ...rest } = pending.value
    pending.value = rest
  }
}
async function startJob(fn) {
  error.value = ''
  sending.value = true
  try {
    const res = await fn()
    emit('job', res.job || res)
    return true
  } catch (e) {
    error.value = errorMessage(e)
    return false
  } finally {
    sending.value = false
  }
}
const backup = (a) => startJob(() => api('POST', `${base(a)}/backup`, {}))
async function update() {
  const a = updating.value
  if (await startJob(() => api('POST', `${base(a)}/update`, {}))) updating.value = null
}
async function uninstall() {
  const a = removing.value
  if (await startJob(() => api('DELETE', base(a), { confirm: confirmText.value.trim() }))) removing.value = null
}
function askRemove(a) {
  confirmText.value = ''
  removing.value = a
}
const openSite = (a, tab) => router.push({ name: 'websites', query: { site: String(a.site_id), ...(tab ? { tab } : {}) } })
const logURL = (a) => (isInstance(a) ? `${base(a)}/logs` : `api/sites/${a.site_id}/app/logs`)

function actions(a) {
  const site = a.site_id ? siteOf(a) : null
  const docker = a.kind !== 'php'
  const running = a.status === 'running' || a.status === 'partial'
  const ready = a.state !== 'installing' && !pending.value[a.name]
  return [
    site && { label: t('apps.open'), icon: 'external', inline: true, href: siteURL(site, a.domain) },
    a.kind === 'service' && { label: t('apps.credentials'), icon: 'key', inline: true, onClick: () => (creds.value = a) },
    a.kind === 'docker' && !a.legacy && { label: t('apps.credentials'), icon: 'key', onClick: () => (creds.value = a) },
    docker && { label: t('logs.button'), icon: 'logs', onClick: () => (logs.value = a) },
    docker && ready && !running && { label: t('services.start'), icon: 'play', disabled: props.busy, onClick: () => power(a, 'start') },
    docker && ready && running && { label: t('services.stop'), icon: 'stop', disabled: props.busy, onClick: () => power(a, 'stop') },
    docker && ready && { label: t('services.restart'), icon: 'refresh', disabled: props.busy, onClick: () => power(a, 'restart') },
    docker && a.update_available && { label: a.legacy ? t('apps.convert') : t('apps.update'), icon: 'up', disabled: props.busy, onClick: () => (updating.value = a) },
    isInstance(a) && ready && { label: t('apps.backup'), icon: 'archive', disabled: props.busy, onClick: () => backup(a) },
    site && { label: t('apps.site_settings'), icon: 'settings', onClick: () => openSite(a) },
    a.kind === 'service' && { label: t('apps.uninstall'), icon: 'trash', danger: true, disabled: props.busy, onClick: () => askRemove(a) },
    site && { label: t('apps.delete_site'), icon: 'trash', danger: true, onClick: () => openSite(a, 'overview') },
  ].filter(Boolean)
}
</script>

<template>
  <section class="panel" :aria-label="t('apps.tab_installed')">
    <div class="panel-tools">
      <label class="search-field">
        <Icon name="search" :size="15" />
        <input v-model="q" type="search" :placeholder="t('apps.search_installed')" :aria-label="t('apps.search_installed')" />
      </label>
      <div class="tabs" role="group" :aria-label="t('apps.filter_kind')">
        <button v-for="k in ['all', 'site', 'service']" :key="k" type="button" :aria-pressed="show === k" @click="show = k">
          {{ t(`apps.show_${k}`) }} <span class="n">{{ counts[k] }}</span>
        </button>
      </div>
    </div>
    <p v-if="error" class="alert alert-crit pre-wrap as-err" role="alert">{{ error }}</p>

    <div v-if="!installed.length" class="empty">
      <div class="empty-icon"><Icon name="package" :size="26" /></div>
      <p class="empty-lead">{{ t('apps.none_installed') }}</p>
      <p class="muted">{{ t('apps.none_installed_hint') }}</p>
      <button type="button" class="btn btn-primary" @click="emit('browse')"><Icon name="package" />{{ t('apps.browse') }}</button>
    </div>
    <div v-else-if="!rows.length" class="empty">
      <p class="empty-lead">{{ t('apps.none_match') }}</p>
    </div>
    <div v-else class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>{{ t('apps.col_app') }}</th>
            <th class="ai-wide">{{ t('apps.col_version') }}</th>
            <th class="ai-wide">{{ t('apps.col_status') }}</th>
            <th class="ai-wide">{{ t('apps.col_memory') }}</th>
            <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="a in rows" :key="a.kind + a.name + (a.site_id || '')">
            <td>
              <div class="name-ico">
                <AppLogo :src="logoOf(a)" size="sm" />
                <div class="name-ico-text">
                  <div class="ai-name">{{ nameOf(a) }}</div>
                  <div class="name-ico-sub" :class="{ mono: !a.domain }">
                    <a v-if="a.site_id" href="#" @click.prevent="openSite(a)">{{ where(a) }}</a>
                    <template v-else>{{ where(a) }}</template>
                  </div>
                  <!-- phones: the other columns are hidden, their essentials go here -->
                  <div class="ai-phone">
                    <span class="status-pill" :class="status(a).cls">{{ status(a).text }}</span>
                    <span class="mono">{{ a.version }}</span>
                    <span v-if="a.legacy" class="tag tag-warn">{{ t('apps.legacy') }}</span>
                    <span v-else-if="a.update_available" class="tag tag-info">{{ t('apps.update_to', { v: a.catalog_version }) }}</span>
                  </div>
                </div>
              </div>
            </td>
            <td class="small ai-wide">
              <div class="ai-ver">
                <span class="mono">{{ a.version || '—' }}</span>
                <span v-if="a.legacy" class="tag tag-warn" :title="t('apps.legacy_hint')">{{ t('apps.legacy') }}</span>
                <span v-else-if="a.update_available" class="tag tag-info">{{ t('apps.update_to', { v: a.catalog_version }) }}</span>
              </div>
              <div v-if="a.legacy" class="name-ico-sub">{{ t('apps.legacy_hint') }}</div>
              <div v-else-if="a.update_available && a.kind === 'php'" class="name-ico-sub">{{ t('apps.update_in_app') }}</div>
            </td>
            <td class="small ai-wide" :aria-busy="!!pending[a.name]"><span class="status-pill" :class="status(a).cls">{{ status(a).text }}</span></td>
            <td class="small ai-mem ai-wide">{{ a.memory_mb ? mb(a.memory_mb, locale) : '—' }}</td>
            <td class="actions"><RowActions :actions="actions(a)" /></td>
          </tr>
        </tbody>
      </table>
    </div>

    <AppCredentials v-if="creds" :name="creds.name" :title="nameOf(creds)" @close="creds = null" />
    <Modal v-if="logs" wide :title="t('apps.logs_of', { app: nameOf(logs) })" @close="logs = null">
      <LogViewer :key="logs.name" :url="() => logURL(logs)" />
    </Modal>

    <Modal v-if="updating" :title="t(updating.legacy ? 'apps.convert_title' : 'apps.update_title', { app: nameOf(updating), v: updating.catalog_version })" @close="updating = null">
      <div class="stack">
        <p class="as-p">{{ updating.legacy ? t('apps.convert_body') : t('apps.update_body') }}</p>
        <p class="muted small as-p">{{ t('apps.update_backup') }}</p>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="updating = null">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="busy || sending" @click="update"><Icon name="up" :size="15" />{{ updating.legacy ? t('apps.convert') : t('apps.update') }}</button>
        </div>
      </div>
    </Modal>

    <Modal v-if="removing" danger :title="t('apps.uninstall_title', { app: nameOf(removing) })" @close="removing = null">
      <form class="stack" @submit.prevent="uninstall">
        <p class="as-p">{{ t('apps.uninstall_body') }}</p>
        <label class="field">
          <span>{{ t('apps.type_name', { name: removing.name }) }}</span>
          <input v-model="confirmText" class="input mono" autocomplete="off" spellcheck="false" autocapitalize="off" />
        </label>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="removing = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-danger btn-solid" :disabled="busy || sending || confirmText.trim() !== removing.name"><Icon name="trash" :size="15" />{{ t('apps.uninstall') }}</button>
        </div>
      </form>
    </Modal>
  </section>
</template>

<style scoped>
.ai-name { font-weight: 700; }
.ai-ver { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; }
.ai-mem { white-space: nowrap; font-variant-numeric: tabular-nums; }
.as-err { margin: 12px 18px 0; }
.as-p { margin: 0; }
.ai-phone { display: none; }
@media (max-width: 640px) {
  .ai-wide { display: none; }
  .ai-phone { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; margin-top: 4px; font-size: 12px; }
  .table .name-ico { min-width: 0; }
}
.panel-tools .tabs .n { font-size: 11.5px; color: var(--muted); font-weight: 600; margin-left: 2px; }
</style>
