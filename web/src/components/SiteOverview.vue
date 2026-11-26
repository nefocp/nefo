<script setup>
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { daysLeft } from '../attention.js'
import { appName } from '../logos.js'
import { SERVER_NAMES } from '../webservers.js'
import Icon from './Icon.vue'
import SiteCloneModal from './SiteCloneModal.vue'

// SiteOverview is the first tab of the site drawer: the facts about a site
// and its quick switches. Changes go up as events; the drawer calls the API.
const props = defineProps({
  site: { type: Object, required: true },
  server: { type: String, default: '' },
  busy: { type: Boolean, default: false },
  database: { type: Object, default: null },
})
const emit = defineEmits(['go', 'force-https', 'suspend', 'app', 'deps', 'delete', 'install-wp', 'note', 'job'])
const { t, locale } = useI18n()

const fmtDate = (v) => new Date(v).toLocaleDateString(locale.value, { year: 'numeric', month: 'short', day: 'numeric' })
const days = computed(() => daysLeft(props.site.cert_expires))
const isDocker = computed(() => (props.site.app || '').startsWith('docker:'))
const hasService = computed(() => !!props.site.app_command || isDocker.value)
const runtime = computed(() => {
  const s = props.site
  if (s.php_version) return `PHP ${s.php_version}`
  if (isDocker.value) return `Docker · ${appName(s.app)}`
  if (s.proxy_port) return (s.app_runtime === 'python' ? 'Python' : s.app_runtime?.startsWith('node:') ? `Node.js ${s.app_runtime.slice(5)}` : 'Node.js') + ` · ${t('websites.app_on', { port: s.proxy_port })}`
  return t('websites.static')
})
const sslText = computed(() => {
  const s = props.site
  if (!s.ssl) return t('websites.https_off')
  if (s.ssl_error) return t('drawer.ssl_renew_failed')
  if (days.value !== null && days.value < 0) return t('drawer.ssl_expired')
  return t('drawer.ssl_days', { n: days.value ?? '?' }, days.value ?? 2) + (s.ssl_source === 'custom' ? ' · ' + t('ssl.own_short') : ' · Let’s Encrypt')
})
const sslClass = computed(() => {
  const s = props.site
  if (!s.ssl) return 'muted'
  if (s.ssl_error || (days.value !== null && days.value < 0)) return 'text-crit'
  if (days.value !== null && days.value <= 14) return 'text-warn'
  return 'text-ok'
})
// The note: one line of the administrator's own text, shown under the domain
// in the websites table. Saved by the drawer (PUT api/sites/{id}/note).
const editingNote = ref(false)
const noteDraft = ref('')
const noteInput = ref(null)
function editNote() {
  noteDraft.value = props.site.note || ''
  editingNote.value = true
  nextTick(() => noteInput.value?.focus())
}
function saveNote() {
  const v = noteDraft.value.trim()
  editingNote.value = false
  if (v !== (props.site.note || '')) emit('note', v)
}

const cloning = ref(false)
const wafText = computed(() => (props.site.waf === 'block' ? t('drawer.waf_block') : props.site.waf === 'detect' ? t('drawer.waf_detect') : t('drawer.off')))
</script>

<template>
  <div class="stack">
    <section class="card">
      <form v-if="editingNote" class="ov-note" @submit.prevent="saveNote" @keydown.esc.stop="editingNote = false">
        <label class="sr-only" for="ov-note-input">{{ t('drawer.note') }}</label>
        <input id="ov-note-input" ref="noteInput" v-model="noteDraft" class="input" maxlength="200" :placeholder="t('drawer.note_ph')" autocomplete="off" />
        <button type="submit" class="btn btn-primary btn-sm" :disabled="busy">{{ t('drawer.note_save') }}</button>
        <button type="button" class="btn btn-ghost btn-sm" @click="editingNote = false">{{ t('drawer.note_cancel') }}</button>
      </form>
      <div v-else class="ov-note">
        <Icon name="edit" :size="15" class="muted" />
        <span v-if="site.note" class="ov-note-text">{{ site.note }}</span>
        <span v-else class="muted">{{ t('drawer.note_none') }}</span>
        <button type="button" class="link-btn small" @click="editNote">{{ site.note ? t('drawer.edit') : t('drawer.note_add') }}</button>
      </div>
      <dl class="kv ov-kv">
        <div>
          <dt>{{ t('drawer.root') }}</dt>
          <dd class="mono ov-path">{{ site.root }}</dd>
          <RouterLink class="small" :to="{ name: 'files', query: { path: site.root } }">{{ t('drawer.open_files') }}</RouterLink>
        </div>
        <div><dt>{{ t('drawer.sys_user') }}</dt><dd class="mono">{{ site.sys_user }}</dd></div>
        <div>
          <dt>{{ t('drawer.domains') }}</dt>
          <dd>{{ site.domain }}<template v-if="(site.aliases || []).length"> + {{ (site.aliases || []).length }}</template></dd>
          <button type="button" class="link-btn small" @click="emit('go', 'domains')">{{ t('drawer.edit') }}</button>
        </div>
        <div>
          <dt>{{ site.php_version ? 'PHP' : t('drawer.runtime') }}</dt>
          <dd>{{ runtime }}</dd>
          <button v-if="site.php_version || (site.proxy_port && !isDocker)" type="button" class="link-btn small" @click="emit('go', site.php_version ? 'php' : 'runtime')">{{ t('drawer.edit') }}</button>
        </div>
        <div v-if="server"><dt>{{ t('drawer.web_server') }}</dt><dd>{{ SERVER_NAMES[server] || server }}</dd></div>
        <div>
          <dt>HTTPS</dt>
          <dd :class="sslClass">{{ sslText }}</dd>
          <button type="button" class="link-btn small" @click="emit('go', 'ssl')">{{ site.ssl ? t('drawer.manage') : t('websites.https_enable') }}</button>
        </div>
        <div><dt>{{ t('drawer.waf') }}</dt><dd>{{ wafText }}</dd></div>
        <div><dt>{{ t('drawer.tab_sftp') }}</dt><dd>{{ site.sftp ? t('drawer.on') : t('drawer.off') }}</dd></div>
        <div v-if="database"><dt>{{ t('nav.databases') }}</dt><dd class="mono">{{ database.name }}</dd></div>
        <div><dt>{{ t('drawer.created') }}</dt><dd>{{ fmtDate(site.created_at) }}</dd></div>
        <div v-if="site.expires_at"><dt>{{ t('drawer.expires') }}</dt><dd>{{ fmtDate(new Date(new Date(site.expires_at).getTime() - 86400000)) }}</dd></div>
      </dl>
    </section>

    <div class="ov-opts">
      <div class="opt">
        <div class="opt-text">
          <div class="opt-title">{{ t('drawer.running') }}</div>
          <div class="opt-hint">{{ site.disabled ? t('drawer.running_off_hint') : t('drawer.running_on_hint') }}</div>
        </div>
        <button type="button" class="switch switch-ok" role="switch" :aria-checked="!site.disabled" :aria-label="t('drawer.running')" :disabled="busy" @click="emit('suspend', !site.disabled)"></button>
      </div>
      <div class="opt">
        <div class="opt-text">
          <div class="opt-title">{{ t('websites.force_https') }}</div>
          <div class="opt-hint">{{ site.ssl ? t('drawer.force_https_hint') : t('drawer.force_https_needs_ssl') }}</div>
        </div>
        <button type="button" class="switch" role="switch" :aria-checked="!!site.force_https" :aria-label="t('websites.force_https')" :disabled="!site.ssl || busy" @click="emit('force-https')"></button>
      </div>
    </div>

    <section v-if="hasService" class="card">
      <div class="card-head">
        <h3>{{ t('drawer.app_service') }}</h3>
        <span class="status-pill" :class="site.app_status === 'active' ? 'ok' : 'crit'">{{ site.app_status || '—' }}</span>
      </div>
      <div class="row">
        <button type="button" class="btn btn-sm" @click="emit('app', site.app_status === 'active' ? 'restart' : 'start')"><Icon name="play" :size="14" />{{ site.app_status === 'active' ? t('services.restart') : t('services.start') }}</button>
        <button v-if="site.app_status === 'active'" type="button" class="btn btn-sm" @click="emit('app', 'stop')"><Icon name="stop" :size="14" />{{ t('services.stop') }}</button>
        <button v-if="site.app_command" type="button" class="btn btn-sm" :disabled="busy" @click="emit('deps')"><Icon name="download" :size="14" />{{ t('websites.app_deps') }}</button>
        <button type="button" class="btn btn-sm btn-ghost" @click="emit('go', 'logs')"><Icon name="logs" :size="14" />{{ t('websites.app_logs') }}</button>
      </div>
    </section>

    <section v-if="site.php_version && !site.app" class="card">
      <div class="card-head" style="margin: 0">
        <div>
          <h3 style="margin: 0 0 2px">{{ t('drawer.install_app') }}</h3>
          <p class="muted small" style="margin: 0">{{ t('drawer.install_app_hint') }}</p>
        </div>
        <div class="row">
          <button type="button" class="btn" :disabled="busy" @click="emit('install-wp')"><Icon name="wordpress" :size="15" />{{ t('websites.wp_install') }}</button>
          <RouterLink :to="{ name: 'apps' }" class="btn btn-ghost">{{ t('nav.apps') }}<Icon name="chevronRight" :size="15" /></RouterLink>
        </div>
      </div>
    </section>

    <section v-if="!site.proxy_port && !isDocker" class="card">
      <div class="card-head" style="margin: 0">
        <div>
          <h3 style="margin: 0 0 2px">{{ t('clone.card') }}</h3>
          <p class="muted small" style="margin: 0">{{ t('clone.card_hint') }}</p>
        </div>
        <button type="button" class="btn" :disabled="busy" @click="cloning = true"><Icon name="copy" :size="15" />{{ t('clone.open') }}</button>
      </div>
    </section>
    <SiteCloneModal v-if="cloning" :site="site" @close="cloning = false" @started="(j) => emit('job', j)" />

    <section class="card card-danger">
      <div class="card-head">
        <div>
          <h3 style="margin: 0 0 2px">{{ t('drawer.danger') }}</h3>
          <p class="muted small" style="margin: 0">{{ t('websites.delete_body') }}</p>
        </div>
        <button type="button" class="btn btn-danger" :disabled="busy" @click="emit('delete')"><Icon name="trash" :size="15" />{{ t('websites.delete') }}</button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.ov-kv { grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); }
.ov-kv dd { margin-bottom: 2px; }
.ov-path { font-weight: 500; font-size: 12.5px; }
.ov-opts { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 12px; }
.card-danger .card-head { margin: 0; }
.ov-note { display: flex; align-items: center; gap: 8px; min-height: 34px; padding-bottom: 12px; margin-bottom: 14px; border-bottom: 1px solid var(--border); }
.ov-note .input { flex: 1; min-width: 0; }
.ov-note-text { font-weight: 500; overflow-wrap: anywhere; }
@media (max-width: 640px) {
  form.ov-note { flex-wrap: wrap; }
  form.ov-note .input { flex-basis: 100%; }
}
</style>
