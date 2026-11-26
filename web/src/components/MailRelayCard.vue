<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { useJob } from '../composables/useJob.js'
import JobLog from './JobLog.vue'

const { t, te } = useI18n()
const st = ref(null)
const error = ref('')
const notice = ref('')
const testTo = ref('')
const testResult = ref(null) // { status, detail, result }: status is Postfix's (sent, deferred, bounced…)
const testing = ref(false)
const form = ref({ preset: 'gmail', host: 'smtp.gmail.com', port: 587, security: 'starttls', username: '', password: '', from: '' })
const { job, running, watch: watchJob, clear: clearJob } = useJob(load)

// Common providers; the user name hint says what they expect.
const presets = {
  gmail: { host: 'smtp.gmail.com', port: 587, security: 'starttls', hint: 'mail_hint_gmail' },
  outlook: { host: 'smtp.office365.com', port: 587, security: 'starttls', hint: 'mail_hint_outlook' },
  sendgrid: { host: 'smtp.sendgrid.net', port: 587, security: 'starttls', hint: 'mail_hint_sendgrid', user: 'apikey' },
  mailgun: { host: 'smtp.mailgun.org', port: 587, security: 'starttls', hint: 'mail_hint_mailgun' },
  ses: { host: 'email-smtp.us-east-1.amazonaws.com', port: 587, security: 'starttls', hint: 'mail_hint_ses' },
  brevo: { host: 'smtp-relay.brevo.com', port: 587, security: 'starttls', hint: 'mail_hint_brevo' },
  custom: { hint: 'mail_hint_custom' },
}

function applyPreset() {
  const p = presets[form.value.preset]
  if (p.host) Object.assign(form.value, { host: p.host, port: p.port, security: p.security })
  if (p.user && !form.value.username) form.value.username = p.user
}

async function load() {
  try {
    st.value = await api('GET', 'api/mail-relay')
    const s = st.value.settings
    if (s) {
      const preset = Object.keys(presets).find((k) => presets[k].host === s.host) || 'custom'
      form.value = { preset, host: s.host, port: s.port, security: s.security, username: s.username, password: '', from: s.from }
    }
  } catch (e) {
    error.value = errorMessage(e)
  }
}
onMounted(load)

async function save() {
  error.value = notice.value = ''
  try {
    const f = form.value
    watchJob(await api('PUT', 'api/mail-relay', { host: f.host, port: Number(f.port), security: f.security, username: f.username, password: f.password, from: f.from }))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function test() {
  error.value = ''
  testResult.value = null
  testing.value = true
  try {
    testResult.value = await api('POST', 'api/mail-relay/test', { to: testTo.value })
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    testing.value = false
  }
}

// The provider's own answer stays as it is; only Postfix's status is translated.
function testText(r) {
  const key = `settings.mail_result_${r.status}`
  return te(key) ? t(key, { detail: r.detail || '' }) : r.result
}

async function remove() {
  if (!window.confirm(t('settings.mail_remove_confirm'))) return
  try {
    watchJob(await api('DELETE', 'api/mail-relay'))
  } catch (e) {
    error.value = errorMessage(e)
  }
}
</script>

<template>
  <section class="card">
    <h2>{{ t('settings.mail_title') }}</h2>
    <p class="muted small">{{ t('settings.mail_hint') }}</p>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />
    <template v-if="st">
      <p v-if="st.configured" class="row">
        <span class="tag" :class="st.running ? 'tag-ok' : 'tag-crit'">{{ st.running ? t('settings.mail_on', { host: st.settings.host }) : t('settings.mail_stopped') }}</span>
        <span v-if="st.queued" class="tag tag-warn">{{ t('settings.mail_queued', { n: st.queued }, st.queued) }}</span>
      </p>
      <p v-if="st.last_error" class="alert alert-warn pre-wrap small">{{ t('settings.mail_last_error') }} {{ st.last_error }}</p>

      <form class="stack form-narrow" @submit.prevent="save">
        <label class="field">
          <span>{{ t('settings.mail_provider') }}</span>
          <select v-model="form.preset" class="input" @change="applyPreset">
            <option value="gmail">Gmail / Google Workspace</option>
            <option value="outlook">Microsoft 365 / Outlook</option>
            <option value="sendgrid">SendGrid</option>
            <option value="mailgun">Mailgun</option>
            <option value="ses">Amazon SES</option>
            <option value="brevo">Brevo</option>
            <option value="custom">{{ t('settings.mail_custom') }}</option>
          </select>
          <small class="muted">{{ t(`settings.${presets[form.preset].hint}`) }}</small>
        </label>
        <div class="row">
          <label class="field grow"><span>{{ t('settings.mail_host') }}</span><input v-model="form.host" class="input mono" required /></label>
          <label class="field" style="max-width: 110px"><span>{{ t('settings.mail_port') }}</span><input v-model.number="form.port" class="input" type="number" min="1" max="65535" required /></label>
          <label class="field" style="max-width: 160px">
            <span>{{ t('settings.mail_security') }}</span>
            <select v-model="form.security" class="input">
              <option value="starttls">STARTTLS</option>
              <option value="tls">SSL/TLS</option>
              <option value="none">{{ t('settings.mail_none') }}</option>
            </select>
          </label>
        </div>
        <div class="row">
          <label class="field grow"><span>{{ t('settings.mail_user') }}</span><input v-model="form.username" class="input" autocomplete="off" /></label>
          <label class="field grow">
            <span>{{ t('settings.mail_password') }}</span>
            <input v-model="form.password" class="input" type="password" autocomplete="new-password" :placeholder="st.configured ? t('settings.mail_password_keep') : ''" />
          </label>
        </div>
        <label class="field">
          <span>{{ t('settings.mail_from') }}</span>
          <input v-model="form.from" class="input" type="email" placeholder="noreply@example.com" />
          <small class="muted">{{ t('settings.mail_from_hint') }}</small>
        </label>
        <div class="row">
          <button class="btn btn-primary" :disabled="running">{{ st.installed ? t('settings.mail_save') : t('settings.mail_install') }}</button>
          <button v-if="st.installed" type="button" class="btn btn-danger" :disabled="running" @click="remove">{{ t('settings.mail_remove') }}</button>
        </div>
      </form>

      <form v-if="st.configured" class="row" @submit.prevent="test">
        <input v-model="testTo" class="input" type="email" required :placeholder="t('settings.mail_test_to')" style="max-width: 280px" />
        <button class="btn" :disabled="testing">{{ testing ? t('settings.mail_testing') : t('settings.mail_test') }}</button>
      </form>
      <p v-if="testResult" class="alert pre-wrap small" :class="testResult.status === 'sent' ? 'alert-ok' : 'alert-warn'">{{ testText(testResult) }}</p>
    </template>
  </section>
</template>
