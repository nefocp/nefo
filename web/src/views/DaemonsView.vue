<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytes, formatUptime } from '../format.js'
import LogViewer from '../components/LogViewer.vue'
import Modal from '../components/Modal.vue'
import RowActions from '../components/RowActions.vue'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../page.js'

const { t } = useI18n()
const data = ref(null) // { daemons, supported }
const sites = ref([])
const error = ref('')
const notice = ref('')
const busy = ref(0) // ID of the program an action is running on
const logsOf = ref(null)
let later = 0

async function load() {
  try {
    data.value = await api('GET', 'api/daemons')
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function loadSites() {
  try {
    sites.value = (await api('GET', 'api/sites')).sites || []
    if (editing.value && !editing.value.site_id) editing.value.site_id = sites.value[0]?.id ?? 0
  } catch {
    // the form says there are no websites
  }
}

// A program that fails at once is restarted after 3 seconds: look again a
// little later so the list shows it.
function reloadSoon() {
  clearTimeout(later)
  later = setTimeout(load, 4000)
}

async function act(d, action) {
  error.value = ''
  notice.value = ''
  if (action === 'stop' && !window.confirm(t('daemons.stop_confirm', { name: d.name }))) return
  busy.value = d.id
  try {
    await api('POST', `api/daemons/${d.id}/${action}`, {})
    notice.value = t(`daemons.done_${action}`, { name: d.name })
    await load()
    if (action !== 'stop') reloadSoon()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = 0
  }
}

async function remove(d) {
  error.value = ''
  notice.value = ''
  if (!window.confirm(t('daemons.delete_confirm', { name: d.name }))) return
  busy.value = d.id
  try {
    await api('DELETE', `api/daemons/${d.id}`)
    notice.value = t('daemons.deleted', { name: d.name })
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = 0
  }
}

// ---- status ----
const running = (d) => d.status?.active === 'active' || d.status?.active === 'activating' || d.status?.active === 'reloading'

function statusTag(d) {
  const s = d.status || {}
  if (s.active === 'active') return { cls: 'ok', text: t('daemons.st_running') }
  if (s.active === 'activating' || s.sub === 'auto-restart') return { cls: 'warn', text: t('daemons.st_restarting') }
  if (s.active === 'failed') return { cls: 'crit', text: t('daemons.st_failed') }
  if (s.active === 'deactivating') return { cls: 'warn', text: t('daemons.st_stopping') }
  if (s.active === 'inactive') return { cls: '', text: t('daemons.st_stopped') }
  return { cls: '', text: t('daemons.st_unknown') }
}

const uptime = (since) => formatUptime(Math.max(0, (Date.now() - new Date(since).getTime()) / 1000), t)

function actions(d) {
  const on = running(d)
  const off = !busy.value && !on && !d.enabled && d.status?.active !== 'failed'
  return [
    { label: t('daemons.start'), icon: 'play', inline: !on, disabled: !!busy.value || on, onClick: () => act(d, 'start') },
    { label: t('daemons.restart'), icon: 'refresh', inline: on, disabled: !!busy.value, onClick: () => act(d, 'restart') },
    { label: t('daemons.stop'), icon: 'stop', danger: true, disabled: !!busy.value || off, onClick: () => act(d, 'stop') },
    { label: t('logs.button'), icon: 'logs', onClick: () => (logsOf.value = d) },
    { label: t('daemons.edit'), icon: 'edit', onClick: () => openForm(d) },
    { label: t('websites.delete'), icon: 'trash', danger: true, disabled: !!busy.value, onClick: () => remove(d) },
  ]
}

// ---- add / edit ----
const editing = ref(null) // form; id 0 for a new program
const formError = ref('')
const saving = ref(false)

function openForm(d) {
  error.value = ''
  notice.value = ''
  formError.value = ''
  loadSites()
  editing.value = d
    ? { id: d.id, name: d.name, site_id: d.site_id, dir: d.dir, command: d.command, env: (d.env || []).join('\n'), enabled: d.enabled }
    : { id: 0, name: '', site_id: sites.value[0]?.id ?? 0, dir: '', command: '', env: '', enabled: true }
}

const formSite = computed(() => sites.value.find((s) => s.id === editing.value?.site_id))
const siteRoot = computed(() => formSite.value?.root || (formSite.value ? `/var/www/${formSite.value.domain}` : ''))

function useLaravelTemplate() {
  const s = formSite.value
  if (!s) return
  const f = editing.value
  f.name = f.name || t('daemons.laravel_name')
  f.dir = ''
  f.command = `php${s.php_version} artisan queue:work --sleep=3 --tries=3 --max-time=3600`
}

async function save() {
  saving.value = true
  formError.value = ''
  const f = editing.value
  const body = {
    name: f.name,
    site_id: Number(f.site_id),
    dir: f.dir,
    command: f.command,
    env: f.env.split(/\r?\n/).map((l) => l.trim()).filter(Boolean),
    enabled: f.enabled,
  }
  try {
    await api(f.id ? 'PUT' : 'POST', f.id ? `api/daemons/${f.id}` : 'api/daemons', body)
    editing.value = null
    notice.value = t('daemons.saved', { name: body.name })
    await load()
    reloadSoon()
  } catch (e) {
    formError.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

const daemons = computed(() => data.value?.daemons || [])
const troubled = computed(() => daemons.value.filter((d) => ['warn', 'crit'].includes(statusTag(d).cls)).length)
usePageMeta(() => ({ sub: data.value ? t('pg.dm_sub', { n: daemons.value.length }, daemons.value.length) : '' }))

onMounted(() => {
  load()
  loadSites()
})
onBeforeUnmount(() => clearTimeout(later))
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('daemons.title') }}</h1></div>
    <Teleport defer to="#topbar-actions">
      <button class="icon-btn bordered" :aria-label="t('files.refresh')" :title="t('files.refresh')" @click="load"><Icon name="refresh" :size="16" /></button>
      <button v-if="data?.supported" class="btn btn-primary" @click="openForm(null)"><Icon name="plus" :size="15" />{{ t('daemons.add') }}</button>
    </Teleport>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <p v-if="data && !data.supported" class="alert alert-warn">{{ t('daemons.unsupported') }}</p>

    <template v-if="!data && !error">
      <div class="kpis"><span v-for="i in 3" :key="i" class="skel" style="height: 72px; border-radius: 14px"></span></div>
      <span class="skel" style="height: 200px; border-radius: 14px"></span>
    </template>

    <template v-if="data">
      <div class="kpis">
        <div class="kpi"><span class="kpi-ico c-blue"><Icon name="activity" :size="20" /></span><div><div class="kpi-val">{{ daemons.length }}</div><div class="kpi-label">{{ t('pg.dm_total') }}</div></div></div>
        <div class="kpi"><span class="kpi-ico c-green"><Icon name="play" :size="20" /></span><div><div class="kpi-val">{{ daemons.filter((d) => d.status?.active === 'active').length }}</div><div class="kpi-label">{{ t('daemons.st_running') }}</div></div></div>
        <div class="kpi"><span class="kpi-ico" :class="troubled ? 'c-red' : 'c-gray'"><Icon name="alert" :size="20" /></span><div><div class="kpi-val">{{ troubled }}</div><div class="kpi-label">{{ t('pg.dm_trouble') }}</div></div></div>
        <div class="kpi"><span class="kpi-ico c-violet"><Icon name="memory" :size="20" /></span><div><div class="kpi-val">{{ formatBytes(daemons.reduce((a, d) => a + (d.status?.memory || 0), 0)) }}</div><div class="kpi-label">{{ t('pg.svc_memory') }}</div></div></div>
      </div>

      <section class="panel">
        <div v-if="!daemons.length" class="empty">
          <div class="empty-icon"><Icon name="activity" :size="26" /></div>
          <p class="empty-lead">{{ t('daemons.empty') }}</p>
          <p class="muted">{{ t('daemons.empty_body') }}</p>
          <button v-if="data.supported" class="btn btn-primary" @click="openForm(null)"><Icon name="plus" />{{ t('daemons.add') }}</button>
        </div>
        <div v-else class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>{{ t('daemons.name') }}</th>
                <th>{{ t('daemons.site') }}</th>
                <th>{{ t('daemons.col_state') }}</th>
                <th>{{ t('daemons.col_memory') }}</th>
                <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="d in daemons" :key="d.id">
                <td>
                  <div class="name-ico">
                    <span class="tile" :class="'dm-' + statusTag(d).cls"><Icon name="activity" :size="16" /></span>
                    <div class="name-ico-text">
                      <div class="dm-name">{{ d.name }}</div>
                      <div class="mono small muted command-cell">{{ d.command }}</div>
                      <div class="mono name-ico-sub">{{ d.dir ? `${d.dir}/` : './' }}</div>
                    </div>
                  </div>
                </td>
                <td class="small">
                  <RouterLink v-if="d.site" :to="{ name: 'websites', query: { site: String(d.site_id) } }">{{ d.site }}</RouterLink>
                  <span v-else class="status-pill crit">{{ t('daemons.site_gone') }}</span>
                  <div class="muted mono">{{ d.user }}</div>
                </td>
                <td class="nowrap">
                  <span class="status-pill" :class="statusTag(d).cls">{{ statusTag(d).text }}</span>
                  <div v-if="d.status?.since && d.status.active === 'active'" class="name-ico-sub">{{ t('services.up_for', { time: uptime(d.status.since) }) }}</div>
                  <div v-if="d.status?.restarts" class="small" :class="d.status.restarts >= 5 ? 'text-crit' : 'muted'">{{ t('daemons.restarts', { n: d.status.restarts }) }}</div>
                  <div v-if="!d.enabled" class="name-ico-sub">{{ t('daemons.not_enabled') }}</div>
                </td>
                <td class="nowrap small">
                  {{ d.status?.memory ? formatBytes(d.status.memory) : '—' }}
                  <div v-if="d.status?.pid" class="muted mono">PID {{ d.status.pid }}</div>
                </td>
                <td class="actions"><RowActions :actions="actions(d)" /></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>

    <Modal v-if="logsOf" wide :title="t('logs.title', { name: logsOf.name })" @close="logsOf = null">
      <LogViewer :key="logsOf.id" :url="() => `api/daemons/${logsOf.id}/logs`" />
    </Modal>

    <Modal v-if="editing" :title="editing.id ? t('daemons.edit_title') : t('daemons.new_title')" @close="editing = null">
      <div v-if="!sites.length" class="empty">
        <div class="empty-icon"><Icon name="globe" :size="24" /></div>
        <p class="empty-lead">{{ t('daemons.no_sites') }}</p>
        <RouterLink class="btn btn-primary" :to="{ name: 'websites' }" @click="editing = null">{{ t('nav.websites') }}</RouterLink>
      </div>
      <form v-else class="stack" @submit.prevent="save">
        <p v-if="formError" class="alert alert-crit pre-wrap" role="alert">{{ formError }}</p>
        <label class="field">
          <span>{{ t('daemons.name') }}</span>
          <input v-model="editing.name" class="input" maxlength="60" required :placeholder="t('daemons.name_placeholder')" />
        </label>
        <label class="field">
          <span>{{ t('daemons.site') }}</span>
          <select v-model.number="editing.site_id" class="input" required>
            <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.domain }} ({{ s.sys_user }})</option>
          </select>
          <small class="muted">{{ t('daemons.site_hint') }}</small>
        </label>
        <label class="field">
          <span>{{ t('daemons.dir') }}</span>
          <div class="dir-row">
            <span class="mono small muted dir-prefix">{{ siteRoot }}/</span>
            <input v-model="editing.dir" class="input mono" maxlength="200" spellcheck="false" :placeholder="t('daemons.dir_placeholder')" />
          </div>
          <small class="muted">{{ t('daemons.dir_hint') }}</small>
        </label>
        <label class="field">
          <span>{{ t('daemons.command') }}</span>
          <textarea v-model="editing.command" class="input mono" rows="2" maxlength="1000" spellcheck="false" required placeholder="php8.3 artisan queue:work"></textarea>
          <small class="muted">
            {{ t('daemons.command_hint') }}
            <a v-if="formSite?.php_version" href="#" @click.prevent="useLaravelTemplate">{{ t('daemons.laravel_template') }}</a>
          </small>
        </label>
        <label class="field">
          <span>{{ t('daemons.env') }}</span>
          <textarea v-model="editing.env" class="input mono" rows="4" spellcheck="false" placeholder="APP_ENV=production&#10;QUEUE_CONNECTION=database"></textarea>
          <small class="muted">{{ t('daemons.env_hint') }}</small>
        </label>
        <div class="opt">
          <div class="opt-text"><div class="opt-title">{{ t('daemons.enabled') }}</div></div>
          <button type="button" class="switch" role="switch" :aria-checked="editing.enabled" :aria-label="t('daemons.enabled')" @click="editing.enabled = !editing.enabled"></button>
        </div>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="editing = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="saving">{{ editing.id ? t('daemons.save_restart') : t('daemons.create') }}</button>
        </div>
      </form>
    </Modal>
  </div>
</template>

<style scoped>
.dm-name { font-weight: 700; }
.tile.dm-ok { background: var(--ok-soft); color: var(--ok); }
.tile.dm-warn { background: var(--warn-soft); color: var(--warn); }
.tile.dm-crit { background: var(--crit-soft); color: var(--crit); }
.dir-row { display: flex; align-items: center; gap: 4px; }
.dir-prefix { white-space: nowrap; max-width: 55%; overflow: hidden; text-overflow: ellipsis; }
.command-cell { max-width: 460px; overflow-wrap: anywhere; }
</style>
