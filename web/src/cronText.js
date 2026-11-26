// cronText describes common cron schedules in words ("Every day at 03:00");
// it returns "" for the ones it does not know, and the page shows the raw
// schedule only.
export function cronText(spec, t, locale) {
  const s = String(spec || '').trim()
  const presets = { '@reboot': 'pg.cron_reboot', '@hourly': 'pg.cron_hourly0', '@daily': 'pg.cron_midnight', '@midnight': 'pg.cron_midnight', '@weekly': 'pg.cron_weekly0', '@monthly': 'pg.cron_monthly0', '@yearly': 'pg.cron_yearly', '@annually': 'pg.cron_yearly' }
  if (presets[s]) return t(presets[s])
  const f = s.split(/\s+/)
  if (f.length !== 5) return ''
  const [mi, h, dom, mon, dow] = f
  const num = (x) => /^\d+$/.test(x)
  const time = () => `${h.padStart(2, '0')}:${mi.padStart(2, '0')}`
  const day = (d) => new Date(2026, 10, 1 + (Number(d) % 7)).toLocaleDateString(locale, { weekday: 'long' })
  if (mon !== '*') return ''
  if (mi === '*' && h === '*' && dom === '*' && dow === '*') return t('pg.cron_every_minute')
  let m = mi.match(/^\*\/(\d+)$/)
  if (m && h === '*' && dom === '*' && dow === '*') return t('pg.cron_every_n_min', { n: m[1] }, Number(m[1]))
  if (num(mi) && h === '*' && dom === '*' && dow === '*') return t('pg.cron_hourly', { m: mi.padStart(2, '0') })
  m = h.match(/^\*\/(\d+)$/)
  if (num(mi) && m && dom === '*' && dow === '*') return t('pg.cron_every_n_hours', { n: m[1], m: mi.padStart(2, '0') }, Number(m[1]))
  if (num(mi) && num(h) && dom === '*' && dow === '*') return t('pg.cron_daily', { time: time() })
  if (num(mi) && num(h) && dom === '*' && num(dow)) return t('pg.cron_weekly', { day: day(dow), time: time() })
  if (num(mi) && num(h) && num(dom) && dow === '*') return t('pg.cron_monthly', { d: dom, time: time() })
  return ''
}
