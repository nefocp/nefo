// Makes the social images: public/og/<lang>.png (1200×630, one per language,
// from the home page's hero text) and the PNG logos (public/logo-512.png,
// public/apple-touch-icon.png). Run it from site/ after the home text of a
// language changes, or after a release:
//
//   node .vitepress/scripts/og.mjs
//
// It needs puppeteer-core and Chrome: set PUPPETEER_CORE to the folder of a
// puppeteer-core install if it isn't resolvable from here, and CHROME to the
// browser if it isn't in the usual Windows place.
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { createRequire } from 'node:module'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const site = join(here, '..', '..')
const pub = join(site, 'public')
const require = createRequire(process.env.PUPPETEER_CORE ? join(process.env.PUPPETEER_CORE, 'x.js') : import.meta.url)
const puppeteer = require('puppeteer-core')
const CHROME = process.env.CHROME || 'C:/Program Files/Google/Chrome/Application/chrome.exe'

const release = JSON.parse(readFileSync(join(site, '.vitepress/data/release.json'), 'utf8'))
const apps = JSON.parse(readFileSync(join(site, '.vitepress/data/apps.json'), 'utf8')).apps.length
const FACTS = { version: release.version, status: release.status, date: release.date, ram: release.ram, languages: String(release.languages), apps: String(apps) }
const fill = (s) => s.replace(/\{(\w+)\}/g, (m, k) => FACTS[k] ?? m)
const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;')
const url = (p) => pathToFileURL(join(pub, p)).href

const LANGS = [['en', 'en', 'en'], ['vi', 'vi', 'vi'], ['zh', 'zh-CN', 'en'], ['es', 'es', 'en'], ['pt', 'pt-BR', 'en'], ['ru', 'ru', 'en'], ['id', 'id', 'en']]

const fontFaces = ['latin', 'latin-ext', 'vietnamese']
  .flatMap((set) => [400, 500, 600, 700, 800].map((w) => `@font-face{font-family:'Be Vietnam Pro';font-weight:${w};src:url(${url(`fonts/be-vietnam-pro-${set}-${w}-normal.woff2`)})}`))
  .join('\n') + `\n@font-face{font-family:'JetBrains Mono';font-weight:500;src:url(${url('fonts/jetbrains-mono-latin-500-normal.woff2')})}`

function card(code, lang, shots) {
  const h = JSON.parse(readFileSync(join(site, `.vitepress/locales/${code}.json`), 'utf8')).home
  const font = lang === 'ru' ? "system-ui,'Segoe UI',Roboto,Arial,sans-serif" : lang.startsWith('zh') ? "'Be Vietnam Pro','Microsoft YaHei','PingFang SC','Noto Sans SC',sans-serif" : "'Be Vietnam Pro',system-ui,sans-serif"
  return `<!doctype html><html lang="${lang}"><meta charset="utf-8"><style>
${fontFaces}
*{box-sizing:border-box;margin:0}
body{width:1200px;height:630px;overflow:hidden;font-family:${font};color:#121a26;background:
 radial-gradient(700px 420px at 0% 0%,rgb(45 212 191/.22),transparent 70%),radial-gradient(600px 420px at 100% 10%,rgb(47 107 219/.12),transparent 70%),#fff}
body::before{content:'';position:absolute;inset:0;background-image:linear-gradient(#e4e8ee 1px,transparent 1px),linear-gradient(90deg,#e4e8ee 1px,transparent 1px);background-size:44px 44px;-webkit-mask-image:radial-gradient(ellipse 70% 70% at 20% 20%,#000 10%,transparent 75%);opacity:.6}
.text{position:absolute;left:64px;top:58px;width:560px}
.brand{display:flex;align-items:center;gap:14px;font:700 34px/1 'Be Vietnam Pro',sans-serif;letter-spacing:-.02em}
.brand img{width:52px;height:52px}
.beta{font:600 14px/1 'JetBrains Mono',monospace;text-transform:uppercase;letter-spacing:.06em;color:#0b7a6e;background:#e2f3f1;border:1px solid #b5e0da;border-radius:6px;padding:6px 8px}
h1{margin-top:38px;font-size:56px;font-weight:800;line-height:1.06;letter-spacing:-.03em}
h1 em{font-style:normal;color:#0b7a6e}
.chips{display:flex;flex-wrap:wrap;gap:10px;margin-top:30px}
.chip{font-size:19px;font-weight:600;color:#465264;background:#fff;border:1px solid #e4e8ee;border-radius:99px;padding:8px 16px}
.chip.a{color:#0b7a6e;background:#e2f3f1;border-color:#b5e0da}
.cmd{position:absolute;left:64px;bottom:52px;width:560px;background:#0b1220;border-radius:14px;padding:16px 20px;color:#e6edf6;font:500 15.5px/1.5 'JetBrains Mono',monospace;word-break:break-all}
.cmd b{color:#2dd4bf;font-weight:500}.cmd i{font-style:normal;color:#93c5fd}
.shot{position:absolute;left:668px;top:64px;width:760px;border:1px solid #e4e8ee;border-radius:16px;overflow:hidden;background:#fff;box-shadow:0 30px 70px -20px rgb(15 23 42/.35),0 8px 24px rgb(15 23 42/.1)}
.bar{display:flex;gap:8px;align-items:center;height:34px;padding:0 14px;background:#f6f8fa;border-bottom:1px solid #e4e8ee}
.bar i{width:11px;height:11px;border-radius:50%;background:#ff5f57}.bar i:nth-child(2){background:#febc2e}.bar i:nth-child(3){background:#28c840}
.shot img{display:block;width:100%}
</style><body>
<div class="text">
 <div class="brand"><img src="${url('mark-light.svg')}" alt="">Nefo<span class="beta">beta</span></div>
 <h1>${fill(h.h1)}</h1>
 <div class="chips"><span class="chip a">${esc(fill(h.badges.license))}</span><span class="chip">${esc(fill(h.badges.version))}</span><span class="chip">~${esc(release.ram)} RAM</span><span class="chip">${esc(fill(h.languages))}</span></div>
</div>
<div class="cmd"><b>$</b> curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh <i>| sudo bash</i></div>
<div class="shot"><div class="bar"><i></i><i></i><i></i></div><img src="${url(`screens/${shots}/dashboard-light.webp`)}" alt=""></div>
</body></html>`
}

const logo = (size, pad) => `<!doctype html><meta charset="utf-8"><style>*{margin:0}body{width:${size}px;height:${size}px;background:transparent}img{display:block;width:${size - 2 * pad}px;height:${size - 2 * pad}px;margin:${pad}px}</style><body><img src="${url('mark-light.svg')}" alt="">`

mkdirSync(join(pub, 'og'), { recursive: true })
const browser = await puppeteer.launch({ executablePath: CHROME, headless: true, args: ['--allow-file-access-from-files'] })
const page = await browser.newPage()
const tmp = mkdtempSync(join(tmpdir(), 'nefo-og-'))
async function show(html) {
  const f = join(tmp, 'card.html')
  writeFileSync(f, html)
  await page.goto(pathToFileURL(f).href + '?' + Date.now(), { waitUntil: 'load' })
  await page.evaluate(() => document.fonts.ready)
}
for (const [code, lang, shots] of LANGS) {
  await page.setViewport({ width: 1200, height: 630, deviceScaleFactor: 1 })
  await show(card(code, lang, shots))
  await page.screenshot({ path: join(pub, 'og', `${code}.png`) })
  console.log('og/' + code + '.png')
}
for (const [file, size, pad] of [['logo-512.png', 512, 0], ['apple-touch-icon.png', 180, 0]]) {
  await page.setViewport({ width: size, height: size, deviceScaleFactor: 1 })
  await show(logo(size, pad))
  await page.screenshot({ path: join(pub, file), omitBackground: true })
  console.log(file)
}
await browser.close()
rmSync(tmp, { recursive: true, force: true })
