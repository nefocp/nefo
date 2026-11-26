import { ApiError } from './api.js'
import { i18n } from './i18n.js'

// errorMessage turns an API error code into a translated sentence. Some
// errors carry a detail (a domain name, or the server's own error text) that
// the message can include as {detail}.
export function errorMessage(e) {
  const { t, te } = i18n.global
  if (e instanceof ApiError) {
    if (e.code === 'too_many_attempts') {
      const seconds = e.data?.retry_after ?? 60
      return t('errors.too_many_attempts', { seconds }, Number(seconds) || 2)
    }
    const key = `errors.${e.code}`
    if (te(key)) return t(key, { detail: e.data?.detail ?? '' })
  }
  return t('errors.unknown')
}
