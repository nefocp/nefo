import { reactive } from 'vue'
import { api, ApiError } from './api.js'

// user: { username, totp_enabled } or null. checked: whether we asked the server yet.
export const auth = reactive({ user: null, checked: false })

export async function loadUser() {
  try {
    auth.user = await api('GET', 'api/auth/me')
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) auth.user = null
    else throw e
  } finally {
    auth.checked = true
  }
  return auth.user
}

export async function login(username, password, code) {
  auth.user = await api('POST', 'api/auth/login', { username, password, code })
  auth.checked = true
  return auth.user
}

export async function logout() {
  try {
    await api('POST', 'api/auth/logout', {})
  } finally {
    auth.user = null
  }
}
