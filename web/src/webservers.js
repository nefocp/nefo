// The web servers the panel can install, one per server. RAM is about what
// each uses while idle with no website, measured in the test sandbox (the
// sum of its processes' resident memory).
export const SERVER_IDS = ['nginx', 'apache', 'openlitespeed']

export const SERVER_NAMES = { nginx: 'Nginx', apache: 'Apache', openlitespeed: 'OpenLiteSpeed' }

export const SERVER_RAM = { nginx: 15, apache: 40, openlitespeed: 26 }

// serverName returns the display name of a web server id ("" for none).
export function serverName(id) {
  return SERVER_NAMES[id] || ''
}

// hasRateLimit reports whether the web server can limit requests per
// visitor: only Nginx has a light way to do it.
export function hasRateLimit(id) {
  return id !== 'apache' && id !== 'openlitespeed'
}
