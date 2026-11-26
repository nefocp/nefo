// Shared by the DNS page and "point DNS here" in the site drawer.

// The record types the panel can add and change (the others are listed
// read-only), and those Cloudflare can proxy.
export const EDITABLE = ['A', 'AAAA', 'CNAME', 'TXT', 'MX', 'CAA']
export const PROXIABLE = ['A', 'AAAA', 'CNAME']

// TTL choices in seconds; 1 is Cloudflare's "automatic".
export const TTLS = [1, 60, 120, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400]

export function ttlText(ttl, t) {
  if (!ttl || ttl === 1) return t('dns.ttl_auto')
  if (ttl % 86400 === 0) return t('dns.ttl_day', { n: ttl / 86400 }, ttl / 86400)
  if (ttl % 3600 === 0) return t('dns.ttl_h', { n: ttl / 3600 })
  if (ttl % 60 === 0) return t('dns.ttl_min', { n: ttl / 60 })
  return t('dns.ttl_s', { n: ttl })
}

// The name this panel is opened with, when it is a name and not an IP
// address: Cloudflare's proxy does not pass the panel's port, so it must
// stay "DNS only".
const host = (typeof location !== 'undefined' ? location.hostname : '').toLowerCase()
export const panelHost = !host || host === 'localhost' || /^[\d.]+$/.test(host) || host.includes(':') || host.startsWith('[') ? '' : host
