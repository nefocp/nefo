<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'

const { t, te } = useI18n()
const s = ref(null)
const error = ref('')
const notice = ref('')
const results = ref(null)
const busy = ref(false)

onMounted(async () => {
  try {
    s.value = await api('GET', 'api/alerts')
  } catch (e) {
    error.value = errorMessage(e)
  }
})

async function save() {
  error.value = ''
  notice.value = ''
  results.value = null
  busy.value = true
  try {
    const body = { ...s.value }
    for (const k of ['smtp_port', 'cpu_percent', 'mem_percent', 'disk_percent']) body[k] = Number(body[k]) || 0
    await api('PUT', 'api/alerts', body)
    notice.value = t('alerts.saved')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function test() {
  error.value = ''
  busy.value = true
  try {
    results.value = (await api('POST', 'api/alerts/test', {})).results
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

// Each channel answers { ok } or { error, detail }, error being a code
// translated like the API's own errors.
const channelName = (ch) => (ch === 'email' ? t('alerts.email') : ch === 'telegram' ? 'Telegram' : ch)
function resultText(r) {
  if (r.ok) return t('alerts.sent')
  const key = `errors.${r.error}`
  return te(key) ? t(key, { detail: r.detail || '' }) : r.detail || r.error
}
</script>

<template>
  <section class="card">
    <h2>{{ t('alerts.title') }}</h2>
    <p class="muted small">{{ t('alerts.hint') }}</p>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <div v-if="results" class="alert" :class="Object.values(results).every((r) => r.ok) ? 'alert-ok' : 'alert-warn'">
      <div v-if="!Object.keys(results).length">{{ t('alerts.no_channel') }}</div>
      <div v-for="(r, ch) in results" :key="ch">{{ channelName(ch) }}: {{ resultText(r) }}</div>
    </div>
    <form v-if="s" class="stack form-narrow" @submit.prevent="save">
      <h3 class="subhead">Telegram</h3>
      <div class="row">
        <label class="field grow">
          <span>{{ t('alerts.bot_token') }}</span>
          <input v-model="s.telegram_token" class="input mono" autocomplete="off" placeholder="123456789:AA…" />
        </label>
        <label class="field grow">
          <span>{{ t('alerts.chat_id') }}</span>
          <input v-model="s.telegram_chat" class="input mono" placeholder="123456789" />
        </label>
      </div>
      <small class="muted">{{ t('alerts.telegram_hint') }}</small>

      <h3 class="subhead">{{ t('alerts.email') }}</h3>
      <div class="row">
        <label class="field grow">
          <span>{{ t('alerts.smtp_host') }}</span>
          <input v-model="s.smtp_host" class="input mono" placeholder="smtp.example.com" />
        </label>
        <label class="field">
          <span>{{ t('alerts.smtp_port') }}</span>
          <input v-model="s.smtp_port" class="input input-auto" type="number" min="1" max="65535" />
        </label>
      </div>
      <div class="row">
        <label class="field grow">
          <span>{{ t('alerts.smtp_user') }}</span>
          <input v-model="s.smtp_user" class="input" autocomplete="off" />
        </label>
        <label class="field grow">
          <span>{{ t('alerts.smtp_password') }}</span>
          <input v-model="s.smtp_password" class="input" type="password" autocomplete="new-password" />
        </label>
      </div>
      <div class="row">
        <label class="field grow">
          <span>{{ t('alerts.from') }}</span>
          <input v-model="s.email_from" class="input" type="email" />
        </label>
        <label class="field grow">
          <span>{{ t('alerts.to') }}</span>
          <input v-model="s.email_to" class="input" type="email" />
        </label>
      </div>

      <h3 class="subhead">{{ t('alerts.when') }}</h3>
      <div class="row">
        <label class="field grow">
          <span>{{ t('alerts.cpu') }}</span>
          <input v-model="s.cpu_percent" class="input" type="number" min="0" max="100" />
        </label>
        <label class="field grow">
          <span>{{ t('alerts.mem') }}</span>
          <input v-model="s.mem_percent" class="input" type="number" min="0" max="100" />
        </label>
        <label class="field grow">
          <span>{{ t('alerts.disk') }}</span>
          <input v-model="s.disk_percent" class="input" type="number" min="0" max="100" />
        </label>
      </div>
      <small class="muted">{{ t('alerts.zero_off') }}</small>
      <label class="check"><input v-model="s.services" type="checkbox" /><span>{{ t('alerts.services') }}</span></label>
      <label class="check"><input v-model="s.events" type="checkbox" /><span>{{ t('alerts.events') }}</span></label>
      <div class="row">
        <button class="btn btn-primary" :disabled="busy">{{ t('files.save') }}</button>
        <button type="button" class="btn" :disabled="busy" @click="test">{{ t('alerts.test') }}</button>
      </div>
    </form>
  </section>
</template>
