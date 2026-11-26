<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'

const { t } = useI18n()
const st = ref(null)
const token = ref('')
const error = ref('')
const notice = ref('')
const busy = ref(false)

onMounted(async () => {
  try {
    st.value = await api('GET', 'api/settings/cloudflare')
  } catch (e) {
    error.value = errorMessage(e)
  }
})

async function save(value) {
  error.value = ''
  notice.value = ''
  busy.value = true
  try {
    st.value = await api('PUT', 'api/settings/cloudflare', { token: value })
    token.value = ''
    notice.value = value ? t('cloudflare.saved') : t('cloudflare.removed')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="card">
    <h2>{{ t('cloudflare.title') }}</h2>
    <p class="muted small">{{ t('cloudflare.hint') }}</p>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <template v-if="st">
      <p v-if="st.configured">
        <span class="tag" :class="st.error ? 'tag-crit' : 'tag-ok'">{{ t('cloudflare.configured', { hint: st.hint }) }}</span>
        <span v-if="st.zones.length" class="muted small"> {{ t('cloudflare.zones', { zones: st.zones.join(', ') }) }}</span>
      </p>
      <p v-if="st.error" class="alert alert-warn pre-wrap">{{ st.error }}</p>
      <form class="stack form-narrow" @submit.prevent="save(token)">
        <label class="field">
          <span>{{ st.configured ? t('cloudflare.replace') : t('cloudflare.token') }}</span>
          <input v-model="token" class="input mono" type="password" autocomplete="off" spellcheck="false" required />
          <small class="muted">{{ t('cloudflare.token_hint') }}</small>
        </label>
        <div class="row">
          <button class="btn btn-primary" :disabled="busy">{{ t('cloudflare.save') }}</button>
          <button v-if="st.configured" type="button" class="btn btn-danger" :disabled="busy" @click="save('')">{{ t('cloudflare.remove') }}</button>
        </div>
      </form>
    </template>
  </section>
</template>
