<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { auth, logout } from '../auth.js'
import { LOCALES, setLocale } from '../i18n.js'
import { themeMode, setTheme } from '../theme.js'
import { usePopover } from '../composables/usePopover.js'
import Icon from './Icon.vue'

const { t, locale } = useI18n()
const router = useRouter()
const root = ref(null)
const btn = ref(null)
const { open, toggle, hide } = usePopover(root, btn)

async function signOut() {
  hide()
  try {
    await logout()
  } catch {
    // signed out locally either way
  }
  router.replace({ name: 'login' })
}
</script>

<template>
  <div ref="root" style="position: relative">
    <button ref="btn" type="button" class="avatar" :aria-label="t('account.menu', { user: auth.user?.username || '' })" :aria-expanded="open" aria-haspopup="menu" @click="toggle">
      {{ auth.user?.username?.[0] || '?' }}
    </button>
    <div v-if="open" class="menu account-menu" role="menu">
      <div class="account-head">
        <span class="avatar" aria-hidden="true">{{ auth.user?.username?.[0] }}</span>
        <div>
          <div class="account-name">{{ auth.user?.username }}</div>
          <div class="muted small">{{ auth.user?.totp_enabled ? t('account.2fa_on') : t('account.2fa_off') }}</div>
        </div>
      </div>
      <div class="menu-sep"></div>
      <div class="menu-label">{{ t('account.theme') }}</div>
      <div class="tabs account-seg" role="group" :aria-label="t('account.theme')">
        <button v-for="m in ['system', 'light', 'dark']" :key="m" type="button" :aria-pressed="themeMode === m" @click="setTheme(m)">
          <Icon :name="m === 'system' ? 'monitor' : m === 'light' ? 'sun' : 'moon'" :size="14" />{{ t('account.theme_' + m) }}
        </button>
      </div>
      <div class="menu-label">{{ t('common.language') }}</div>
      <div class="tabs account-langs" role="group" :aria-label="t('common.language')">
        <button v-for="l in LOCALES" :key="l.code" type="button" :aria-pressed="locale === l.code" :lang="l.code" @click="setLocale(l.code)">{{ l.name }}</button>
      </div>
      <div class="menu-sep"></div>
      <RouterLink :to="{ name: 'security' }" class="menu-item" role="menuitem" @click="hide"><Icon name="lock" :size="16" />{{ t('account.security') }}</RouterLink>
      <RouterLink :to="{ name: 'settings' }" class="menu-item" role="menuitem" @click="hide"><Icon name="settings" :size="16" />{{ t('nav.settings') }}</RouterLink>
      <button type="button" class="menu-item" role="menuitem" @click="signOut"><Icon name="logout" :size="16" />{{ t('nav.logout') }}</button>
    </div>
  </div>
</template>

<style scoped>
.account-menu { width: min(324px, calc(100vw - 24px)); top: calc(100% + 8px); }
.account-head { display: flex; align-items: center; gap: 10px; padding: 8px 8px 6px; }
.account-head .avatar { width: 34px; height: 34px; cursor: default; }
.account-name { font-weight: 700; }
.account-seg { display: flex; margin: 2px 6px 8px; width: calc(100% - 12px); }
.account-seg { flex-wrap: nowrap; }
.account-seg button { flex: 1; justify-content: center; padding: 0 8px; }
/* seven languages: two columns, the odd last one across */
.tabs.account-langs { display: grid; grid-template-columns: 1fr 1fr; margin: 2px 6px 8px; width: calc(100% - 12px); }
.account-langs button { justify-content: center; padding: 0 8px; white-space: nowrap; }
.account-langs button:last-child:nth-child(odd) { grid-column: 1 / -1; }
</style>
