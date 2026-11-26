const UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

export function formatBytes(n) {
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  let i = 0
  while (n >= 1024 && i < UNITS.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(n >= 100 || i === 0 ? 0 : 1)} ${UNITS[i]}`
}

export function formatUptime(seconds, t) {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const parts = []
  if (d) parts.push(t('units.d', { n: d }))
  if (d || h) parts.push(t('units.h', { n: h }))
  parts.push(t('units.m', { n: m }))
  return parts.join(' ')
}

export function formatDateTime(value, locale) {
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? String(value) : d.toLocaleString(locale)
}

// formatBytesL is formatBytes with the locale's decimal sign ("1,4 GB" in Vietnamese).
export function formatBytesL(n, locale, digits = 1) {
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  let i = 0
  while (n >= 1024 && i < UNITS.length - 1) {
    n /= 1024
    i++
  }
  const d = n >= 100 || i === 0 ? 0 : digits
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: d, minimumFractionDigits: 0 }).format(n)} ${UNITS[i]}`
}

// formatNumber writes a number the locale's way (1.284 in Vietnamese).
export function formatNumber(n, locale, digits = 0) {
  return new Intl.NumberFormat(locale, { maximumFractionDigits: digits }).format(Number(n) || 0)
}

// relativeTime says how long ago a date was: "3 hours ago".
export function relativeTime(value, locale) {
  const ms = new Date(value).getTime()
  if (!Number.isFinite(ms)) return ''
  const s = Math.round((ms - Date.now()) / 1000)
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  const abs = Math.abs(s)
  if (abs < 60) return rtf.format(s, 'second')
  if (abs < 3600) return rtf.format(Math.round(s / 60), 'minute')
  if (abs < 86400) return rtf.format(Math.round(s / 3600), 'hour')
  if (abs < 86400 * 30) return rtf.format(Math.round(s / 86400), 'day')
  return new Date(ms).toLocaleDateString(locale)
}
