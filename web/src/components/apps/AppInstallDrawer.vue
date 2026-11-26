<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../../api.js'
import { errorMessage } from '../../errors.js'
import { server } from '../../server.js'
import Drawer from '../Drawer.vue'
import AppLogo from '../AppLogo.vue'
import Icon from '../Icon.vue'
import { tx, logoSrc, problemText, mb, diskMB, COUNTRIES, regionName, currencies, currencyName } from './appstore.js'

// AppInstallDrawer installs one catalog app: the fields its kind needs (a
// new website for Docker apps, an empty PHP website for PHP apps, nothing
// for services), its own inputs, and a live check of the server while the
// fields change. "started" hands the job (and a PHP app's account) back.
const props = defineProps({
  app: { type: Object, required: true },
  sites: { type: Array, default: () => [] },
  dockerReady: { type: Boolean, default: true },
  busy: { type: Boolean, default: false }, // another job is running
})
const emit = defineEmits(['close', 'started'])
const { t, te, locale } = useI18n()

const kind = computed(() => props.app.kind)
const isPHP = computed(() => kind.value === 'php')

// PHP apps go into a PHP website with nothing installed yet.
const cmpVer = (a, b) => {
  const x = String(a).split('.').map(Number)
  const y = String(b).split('.').map(Number)
  return (x[0] - y[0]) || ((x[1] || 0) - (y[1] || 0))
}
const candidates = computed(() =>
  props.sites.filter((s) => s.php_version && !s.proxy_port && !s.app &&
    (!props.app.min_php || cmpVer(s.php_version, props.app.min_php) >= 0) &&
    (!props.app.max_php || cmpVer(s.php_version, props.app.max_php) <= 0)),
)

const inputs = computed(() => Object.entries(props.app.inputs || {}))
const form = ref({
  domain: '',
  aliases: '',
  admin_email: '',
  site_id: candidates.value[0]?.id || '',
  title: '',
  admin_user: '', // not "admin": it is the first name attackers try
  admin_password: '',
  inputs: Object.fromEntries(inputs.value.map(([k, v]) => [k, v.default || ''])),
})

function body() {
  const f = form.value
  const b = { inputs: { ...f.inputs } }
  if (kind.value === 'docker') {
    b.domain = f.domain.trim().toLowerCase()
    b.aliases = f.aliases.split(/[\s,]+/).map((x) => x.trim().toLowerCase()).filter(Boolean)
  }
  if (props.app.needs_admin_email || (isPHP.value && props.app.admin_form)) b.admin_email = f.admin_email.trim()
  if (isPHP.value) {
    b.site_id = Number(f.site_id) || 0
    if (props.app.admin_form) {
      b.title = f.title.trim() || props.app.name
      b.admin_user = f.admin_user.trim()
      b.admin_password = f.admin_password
    }
  }
  return b
}

// ---- the live check ----
const check = ref(null) // the last answer of POST /check
const checking = ref(false)
let seq = 0
let timer = 0
async function runCheck() {
  const n = ++seq
  checking.value = true
  try {
    const res = await api('POST', `api/apps/${props.app.id}/check`, body())
    if (n === seq) check.value = res
  } catch {
    // the install itself reports problems
  } finally {
    if (n === seq) checking.value = false
  }
}
watch(form, () => {
  clearTimeout(timer)
  timer = setTimeout(runCheck, 450)
}, { deep: true })
onMounted(runCheck)
onBeforeUnmount(() => clearTimeout(timer))

// Until the check reaches the resources (the domain is still empty, say),
// the catalog's own check is shown.
const refusals = computed(() => check.value?.refusals || props.app.check?.refusals || [])
const warnings = computed(() => check.value?.warnings || props.app.check?.warnings || [])
// A problem with the fields themselves, shown once the main field is filled.
const fieldsTouched = computed(() => (kind.value === 'docker' ? !!form.value.domain.trim() : isPHP.value ? !!form.value.site_id : true))
const checkError = computed(() => {
  const c = check.value
  if (!c || c.ok || !c.error || !fieldsTouched.value) return ''
  if (refusals.value.some((r) => r.code === c.error)) return '' // already listed
  if (c.error === 'invalid_email' && !form.value.admin_email.trim()) return ''
  if (c.error === 'invalid_wp_user' && !form.value.admin_user.trim()) return ''
  if (c.error === 'invalid_app_password' && passwordShort.value) return '' // shown at the field
  return te(`errors.${c.error}`) ? t(`errors.${c.error}`, { detail: c.detail || '' }) : t('errors.unknown')
})

// ---- what it needs, what the server has ----
const r = computed(() => props.app.resources || {})
const serverRAM = computed(() => (server.stats?.mem_total ? Math.round(server.stats.mem_total / 1048576) : 0))
const serverFree = computed(() => (server.stats?.disk_total ? Math.round((server.stats.disk_total - server.stats.disk_used) / 1048576) : 0))
const needDisk = computed(() => r.value.min_free_disk_mb || diskMB(props.app) + (kind.value === 'php' ? 0 : 1024))
const limit = computed(() => check.value?.memory_limit_mb || props.app.check?.memory_limit_mb || r.value.memory_limit_mb || 0)
onMounted(async () => {
  if (!server.stats) {
    try {
      server.stats = await api('GET', 'api/system/stats')
    } catch {
      // the summary then leaves the server's side out
    }
  }
})

// The server's rule for an administrator password: empty for a random one,
// or 12 to 128 characters (the field stops at 128).
const passwordShort = computed(() => {
  const n = form.value.admin_password.length
  return isPHP.value && !!props.app.admin_form && n > 0 && n < 12
})

const needsDocker = computed(() => !isPHP.value && !props.dockerReady)
const noSite = computed(() => isPHP.value && !candidates.value.length)
const canInstall = computed(() => !needsDocker.value && !noSite.value && !refusals.value.length && !passwordShort.value && !props.busy && !sending.value)

const notes = computed(() => {
  const out = []
  const a = props.app
  if (tx(a.notes, locale.value)) out.push(tx(a.notes, locale.value))
  if (a.admin_method && a.admin_method !== 'form' && te(`apps.admin_${a.admin_method}`)) out.push(t(`apps.admin_${a.admin_method}`))
  if (a.id === 'code-server') out.push(t('apps.code_server_root'))
  if (a.needs_https) out.push(t('apps.needs_https'))
  if (kind.value === 'docker') out.push(t('apps.suspended_until_ready'))
  if (kind.value === 'service') out.push(t('apps.service_local'))
  return out
})

const countryList = computed(() => COUNTRIES.map((c) => ({ c, n: regionName(c, locale.value) })).sort((a, b) => a.n.localeCompare(b.n, locale.value)))
const currencyList = computed(() => currencies().map((c) => ({ c, n: currencyName(c, locale.value) })))

// ---- install ----
const sending = ref(false)
const error = ref('')
const errEl = ref(null)
async function install() {
  if (!canInstall.value) return
  error.value = ''
  sending.value = true
  try {
    const res = await api('POST', `api/apps/${props.app.id}/install`, body())
    emit('started', res)
  } catch (e) {
    error.value = errorMessage(e)
    await nextTick()
    errEl.value?.scrollIntoView({ block: 'nearest' })
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <Drawer class="as-drawer" :label="t('apps.install_title', { app: app.name })" @close="emit('close')">
    <template #head>
      <AppLogo :src="logoSrc(app.logo)" size="lg" />
      <div class="as-dh">
        <h2 class="drawer-title">{{ app.name }}</h2>
        <div class="as-dh-chips">
          <span class="chip chip-text mono">{{ app.version }}</span>
          <span class="chip chip-text">{{ t(`apps.kind_${app.kind}`) }}</span>
          <span class="chip chip-text">{{ te(`apps.cat_${app.category}`) ? t(`apps.cat_${app.category}`) : app.category }}</span>
        </div>
      </div>
      <button type="button" class="icon-btn" :aria-label="t('common.close')" @click="emit('close')"><Icon name="x" /></button>
    </template>

    <form class="as-dform" novalidate @submit.prevent="install">
      <div class="as-dscroll">
        <p class="as-ddesc">{{ tx(app.description, locale) }}</p>
        <p class="as-dlic small">
          <span class="muted">{{ t('apps.licence') }}:</span> {{ app.license }}
          <span v-if="app.source_available" class="tag tag-warn">{{ t('apps.badge_source') }}</span>
          <a v-if="app.homepage" :href="app.homepage" target="_blank" rel="noopener noreferrer" class="as-home">{{ t('apps.homepage') }}<Icon name="external" :size="13" /></a>
        </p>
        <p v-if="app.source_available" class="muted small as-dlic-note">{{ t('apps.source_available') }}</p>

        <div v-if="needsDocker" class="alert alert-warn">
          {{ t('apps.needs_docker_long') }} <RouterLink :to="{ name: 'software' }">{{ t('nav.software') }}</RouterLink>
        </div>
        <div v-if="refusals.length" class="alert alert-crit" role="alert">
          <strong>{{ t('apps.refused_title') }}</strong>
          <ul><li v-for="p in refusals" :key="p.code">{{ problemText(p, t, te) }}</li></ul>
        </div>
        <div v-if="warnings.length" class="alert alert-warn as-warn" role="status">
          <strong>{{ t('apps.warn_title') }}</strong>
          <ul><li v-for="p in warnings" :key="p.code">{{ problemText(p, t, te) }}</li></ul>
        </div>

        <section class="panel">
          <div class="panel-head">
            <h3 class="panel-title"><Icon :name="isPHP ? 'globe' : kind === 'docker' ? 'globe' : 'server'" :size="16" />{{ t(`apps.where_${kind}`) }}</h3>
            <span v-if="checking" class="spinner" :aria-label="t('apps.checking')"></span>
          </div>
          <div class="panel-body stack">
            <!-- a Docker app gets its own new website -->
            <template v-if="kind === 'docker'">
              <label class="field">
                <span>{{ t('apps.domain') }}</span>
                <input v-model="form.domain" class="input mono" placeholder="app.example.com" autocomplete="off" spellcheck="false" autocapitalize="off" required />
                <small class="muted">{{ t('apps.domain_hint') }}</small>
              </label>
              <label class="field">
                <span>{{ t('apps.aliases') }} <span class="muted">({{ t('apps.optional') }})</span></span>
                <input v-model="form.aliases" class="input mono" placeholder="www.app.example.com" autocomplete="off" spellcheck="false" autocapitalize="off" />
              </label>
            </template>

            <!-- a PHP app goes into an empty PHP website -->
            <template v-if="isPHP">
              <div v-if="noSite" class="alert alert-warn">
                {{ app.min_php ? t('apps.no_site', { php: app.min_php }) : t('apps.no_site_any') }}
                <RouterLink :to="{ name: 'websites', query: { new: '1' } }">{{ t('apps.create_site') }}</RouterLink>
              </div>
              <label v-else class="field">
                <span>{{ t('apps.site') }}</span>
                <select v-model="form.site_id" class="input" required>
                  <option v-for="s in candidates" :key="s.id" :value="s.id">{{ s.domain }} (PHP {{ s.php_version }})</option>
                </select>
                <small class="muted">{{ app.composer ? t('apps.site_hint_composer') : app.site_folder ? t('apps.site_hint_site_folder') : t('apps.site_hint') }}</small>
              </label>
              <template v-if="app.admin_form && !noSite">
                <label class="field">
                  <span>{{ t('apps.site_title') }}</span>
                  <input v-model="form.title" class="input" :placeholder="app.name" maxlength="200" />
                </label>
                <div class="row as-two">
                  <label class="field grow">
                    <span>{{ t('apps.admin_user') }}</span>
                    <input v-model="form.admin_user" class="input" autocomplete="off" required maxlength="60" />
                  </label>
                  <label class="field grow">
                    <span>{{ t('apps.admin_email') }}</span>
                    <input v-model="form.admin_email" class="input" type="email" autocomplete="email" required />
                  </label>
                </div>
                <label class="field">
                  <span>{{ t('apps.admin_password') }}</span>
                  <input v-model="form.admin_password" class="input" type="password" autocomplete="new-password" minlength="12" maxlength="128" :placeholder="t('apps.password_auto')" />
                  <small v-if="passwordShort" class="text-crit">{{ t('errors.invalid_app_password') }}</small>
                </label>
              </template>
            </template>

            <label v-if="app.needs_admin_email" class="field">
              <span>{{ t('apps.admin_email') }}</span>
              <input v-model="form.admin_email" class="input" type="email" autocomplete="email" placeholder="you@example.com" required />
              <small class="muted">{{ t('apps.admin_email_hint') }}</small>
            </label>

            <!-- the app's own settings -->
            <label v-for="[k, inp] in inputs" :key="k" class="field">
              <span>{{ tx(inp.label, locale) || k }}<span v-if="inp.optional" class="muted"> ({{ t('apps.optional') }})</span></span>
              <select v-if="inp.type === 'country'" v-model="form.inputs[k]" class="input">
                <option v-for="x in countryList" :key="x.c" :value="x.c">{{ x.n }} ({{ x.c }})</option>
              </select>
              <select v-else-if="inp.type === 'currency'" v-model="form.inputs[k]" class="input">
                <option v-for="x in currencyList" :key="x.c" :value="x.c">{{ x.c }} · {{ x.n }}</option>
              </select>
              <input v-else-if="inp.type === 'port'" v-model="form.inputs[k]" class="input" type="number" min="1024" max="65535" />
              <input v-else v-model="form.inputs[k]" class="input" />
            </label>

            <p v-if="kind === 'service' && !inputs.length" class="muted small as-none">{{ t('apps.service_nothing') }}</p>
            <p v-if="checkError" class="alert alert-crit as-fielderr" role="alert">{{ checkError }}</p>
          </div>
        </section>

        <section class="panel">
          <div class="panel-head"><h3 class="panel-title"><Icon name="memory" :size="16" />{{ t('apps.res_title') }}</h3></div>
          <div class="panel-body as-res">
            <div class="as-res-row">
              <span class="muted">{{ t('apps.res_ram') }}</span>
              <span>
                <strong>{{ t('apps.res_ram_need', { min: mb(r.min_ram_mb, locale), rec: mb(r.recommended_ram_mb, locale) }) }}</strong>
                <span v-if="serverRAM" class="muted"> · {{ t('apps.res_server_has', { size: mb(serverRAM, locale) }) }}</span>
              </span>
            </div>
            <div v-if="limit || r.idle_ram_mb" class="as-res-row">
              <span class="muted">{{ t('apps.res_usage') }}</span>
              <span>
                <template v-if="r.idle_ram_mb">{{ t('apps.res_idle', { size: mb(r.idle_ram_mb, locale) }) }}</template>
                <template v-if="r.idle_ram_mb && limit"> · </template>
                <template v-if="limit">{{ t('apps.res_limit', { size: mb(limit, locale) }) }}</template>
              </span>
            </div>
            <div class="as-res-row">
              <span class="muted">{{ t('apps.res_disk') }}</span>
              <span>
                <strong>{{ t('apps.res_disk_need', { size: mb(needDisk, locale) }) }}</strong>
                <span v-if="serverFree" class="muted"> · {{ t('apps.res_server_free', { size: mb(serverFree, locale) }) }}</span>
              </span>
            </div>
          </div>
        </section>

        <ul v-if="notes.length" class="as-notes">
          <li v-for="(n, i) in notes" :key="i"><Icon name="info" :size="15" /><span>{{ n }}</span></li>
        </ul>
        <p v-if="error" ref="errEl" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
      </div>

      <footer class="as-dfoot">
        <span class="muted small as-dfoot-note">{{ busy ? t('apps.busy_job') : t('apps.install_note') }}</span>
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="!canInstall">
          <span v-if="sending" class="spinner" aria-hidden="true"></span><Icon v-else name="download" :size="15" />{{ t('apps.install') }}
        </button>
      </footer>
    </form>
  </Drawer>
</template>

<style scoped>
.drawer.as-drawer { width: min(640px, 100vw); }
.as-dh { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 6px; }
.as-dh-chips { display: flex; flex-wrap: wrap; gap: 6px; }
.as-dform { flex: 1; min-height: 0; display: flex; flex-direction: column; }
.as-dscroll { flex: 1; min-height: 0; overflow-y: auto; padding: 20px 24px 24px; display: flex; flex-direction: column; gap: 14px; }
.as-dscroll > .alert, .as-dscroll > p { margin: 0; }
.as-ddesc { font-size: 14.5px; color: var(--text); }
.as-dlic { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; }
.as-dlic-note { margin-top: -8px !important; }
.as-home { display: inline-flex; align-items: center; gap: 4px; font-weight: 600; }
.as-two { gap: 12px; }
.as-none, .as-fielderr { margin: 0; }
.as-res { display: flex; flex-direction: column; gap: 8px; font-size: 13.5px; }
.as-res-row { display: grid; grid-template-columns: 150px 1fr; gap: 12px; }
.as-notes { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
.as-notes li { display: flex; gap: 8px; align-items: flex-start; color: var(--text-2); font-size: 13px; line-height: 1.45; }
.as-notes .icon { flex: none; margin-top: 2px; color: var(--accent); }
.as-dfoot {
  display: flex; align-items: center; gap: 8px; flex-wrap: wrap; padding: 12px 24px;
  background: var(--surface); border-top: 1px solid var(--border);
}
.as-dfoot-note { flex: 1 1 200px; }
@media (max-width: 640px) {
  .as-dscroll { padding: 16px; }
  .as-dfoot { padding: 10px 16px; }
  .as-dfoot .btn { flex: 1; justify-content: center; }
  .as-res-row { grid-template-columns: 1fr; gap: 2px; }
}
</style>
