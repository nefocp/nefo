// Logos of software and applications (see assets/logos/SOURCES.md). They are
// shown on a white rounded tile (components/AppLogo.vue) in both themes.
const files = import.meta.glob('./assets/logos/*.svg', { eager: true, query: '?url', import: 'default' })

export const LOGOS = Object.fromEntries(
  Object.entries(files).map(([path, url]) => [path.replace(/^.*\/([^/]+)\.svg$/, '$1'), url]),
)

// Other names the panel uses for the same software.
const ALIASES = {
  apache2: 'apache',
  lshttpd: 'openlitespeed',
  ols: 'openlitespeed',
  node: 'nodejs',
  'redis-server': 'redis',
  modsecurity: 'waf',
  'uptimekuma': 'uptime-kuma',
}

// logoURL returns the logo of a name such as "nginx", "php8.3", "docker:ghost"
// or "wordpress", or "" when there is none.
export function logoURL(name) {
  if (!name) return ''
  let k = String(name).toLowerCase().replace(/^docker:/, '')
  if (/^php[0-9.]*(-fpm)?$/.test(k)) k = 'php'
  if (/^mariadb-[0-9.]+$/.test(k)) k = 'mariadb' // a series from MariaDB's repository
  k = ALIASES[k] || k
  return LOGOS[k] || ''
}

// osLogo picks the logo of an operating system name from /etc/os-release.
export function osLogo(os) {
  const s = String(os || '').toLowerCase()
  if (s.includes('ubuntu')) return LOGOS.ubuntu
  if (s.includes('debian')) return LOGOS.debian
  return ''
}

// Display names of the applications the panel installs on a website.
export const APP_NAMES = {
  wordpress: 'WordPress',
  laravel: 'Laravel',
  drupal: 'Drupal',
  joomla: 'Joomla',
  nextcloud: 'Nextcloud',
  opencart: 'OpenCart',
  moodle: 'Moodle',
  ghost: 'Ghost',
  'uptime-kuma': 'Uptime Kuma',
  n8n: 'n8n',
  gitea: 'Gitea',
  umami: 'Umami',
  vaultwarden: 'Vaultwarden',
  filebrowser: 'File Browser',
  'code-server': 'code-server',
  'open-webui': 'Open WebUI',
  ollama: 'Ollama',
  wikijs: 'Wiki.js',
  directus: 'Directus',
  odoo: 'Odoo',
  'adguard-home': 'AdGuard Home',
  'stirling-pdf': 'Stirling-PDF',
  'wg-easy': 'WireGuard Easy',
}

export function appName(app) {
  const id = String(app || '').replace(/^docker:/, '')
  return APP_NAMES[id] || id
}
