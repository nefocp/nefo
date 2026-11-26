<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { toast } from '../toast.js'

// SiteHTTPS holds the HTTPS settings beyond the certificate, for the SSL tab
// of the site drawer: part "hsts" is the HSTS switch (it needs HTTPS and the
// redirect), part "off" turns HTTPS off again. Saved sites go up with
// "saved".
const props = defineProps({
  site: { type: Object, required: true },
  part: { type: String, required: true },
  busy: { type: Boolean, default: false },
})
const emit = defineEmits(['saved'])
const { t } = useI18n()
const saving = ref(false)

async function setHSTS(enabled, subdomains) {
  const d = props.site.domain
  if (enabled && !props.site.hsts && !window.confirm(t('sitehttps.hsts_confirm', { domain: d }))) return false
  if (enabled && subdomains && !props.site.hsts_subdomains && !window.confirm(t('sitehttps.hsts_subdomains_confirm', { domain: d }))) return false
  saving.value = true
  try {
    emit('saved', await api('PUT', `api/sites/${props.site.id}/hsts`, { enabled, include_subdomains: subdomains }))
    return true
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
    return false
  } finally {
    saving.value = false
  }
}

// The checkbox goes back if the change is cancelled or refused.
async function setSubdomains(ev) {
  const box = ev.target
  if (!(await setHSTS(true, box.checked))) box.checked = !!props.site.hsts_subdomains
}

async function turnOff() {
  const key = props.site.hsts ? 'sitehttps.off_confirm_hsts' : 'sitehttps.off_confirm'
  if (!window.confirm(t(key, { domain: props.site.domain }))) return
  saving.value = true
  try {
    emit('saved', await api('DELETE', `api/sites/${props.site.id}/ssl`))
  } catch (e) {
    toast(errorMessage(e), 'crit', 7000)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div v-if="part === 'hsts'" class="opt hsts">
    <div class="opt-text">
      <div class="opt-title">{{ t('sitehttps.hsts') }}</div>
      <div class="opt-hint">{{ site.ssl && site.force_https ? t('sitehttps.hsts_hint') : t('sitehttps.hsts_needs_redirect') }}</div>
      <template v-if="site.hsts">
        <label class="check hsts-sub">
          <input type="checkbox" :checked="!!site.hsts_subdomains" :disabled="saving || busy" @change="setSubdomains" />
          <span>{{ t('sitehttps.hsts_subdomains') }}</span>
        </label>
        <div class="small text-warn">{{ t('sitehttps.hsts_on_warn') }}</div>
      </template>
    </div>
    <button
      type="button"
      class="switch"
      role="switch"
      :aria-checked="!!site.hsts"
      :aria-label="t('sitehttps.hsts')"
      :disabled="(!site.hsts && !(site.ssl && site.force_https)) || saving || busy"
      @click="setHSTS(!site.hsts, false)"
    ></button>
  </div>

  <section v-else-if="part === 'off'" class="card">
    <div class="card-head off-head">
      <div>
        <h3 class="off-title">{{ t('sitehttps.off_title') }}</h3>
        <p class="muted small off-hint">{{ t('sitehttps.off_hint') }}</p>
      </div>
      <button type="button" class="btn btn-danger" :disabled="saving || busy" @click="turnOff">{{ t('sitehttps.off_button') }}</button>
    </div>
  </section>
</template>

<style scoped>
.hsts { margin-bottom: 16px; }
.hsts-sub { margin: 8px 0 4px; }
.off-head { margin: 0; }
.off-title { margin: 0 0 2px; font-size: 14px; }
.off-hint { margin: 0; }
</style>
