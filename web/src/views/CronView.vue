<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import RowActions from '../components/RowActions.vue'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import JobLog from '../components/JobLog.vue'
import LogViewer from '../components/LogViewer.vue'
import Modal from '../components/Modal.vue'
import Icon from '../components/Icon.vue'
import { useJob } from '../composables/useJob.js'
import { usePageMeta } from '../page.js'
import { cronText } from '../cronText.js'

const { t, locale } = useI18n()
const data = ref(null)
const error = ref('')
const notice = ref('')
const { job, running: jobRunning, watch: watchJob, clear: clearJob } = useJob()

async function load() {
  try {
    data.value = await api('GET', 'api/cron')
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const presets = computed(() => [
  { value: '*/5 * * * *', label: t('cron.every_5min') },
  { value: '@hourly', label: t('cron.hourly') },
  { value: '0 3 * * *', label: t('cron.daily_3') },
  { value: '0 4 * * 0', label: t('cron.weekly') },
  { value: '@reboot', label: t('cron.reboot') },
])

// ---- edit ----
const editing = ref(null) // form, with id for an existing job
const busy = ref(false)

function openForm(job) {
  error.value = ''
  notice.value = ''
  editing.value = job
    ? { ...job }
    : { id: 0, name: '', schedule: '*/5 * * * *', command: '', run_as: 'root', enabled: true }
}

function useWordPressTemplate() {
  const u = (data.value.users || []).find((x) => x.user === editing.value.run_as && x.site)
  if (!u) return
  editing.value.name = editing.value.name || `WordPress cron: ${u.site}`
  editing.value.schedule = '*/5 * * * *'
  editing.value.command = `php${u.php || ''} /var/www/${u.site}/public/wp-cron.php`
}

async function save() {
  busy.value = true
  error.value = ''
  const f = editing.value
  const body = { name: f.name, schedule: f.schedule, command: f.command, run_as: f.run_as, enabled: f.enabled }
  try {
    await api(f.id ? 'PUT' : 'POST', f.id ? `api/cron/${f.id}` : 'api/cron', body)
    editing.value = null
    notice.value = t('cron.saved')
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function toggle(c) {
  try {
    await api('PUT', `api/cron/${c.id}`, { ...c, enabled: !c.enabled })
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function remove(c) {
  if (!window.confirm(t('cron.delete_confirm', { name: c.name }))) return
  try {
    await api('DELETE', `api/cron/${c.id}`)
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function runNow(c) {
  error.value = ''
  try {
    watchJob(await api('POST', `api/cron/${c.id}/run`, {}))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const logJob = ref(null)
const show = ref('all')
const q = ref('')
const jobs = computed(() => data.value?.jobs || [])
const shown = computed(() => {
  const f = q.value.trim().toLowerCase()
  return jobs.value.filter((c) => (show.value === 'all' || c.enabled === (show.value === 'on')) && (!f || `${c.name} ${c.command} ${c.run_as} ${c.site || ''}`.toLowerCase().includes(f)))
})
usePageMeta(() => ({ sub: data.value ? t('pg.cron_sub', { n: jobs.value.length, on: jobs.value.filter((c) => c.enabled).length }, jobs.value.length) : '' }))
const userLabel = (u) => (u.site ? `${u.user} (${u.site})` : u.user)

onMounted(load)
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('cron.title') }}</h1></div>
    <Teleport defer to="#topbar-actions">
      <button v-if="data" class="btn btn-primary" @click="openForm(null)"><Icon name="plus" :size="15" />{{ t('cron.add') }}</button>
    </Teleport>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />

    <template v-if="!data && !error">
      <div class="kpis"><span v-for="i in 3" :key="i" class="skel" style="height: 72px; border-radius: 14px"></span></div>
      <span class="skel" style="height: 220px; border-radius: 14px"></span>
    </template>

    <template v-if="data">
      <div class="kpis">
        <div class="kpi"><span class="kpi-ico c-orange"><Icon name="clock" :size="20" /></span><div><div class="kpi-val">{{ jobs.length }}</div><div class="kpi-label">{{ t('pg.cron_kpi_total') }}</div></div></div>
        <div class="kpi"><span class="kpi-ico c-green"><Icon name="play" :size="20" /></span><div><div class="kpi-val">{{ jobs.filter((c) => c.enabled).length }}</div><div class="kpi-label">{{ t('pg.cron_kpi_on') }}</div></div></div>
        <div class="kpi"><span class="kpi-ico c-blue"><Icon name="globe" :size="20" /></span><div><div class="kpi-val">{{ jobs.filter((c) => c.run_as !== 'root').length }}</div><div class="kpi-label">{{ t('pg.cron_kpi_sites') }}</div></div></div>
      </div>

      <section class="panel">
        <div class="panel-tools">
          <div class="tabs" role="group" :aria-label="t('pg.cron_show')">
            <button v-for="f in ['all', 'on', 'off']" :key="f" type="button" :aria-pressed="show === f" @click="show = f">{{ t('pg.cron_show_' + f) }}</button>
          </div>
          <span class="tools-gap"></span>
          <label class="search-field">
            <Icon name="search" :size="15" />
            <input v-model="q" type="search" :placeholder="t('pg.cron_filter')" :aria-label="t('pg.cron_filter')" />
          </label>
        </div>
        <div v-if="!jobs.length" class="empty">
          <div class="empty-icon"><Icon name="clock" :size="26" /></div>
          <p class="empty-lead">{{ t('cron.empty') }}</p>
          <p class="muted">{{ t('cron.subtitle') }}</p>
          <button class="btn btn-primary" @click="openForm(null)"><Icon name="plus" />{{ t('cron.add') }}</button>
        </div>
        <p v-else-if="!shown.length" class="muted panel-body" style="margin: 0">{{ t('pg.no_match') }}</p>
        <div v-else class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>{{ t('pg.cron_on') }}</th>
                <th>{{ t('cron.name') }}</th>
                <th>{{ t('cron.schedule') }}</th>
                <th>{{ t('cron.run_as') }}</th>
                <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="c in shown" :key="c.id" :class="{ 'row-off': !c.enabled }">
                <td>
                  <button type="button" class="switch switch-sm" role="switch" :aria-checked="c.enabled" :aria-label="t('pg.cron_toggle', { name: c.name })" @click="toggle(c)"></button>
                </td>
                <td class="cron-name">
                  <div class="cron-title">{{ c.name }}</div>
                  <div class="mono small muted command-cell">{{ c.command }}</div>
                </td>
                <td class="nowrap">
                  <div>{{ cronText(c.schedule, t, locale) || c.schedule }}</div>
                  <div class="mono small muted">{{ c.schedule }}</div>
                </td>
                <td class="small">
                  <span class="status-pill" :class="c.run_as === 'root' ? 'warn' : ''">{{ c.run_as }}</span>
                  <div v-if="c.site" class="muted name-ico-sub">{{ c.site }}</div>
                </td>
                <td class="actions">
                  <RowActions :actions="[
                    { label: t('cron.run'), icon: 'play', inline: true, disabled: jobRunning, onClick: () => runNow(c) },
                    { label: t('logs.button'), icon: 'logs', onClick: () => (logJob = c) },
                    { label: t('cron.edit'), icon: 'edit', onClick: () => openForm(c) },
                    { label: t('websites.delete'), icon: 'trash', danger: true, onClick: () => remove(c) },
                  ]" />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>

    <Modal v-if="logJob" wide :title="t('logs.title', { name: logJob.name })" @close="logJob = null"><LogViewer :key="logJob.id" :url="() => `api/cron/${logJob.id}/log`" /></Modal>

    <Modal v-if="editing" :title="editing.id ? t('cron.edit_title') : t('cron.new_title')" @close="editing = null">
      <form class="stack" @submit.prevent="save">
        <label class="field">
          <span>{{ t('cron.name') }}</span>
          <input v-model="editing.name" class="input" maxlength="100" required />
        </label>
        <label class="field">
          <span>{{ t('cron.run_as') }}</span>
          <select v-model="editing.run_as" class="input">
            <option v-for="u in data.users || []" :key="u.user" :value="u.user">{{ userLabel(u) }}</option>
          </select>
          <small class="muted">{{ t('cron.run_as_hint') }}</small>
        </label>
        <div class="field">
          <span>{{ t('cron.schedule') }}</span>
          <div class="cron-presets" role="group" :aria-label="t('cron.presets')">
            <button v-for="p in presets" :key="p.value" type="button" class="chip chip-text" :aria-pressed="editing.schedule === p.value" @click="editing.schedule = p.value">{{ p.label }}</button>
          </div>
          <input v-model="editing.schedule" class="input mono" spellcheck="false" required :aria-label="t('cron.schedule')" />
          <small class="muted">
            <template v-if="cronText(editing.schedule, t, locale)"><strong class="text-ok">{{ cronText(editing.schedule, t, locale) }}</strong> · </template>{{ t('cron.schedule_hint') }}
          </small>
        </div>
        <label class="field">
          <span>{{ t('cron.command') }}</span>
          <textarea v-model="editing.command" class="input mono" rows="3" spellcheck="false" maxlength="2000" required></textarea>
          <small class="muted">
            {{ t('cron.command_hint') }}
            <a v-if="(data.users || []).some((u) => u.user === editing.run_as && u.site && u.php)" href="#" @click.prevent="useWordPressTemplate">{{ t('cron.wp_template') }}</a>
          </small>
        </label>
        <div class="opt">
          <div class="opt-text"><div class="opt-title">{{ t('cron.enabled') }}</div></div>
          <button type="button" class="switch" role="switch" :aria-checked="editing.enabled" :aria-label="t('cron.enabled')" @click="editing.enabled = !editing.enabled"></button>
        </div>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="editing = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="busy">{{ t('files.save') }}</button>
        </div>
      </form>
    </Modal>
  </div>
</template>

<style scoped>
.tools-gap { flex: 1; }
.cron-name { min-width: 220px; }
.cron-title { font-weight: 600; }
.row-off td { color: var(--muted); }
.cron-presets { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 4px; }
.cron-presets .chip { cursor: pointer; font-weight: 500; }
.cron-presets .chip[aria-pressed='true'] { background: var(--accent-soft); color: var(--accent); border-color: color-mix(in srgb, var(--accent) 40%, var(--border)); }
</style>
