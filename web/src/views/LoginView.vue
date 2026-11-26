<script setup>
import { ref, nextTick, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { login } from '../auth.js'
import { ApiError } from '../api.js'
import { errorMessage } from '../errors.js'
import { BRAND, setDocumentTitle } from '../brand.js'
import { isDark, toggleTheme } from '../theme.js'
import LanguageSelect from '../components/LanguageSelect.vue'
import BrandMark from '../components/BrandMark.vue'
import Icon from '../components/Icon.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const username = ref('')
const password = ref('')
const code = ref('')
const needCode = ref(false)
const busy = ref(false)
const error = ref('')
const codeInput = ref(null)
const showPw = ref(false)

onMounted(() => setDocumentTitle(t('login.title')))

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await login(username.value, password.value, needCode.value ? code.value : '')
    const next = route.query.next
    router.replace(typeof next === 'string' && next.startsWith('/') ? next : '/')
  } catch (e) {
    if (e instanceof ApiError && e.code === 'totp_required') {
      needCode.value = true
      await nextTick()
      codeInput.value?.focus()
    } else {
      error.value = errorMessage(e)
      if (e instanceof ApiError && e.code === 'invalid_totp') code.value = ''
    }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <aside class="auth-hero">
      <div class="auth-brand"><BrandMark /><span class="brand-name">{{ BRAND.name }}</span></div>
      <div>
        <h2>{{ t('login.hero_title') }}</h2>
        <p>{{ t('login.hero_body') }}</p>
        <ul class="auth-points">
          <li><span class="auth-point-ico"><Icon name="globe" :size="17" /></span>{{ t('login.point_sites') }}</li>
          <li><span class="auth-point-ico"><Icon name="shieldCheck" :size="17" /></span>{{ t('login.point_secure') }}</li>
          <li><span class="auth-point-ico"><Icon name="archive" :size="17" /></span>{{ t('login.point_backups') }}</li>
        </ul>
      </div>
      <p class="auth-foot">{{ t('login.hero_foot') }}</p>
    </aside>
    <main class="auth-side">
      <div class="auth-tools">
        <button type="button" class="icon-btn" :aria-label="isDark ? t('nav.theme_light') : t('nav.theme_dark')" @click="toggleTheme">
          <Icon :name="isDark ? 'sun' : 'moon'" :size="18" />
        </button>
        <LanguageSelect />
      </div>
      <form class="auth-card" @submit.prevent="submit">
        <div class="auth-brand auth-brand-small"><BrandMark :size="30" /><span class="brand-name">{{ BRAND.name }}</span></div>
        <div>
          <h1>{{ t('login.title') }}</h1>
          <p class="muted" style="margin: 0">{{ t('login.subtitle') }}</p>
        </div>

        <p v-if="route.query.changed && !error" class="alert alert-ok">{{ t('login.password_changed') }}</p>
        <p v-if="error" class="alert alert-crit" role="alert">{{ error }}</p>

        <label class="field">
          <span>{{ t('login.username') }}</span>
          <input v-model="username" class="input" autocomplete="username" autocapitalize="none" spellcheck="false" required />
        </label>
        <label class="field">
          <span>{{ t('login.password') }}</span>
          <span class="pw-wrap">
            <input v-model="password" class="input" :type="showPw ? 'text' : 'password'" autocomplete="current-password" required />
            <button type="button" class="icon-btn icon-btn-sm pw-eye" :aria-label="showPw ? t('login.hide_password') : t('login.show_password')" :aria-pressed="showPw" @click="showPw = !showPw">
              <Icon :name="showPw ? 'eyeOff' : 'eye'" :size="16" />
            </button>
          </span>
        </label>
        <label v-if="needCode" class="field">
          <span>{{ t('login.code') }}</span>
          <input ref="codeInput" v-model="code" class="input input-code" inputmode="numeric" autocomplete="one-time-code" maxlength="7" required />
          <small class="muted">{{ t('login.code_hint') }}</small>
        </label>

        <button class="btn btn-primary btn-lg btn-block" :disabled="busy">
          <span v-if="busy" class="spinner" aria-hidden="true"></span>
          {{ busy ? t('login.submitting') : t('login.submit') }}
        </button>
      </form>
    </main>
  </div>
</template>

<style scoped>
.pw-wrap { position: relative; display: block; }
.pw-wrap .input { padding-right: 44px; }
.pw-eye { position: absolute; right: 3px; top: 50%; transform: translateY(-50%); }
@media (min-width: 901px) {
  .auth-brand-small { display: none; }
}
@media (max-width: 640px) {
  .pw-eye { right: 0; }
}
</style>
