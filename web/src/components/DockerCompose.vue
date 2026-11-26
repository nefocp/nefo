<script setup>
import { ref, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatDateTime } from '../format.js'
import Modal from './Modal.vue'
import RowActions from './RowActions.vue'
import LogViewer from './LogViewer.vue'
import Icon from './Icon.vue'

// DockerCompose lists the Compose projects and edits them. Changes run as
// jobs: the "job" event hands each one to the page, which shows its log and
// bumps "version" when it has finished.
const props = defineProps({
  version: { type: Number, default: 0 },
  busy: { type: Boolean, default: false }, // a job is running
})
const emit = defineEmits(['job'])
const { t, locale } = useI18n()

const data = ref(null)
const error = ref('')
const editing = ref(null) // { create, name, yaml, containers }
const formError = ref('')
const saving = ref(false)
const logs = ref(null)
const deleting = ref(null) // { name, volumes }

const EXAMPLE = `services:
  web:
    image: traefik/whoami
    restart: unless-stopped
    ports:
      - "127.0.0.1:8081:80"
`

async function load() {
  try {
    data.value = await api('GET', 'api/docker/compose')
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

function create() {
  formError.value = ''
  editing.value = { create: true, name: '', yaml: EXAMPLE, containers: [] }
}

async function edit(p) {
  error.value = ''
  formError.value = ''
  try {
    const res = await api('GET', `api/docker/compose/${p.name}`)
    editing.value = { create: false, name: p.name, yaml: res.yaml, containers: res.containers }
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function save() {
  formError.value = ''
  saving.value = true
  const f = editing.value
  try {
    const res = f.create
      ? await api('POST', 'api/docker/compose', { name: f.name.trim(), yaml: f.yaml })
      : await api('PUT', `api/docker/compose/${f.name}`, { yaml: f.yaml })
    emit('job', res.job)
    editing.value = null
  } catch (e) {
    formError.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

async function act(p, action) {
  error.value = ''
  if (action === 'down' && !window.confirm(t('docker.compose_down_confirm', { name: p.name }))) return
  try {
    emit('job', (await api('POST', `api/docker/compose/${p.name}/${action}`, {})).job)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function remove() {
  error.value = ''
  const { name, volumes } = deleting.value
  try {
    emit('job', (await api('DELETE', `api/docker/compose/${name}`, { volumes })).job)
    deleting.value = null
  } catch (e) {
    error.value = errorMessage(e)
    deleting.value = null
  }
}

const stateTag = (s) => ({ running: 'tag-ok', partial: 'tag-warn', stopped: 'tag-warn' })[s] || ''

watch(() => props.version, load)
onMounted(load)
defineExpose({ create })
</script>

<template>
  <div>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="data && data.installed && !data.compose_installed" class="alert alert-warn">{{ t('docker.compose_missing') }}</p>

    <section v-if="data" class="card">
      <div class="card-head">
        <h2>{{ t('docker.compose_title') }}</h2>
      </div>
      <p class="muted small">{{ t('docker.compose_intro') }}</p>
      <div v-if="!data.projects.length" class="empty">
        <p class="empty-lead">{{ t('docker.compose_none') }}</p>
        <button class="btn btn-primary" @click="create"><Icon name="plus" />{{ t('docker.compose_new') }}</button>
      </div>
      <div v-else class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>{{ t('docker.col_project') }}</th>
              <th>{{ t('docker.col_state') }}</th>
              <th>{{ t('docker.col_containers') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in data.projects" :key="p.name">
              <td>
                <strong class="mono">{{ p.name }}</strong>
                <div class="muted small">{{ t('docker.compose_saved', { time: formatDateTime(p.updated, locale) }) }}</div>
              </td>
              <td><span class="tag" :class="stateTag(p.state)">{{ t(`docker.state_${p.state}`) }}</span></td>
              <td class="small nowrap">{{ t('docker.compose_count', { running: p.running, total: p.containers }) }}</td>
              <td class="actions">
                <RowActions :actions="[
                  p.state === 'running'
                    ? { label: t('services.stop'), icon: 'stop', inline: true, disabled: busy, onClick: () => act(p, 'stop') }
                    : { label: t('services.start'), icon: 'play', inline: true, disabled: busy, onClick: () => act(p, 'start') },
                  { label: t('docker.compose_edit'), icon: 'code', onClick: () => edit(p) },
                  { label: t('logs.button'), icon: 'logs', onClick: () => (logs = p.name) },
                  { label: t('services.restart'), icon: 'refresh', disabled: busy, hidden: !p.containers, onClick: () => act(p, 'restart') },
                  { label: t('docker.compose_update'), icon: 'download', disabled: busy, onClick: () => act(p, 'update') },
                  { label: t('docker.compose_down'), icon: 'stop', disabled: busy, hidden: !p.containers, onClick: () => act(p, 'down') },
                  { label: t('websites.delete'), icon: 'trash', danger: true, disabled: busy, onClick: () => (deleting = { name: p.name, volumes: false }) },
                ]" />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <Modal v-if="editing" wide :title="editing.create ? t('docker.compose_new') : t('docker.compose_edit_title', { name: editing.name })" @close="editing = null">
      <form class="stack" @submit.prevent="save">
        <label v-if="editing.create" class="field">
          <span>{{ t('docker.compose_name') }}</span>
          <input v-model="editing.name" class="input mono" placeholder="my-app" required maxlength="40" pattern="[a-z0-9][a-z0-9\-]*" autocomplete="off" />
          <small class="muted">{{ t('docker.compose_name_hint') }}</small>
        </label>
        <label class="field">
          <span>{{ t('docker.compose_file') }}</span>
          <textarea v-model="editing.yaml" class="input mono compose-yaml" rows="18" spellcheck="false" autocapitalize="off" autocomplete="off" required></textarea>
        </label>
        <p class="muted small">{{ t('docker.compose_rules') }}</p>
        <div v-if="editing.containers.length" class="table-wrap">
          <table class="table">
            <tbody>
              <tr v-for="c in editing.containers" :key="c.id">
                <td><strong>{{ c.service }}</strong><div class="muted small mono">{{ c.image }}</div></td>
                <td class="small"><span class="dot" :class="c.state === 'running' ? 'dot-ok' : 'dot-warn'" aria-hidden="true"></span>{{ c.status }}</td>
                <td class="mono small">{{ c.ports }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-if="formError" class="alert alert-crit pre-wrap" role="alert">{{ formError }}</p>
        <div class="modal-actions">
          <button v-if="editing.create" type="button" class="btn btn-ghost" @click="editing.yaml = EXAMPLE">{{ t('docker.compose_example') }}</button>
          <span class="toolbar-gap"></span>
          <button type="button" class="btn btn-ghost" @click="editing = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="saving || busy">{{ editing.create ? t('docker.compose_create') : t('docker.compose_save') }}</button>
        </div>
      </form>
    </Modal>

    <Modal v-if="logs" wide :title="t('logs.title', { name: logs })" @close="logs = null">
      <LogViewer :key="logs" :url="() => `api/docker/compose/${logs}/logs`" />
    </Modal>

    <Modal v-if="deleting" danger :title="t('docker.compose_delete_title', { name: deleting.name })" @close="deleting = null">
      <form class="stack" @submit.prevent="remove">
        <p>{{ t('docker.compose_delete_text') }}</p>
        <label class="check"><input v-model="deleting.volumes" type="checkbox" />{{ t('docker.compose_delete_volumes') }}</label>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="deleting = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-danger btn-solid" :disabled="busy">{{ t('websites.delete') }}</button>
        </div>
      </form>
    </Modal>
  </div>
</template>

<style scoped>
.compose-yaml { min-height: 340px; tab-size: 2; white-space: pre; }
.modal-actions .toolbar-gap { flex: 1; }
</style>
