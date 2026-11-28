// Copies what the home page needs from the panel's sources into the site, so
// the site builds on its own:
//   - .vitepress/data/apps.json: the app store catalog (id, name, category,
//     kind and the description in every language), from internal/appstore;
//   - public/logos/: the app logos and the software logos the home shows,
//     with their SOURCES files (licences).
// Run it from site/ after the catalog changes:  node .vitepress/scripts/sync-apps.mjs
import { copyFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const site = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const repo = join(site, '..')
const catalog = JSON.parse(readFileSync(join(repo, 'internal/appstore/catalog.json'), 'utf8'))

const apps = catalog.apps.map((a) => ({
  id: a.id,
  name: a.name,
  category: a.category,
  kind: a.kind,
  description: a.description,
}))
writeFileSync(join(site, '.vitepress/data/apps.json'), JSON.stringify({ catalog_version: catalog.catalog_version, apps }, null, 1) + '\n')

// software logos shown in the logo row, the feature rows and the install steps
const SOFTWARE = ['nginx', 'apache', 'openlitespeed', 'php', 'mariadb', 'mysql', 'postgresql', 'redis', 'nodejs', 'python', 'java', 'docker', 'wordpress', 'ubuntu', 'debian', 'phpmyadmin']
const out = join(site, 'public/logos')
mkdirSync(out, { recursive: true })
const from = [join(repo, 'internal/appstore/logos'), join(repo, 'web/src/assets/logos')]
const missing = []
for (const id of [...new Set([...apps.map((a) => a.id), ...SOFTWARE])]) {
  const src = from.map((d) => join(d, id + '.svg')).find((p) => existsSync(p))
  if (src) copyFileSync(src, join(out, id + '.svg'))
  else missing.push(id)
}
copyFileSync(join(repo, 'internal/appstore/logos/SOURCES.md'), join(out, 'SOURCES-appstore.txt'))
copyFileSync(join(repo, 'web/src/assets/logos/SOURCES.md'), join(out, 'SOURCES-panel.txt'))
copyFileSync(join(repo, 'web/src/assets/logos/LICENSE-MIT.md'), join(out, 'LICENSE-MIT.txt'))
console.log(`${apps.length} apps; logos in public/logos${missing.length ? '; no logo for ' + missing.join(', ') : ''}`)
