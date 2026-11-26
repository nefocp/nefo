<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'

// section shows one part (the site drawer has a tab for each): "suspend"
// (with the expiry date), "redirect", "protect", "hotlink" or "404". Empty
// shows all. Saving always sends every field.
const props = defineProps({ site: { type: Object, required: true }, section: { type: String, default: '' } })
const show = (part) => !props.section || props.section === part
const emit = defineEmits(['saved', 'close'])
const { t } = useI18n()

const form = ref({
  disabled: props.site.disabled,
  redirect: !!props.site.redirect_url,
  redirect_url: props.site.redirect_url,
  protect: !!props.site.auth_user,
  auth_user: props.site.auth_user || 'admin',
  auth_password: '',
  custom_404: !!props.site.custom_404,
  hotlink: !!props.site.hotlink,
  hotlink_allow: (props.site.hotlink_allow || []).join('\n'),
  // The site works until the end of the chosen day; the API stores the next midnight.
  expires: props.site.expires_at ? new Date(new Date(props.site.expires_at).getTime() - 86400000).toISOString().slice(0, 10) : '',
})
const error = ref('')
const busy = ref(false)

async function save() {
  error.value = ''
  busy.value = true
  const f = form.value
  try {
    await api('PUT', `api/sites/${props.site.id}/access`, {
      disabled: f.disabled,
      redirect_url: f.redirect ? f.redirect_url.trim() : '',
      auth_user: f.protect ? f.auth_user.trim() : '',
      auth_password: f.protect ? f.auth_password : '',
    })
    const site = await api('PUT', `api/sites/${props.site.id}/options`, {
      custom_404: f.custom_404,
      hotlink: f.hotlink,
      hotlink_allow: f.hotlink_allow.split(/[\s,]+/).filter(Boolean),
      expires_at: f.expires,
    })
    emit('saved', site)
    emit('close')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="stack" @submit.prevent="save">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <template v-if="show('suspend')">
      <label class="check"><input v-model="form.disabled" type="checkbox" /><span>{{ t('access.disable') }}</span></label>
      <small class="muted">{{ t('access.disable_hint') }}</small>
    </template>

    <template v-if="show('redirect')">
      <p v-if="section && form.disabled" class="alert alert-warn">{{ t('drawer.redirect_suspended') }}</p>
      <label class="check"><input v-model="form.redirect" type="checkbox" :disabled="form.disabled" /><span>{{ t('access.redirect') }}</span></label>
      <label v-if="form.redirect && !form.disabled" class="field">
        <span>{{ t('access.redirect_url') }}</span>
        <input v-model="form.redirect_url" class="input mono" placeholder="https://new-domain.com" required />
        <small class="muted">{{ t('access.redirect_hint') }}</small>
      </label>
    </template>

    <template v-if="show('protect')">
      <label class="check"><input v-model="form.protect" type="checkbox" /><span>{{ t('access.protect') }}</span></label>
      <div v-if="form.protect" class="row">
        <label class="field grow">
          <span>{{ t('access.user') }}</span>
          <input v-model="form.auth_user" class="input" autocomplete="off" maxlength="64" required />
        </label>
        <label class="field grow">
          <span>{{ t('access.password') }}</span>
          <input v-model="form.auth_password" class="input" type="password" autocomplete="new-password" minlength="8" maxlength="72" :required="!site.auth_user" />
        </label>
      </div>
      <small v-if="form.protect" class="muted">{{ site.auth_user ? t('access.password_keep') : t('access.protect_hint') }}</small>
    </template>

    <hr v-if="!section" class="sep" />
    <template v-if="show('404')">
      <label class="check"><input v-model="form.custom_404" type="checkbox" /><span>{{ t('access.custom_404') }}</span></label>
      <small class="muted">{{ t('access.custom_404_hint') }}</small>
    </template>

    <template v-if="show('hotlink')">
      <label class="check"><input v-model="form.hotlink" type="checkbox" /><span>{{ t('access.hotlink') }}</span></label>
      <label v-if="form.hotlink" class="field">
        <span>{{ t('access.hotlink_allow') }}</span>
        <textarea v-model="form.hotlink_allow" class="input mono" rows="2" placeholder="partner.example.com"></textarea>
        <small class="muted">{{ t('access.hotlink_hint') }}</small>
      </label>
    </template>

    <label v-if="show('suspend')" class="field">
      <span>{{ t('access.expires') }}</span>
      <div class="row">
        <input v-model="form.expires" class="input" type="date" style="max-width: 200px" />
        <button v-if="form.expires" type="button" class="btn btn-sm btn-ghost" @click="form.expires = ''">{{ t('access.expires_never') }}</button>
      </div>
      <small class="muted">{{ t('access.expires_hint') }}</small>
    </label>

    <div class="modal-actions">
      <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button class="btn btn-primary" :disabled="busy">{{ t('files.save') }}</button>
    </div>
  </form>
</template>

<style scoped>
.sep { border: 0; border-top: 1px solid var(--border); margin: 6px 0; }
</style>
