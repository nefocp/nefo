import { computed } from 'vue'
import { auth } from './auth.js'
import { store, load, list } from './data.js'
import { server } from './server.js'

// Things that need the administrator, worked out in the browser from
// answers the panel already gives: the account (2FA), SSH and firewall
// settings, certificates and expiry dates of websites, failed scheduled
// backups, a nearly full disk, pending security updates and a restart that
// an update needs. Each item: { id, sev: crit|warn, icon,
// title: [key, params, plural count], sub: [key, params], to, action }.
// The texts are i18n keys, so a change of language needs no new request.

const DAY = 86400000

export function refreshAttention(force = false) {
  return Promise.allSettled([
    load('api/sites', { force }),
    load('api/security', { ttl: 5 * 60000, force }),
    load('api/backups', { ttl: 60000, force }),
    // The last update check: the panel refreshes it every few hours, so asking is cheap.
    load('api/system/updates', { ttl: 10 * 60000, force }),
    // Monitored websites that are down (internal/uptime): kept in memory, cheap.
    load('api/uptime', { ttl: 60000, force }),
    // The last security check's counts (it runs on demand and once a day).
    load('api/security/check', { ttl: 5 * 60000, force }),
  ])
}

export const daysLeft = (iso) => (iso ? Math.floor((new Date(iso).getTime() - Date.now()) / DAY) : null)

export const attention = computed(() => {
  const out = []
  if (auth.user && !auth.user.totp_enabled) {
    out.push({ id: '2fa', sev: 'crit', icon: 'lock', title: ['attention.no_2fa'], sub: ['attention.no_2fa_sub'], to: { name: 'security' }, action: 'attention.enable', primary: true })
  }
  const sec = store['api/security']
  if (sec?.ssh && sec.ssh.permit_root_login === 'yes' && sec.ssh.password_auth) {
    out.push({ id: 'ssh', sev: 'crit', icon: 'terminal', title: ['attention.ssh_root'], sub: ['attention.ssh_root_sub'], to: { name: 'firewall' }, action: 'attention.fix' })
  }
  if (sec?.firewall?.installed && !sec.firewall.active) {
    out.push({ id: 'fw', sev: 'warn', icon: 'shield', title: ['attention.firewall_off'], sub: ['attention.firewall_off_sub'], to: { name: 'firewall' }, action: 'attention.fix' })
  }
  // The security check: one line for its open problems (notes are left out).
  const cc = store['api/security/check']?.report ? store['api/security/check'].counts : null
  if (cc && cc.critical + cc.warning > 0) {
    const n = cc.critical + cc.warning
    out.push({ id: 'seccheck', sev: cc.critical ? 'crit' : 'warn', icon: 'shieldAlert', title: ['attention.check_problems', { n }, n], sub: ['attention.check_problems_sub', { crit: cc.critical, warn: cc.warning }], to: { name: 'security-check' }, action: 'attention.view' })
  }
  const upd = store['api/system/updates']
  if (upd?.security > 0) {
    out.push({ id: 'secupd', sev: 'warn', icon: 'download', title: ['attention.security_updates', { n: upd.security }, upd.security], sub: ['attention.security_updates_sub'], to: { name: 'system' }, action: 'attention.view' })
  }
  if (upd?.reboot?.required) {
    out.push({ id: 'reboot', sev: 'warn', icon: 'power', title: ['attention.reboot_required'], sub: ['attention.reboot_required_sub'], to: { name: 'settings' }, action: 'attention.view' })
  }
  const down = new Map(list(store['api/uptime']?.sites).filter((x) => x.state === 'down').map((x) => [x.site_id, x]))
  for (const s of list(store['api/sites']?.sites)) {
    const dn = down.get(s.id)
    if (dn) {
      const why = dn.last?.error || (dn.last?.code ? 'HTTP ' + dn.last.code : '')
      out.push({ id: 'down' + s.id, sev: 'crit', icon: 'activity', title: ['attention.site_down', { domain: s.domain }], sub: why ? ['attention.raw', { text: firstLine(why) }] : ['attention.site_down_sub'], to: { name: 'websites', query: { site: String(s.id), tab: 'monitor' } }, action: 'attention.view' })
    }
    const to = { name: 'websites', query: { site: String(s.id), tab: 'ssl' } }
    const d = daysLeft(s.cert_expires)
    if (s.ssl && s.ssl_error) {
      out.push({ id: 'sslerr' + s.id, sev: 'crit', icon: 'shieldAlert', title: ['attention.ssl_failed', { domain: s.domain }], sub: ['attention.raw', { text: firstLine(s.ssl_error) }], to, action: 'attention.view' })
    } else if (s.ssl && d !== null && d < 0) {
      out.push({ id: 'sslexp' + s.id, sev: 'crit', icon: 'shieldAlert', title: ['attention.ssl_expired', { domain: s.domain }], sub: ['attention.ssl_expired_sub'], to, action: 'attention.view' })
    } else if (s.ssl && d !== null && d <= 14) {
      out.push({ id: 'sslsoon' + s.id, sev: 'warn', icon: 'shield', title: ['attention.ssl_soon', { domain: s.domain, n: d }, d], sub: [s.ssl_source === 'custom' ? 'attention.ssl_soon_custom' : 'attention.ssl_soon_sub'], to, action: 'attention.view' })
    }
    const e = daysLeft(s.expires_at)
    if (!s.disabled && e !== null && e <= 7) {
      out.push({ id: 'exp' + s.id, sev: 'warn', icon: 'calendar', title: ['attention.site_expires', { domain: s.domain, n: Math.max(e, 0) }, Math.max(e, 0)], sub: ['attention.site_expires_sub'], to: { name: 'websites', query: { site: String(s.id), tab: 'expiry' } }, action: 'attention.view' })
    }
  }
  for (const sc of list(store['api/backups']?.schedules)) {
    if (sc.last_error) {
      out.push({ id: 'bk' + sc.id, sev: 'crit', icon: 'archive', title: ['attention.backup_failed', { target: sc.target }], sub: ['attention.raw', { text: firstLine(sc.last_error) }], to: { name: 'backups' }, action: 'attention.view' })
    }
  }
  const st = server.stats
  if (st?.supported && st.disk_total) {
    const p = Math.round((st.disk_used / st.disk_total) * 100)
    if (p >= 90) out.push({ id: 'disk', sev: 'crit', icon: 'disk', title: ['attention.disk_full', { pct: p }], sub: ['attention.disk_full_sub'], to: { name: 'files' }, action: 'attention.view' })
  }
  const rank = { crit: 0, warn: 1 }
  return out.sort((a, b) => rank[a.sev] - rank[b.sev])
})

export const critCount = computed(() => attention.value.filter((a) => a.sev === 'crit').length)

function firstLine(s) {
  const l = String(s || '').split('\n').find((x) => x.trim()) || ''
  return l.length > 140 ? l.slice(0, 140) + '…' : l
}
