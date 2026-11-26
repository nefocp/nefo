// Helpers shared by the app store's view and components.

// Catalog categories, in the order the filter shows them.
export const CATEGORIES = ['cms', 'ecommerce', 'education', 'productivity', 'storage', 'ai', 'devtools', 'database',
  'analytics', 'monitoring', 'automation', 'security', 'network', 'business']

// tx picks the reader's language from a catalog text ({ en, vi }).
export function tx(text, locale) {
  if (!text) return ''
  if (typeof text === 'string') return text
  return text[locale] || text.en || ''
}

// logoSrc is the address of a catalog logo, served by the panel.
export const logoSrc = (logo) => (logo ? `api/apps/logos/${encodeURIComponent(logo)}` : '')

// problemText turns a refusal or warning ({ code, detail }) into a sentence.
export function problemText(p, t, te) {
  if (!p) return ''
  const detail = p.detail || ''
  if (te(`apps.warn_${p.code}`)) return t(`apps.warn_${p.code}`, { detail })
  if (te(`errors.${p.code}`)) return t(`errors.${p.code}`, { detail })
  return t('errors.unknown')
}

// mb writes a size given in MB ("640 MB", "2,4 GB").
export function mb(n, locale) {
  const v = Number(n) || 0
  if (v >= 1024) return `${(v / 1024).toLocaleString(locale, { maximumFractionDigits: 1 })} GB`
  return `${Math.round(v).toLocaleString(locale)} MB`
}

// diskMB is the disk an app takes once installed: its images and its data.
export const diskMB = (a) => (a?.resources?.image_mb || 0) + (a?.resources?.disk_mb || 0)

// siteURL is a website's address, https when it has a certificate.
export const siteURL = (site, domain) => `${site?.ssl ? 'https' : 'http'}://${site?.domain || domain}/`

// The two-letter country codes (ISO 3166-1), for the store country input.
export const COUNTRIES = ('AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ ' +
  'CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ DE DJ DK DM DO DZ EC EE EG EH ER ES ET FI FJ FK FM FO FR GA GB GD GE GF ' +
  'GG GH GI GL GM GN GP GQ GR GS GT GU GW GY HK HM HN HR HT HU ID IE IL IM IN IO IQ IR IS IT JE JM JO JP KE KG KH KI KM KN KP KR KW ' +
  'KY KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ NA NC NE NF NG NI NL ' +
  'NO NP NR NU NZ OM PA PE PF PG PH PK PL PM PN PR PS PT PW PY QA RE RO RS RU RW SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST ' +
  'SV SX SY SZ TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ UA UG UM US UY UZ VA VC VE VG VI VN VU WF WS YE YT ZA ZM ZW').split(' ')

// regionName and currencyName give the reader's name of a code, or the code.
export function regionName(code, locale) {
  try {
    return new Intl.DisplayNames([locale], { type: 'region' }).of(code) || code
  } catch {
    return code
  }
}
export function currencies() {
  try {
    return Intl.supportedValuesOf('currency')
  } catch {
    return ['USD', 'EUR', 'GBP', 'JPY', 'VND', 'CNY', 'AUD', 'CAD', 'SGD', 'THB', 'INR', 'KRW']
  }
}
export function currencyName(code, locale) {
  try {
    return new Intl.DisplayNames([locale], { type: 'currency' }).of(code) || code
  } catch {
    return code
  }
}
