<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'

// hideStatus drops the first line (the certificate now) when the page
// around already shows it, as the site drawer does.
const props = defineProps({ site: { type: Object, required: true }, busy: { type: Boolean, default: false }, hideStatus: { type: Boolean, default: false } })
const emit = defineEmits(['issue', 'saved', 'close'])
const { t, locale } = useI18n()

const wildcard = computed(() => (props.site.aliases || []).some((a) => a.startsWith('*.')))
const dns = ref(props.site.ssl_dns || wildcard.value)
const cloudflare = ref(null)
const cert = ref('')
const key = ref('')
const error = ref('')
const saving = ref(false)
const warnings = ref([])

onMounted(async () => {
  try {
    cloudflare.value = await api('GET', 'api/settings/cloudflare')
  } catch {
    cloudflare.value = { configured: false }
  }
})

const fmt = (v) => new Date(v).toLocaleDateString(locale.value, { year: 'numeric', month: 'short', day: 'numeric' })

function issue() {
  emit('issue', { dns: dns.value })
  emit('close')
}

async function install() {
  error.value = ''
  warnings.value = []
  saving.value = true
  try {
    const res = await api('POST', `api/sites/${props.site.id}/certificate`, { cert: cert.value, key: key.value })
    emit('saved', res.site)
    cert.value = key.value = ''
    if (res.warnings?.length) warnings.value = res.warnings
    else emit('close')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

function readFile(ev, target) {
  const f = ev.target.files?.[0]
  if (!f) return
  f.text().then((txt) => (target === 'cert' ? (cert.value = txt) : (key.value = txt)))
}
</script>

<template>
  <div class="stack" :class="{ 'no-status': hideStatus }">
    <template v-if="!hideStatus">
    <p v-if="site.ssl" class="ssl-now">
      <span class="tag tag-ok">{{ site.ssl_source === 'custom' ? t('ssl.source_custom') : t('ssl.source_le') }}</span>
      {{ t('ssl.valid_until', { date: fmt(site.cert_expires) }) }}
    </p>
    <p v-else class="muted">{{ t('ssl.none') }}</p>
    </template>

    <div v-if="warnings.length" class="alert alert-warn stack">
      <strong>{{ t('ssl.installed_with_warnings') }}</strong>
      <span v-for="w in warnings" :key="w.code">{{ t(`ssl.warn_${w.code}`, { detail: w.detail }) }}</span>
    </div>

    <section class="ssl-part">
      <h3>{{ t('ssl.le_title') }}</h3>
      <p class="muted small">{{ t('ssl.le_hint') }}</p>
      <label class="check">
        <input v-model="dns" type="checkbox" :disabled="wildcard" /><span>{{ t('ssl.dns') }}</span>
      </label>
      <small class="muted">{{ wildcard ? t('ssl.dns_wildcard') : t('ssl.dns_hint') }}</small>
      <p v-if="dns && cloudflare && !cloudflare.configured" class="alert alert-warn">
        {{ t('ssl.dns_no_token') }} <RouterLink :to="{ name: 'settings' }" @click="emit('close')">{{ t('nav.settings') }}</RouterLink>
      </p>
      <div class="row">
        <button type="button" class="btn btn-primary" :disabled="busy || (dns && !cloudflare?.configured)" @click="issue">
          {{ site.ssl && site.ssl_source !== 'custom' ? t('websites.reissue') : t('ssl.le_get') }}
        </button>
      </div>
    </section>

    <form class="ssl-part stack" @submit.prevent="install">
      <h3>{{ t('ssl.own_title') }}</h3>
      <p class="muted small">{{ t('ssl.own_hint') }}</p>
      <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
      <label class="field">
        <span>{{ t('ssl.cert') }}</span>
        <textarea v-model="cert" class="input mono" rows="4" spellcheck="false" placeholder="-----BEGIN CERTIFICATE-----" required></textarea>
        <input type="file" accept=".pem,.crt,.cer,.txt" class="small" @change="readFile($event, 'cert')" />
      </label>
      <label class="field">
        <span>{{ t('ssl.key') }}</span>
        <textarea v-model="key" class="input mono" rows="4" spellcheck="false" placeholder="-----BEGIN PRIVATE KEY-----" required></textarea>
        <input type="file" accept=".pem,.key,.txt" class="small" @change="readFile($event, 'key')" />
      </label>
      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.close') }}</button>
        <button class="btn btn-primary" :disabled="saving || busy">{{ t('ssl.own_install') }}</button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.ssl-part { border-top: 1px solid var(--border); padding-top: 12px; }
.ssl-part h3 { margin: 0 0 4px; font-size: 14px; }
.ssl-now { margin: 0; display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.no-status > .ssl-part:first-of-type { border-top: 0; padding-top: 0; }
</style>
