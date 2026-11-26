<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import Icon from '../components/Icon.vue'
import BrandMark from '../components/BrandMark.vue'
import PowerDialog from '../components/PowerDialog.vue'
import { BRAND } from '../brand.js'
import { server } from '../server.js'
import { usePageMeta } from '../page.js'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import JobLog from '../components/JobLog.vue'
import { useJob } from '../composables/useJob.js'
import AlertsCard from '../components/AlertsCard.vue'
import CloudflareCard from '../components/CloudflareCard.vue'
import MailRelayCard from '../components/MailRelayCard.vue'

const { t, locale } = useI18n()
const data = ref(null)
const error = ref('')
const notice = ref('')
const domain = ref('')
const update = ref(null)
const checking = ref(false)
const { job, running: jobRunning, watch: watchJob, resume, clear: clearJob } = useJob(() => load())

async function load() {
  try {
    data.value = await api('GET', 'api/panel')
    if (!domain.value) domain.value = data.value.domain || window.location.hostname
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function getCertificate() {
  error.value = ''
  notice.value = ''
  try {
    watchJob(await api('POST', 'api/panel/certificate', { domain: domain.value.trim().toLowerCase() }))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function removeCertificate() {
  if (!window.confirm(t('settings.cert_remove_confirm'))) return
  try {
    await api('DELETE', 'api/panel/certificate')
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function checkUpdate() {
  error.value = ''
  checking.value = true
  try {
    update.value = await api('GET', 'api/panel/update')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    checking.value = false
  }
}

async function applyUpdate() {
  if (!window.confirm(t('settings.update_confirm', { v: update.value.latest }))) return
  try {
    watchJob(await api('POST', 'api/panel/update', {}))
    setTimeout(() => window.location.reload(), 20000)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const panelURL = () => `https://${domain.value || 'example.com'}:${data.value.port}${window.location.pathname}`
const formatDate = (v) => new Date(v).toLocaleDateString(locale.value, { dateStyle: 'medium' })

// ---- tabs ----
const route = useRoute()
const router = useRouter()
const tabs = computed(() => [
  { id: 'panel', icon: 'dashboard', label: t('pg.set_tab_panel') },
  { id: 'cert', icon: 'lock', label: t('pg.set_tab_cert') },
  { id: 'mail', icon: 'upload', label: t('pg.set_tab_mail') },
  { id: 'alerts', icon: 'bell', label: t('pg.set_tab_alerts') },
  { id: 'cloudflare', icon: 'cloud', label: t('pg.set_tab_cloudflare') },
])
const tab = computed(() => (tabs.value.some((x) => x.id === route.query.tab) ? route.query.tab : 'panel'))
const setTab = (id) => router.replace({ query: { ...route.query, tab: id === 'panel' ? undefined : id } })
const power = ref('') // '', 'panel' or 'reboot': the dialog shown
usePageMeta(() => ({ sub: data.value ? `${BRAND.name} ${data.value.version}` : '' }))

onMounted(async () => {
  await load()
  resume(['panel_certificate', 'update'])
})
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('settings.title') }}</h1></div>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />

    <nav class="ptabs" role="tablist" :aria-label="t('settings.title')">
      <button v-for="x in tabs" :key="x.id" type="button" role="tab" :aria-selected="tab === x.id" @click="setTab(x.id)">
        <Icon :name="x.icon" :size="16" />{{ x.label }}
      </button>
    </nav>

    <span v-if="!data && !error" class="skel" style="height: 240px; border-radius: 14px"></span>

    <template v-if="data">
      <!-- panel: version, updates, power -->
      <template v-if="tab === 'panel'">
        <section class="svc-head">
          <BrandMark :size="48" />
          <div class="svc-text">
            <h2 class="svc-title">{{ BRAND.name }} <span class="muted" style="font-weight: 600">{{ data.version }}</span></h2>
            <div class="svc-meta">
              <span><Icon name="server" :size="14" />{{ server.info?.hostname }}</span>
              <span v-if="server.info?.os"><Icon name="monitor" :size="14" />{{ server.info.os }}</span>
              <span><Icon name="link" :size="14" />{{ t('pg.set_port', { port: data.port }) }}</span>
            </div>
          </div>
          <div class="svc-actions">
            <template v-if="update?.available">
              <span class="status-pill warn">{{ t('settings.update_available', { v: update.latest }) }}</span>
              <button class="btn btn-primary" :disabled="jobRunning" @click="applyUpdate"><Icon name="download" :size="15" />{{ t('settings.update_install') }}</button>
            </template>
            <span v-else-if="update" class="status-pill ok">{{ t('settings.up_to_date') }}</span>
            <button v-if="!update?.available" class="btn" :disabled="checking || jobRunning" @click="checkUpdate">
              <span v-if="checking" class="spinner" aria-hidden="true"></span><Icon v-else name="refresh" :size="15" />{{ t('settings.update_check') }}
            </button>
          </div>
        </section>
        <p class="muted small" style="margin: -6px 0 0">{{ t('settings.update_hint') }}</p>

        <section class="panel">
          <div class="panel-head"><h2 class="panel-title"><Icon name="power" :size="17" />{{ t('power.title') }}</h2></div>
          <div class="power-row">
            <span class="sev info"><Icon name="refresh" :size="16" /></span>
            <div class="power-text">
              <div class="alert-row-title">{{ t('power.restart') }}</div>
              <div class="alert-row-sub">{{ t('power.restart_body') }}</div>
            </div>
            <button class="btn" :disabled="jobRunning" @click="power = 'panel'">{{ t('power.restart') }}</button>
          </div>
          <div class="power-row">
            <span class="sev crit"><Icon name="power" :size="16" /></span>
            <div class="power-text">
              <div class="alert-row-title">{{ t('power.reboot') }}</div>
              <div class="alert-row-sub">{{ t('power.reboot_body') }}</div>
            </div>
            <button class="btn btn-danger" :disabled="jobRunning" @click="power = 'reboot'">{{ t('power.reboot') }}</button>
          </div>
          <p v-if="jobRunning" class="muted small panel-foot" style="margin: 0">{{ t('power.busy_hint') }}</p>
        </section>
      </template>

      <!-- the panel's own certificate -->
      <section v-else-if="tab === 'cert'" class="panel">
        <div class="panel-head">
          <h2 class="panel-title"><Icon name="lock" :size="17" />{{ t('settings.cert_title') }}</h2>
          <span class="status-pill" :class="data.cert ? 'ok' : 'warn'">{{ data.cert ? t('drawer.ssl_state_ok') : t('pg.set_self_signed') }}</span>
        </div>
        <div class="panel-body stack">
          <div v-if="data.cert" class="cert-box">
            <Icon name="shieldCheck" :size="20" class="text-ok" />
            <span>{{ t('settings.cert_active', { names: (data.cert.names || []).join(', '), issuer: data.cert.issuer, date: formatDate(data.cert.not_after) }) }}</span>
          </div>
          <p v-else class="muted" style="margin: 0">{{ t('settings.cert_none') }}</p>
          <p v-if="!data.web_ready" class="alert alert-warn" style="margin: 0">
            {{ t('settings.cert_needs_web') }} <RouterLink :to="{ name: 'websites' }">{{ t('nav.websites') }}</RouterLink>
          </p>
          <form v-else class="stack form-narrow" @submit.prevent="getCertificate">
            <label class="field">
              <span>{{ t('settings.domain') }}</span>
              <input v-model="domain" class="input mono" placeholder="panel.example.com" required />
              <small class="muted">{{ t('settings.domain_hint') }}</small>
            </label>
            <div class="row">
              <button class="btn btn-primary" :disabled="jobRunning">{{ data.cert ? t('settings.cert_renew') : t('settings.cert_get') }}</button>
              <button v-if="data.cert" type="button" class="btn btn-danger" @click="removeCertificate">{{ t('websites.delete') }}</button>
            </div>
            <p v-if="data.cert" class="muted small" style="margin: 0">{{ t('settings.cert_url', { url: panelURL() }) }}</p>
          </form>
        </div>
      </section>

      <MailRelayCard v-else-if="tab === 'mail'" />
      <AlertsCard v-else-if="tab === 'alerts'" />
      <CloudflareCard v-else-if="tab === 'cloudflare'" />
    </template>

    <PowerDialog v-if="power" :mode="power" @close="power = ''" />
  </div>
</template>

<style scoped>
.power-row { display: flex; align-items: center; gap: 12px; padding: 14px 18px; border-bottom: 1px solid var(--border); flex-wrap: wrap; }
.power-row:last-of-type { border-bottom: 0; }
.power-text { flex: 1 1 260px; min-width: 0; }
.cert-box { display: flex; align-items: center; gap: 10px; padding: 12px 14px; border-radius: 12px; background: var(--ok-soft); color: var(--text); }
:deep(.card) { margin-bottom: 0; }
</style>
