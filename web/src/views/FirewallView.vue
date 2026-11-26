<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../page.js'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import JobLog from '../components/JobLog.vue'
import Modal from '../components/Modal.vue'
import { useJob } from '../composables/useJob.js'

const { t } = useI18n()
const data = ref(null)
const error = ref('')
const notice = ref('')
const { job, running: jobRunning, watch: watchJob, resume, clear: clearJob } = useJob(() => load())

async function load() {
  try {
    data.value = await api('GET', 'api/security')
    ssh.value = { ...data.value.ssh }
  } catch (e) {
    error.value = errorMessage(e)
  }
}

function clearMessages() {
  error.value = ''
  notice.value = ''
}

async function call(method, path, body, okText) {
  clearMessages()
  try {
    await api(method, path, body)
    if (okText) notice.value = okText
    await load()
    return true
  } catch (e) {
    error.value = errorMessage(e)
    return false
  }
}

// ---- firewall ----
const rule = ref({ action: 'allow', port: '', proto: 'tcp', from: '', comment: '' })
const showRule = ref(false)

async function addRule() {
  if (await call('POST', 'api/security/firewall/rules', rule.value, t('firewall.rule_added'))) {
    showRule.value = false
    rule.value = { action: 'allow', port: '', proto: 'tcp', from: '', comment: '' }
  }
}

function deleteRule(r) {
  if (!window.confirm(t('firewall.delete_confirm', { rule: `${r.action} ${r.to} ← ${r.from}` }))) return
  call('POST', 'api/security/firewall/rules/delete', { number: r.number, to: r.to, from: r.from }, t('firewall.rule_deleted'))
}

function enableFirewall() {
  const f = data.value.firewall
  const web = (f.web_ports || []).join(', ')
  const msg = web
    ? t('firewall.enable_confirm_web', { ssh: f.ssh_port, panel: f.panel_port, web })
    : t('firewall.enable_confirm', { ssh: f.ssh_port, panel: f.panel_port })
  if (!window.confirm(msg)) return
  call('POST', 'api/security/firewall/enable', {}, t('firewall.enabled'))
}

// ---- SSH ----
const ssh = ref(null)

function saveSSH() {
  const s = ssh.value
  if (s.port !== data.value.ssh.port && !window.confirm(t('firewall.ssh_port_confirm', { port: s.port }))) return
  call('PUT', 'api/security/ssh', { port: Number(s.port), password_auth: s.password_auth, permit_root_login: s.permit_root_login }, t('firewall.ssh_saved'))
}

// ---- fail2ban ----
async function installFail2ban() {
  clearMessages()
  try {
    watchJob(await api('POST', 'api/security/fail2ban/install', {}))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// ---- layout ----
const route = useRoute()
const router = useRouter()
const tab = computed(() => (['ssh', 'f2b'].includes(route.query.tab) ? route.query.tab : 'fw'))
const setTab = (id) => router.replace({ query: { ...route.query, tab: id === 'fw' ? undefined : id } })
const fw = computed(() => data.value?.firewall || {})
const f2b = computed(() => data.value?.fail2ban || {})
const rules = computed(() => fw.value.rules || [])
// While ufw is off it lists its rules only as commands ("allow 22/tcp"), unnumbered.
const added = computed(() => fw.value.added || [])
const ruleCount = computed(() => (fw.value.active ? rules.value.length : added.value.length))
const bannedNow = computed(() => (f2b.value.jails || []).reduce((a, j) => a + (j.banned || []).length, 0))
// ufw's actions: LIMIT allows, at a slower rate; REJECT refuses, as DENY.
const actionPill = (a) => (a === 'ALLOW' || a === 'LIMIT' ? 'ok' : 'crit')
const actionLabel = (a) => t({ ALLOW: 'firewall.allow', LIMIT: 'firewall.limit', REJECT: 'firewall.reject' }[a] || 'firewall.deny')
const loginText = (l) => `${l.user} (${[l.key && t('firewall.login_key'), l.password && t('firewall.login_password')].filter(Boolean).join(', ')})`
const sshRisk = computed(() => data.value?.ssh?.password_auth && data.value?.ssh?.permit_root_login === 'yes')
usePageMeta(() => ({ sub: data.value ? [fw.value.active ? t('pg.fw_active') : t('attention.firewall_off'), `SSH ${data.value.ssh.port}`, f2b.value.running ? 'fail2ban' : ''].filter(Boolean).join(' · ') : '' }))

onMounted(async () => {
  await load()
  resume(['install_fail2ban'])
})
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('firewall.title') }}</h1></div>
    <Teleport defer to="#topbar-actions">
      <button v-if="data?.firewall?.installed" class="btn btn-primary" @click="showRule = true; clearMessages()"><Icon name="plus" :size="15" />{{ t('firewall.add_rule') }}</button>
    </Teleport>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />

    <template v-if="!data && !error">
      <div class="kpis"><span v-for="i in 3" :key="i" class="skel" style="height: 72px; border-radius: 14px"></span></div>
      <span class="skel" style="height: 240px; border-radius: 14px"></span>
    </template>

    <template v-if="data">
      <div class="kpis">
        <button type="button" class="kpi kpi-btn" :aria-pressed="tab === 'fw'" @click="setTab('fw')">
          <span class="kpi-ico" :class="fw.active ? 'c-green' : 'c-red'"><Icon name="shield" :size="20" /></span>
          <div>
            <div class="kpi-val"><span class="status-pill" :class="fw.active ? 'ok' : 'crit'">{{ fw.active ? t('drawer.on') : t('drawer.off') }}</span></div>
            <div class="kpi-label">{{ t('pg.fw_kpi_fw', { n: ruleCount }, ruleCount) }}</div>
          </div>
        </button>
        <button type="button" class="kpi kpi-btn" :aria-pressed="tab === 'ssh'" @click="setTab('ssh')">
          <span class="kpi-ico" :class="sshRisk ? 'c-orange' : 'c-teal'"><Icon name="terminal" :size="20" /></span>
          <div>
            <div class="kpi-val">{{ t('pg.fw_port', { port: data.ssh.port }) }}</div>
            <div class="kpi-label">{{ data.ssh.password_auth ? t('pg.fw_ssh_pw') : t('pg.fw_ssh_keys') }}</div>
          </div>
        </button>
        <button type="button" class="kpi kpi-btn" :aria-pressed="tab === 'f2b'" @click="setTab('f2b')">
          <span class="kpi-ico" :class="f2b.running ? 'c-blue' : 'c-gray'"><Icon name="shieldCheck" :size="20" /></span>
          <div>
            <div class="kpi-val">{{ f2b.installed ? (f2b.running ? t('pg.fw_banned', { n: bannedNow }) : t('pg.svc_state_inactive')) : t('pg.not_installed') }}</div>
            <div class="kpi-label">fail2ban</div>
          </div>
        </button>
      </div>

      <nav class="ptabs" role="tablist" :aria-label="t('firewall.title')">
        <button type="button" role="tab" :aria-selected="tab === 'fw'" @click="setTab('fw')"><Icon name="shield" :size="16" />{{ t('firewall.fw_title') }}<span class="n">{{ ruleCount }}</span></button>
        <button type="button" role="tab" :aria-selected="tab === 'ssh'" @click="setTab('ssh')"><Icon name="terminal" :size="16" />{{ t('firewall.ssh_title') }}</button>
        <button type="button" role="tab" :aria-selected="tab === 'f2b'" @click="setTab('f2b')"><Icon name="shieldCheck" :size="16" />{{ t('firewall.f2b_title') }}</button>
      </nav>

      <!-- firewall -->
      <template v-if="tab === 'fw'">
        <div v-if="!fw.installed" class="card empty">
          <div class="empty-icon"><Icon name="shield" :size="26" /></div>
          <p class="empty-lead">{{ t('firewall.no_ufw') }}</p>
        </div>
        <template v-else-if="!fw.active">
          <div class="card empty">
            <div class="empty-icon fw-off"><Icon name="shieldAlert" :size="26" /></div>
            <p class="empty-lead">{{ t('attention.firewall_off') }}</p>
            <p class="muted">{{ t('firewall.inactive') }}</p>
            <button class="btn btn-primary" @click="enableFirewall"><Icon name="shieldCheck" />{{ t('firewall.enable') }}</button>
          </div>
          <section class="panel">
            <div class="panel-tools">
              <span class="status-pill crit">{{ t('drawer.off') }}</span>
              <span class="muted small">{{ t('firewall.your_ip', { ip: data.your_ip }) }}</span>
              <span class="tools-gap"></span>
              <button class="btn btn-sm" @click="showRule = true; clearMessages()"><Icon name="plus" :size="14" />{{ t('firewall.add_rule') }}</button>
            </div>
            <p class="muted small added-hint">{{ t('firewall.added_hint') }}</p>
            <div class="table-wrap">
              <table class="table">
                <thead>
                  <tr><th>{{ t('firewall.added_rule') }}</th></tr>
                </thead>
                <tbody>
                  <tr v-for="(c, i) in added" :key="i"><td class="mono small">ufw {{ c }}</td></tr>
                  <tr v-if="!added.length"><td class="muted">{{ t('firewall.added_none') }}</td></tr>
                </tbody>
              </table>
            </div>
          </section>
        </template>
        <section v-else class="panel">
          <div class="panel-tools">
            <span class="status-pill ok">{{ t('pg.fw_active') }}</span>
            <span class="muted small">{{ t('firewall.your_ip', { ip: data.your_ip }) }}</span>
            <span class="tools-gap"></span>
            <button class="btn btn-sm" @click="showRule = true; clearMessages()"><Icon name="plus" :size="14" />{{ t('firewall.add_rule') }}</button>
          </div>
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>{{ t('firewall.to') }}</th>
                  <th>{{ t('firewall.action') }}</th>
                  <th>{{ t('firewall.from') }}</th>
                  <th>{{ t('firewall.comment') }}</th>
                  <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="r in rules" :key="r.number">
                  <td><span class="pill mono">{{ r.to }}</span></td>
                  <td><span class="status-pill" :class="actionPill(r.action)" :title="r.action === 'LIMIT' ? t('firewall.limit_hint') : undefined">{{ actionLabel(r.action) }}</span></td>
                  <td class="mono small">{{ /^anywhere/i.test(r.from) ? t('pg.fw_anywhere') + (r.v6 ? ' (IPv6)' : '') : r.from }}</td>
                  <td class="small">{{ r.comment || '—' }}</td>
                  <td class="actions">
                    <span v-if="r.protected" class="tag" :title="t('firewall.protected_hint', { ip: data.your_ip })"><Icon name="lock" :size="12" />{{ t('firewall.protected') }}</span>
                    <button v-else class="icon-btn icon-btn-sm danger" :aria-label="t('pg.fw_delete_rule', { rule: r.to })" :title="t('websites.delete')" @click="deleteRule(r)"><Icon name="trash" :size="15" /></button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </template>

      <!-- SSH -->
      <section v-else-if="tab === 'ssh' && ssh" class="panel">
        <div class="panel-head">
          <h2 class="panel-title"><Icon name="terminal" :size="17" />{{ t('firewall.ssh_title') }}</h2>
          <span v-if="data.ssh.socket_activated" class="tag">{{ t('pg.fw_socket') }}</span>
        </div>
        <form class="panel-body stack" @submit.prevent="saveSSH">
          <p v-if="!data.ssh.root_has_keys" class="alert alert-warn" style="margin: 0">{{ t('firewall.no_keys') }}</p>
          <p v-if="data.ssh.logins && !data.ssh.logins.length" class="alert alert-warn" style="margin: 0">{{ t('firewall.logins_none') }}</p>
          <p v-else-if="data.ssh.logins" class="muted small" style="margin: 0">
            {{ t('firewall.logins') }}
            <span v-for="l in data.ssh.logins" :key="l.user" class="pill mono login">{{ loginText(l) }}</span>
          </p>
          <div class="ssh-grid">
            <label class="field">
              <span>{{ t('firewall.ssh_port') }}</span>
              <input v-model.number="ssh.port" class="input mono" type="number" min="1" max="65535" :disabled="data.ssh.socket_activated" required />
            </label>
            <label class="field">
              <span>{{ t('firewall.root_login') }}</span>
              <select v-model="ssh.permit_root_login" class="input">
                <option value="prohibit-password">{{ t('firewall.root_keys') }}</option>
                <option value="yes">{{ t('firewall.root_yes') }}</option>
                <option value="no">{{ t('firewall.root_no') }}</option>
              </select>
            </label>
          </div>
          <div class="opt">
            <div class="opt-text">
              <div class="opt-title">{{ t('firewall.password_auth') }}</div>
              <div class="opt-hint">{{ ssh.password_auth ? t('pg.fw_pw_on') : t('pg.fw_pw_off') }}</div>
            </div>
            <button type="button" class="switch" role="switch" :aria-checked="ssh.password_auth" :aria-label="t('firewall.password_auth')" @click="ssh.password_auth = !ssh.password_auth"></button>
          </div>
          <p class="muted small" style="margin: 0">{{ t('firewall.ssh_hint') }}</p>
          <div><button class="btn btn-primary"><Icon name="check" :size="15" />{{ t('files.save') }}</button></div>
        </form>
      </section>

      <!-- fail2ban -->
      <template v-else-if="tab === 'f2b'">
        <div v-if="!f2b.installed" class="card empty">
          <div class="empty-icon"><Icon name="shieldCheck" :size="26" /></div>
          <p class="empty-lead">{{ t('firewall.f2b_title') }}</p>
          <p class="muted">{{ t('firewall.f2b_body') }}</p>
          <button class="btn btn-primary" :disabled="jobRunning" @click="installFail2ban"><Icon name="download" />{{ t('firewall.f2b_install') }}</button>
        </div>
        <p v-else-if="!f2b.running" class="alert alert-warn">{{ t('firewall.f2b_stopped') }}</p>
        <section v-else class="panel">
          <div class="panel-tools">
            <span class="status-pill ok">{{ t('pg.svc_state_active') }}</span>
            <span class="muted small">{{ t('firewall.f2b_body') }}</span>
          </div>
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>{{ t('firewall.jail') }}</th>
                  <th>{{ t('firewall.failed') }}</th>
                  <th>{{ t('firewall.banned') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="j in f2b.jails || []" :key="j.name">
                  <td><strong>{{ j.name }}</strong><div class="name-ico-sub">{{ t('firewall.total_banned', { n: j.total_banned }) }}</div></td>
                  <td class="num">{{ j.failed }}</td>
                  <td>
                    <span v-if="!(j.banned || []).length" class="muted">—</span>
                    <span v-for="ip in j.banned || []" :key="ip" class="ban">
                      <span class="pill mono">{{ ip }}</span>
                      <button class="btn btn-sm" @click="call('POST', 'api/security/fail2ban/unban', { jail: j.name, ip }, t('firewall.unbanned', { ip }))">{{ t('firewall.unban') }}</button>
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </template>

      <Modal v-if="showRule" :title="t('firewall.add_rule')" @close="showRule = false">
        <form class="stack" @submit.prevent="addRule">
          <div class="tabs rule-act" role="radiogroup" :aria-label="t('firewall.action')">
            <button type="button" role="radio" :aria-checked="rule.action === 'allow'" :aria-pressed="rule.action === 'allow'" @click="rule.action = 'allow'"><Icon name="check" :size="14" />{{ t('firewall.allow') }}</button>
            <button type="button" role="radio" :aria-checked="rule.action === 'deny'" :aria-pressed="rule.action === 'deny'" @click="rule.action = 'deny'"><Icon name="ban" :size="14" />{{ t('firewall.deny') }}</button>
          </div>
          <div class="row">
            <label class="field grow">
              <span>{{ t('firewall.port') }}</span>
              <input v-model="rule.port" class="input mono" placeholder="8080" required />
            </label>
            <label class="field grow">
              <span>{{ t('procs.col_proto') }}</span>
              <select v-model="rule.proto" class="input">
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
                <option value="">TCP + UDP</option>
              </select>
            </label>
          </div>
          <label class="field">
            <span>{{ t('firewall.from') }}</span>
            <input v-model="rule.from" class="input mono" :placeholder="t('firewall.from_any')" />
          </label>
          <label class="field">
            <span>{{ t('firewall.comment') }}</span>
            <input v-model="rule.comment" class="input" maxlength="60" />
          </label>
          <div class="modal-actions">
            <button type="button" class="btn btn-ghost" @click="showRule = false">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary">{{ t('firewall.add') }}</button>
          </div>
        </form>
      </Modal>
    </template>
  </div>
</template>

<style scoped>
.tools-gap { flex: 1; }
.kpi-btn { cursor: pointer; text-align: left; font: inherit; color: inherit; }
.kpi-btn:hover { border-color: var(--border-strong); }
.kpi-btn[aria-pressed='true'] { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.empty-icon.fw-off { background: var(--crit-soft); color: var(--crit); }
.added-hint { margin: 0; padding: 12px 18px 0; }
.ssh-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 14px; max-width: 640px; }
.rule-act { display: flex; width: 100%; }
.rule-act button { flex: 1; justify-content: center; }
.num { font-variant-numeric: tabular-nums; }
.login { margin-left: 6px; }
</style>
