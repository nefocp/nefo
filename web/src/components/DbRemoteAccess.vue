<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Modal from './Modal.vue'
import Icon from './Icon.vue'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { dbEngineName } from '../features.js'

// DbRemoteAccess edits the client addresses that may connect to one
// database from other machines. It emits "saved" with the restart job, if
// MariaDB has to restart to listen on all interfaces.
const props = defineProps({
  db: { type: Object, required: true },
})
const emit = defineEmits(['close', 'saved'])
const { t } = useI18n()
const engine = dbEngineName() // "MariaDB" or "MySQL", for the messages

const MAX = 10
const state = ref(null)
const hosts = ref([''])
const error = ref('')
const notice = ref('')
const busy = ref(false)
const restarting = ref(false) // a restart job was started from here

function fill(s) {
  state.value = s
  hosts.value = s.hosts.length ? [...s.hosts] : ['']
}

async function load() {
  try {
    fill(await api('GET', `api/databases/${props.db.id}/remote`))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const entered = computed(() => hosts.value.map((h) => h.trim()).filter(Boolean))

function addRow() {
  if (hosts.value.length < MAX) hosts.value.push('')
}

function removeRow(i) {
  hosts.value.splice(i, 1)
  if (!hosts.value.length) hosts.value.push('')
}

async function save(list) {
  error.value = ''
  notice.value = ''
  busy.value = true
  try {
    const res = await api('PUT', `api/databases/${props.db.id}/remote`, { hosts: list })
    fill(res.remote)
    restarting.value = !!res.job
    notice.value = list.length ? t('databases.remote_saved') : t('databases.remote_disabled')
    if (res.job) notice.value += ' ' + t('databases.remote_restarting', { engine })
    emit('saved', { job: res.job || null, hosts: res.remote.hosts })
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

function turnOff() {
  if (window.confirm(t('databases.remote_off_confirm', { name: props.db.name }))) save([])
}

async function restartNow() {
  error.value = ''
  busy.value = true
  try {
    const job = await api('POST', 'api/db/restart', {})
    restarting.value = true
    notice.value = t('databases.remote_restarting', { engine })
    emit('saved', { job, hosts: state.value.hosts })
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>

<template>
  <Modal :title="t('databases.remote_title', { name: db.name })" @close="emit('close')">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <p class="muted">{{ t('databases.remote_intro') }}</p>

    <template v-if="state">
      <p v-if="!state.firewall_installed" class="alert alert-crit">{{ t('databases.remote_no_firewall') }}</p>
      <p v-else-if="!state.firewall_active" class="alert alert-warn">
        {{ t('databases.remote_firewall_inactive', { engine }) }}
        <RouterLink :to="{ name: 'firewall' }">{{ t('nav.firewall') }}</RouterLink>
      </p>

      <p v-if="state.hosts.length" class="row small">
        <span class="tag tag-warn">{{ t('databases.remote_on') }}</span>
        <span v-if="state.restart_pending && !restarting" class="text-crit">{{ t('databases.remote_restart_pending', { engine }) }}</span>
        <span v-else-if="state.listening" class="muted">{{ t('databases.remote_listening', { port: state.port, engine }) }}</span>
        <button v-if="state.restart_pending && !restarting" type="button" class="btn btn-sm" :disabled="busy" @click="restartNow">
          <Icon name="refresh" :size="15" />{{ t('databases.remote_restart', { engine }) }}
        </button>
      </p>

      <form class="stack" @submit.prevent="save(entered)">
        <fieldset class="stack-tight">
          <legend class="field-label">{{ t('databases.remote_hosts') }}</legend>
          <div v-for="(h, i) in hosts" :key="i" class="row">
            <input
              v-model="hosts[i]"
              class="input grow mono"
              :placeholder="i === 0 ? '203.0.113.5' : '10.1.0.0/16'"
              maxlength="64"
              spellcheck="false"
              autocomplete="off"
              :aria-label="t('databases.remote_host_n', { n: i + 1 })"
            />
            <button type="button" class="icon-btn" :aria-label="t('databases.remote_remove_host')" @click="removeRow(i)"><Icon name="x" /></button>
          </div>
          <small class="muted">{{ t('databases.remote_hosts_hint', { max: MAX }) }}</small>
        </fieldset>
        <div>
          <button type="button" class="btn btn-sm btn-ghost" :disabled="hosts.length >= MAX" @click="addRow">
            <Icon name="plus" :size="15" />{{ t('databases.remote_add_host') }}
          </button>
        </div>

        <div class="alert alert-warn small">
          <strong>{{ t('databases.remote_security_title') }}</strong>
          <ul class="tips">
            <li>{{ t('databases.remote_tip_firewall', { port: state.port }) }}</li>
            <li>{{ t('databases.remote_tip_static') }}</li>
            <li>{{ t('databases.remote_tip_tls') }}</li>
            <li>{{ t('databases.remote_tip_restart', { engine }) }}</li>
          </ul>
        </div>
        <p class="small muted">{{ t('databases.remote_connect_hint', { user: db.username, port: state.port }) }}</p>

        <div class="modal-actions">
          <button v-if="state.hosts.length" type="button" class="btn btn-danger" :disabled="busy" @click="turnOff">{{ t('databases.remote_turn_off') }}</button>
          <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.close') }}</button>
          <button class="btn btn-primary" :disabled="busy || !entered.length">{{ t('databases.remote_save') }}</button>
        </div>
      </form>
    </template>
  </Modal>
</template>

<style scoped>
fieldset { border: 0; padding: 0; margin: 0; }
.stack-tight { display: flex; flex-direction: column; gap: 8px; }
.field-label { font-size: 13px; font-weight: 600; color: var(--text-2); margin-bottom: 4px; padding: 0; }
.tips { margin: 6px 0 0; padding-left: 18px; }
.tips li + li { margin-top: 3px; }
</style>
