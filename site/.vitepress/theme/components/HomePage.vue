<script setup lang="ts">
// The home page. Its text comes from .vitepress/locales/<code>.json (the
// "home" key), filled with the release facts and passed in by config.mts as
// frontmatter.home; the app list comes from the app store catalog.
import { computed, onMounted, ref } from 'vue'
import { useData } from 'vitepress'
import Ico from './Ico.vue'
import ThemedImg from './ThemedImg.vue'
import { GITHUB, useNf } from '../lib'
import release from '../../data/release.json'

type App = { id: string; name: string; cat: string; kind: string; desc: string }

const { frontmatter, isDark } = useData()
const nf = useNf()
const t = computed(() => frontmatter.value.home as any)
const apps = computed(() => (frontmatter.value.apps || []) as App[])
const shots = computed(() => `/screens/${frontmatter.value.shots || 'en'}/`)
// The feature rows' crops exist in English and Vietnamese only.
const crops = computed(() => (['en', 'vi'].includes(frontmatter.value.shots) ? shots.value : '/screens/en/'))
const root = computed(() => nf.value.root)
const doc = (slug = '') => `${root.value}docs/${slug}`

const INSTALL = 'curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash'
const PANEL_URL = ['https://203.0.113.10:28443/', 'k3x9p2qa7m1z', '/']

// the software in the logo row: [logo, name, versions]
const STACK = [
  ['nginx', 'Nginx'], ['apache', 'Apache'], ['openlitespeed', 'OpenLiteSpeed'], ['php', 'PHP', '5.6–8.6'],
  ['mariadb', 'MariaDB'], ['mysql', 'MySQL', '8.4'], ['postgresql', 'PostgreSQL'], ['redis', 'Redis'],
  ['nodejs', 'Node.js'], ['python', 'Python'], ['java', 'Java'], ['docker', 'Docker'], ['wordpress', 'WordPress'],
  ['ubuntu', 'Ubuntu'], ['debian', 'Debian'],
]

// ---- screen tour: a tab per panel page, a light/dark switch on the window
const TABS: [string, string, string][] = [
  ['websites', 'globe', 'websites'],
  ['site', 'sliders', 'websites'],
  ['apps', 'pkg', 'apps'],
  ['databases', 'db', 'databases'],
  ['backups', 'archive', 'backups'],
  ['security', 'shield', 'security'],
  ['software', 'chip', 'software'],
]
const tab = ref('websites')
const seen = ref(new Set(['websites']))
const picked = ref<'light' | 'dark' | null>(null)
const tourMode = computed(() => picked.value ?? (isDark.value ? 'dark' : 'light'))
const mounted = ref(false)
onMounted(() => (mounted.value = true))
function pick(k: string) {
  tab.value = k
  seen.value = new Set([...seen.value, k])
}
function onTabKey(e: KeyboardEvent, i: number) {
  const d = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0
  if (!d) return
  e.preventDefault()
  const k = TABS[(i + d + TABS.length) % TABS.length][0]
  pick(k)
  ;(document.getElementById('nf-tab-' + k) as HTMLElement | null)?.focus()
}

// ---- feature rows: [key, icon, crop, url in the frame, docs page, logos]
const ROWS: [string, string, string, string, string, string[]][] = [
  ['web', 'globe', 'site', 'shop.example.com', 'websites', ['nginx:Nginx', 'apache:Apache', 'openlitespeed:OpenLiteSpeed', 'php:PHP', 'nodejs:Node.js', 'python:Python', 'java:Java']],
  ['db', 'db', 'databases', '', 'databases', ['mariadb:MariaDB', 'mysql:MySQL', 'postgresql:PostgreSQL', 'redis:Redis', 'phpmyadmin:phpMyAdmin']],
  ['bak', 'archive', 'backups', '', 'backups', []],
  ['sec', 'shield', 'security', '', 'security', []],
  ['sw', 'chip', 'software', '', 'software', []],
]
const CROP: Record<string, [number, number]> = { site: [1980, 1320], databases: [1800, 1200], backups: [2100, 1400], security: [1800, 1200], software: [1800, 1200] }
const ALSO: [string, string, string][] = [
  ['files', 'folder', 'files'], ['terminal', 'term', 'system'], ['cron', 'clock', 'system'],
  ['monitoring', 'pulse', 'system'], ['alerts', 'bell', 'mail-alerts'], ['dns', 'cloud', 'dns'],
  ['wordpress', 'wp', 'wordpress'], ['docker', 'box', 'docker'], ['api', 'key', 'api'],
]

// ---- app store: well-known apps first, then the catalog's order
const FIRST = ['wordpress', 'woocommerce', 'nextcloud', 'ghost', 'uptime-kuma', 'n8n', 'vaultwarden', 'gitea', 'open-webui', 'grafana', 'joomla', 'umami']
const sortedApps = computed(() => [...apps.value].sort((a, b) => (FIRST.indexOf(a.id) + 1 || 99) - (FIRST.indexOf(b.id) + 1 || 99)))
const cats = computed(() => Object.keys(t.value.apps.cats).filter((c) => apps.value.some((a) => a.cat === c)))
const count = (c: string) => apps.value.filter((a) => a.cat === c).length
const cat = ref('all')
const expanded = ref(false)
const visibleApps = computed(() => {
  const list = sortedApps.value.filter((a) => cat.value === 'all' || a.cat === cat.value)
  return new Set((cat.value === 'all' && !expanded.value ? list.slice(0, 12) : list).map((a) => a.id))
})

// ---- copy the install command
const copied = ref('')
async function copy(where: string) {
  try {
    await navigator.clipboard.writeText(INSTALL)
    copied.value = where
    setTimeout(() => (copied.value = ''), 1600)
  } catch {
    // no clipboard (old browser, or not allowed): the command stays selectable
  }
}

const faqLead = computed(() => t.value.faq.lead.replace('{docs}', doc()).replace('{faq}', doc('faq')))
</script>

<template>
  <div class="nf-home">
    <!-- hero: what Nefo is, the install command, the dashboard -->
    <section class="hero">
      <div class="wrap">
        <div class="hero-text">
          <div class="hero-badges">
            <a class="chip chip-accent" :href="GITHUB"><Ico name="github" />{{ t.badges.license }}</a>
            <a class="chip" :href="`${root}changelog`"><span class="dot" />{{ t.badges.version }}</a>
          </div>
          <h1 v-html="t.h1" />
          <p class="lead">{{ t.lead }}</p>
          <div class="cta">
            <a class="btn btn-primary" href="#install"><Ico name="download" />{{ t.ctaInstall }}</a>
            <a class="btn btn-line" :href="doc()"><Ico name="book" />{{ t.ctaDocs }}</a>
          </div>
          <div class="cmd">
            <div class="cmd-top">
              <span># {{ t.cmdTitle }}</span>
              <button class="copy" type="button" @click="copy('hero')"><Ico name="copy" /><span>{{ copied === 'hero' ? t.copied : t.copy }}</span></button>
            </div>
            <pre><code><span class="p" aria-hidden="true">$ </span>curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh <span class="f">| sudo bash</span></code></pre>
          </div>
          <ul class="facts">
            <li><img src="/logos/ubuntu.svg" alt="" width="18" height="18" />Ubuntu 22.04 · 24.04 · 26.04</li>
            <li><img src="/logos/debian.svg" alt="" width="18" height="18" />Debian 12 · 13</li>
            <li><Ico name="lang" />{{ t.languages }}</li>
          </ul>
        </div>
        <div class="hero-shot">
          <div class="frame">
            <div class="frame-bar">
              <span class="lights" aria-hidden="true"><i /><i /><i /></span>
              <span class="url"><Ico name="lock" /><span>{{ PANEL_URL[0] }}<b>{{ PANEL_URL[1] }}</b>{{ PANEL_URL[2] }}</span></span>
              <span class="spacer" />
            </div>
            <ThemedImg :src="shots + 'dashboard'" :w="3200" :h="2000" :alt="t.shotAlt" sizes="(max-width: 1080px) calc(100vw - 32px), 880px" high />
          </div>
          <div class="float phone">
            <ThemedImg :src="shots + 'dashboard-phone'" :w="780" :h="1688" :alt="t.phoneAlt" sizes="200px" />
          </div>
        </div>
      </div>
    </section>

    <!-- the software it installs -->
    <section class="stack" :aria-label="t.stackLabel">
      <div class="wrap">
        <p class="label">{{ t.stackLabel }}</p>
        <ul>
          <li v-for="[k, n, v] in STACK" :key="k"><img :src="`/logos/${k}.svg`" alt="" width="24" height="24" loading="lazy" />{{ n }}<small v-if="v">{{ v }}</small></li>
        </ul>
      </div>
    </section>

    <!-- screen tour -->
    <section id="screens" class="sec sec-alt">
      <div class="wrap">
        <div class="head">
          <span class="eyebrow">{{ t.tour.eyebrow }}</span>
          <h2>{{ t.tour.h2 }}</h2>
          <p>{{ t.tour.lead }}</p>
        </div>
        <div class="tabs" role="tablist">
          <button
            v-for="([k, icon], i) in TABS"
            :id="'nf-tab-' + k"
            :key="k"
            class="tab"
            type="button"
            role="tab"
            :aria-selected="tab === k"
            :aria-controls="'nf-pane-' + k"
            :tabindex="tab === k ? 0 : -1"
            @click="pick(k)"
            @keydown="onTabKey($event, i)"
          >
            <Ico :name="icon" />{{ t.tour.tabs[k].label }}
          </button>
        </div>
        <div class="tour">
          <div class="frame">
            <div class="frame-bar">
              <span class="lights" aria-hidden="true"><i /><i /><i /></span>
              <span class="url"><Ico name="lock" /><span>{{ PANEL_URL[0] }}<b>{{ PANEL_URL[1] }}</b>{{ PANEL_URL[2] }}</span></span>
              <span class="shot-switch" role="group">
                <button type="button" :aria-pressed="mounted && tourMode === 'light'" @click="picked = 'light'"><Ico name="sun" />{{ t.tour.light }}</button>
                <button type="button" :aria-pressed="mounted && tourMode === 'dark'" @click="picked = 'dark'"><Ico name="moon" />{{ t.tour.dark }}</button>
              </span>
            </div>
            <div class="panes">
              <div
                v-for="[k] in TABS"
                :id="'nf-pane-' + k"
                :key="k"
                class="pane"
                :class="{ on: tab === k }"
                role="tabpanel"
                :aria-labelledby="'nf-tab-' + k"
              >
                <ThemedImg
                  v-if="seen.has(k)"
                  :src="shots + k"
                  :w="3200"
                  :h="2000"
                  :alt="t.tour.tabs[k].alt"
                  sizes="(max-width: 1240px) calc(100vw - 32px), 1160px"
                  :mode="tourMode"
                  lazy
                />
              </div>
            </div>
          </div>
          <div v-for="[k, , slug] in TABS" v-show="tab === k" :key="k" class="caption">
            <p v-html="t.tour.tabs[k].caption" />
            <a class="more" :href="doc(slug)">{{ t.tour.docs }}: {{ t.tour.tabs[k].label }} <Ico name="arrow" /></a>
          </div>
        </div>
      </div>
    </section>

    <!-- features -->
    <section id="features" class="sec">
      <div class="wrap">
        <div class="head">
          <span class="eyebrow">{{ t.features.eyebrow }}</span>
          <h2>{{ t.features.h2 }}</h2>
          <p>{{ t.features.lead }}</p>
        </div>
        <div class="rows">
          <article v-for="([k, icon, crop, url, slug, logos], n) in ROWS" :key="k" class="row" :class="['k-' + k, { flip: n % 2 }]">
            <div class="row-text">
              <span class="tile"><Ico :name="icon" /></span>
              <span class="kicker">{{ t.features.rows[k].kicker }}</span>
              <h3>{{ t.features.rows[k].h3 }}</h3>
              <p>{{ t.features.rows[k].p }}</p>
              <ul class="ticks">
                <li v-for="x in t.features.rows[k].ticks" :key="x">{{ x }}</li>
              </ul>
              <div v-if="logos.length" class="logos-inline">
                <span v-for="l in logos" :key="l"><img :src="`/logos/${l.split(':')[0]}.svg`" alt="" width="18" height="18" loading="lazy" />{{ l.split(':')[1] }}</span>
              </div>
              <a class="more" :href="doc(slug)">{{ t.features.rows[k].more }} <Ico name="arrow" /></a>
            </div>
            <div class="row-shot">
              <div class="frame">
                <div class="frame-bar">
                  <span class="lights" aria-hidden="true"><i /><i /><i /></span>
                  <span class="url"><Ico name="lock" /><span>{{ PANEL_URL[0] }}<b>{{ PANEL_URL[1] }}</b>/{{ url }}</span></span>
                  <span class="spacer" />
                </div>
                <ThemedImg :src="crops + 'crop-' + crop" :w="CROP[crop][0]" :h="CROP[crop][1]" :alt="t.features.rows[k].alt" sizes="(max-width: 1080px) calc(100vw - 32px), 680px" lazy />
              </div>
            </div>
          </article>
        </div>
        <div class="also">
          <h3>{{ t.features.alsoTitle }}</h3>
          <ul>
            <li v-for="[k, icon, slug] in ALSO" :key="k">
              <Ico :name="icon" />
              <div><a :href="doc(slug)">{{ t.features.also[k].title }}</a><span>{{ t.features.also[k].text }}</span></div>
            </li>
          </ul>
        </div>
      </div>
    </section>

    <!-- app store -->
    <section id="apps" class="sec sec-alt">
      <div class="wrap">
        <div class="head">
          <span class="eyebrow">{{ t.apps.eyebrow }}</span>
          <h2>{{ t.apps.h2 }}</h2>
          <p>{{ t.apps.lead }}</p>
        </div>
        <div class="cats" role="group">
          <button class="cat" type="button" :aria-pressed="cat === 'all'" @click="cat = 'all'">{{ t.apps.all }} <i>{{ apps.length }}</i></button>
          <button v-for="c in cats" :key="c" class="cat" type="button" :aria-pressed="cat === c" @click="cat = c">{{ t.apps.cats[c] }} <i>{{ count(c) }}</i></button>
        </div>
        <ul class="apps">
          <li v-for="a in sortedApps" v-show="visibleApps.has(a.id)" :key="a.id" class="app">
            <div class="app-top">
              <span class="app-logo"><img :src="`/logos/${a.id}.svg`" alt="" width="30" height="30" loading="lazy" /></span>
              <div>
                <h3>{{ a.name }}</h3>
                <span class="cat-name">{{ t.apps.cats[a.cat] || a.cat }}</span>
              </div>
            </div>
            <p>{{ a.desc }}</p>
            <span class="kind" :class="'kind-' + a.kind">{{ t.apps.kinds[a.kind] }}</span>
          </li>
        </ul>
        <div class="apps-more">
          <button v-if="cat === 'all'" class="btn btn-line" type="button" :aria-expanded="expanded" @click="expanded = !expanded">
            <Ico :name="expanded ? 'minus' : 'plus'" /><span>{{ expanded ? t.apps.showLess : t.apps.showAll }}</span>
          </button>
          <a class="btn btn-line" :href="doc('apps')"><Ico name="book" />{{ t.apps.docs }}</a>
        </div>
      </div>
    </section>

    <!-- install -->
    <section id="install" class="sec ink">
      <div class="wrap">
        <div class="head">
          <span class="eyebrow">{{ t.install.eyebrow }}</span>
          <h2>{{ t.install.h2 }}</h2>
          <p>{{ t.install.lead }}</p>
        </div>
        <div class="install">
          <div>
            <ol class="steps">
              <li v-for="(s, n) in t.install.steps" :key="n">
                <div>
                  <h3>{{ s.title }}</h3>
                  <p v-html="s.text" />
                  <div v-if="n === 0" class="oslist">
                    <span><img src="/logos/ubuntu.svg" alt="" width="18" height="18" loading="lazy" />Ubuntu 22.04 · 24.04 · 26.04</span>
                    <span><img src="/logos/debian.svg" alt="" width="18" height="18" loading="lazy" />Debian 12 · 13</span>
                  </div>
                </div>
              </li>
            </ol>
            <a class="more on-ink" :href="doc('install')">{{ t.install.guide }} <Ico name="arrow" /></a>
          </div>
          <div class="term">
            <div class="term-bar">
              <span class="lights" aria-hidden="true"><i /><i /><i /></span>
              <span class="title">root@web-1: ~</span>
              <button class="copy" type="button" @click="copy('term')"><Ico name="copy" /><span>{{ copied === 'term' ? t.copied : t.copy }}</span></button>
            </div>
            <pre><code><span class="p">root@web-1:~#</span> {{ INSTALL }}
<span class="s">==&gt;</span> Downloading nefo-linux-amd64 (v{{ release.version }})
<span class="s">==&gt;</span> Verifying the release signature
<span class="s">==&gt;</span> Verifying checksum
<span class="s">==&gt;</span> Creating the configuration and the admin account
<span class="s">==&gt;</span> Installing the systemd service

<span class="ok">Nefo is ready.</span>

  URL:      <span class="hl">https://203.0.113.10:28443/k3x9p2qa7m1z/</span>
  Username: <span class="hl">admin</span>
  Password: <span class="hl">••••••••••••••••••••</span>

<span class="w">Save this password now: it is stored only as a hash and cannot be shown again.</span>
<span class="m">After signing in, change it and turn on two-factor authentication.</span></code></pre>
          </div>
        </div>
      </div>
    </section>

    <!-- compare -->
    <section id="compare" class="sec">
      <div class="wrap">
        <div class="head">
          <span class="eyebrow">{{ t.compare.eyebrow }}</span>
          <h2>{{ t.compare.h2 }}</h2>
          <p>{{ t.compare.lead }}</p>
        </div>
        <div class="compare">
          <table>
            <thead>
              <tr>
                <td />
                <th scope="col" class="us"><span><img class="tl" src="/mark-light.svg" alt="" width="20" height="20" /><img class="td" src="/mark-dark.svg" alt="" width="20" height="20" />{{ t.compare.us }}</span></th>
                <th scope="col">{{ t.compare.them }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in t.compare.rows" :key="r.label">
                <th scope="row">{{ r.label }}</th>
                <td class="us" :data-h="t.compare.us"><span class="mark" :class="r.mark"><Ico :name="r.mark === 'yes' ? 'check' : 'minus'" /><span>{{ r.us }}</span></span></td>
                <td class="them" :data-h="t.compare.them">{{ r.them }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="compare-note">{{ t.compare.note }} <a :href="`${root}compare`">{{ t.compare.more }} <Ico name="arrow" /></a></p>
      </div>
    </section>

    <!-- FAQ and where to go next -->
    <section id="faq" class="sec sec-alt">
      <div class="wrap">
        <div class="faq-grid">
          <div class="head">
            <span class="eyebrow">{{ t.faq.eyebrow }}</span>
            <h2>{{ t.faq.h2 }}</h2>
            <p v-html="faqLead" />
          </div>
          <div class="faq">
            <details v-for="(x, n) in t.faq.items" :key="n" :open="n === 0">
              <summary>{{ x.q }}<Ico name="plus" /></summary>
              <p v-html="x.a" />
            </details>
          </div>
        </div>
        <div class="help">
          <a :href="doc()"><span class="tile"><Ico name="book" /></span><div><b>{{ t.help.docs.title }}</b><span>{{ t.help.docs.text }}</span></div></a>
          <a :href="GITHUB"><span class="tile"><Ico name="github" /></span><div><b>{{ t.help.github.title }}</b><span>{{ t.help.github.text }}</span></div></a>
          <a :href="GITHUB + '/releases'"><span class="tile"><Ico name="tag" /></span><div><b>{{ t.help.releases.title }}</b><span>{{ t.help.releases.text }}</span></div></a>
        </div>
      </div>
    </section>

    <section class="band sec-alt">
      <div class="wrap">
        <div class="band-box">
          <div>
            <h2>{{ t.band.h2 }}</h2>
            <p>{{ t.band.p }}</p>
          </div>
          <div class="cta">
            <a class="btn btn-primary" href="#install"><Ico name="download" />{{ t.band.install }}</a>
            <a class="btn btn-line" :href="GITHUB"><Ico name="github" />{{ t.band.github }}</a>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>
