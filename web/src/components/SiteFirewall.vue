<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { hasRateLimit } from '../webservers.js'

// server is the managed web server: Apache and OpenLiteSpeed have no rate
// limit.
// section shows one part (the site drawer has a tab for each): "waf" (the
// firewall and its events) or "block" (blocked addresses and the rate
// limit). Empty shows all. Saving always sends every field.
const props = defineProps({ site: { type: Object, required: true }, server: { type: String, default: 'nginx' }, section: { type: String, default: '' } })
const show = (part) => !props.section || props.section === part
const emit = defineEmits(['saved', 'close'])
const { t } = useI18n()

const fw = ref(null)
const waf = ref('')
const rate = ref(0)
const ips = ref('')
const error = ref('')
const busy = ref(false)

function apply(res) {
  fw.value = res
  waf.value = res.waf
  rate.value = res.rate_limit
  ips.value = (res.blocked_ips || []).join('\n')
}

onMounted(async () => {
  try {
    apply(await api('GET', `api/sites/${props.site.id}/firewall`))
  } catch (e) {
    error.value = errorMessage(e)
  }
})

async function save() {
  error.value = ''
  busy.value = true
  try {
    const res = await api('PUT', `api/sites/${props.site.id}/firewall`, {
      waf: waf.value,
      rate_limit: hasRateLimit(props.server) ? Number(rate.value) || 0 : 0,
      blocked_ips: ips.value.split(/[\s,]+/).filter(Boolean),
    })
    apply(res)
    emit('saved', { ...props.site, waf: res.waf, rate_limit: res.rate_limit, blocked_ips: (res.blocked_ips || []).length })
    emit('close')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="!fw && !error" class="muted">…</div>
  <form v-else class="stack" @submit.prevent="save">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <template v-if="fw">
      <fieldset v-if="show('waf')" class="stack">
        <legend class="field-label">{{ t('firewall.waf') }}</legend>
        <p v-if="!fw.waf_installed" class="alert alert-warn">
          {{ t('firewall.waf_missing') }} <RouterLink :to="{ name: 'software' }" @click="emit('close')">{{ t('nav.software') }}</RouterLink>
        </p>
        <label class="check"><input v-model="waf" type="radio" value="" /><span>{{ t('firewall.waf_off') }}</span></label>
        <label class="check"><input v-model="waf" type="radio" value="detect" :disabled="!fw.waf_installed" /><span>{{ t('firewall.waf_detect') }}</span></label>
        <label class="check"><input v-model="waf" type="radio" value="block" :disabled="!fw.waf_installed" /><span>{{ t('firewall.waf_block') }}</span></label>
        <small class="muted">{{ t('firewall.waf_hint') }}</small>
      </fieldset>

      <label v-if="show('block') && hasRateLimit(server)" class="field">
        <span>{{ t('firewall.rate') }}</span>
        <input v-model.number="rate" class="input" type="number" min="0" max="1000" style="max-width: 160px" />
        <small class="muted">{{ t('firewall.rate_hint') }}</small>
      </label>

      <label v-if="show('block')" class="field">
        <span>{{ t('firewall.blocked') }}</span>
        <textarea v-model="ips" class="input mono" rows="3" spellcheck="false" placeholder="203.0.113.7&#10;198.51.100.0/24"></textarea>
        <small class="muted">{{ t('firewall.blocked_hint') }}</small>
      </label>

      <details v-if="show('waf') && fw.waf_installed" :open="(fw.events || []).length > 0">
        <summary>{{ t('firewall.events', { n: (fw.events || []).length }) }}</summary>
        <p v-if="!(fw.events || []).length" class="muted small">{{ t('firewall.no_events') }}</p>
        <div v-else class="events-wrap">
          <table class="table small">
            <thead><tr><th>{{ t('firewall.ev_time') }}</th><th>IP</th><th>{{ t('firewall.ev_request') }}</th><th>{{ t('firewall.ev_rule') }}</th><th></th></tr></thead>
            <tbody>
              <tr v-for="(e, i) in fw.events || []" :key="i">
                <td class="nowrap">{{ e.time }}</td>
                <td class="mono">{{ e.ip }}</td>
                <td class="mono req">{{ e.method }} {{ e.uri }}</td>
                <td><div v-for="r in e.rules || []" :key="r.id">{{ r.message }} <span class="muted">#{{ r.id }}</span></div></td>
                <td><span class="tag" :class="e.status === 403 ? 'tag-crit' : 'tag-warn'">{{ e.status === 403 ? t('firewall.blocked_tag') : t('firewall.logged_tag') }}</span></td>
              </tr>
            </tbody>
          </table>
        </div>
      </details>
    </template>
    <div class="modal-actions">
      <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button v-if="fw" class="btn btn-primary" :disabled="busy">{{ t('files.save') }}</button>
    </div>
  </form>
</template>

<style scoped>
fieldset { border: 0; padding: 0; margin: 0; }
.field-label { font-size: 13px; font-weight: 600; margin-bottom: 4px; }
.req { word-break: break-all; max-width: 280px; }
.nowrap { white-space: nowrap; }
.events-wrap { overflow-x: auto; }
</style>
