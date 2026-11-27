<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Modal from './Modal.vue'
import Icon from './Icon.vue'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'

// DbPgRemote edits the client addresses that may connect to one PostgreSQL
// database from other machines: pg_hba.conf lines for this database and
// its role, ufw rules for the cluster's port and, while any database
// allows it, listen_addresses = '*'. It emits "saved" with the restart job,
// if PostgreSQL has to restart to change where it listens.
const props = defineProps({
  db: { type: Object, required: true },
})
const emit = defineEmits(['close', 'saved'])
const { t } = useI18n()

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
    fill(await api('GET', `api/postgres/databases/${props.db.id}/remote`))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const entered = computed(() => hosts.value.map((h) => h.trim()).filter(Boolean))
const on = computed(() => !!state.value?.hosts.length)

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
    const res = await api('PUT', `api/postgres/databases/${props.db.id}/remote`, { hosts: list })
    fill(res.remote)
    restarting.value = !!res.job
    notice.value = list.length ? t('databases.remote_saved') : t('databases.remote_disabled')
    if (res.job) notice.value += ' ' + t('pgremote.restarting')
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
    const job = await api('POST', 'api/postgres/restart', {})
    restarting.value = true
    notice.value = t('pgremote.restarting')
    emit('saved', { job, hosts: state.value.hosts })
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

// What the encryption line says: TLS required (with which certificate),
// or not encrypted.
const tlsText = computed(() => {
  const s = state.value
  if (!s) return ''
  if (!s.ssl) return t('pgremote.tls_off')
  return t(`pgremote.tls_cert_${s.certificate || 'custom'}`, { file: s.cert_file || '' })
})

const connString = computed(() => `host=SERVER_IP port=${state.value?.port || 5432} dbname=${props.db.name} user=${props.db.username} sslmode=require`)

onMounted(load)
</script>

<template>
  <Modal :title="t('databases.remote_title', { name: db.name })" wide @close="emit('close')">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <p class="muted">{{ t('pgremote.intro') }}</p>

    <template v-if="state">
      <p v-if="!state.firewall_installed" class="alert alert-crit">{{ t('databases.remote_no_firewall') }}</p>
      <p v-else-if="!state.firewall_active" class="alert alert-warn">
        {{ t('pgremote.firewall_inactive', { port: state.port }) }}
        <RouterLink :to="{ name: 'firewall' }">{{ t('nav.firewall') }}</RouterLink>
      </p>

      <div v-if="on" class="pgr-state">
        <span class="tag tag-warn">{{ t('databases.remote_on') }}</span>
        <span v-if="state.restart_pending && !restarting" class="text-crit small">{{ t('pgremote.restart_pending') }}</span>
        <span v-else-if="state.listening" class="muted small">{{ t('pgremote.listening', { port: state.port }) }}</span>
        <button v-if="state.restart_pending && !restarting" type="button" class="btn btn-sm" :disabled="busy" @click="restartNow">
          <Icon name="refresh" :size="15" />{{ t('pg.restart') }}
        </button>
      </div>

      <div class="pgr-tls" :class="state.ssl ? 'ok' : 'warn'">
        <Icon :name="state.ssl ? 'lock' : 'alert'" :size="16" />
        <div>
          <strong>{{ state.ssl ? t('pgremote.tls_on_title') : t('pgremote.tls_off_title') }}</strong>
          <div class="small">{{ tlsText }}</div>
        </div>
      </div>

      <form class="stack" @submit.prevent="save(entered)">
        <fieldset class="stack-tight">
          <legend class="field-label">{{ t('databases.remote_hosts') }}</legend>
          <div v-for="(h, i) in hosts" :key="i" class="row">
            <input
              v-model="hosts[i]"
              class="input grow mono"
              :placeholder="i === 0 ? '203.0.113.5' : '2001:db8:5::/48'"
              maxlength="64"
              spellcheck="false"
              autocomplete="off"
              :aria-label="t('databases.remote_host_n', { n: i + 1 })"
            />
            <button type="button" class="icon-btn" :aria-label="t('databases.remote_remove_host')" @click="removeRow(i)"><Icon name="x" /></button>
          </div>
          <small class="muted">{{ t('pgremote.hosts_hint', { max: MAX }) }}</small>
        </fieldset>
        <div>
          <button type="button" class="btn btn-sm btn-ghost" :disabled="hosts.length >= MAX" @click="addRow">
            <Icon name="plus" :size="15" />{{ t('databases.remote_add_host') }}
          </button>
        </div>

        <div class="alert alert-warn small">
          <strong>{{ t('databases.remote_security_title') }}</strong>
          <ul class="tips">
            <li>{{ t('pgremote.tip_what', { port: state.port }) }}</li>
            <li>{{ t('databases.remote_tip_static') }}</li>
            <li>{{ t('pgremote.tip_restart') }}</li>
          </ul>
        </div>
        <div class="small muted">
          {{ t('pgremote.connect_hint') }}
          <code class="pgr-conn mono">{{ connString }}</code>
          <span v-if="state.hba_file">{{ state.hba_include ? t('pgremote.hba_include', { file: state.hba_file }) : t('pgremote.hba_block', { file: state.hba_file }) }}</span>
        </div>

        <div class="modal-actions">
          <button v-if="on" type="button" class="btn btn-danger" :disabled="busy" @click="turnOff">{{ t('databases.remote_turn_off') }}</button>
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
.pgr-state { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; margin: 4px 0 12px; }
.pgr-tls { display: flex; gap: 10px; align-items: flex-start; padding: 10px 12px; border-radius: 10px; border: 1px solid var(--border); margin-bottom: 14px; }
.pgr-tls.ok { background: var(--ok-soft); border-color: transparent; }
.pgr-tls.ok .icon { color: var(--ok); }
.pgr-tls.warn { background: var(--warn-soft); border-color: transparent; }
.pgr-tls.warn .icon { color: var(--warn); }
.pgr-tls .icon { flex: none; margin-top: 2px; }
.pgr-conn { display: block; margin: 6px 0; padding: 6px 8px; border-radius: 8px; background: var(--surface-2); overflow-wrap: anywhere; color: var(--text); }
</style>
