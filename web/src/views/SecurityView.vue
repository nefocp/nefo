<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import QRCode from 'qrcode'
import { api } from '../api.js'
import { auth } from '../auth.js'
import { errorMessage } from '../errors.js'
import { formatDateTime, relativeTime } from '../format.js'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../page.js'

const { t, te, locale } = useI18n()
const router = useRouter()

// ---- two-factor authentication ----
const setup = ref(null) // { secret, uri, qr }
const totpCode = ref('')
const totpPassword = ref('')
const totpBusy = ref(false)
const totpError = ref('')
const totpNotice = ref('')
const copied = ref(false)

const groupedSecret = computed(() => setup.value?.secret?.match(/.{1,4}/g)?.join(' ') ?? '')

function resetTotpMessages() {
  totpError.value = ''
  totpNotice.value = ''
}

async function startSetup() {
  resetTotpMessages()
  totpBusy.value = true
  try {
    const res = await api('POST', 'api/auth/2fa/setup', {})
    const qr = await QRCode.toDataURL(res.uri, { margin: 1, width: 200 })
    setup.value = { ...res, qr }
  } catch (e) {
    totpError.value = errorMessage(e)
  } finally {
    totpBusy.value = false
  }
}

async function enableTotp() {
  resetTotpMessages()
  totpBusy.value = true
  try {
    auth.user = await api('POST', 'api/auth/2fa/enable', { code: totpCode.value })
    setup.value = null
    totpCode.value = ''
    totpNotice.value = t('security.twofa_enabled')
    loadAudit()
  } catch (e) {
    totpError.value = errorMessage(e)
  } finally {
    totpBusy.value = false
  }
}

async function disableTotp() {
  resetTotpMessages()
  totpBusy.value = true
  try {
    auth.user = await api('POST', 'api/auth/2fa/disable', { password: totpPassword.value, code: totpCode.value })
    totpCode.value = ''
    totpPassword.value = ''
    totpNotice.value = t('security.twofa_disabled')
    loadAudit()
  } catch (e) {
    totpError.value = errorMessage(e)
  } finally {
    totpBusy.value = false
  }
}

async function copySecret() {
  try {
    await navigator.clipboard.writeText(setup.value.secret)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch {
    // clipboard blocked: the key is still visible to copy by hand
  }
}

// ---- password ----
const pw = ref({ current: '', next: '', confirm: '' })
const pwBusy = ref(false)
const pwError = ref('')

async function changePassword() {
  pwError.value = ''
  if (pw.value.next !== pw.value.confirm) {
    pwError.value = t('errors.password_mismatch')
    return
  }
  pwBusy.value = true
  try {
    await api('POST', 'api/auth/password', { current_password: pw.value.current, new_password: pw.value.next })
    auth.user = null
    router.replace({ name: 'login', query: { changed: '1' } })
  } catch (e) {
    pwError.value = errorMessage(e)
  } finally {
    pwBusy.value = false
  }
}

// ---- activity ----
const entries = ref([])

async function loadAudit() {
  try {
    entries.value = (await api('GET', 'api/audit')).entries || []
  } catch {
    // shown as empty
  }
}

const actionLabel = (a) => (te(`audit.${a}`) ? t(`audit.${a}`) : a)
const failedAction = (a) => a.endsWith('_failed')
const q = ref('')
const onlyFailed = ref(false)
const shownEntries = computed(() => {
  const f = q.value.trim().toLowerCase()
  return entries.value.filter((e) => (!onlyFailed.value || failedAction(e.action)) && (!f || `${actionLabel(e.action)} ${e.username} ${e.ip} ${e.detail}`.toLowerCase().includes(f)))
})
const lastLogin = computed(() => entries.value.find((e) => e.action === 'login'))
const failedLogins = computed(() => entries.value.filter((e) => e.action === 'login_failed').length)
usePageMeta(() => ({ sub: t('security.subtitle') }))

onMounted(loadAudit)
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('security.title') }}</h1></div>

    <section class="svc-head">
      <span class="acc-avatar" aria-hidden="true">{{ auth.user?.username?.[0] }}</span>
      <div class="svc-text">
        <h2 class="svc-title">
          {{ auth.user?.username }}
          <span class="status-pill" :class="auth.user?.totp_enabled ? 'ok' : 'warn'">{{ auth.user?.totp_enabled ? t('account.2fa_on') : t('account.2fa_off') }}</span>
        </h2>
        <div class="svc-meta">
          <span v-if="lastLogin"><Icon name="clock" :size="14" />{{ t('pg.acc_last_login', { when: relativeTime(lastLogin.time, locale), ip: lastLogin.ip }) }}</span>
          <span v-if="failedLogins"><Icon name="alert" :size="14" />{{ t('pg.acc_failed', { n: failedLogins }, failedLogins) }}</span>
        </div>
      </div>
    </section>

    <div class="acc-grid">
      <section class="panel">
        <div class="panel-head">
          <h2 class="panel-title"><Icon name="shieldCheck" :size="17" />{{ t('security.twofa_title') }}</h2>
          <span class="status-pill" :class="auth.user?.totp_enabled ? 'ok' : 'warn'">{{ auth.user?.totp_enabled ? t('drawer.on') : t('drawer.off') }}</span>
        </div>
        <div class="panel-body stack">
          <p v-if="totpNotice" class="alert alert-ok" style="margin: 0">{{ totpNotice }}</p>
          <p v-if="totpError" class="alert alert-crit" role="alert" style="margin: 0">{{ totpError }}</p>

          <template v-if="auth.user?.totp_enabled">
            <p class="muted" style="margin: 0">{{ t('security.twofa_on') }}</p>
            <form class="stack" @submit.prevent="disableTotp">
              <p class="muted small" style="margin: 0">{{ t('security.twofa_disable_hint') }}</p>
              <label class="field">
                <span>{{ t('security.password') }}</span>
                <input v-model="totpPassword" class="input" type="password" autocomplete="current-password" required />
              </label>
              <label class="field">
                <span>{{ t('security.code') }}</span>
                <input v-model="totpCode" class="input input-code" inputmode="numeric" autocomplete="one-time-code" maxlength="7" required />
              </label>
              <div><button class="btn btn-danger" :disabled="totpBusy">{{ t('security.twofa_disable') }}</button></div>
            </form>
          </template>

          <template v-else-if="!setup">
            <div class="acc-callout">
              <span class="sev warn"><Icon name="lock" :size="16" /></span>
              <div>
                <div class="alert-row-title">{{ t('attention.no_2fa') }}</div>
                <div class="alert-row-sub">{{ t('security.twofa_off') }}</div>
              </div>
            </div>
            <div><button class="btn btn-primary" :disabled="totpBusy" @click="startSetup"><Icon name="shieldCheck" :size="15" />{{ t('security.twofa_setup') }}</button></div>
          </template>

          <form v-else class="stack" @submit.prevent="enableTotp">
            <p style="margin: 0">{{ t('security.twofa_scan') }}</p>
            <img class="qr" :src="setup.qr" width="200" height="200" alt="" />
            <div class="field">
              <span>{{ t('security.twofa_key') }}</span>
              <div class="row">
                <code class="secret">{{ groupedSecret }}</code>
                <button type="button" class="btn btn-sm" @click="copySecret">{{ copied ? t('common.copied') : t('common.copy') }}</button>
              </div>
            </div>
            <label class="field">
              <span>{{ t('security.twofa_confirm') }}</span>
              <input v-model="totpCode" class="input input-code" inputmode="numeric" autocomplete="one-time-code" maxlength="7" required />
            </label>
            <div class="row">
              <button class="btn btn-primary" :disabled="totpBusy">{{ t('security.twofa_enable') }}</button>
              <button type="button" class="btn btn-ghost" @click="setup = null">{{ t('common.cancel') }}</button>
            </div>
          </form>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h2 class="panel-title"><Icon name="key" :size="17" />{{ t('security.password_title') }}</h2>
        </div>
        <div class="panel-body">
          <p v-if="pwError" class="alert alert-crit" role="alert">{{ pwError }}</p>
          <form class="stack" @submit.prevent="changePassword">
            <input class="sr-only" :value="auth.user?.username" autocomplete="username" tabindex="-1" aria-hidden="true" readonly />
            <label class="field">
              <span>{{ t('security.current_password') }}</span>
              <input v-model="pw.current" class="input" type="password" autocomplete="current-password" required />
            </label>
            <label class="field">
              <span>{{ t('security.new_password') }}</span>
              <input v-model="pw.next" class="input" type="password" autocomplete="new-password" minlength="12" maxlength="72" required />
            </label>
            <label class="field">
              <span>{{ t('security.confirm_password') }}</span>
              <input v-model="pw.confirm" class="input" type="password" autocomplete="new-password" minlength="12" maxlength="72" required />
              <small class="muted">{{ t('security.password_hint') }}</small>
            </label>
            <div><button class="btn btn-primary" :disabled="pwBusy">{{ t('security.change_password') }}</button></div>
          </form>
        </div>
      </section>
    </div>

    <section class="panel">
      <div class="panel-head">
        <h2 class="panel-title"><Icon name="logs" :size="17" />{{ t('security.activity_title') }}</h2>
      </div>
      <div class="panel-tools">
        <div class="tabs" role="group" :aria-label="t('pg.acc_show')">
          <button type="button" :aria-pressed="!onlyFailed" @click="onlyFailed = false">{{ t('ws.all') }}</button>
          <button type="button" :aria-pressed="onlyFailed" @click="onlyFailed = true">{{ t('pg.acc_only_failed') }}</button>
        </div>
        <span class="tools-gap"></span>
        <label class="search-field">
          <Icon name="search" :size="15" />
          <input v-model="q" type="search" :placeholder="t('pg.acc_filter')" :aria-label="t('pg.acc_filter')" />
        </label>
      </div>
      <div v-if="!entries.length" class="empty"><p class="empty-lead">{{ t('security.no_activity') }}</p></div>
      <p v-else-if="!shownEntries.length" class="muted panel-body" style="margin: 0">{{ t('pg.no_match') }}</p>
      <div v-else class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>{{ t('security.col_time') }}</th>
              <th>{{ t('security.col_action') }}</th>
              <th>{{ t('security.col_user') }}</th>
              <th>{{ t('security.col_ip') }}</th>
              <th>{{ t('security.col_detail') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="e in shownEntries" :key="e.id">
              <td class="nowrap">{{ relativeTime(e.time, locale) }}<div class="name-ico-sub">{{ formatDateTime(e.time, locale) }}</div></td>
              <td>
                <span class="act-cell">
                  <span class="act-dot" :class="failedAction(e.action) ? 'crit' : 'ok'" aria-hidden="true"></span>
                  <span :class="{ 'text-crit': failedAction(e.action) }">{{ actionLabel(e.action) }}</span>
                </span>
              </td>
              <td>{{ e.username }}</td>
              <td class="mono small">{{ e.ip }}</td>
              <td class="small detail-cell">{{ e.detail }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<style scoped>
.tools-gap { flex: 1; }
.acc-avatar {
  width: 52px; height: 52px; border-radius: 50%; display: grid; place-items: center; flex: none;
  background: var(--accent); color: var(--on-accent); font-size: 22px; font-weight: 800; text-transform: uppercase;
}
.acc-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap: 16px; align-items: start; }
.acc-callout { display: flex; gap: 12px; align-items: flex-start; padding: 12px; border-radius: 12px; background: var(--warn-soft); }
.act-cell { display: inline-flex; align-items: center; gap: 8px; }
.act-dot { width: 8px; height: 8px; border-radius: 50%; flex: none; }
.act-dot.ok { background: var(--ok-g); }
.act-dot.crit { background: var(--crit-g); }
.detail-cell { overflow-wrap: anywhere; max-width: 420px; }
@media (max-width: 640px) {
  .acc-grid { grid-template-columns: minmax(0, 1fr); }
}
</style>
