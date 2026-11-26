import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router.js'
import { i18n } from './i18n.js'
import { setUnauthorizedHandler } from './api.js'
import { auth } from './auth.js'
import './theme.js' // sets data-theme before the first paint
import './fonts.css'
import './style.css'

// Any 401 means the session ended (expired, signed out elsewhere, password changed).
setUnauthorizedHandler(() => {
  auth.user = null
  if (router.currentRoute.value.name !== 'login') router.replace({ name: 'login' })
})

createApp(App).use(i18n).use(router).mount('#app')
