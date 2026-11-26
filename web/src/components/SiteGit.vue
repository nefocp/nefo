<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { useJob } from '../composables/useJob.js'
import CopyField from './CopyField.vue'
import JobLog from './JobLog.vue'

const props = defineProps({ site: { type: Object, required: true } })
const emit = defineEmits(['close'])
const { t, locale } = useI18n()

const info = ref(null)
const form = ref({ repo: '', branch: 'main', target: 'public', script: '' })
const error = ref('')
const notice = ref('')
const busy = ref('')
const { job, running, watch: watchJob, clear: clearJob } = useJob(refresh)

const base = computed(() => `api/sites/${props.site.id}/git`)
// Examples, not translated: they are commands and addresses.
const repoExample = 'https://github.com/owner/repo.git'
const scriptExample = 'composer install --no-dev --optimize-autoloader\nphp artisan migrate --force'
const appScriptExample = 'npm ci\nnpm run build'

function apply(i) {
  info.value = i
  form.value = { repo: i.repo, branch: i.branch, target: i.target, script: i.script }
}

async function load() {
  try {
    apply(await api('GET', base.value))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// refresh shows the result of a finished deploy without dropping changes
// typed into the form meanwhile.
async function refresh() {
  try {
    const i = await api('GET', base.value)
    if (changed.value) info.value = i
    else apply(i)
  } catch {
    // the job log already shows the result
  }
}

// Pick up a deploy of this site that is already running (a push, or
// another browser tab).
async function resumeDeploy() {
  try {
    const { jobs } = await api('GET', 'api/jobs')
    const j = jobs.find((j) => j.kind === 'git_deploy' && j.target === props.site.domain && j.status === 'running')
    if (j) watchJob(j)
  } catch {
    // not important
  }
}

onMounted(async () => {
  await load()
  resumeDeploy()
})

const changed = computed(() => {
  const i = info.value
  if (!i?.configured) return true
  const f = form.value
  return f.repo.trim() !== i.repo || f.branch.trim() !== i.branch || f.target !== i.target || f.script.trim() !== i.script
})
const httpsToken = computed(() => /^https:\/\/[^/]*@/i.test(form.value.repo.trim()))
const lastCommit = computed(() => (info.value?.last_commit || '').slice(0, 12))
const fmt = (v) => new Date(v).toLocaleString(locale.value, { dateStyle: 'medium', timeStyle: 'short' })
const statusClass = { ok: 'tag-ok', failed: 'tag-crit' }

async function act(key, method, path, body, done) {
  error.value = ''
  notice.value = ''
  busy.value = key
  try {
    const res = await api(method, path, body)
    await done?.(res)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = ''
  }
}

function save() {
  const first = !info.value?.configured
  const body = { repo: form.value.repo.trim(), branch: form.value.branch.trim(), target: form.value.target, script: form.value.script }
  return act('save', 'PUT', base.value, body, (i) => {
    apply(i)
    notice.value = first ? t('git.saved_first') : t('git.saved')
  })
}

function deploy() {
  clearJob()
  return act('deploy', 'POST', `${base.value}/deploy`, {}, (j) => watchJob(j))
}

function newKey() {
  if (!window.confirm(t('git.new_key_confirm'))) return
  return act('key', 'POST', `${base.value}/key`, {}, (i) => {
    apply(i)
    notice.value = t('git.key_replaced')
  })
}

function newSecret() {
  if (!window.confirm(t('git.new_secret_confirm'))) return
  return act('secret', 'POST', `${base.value}/secret`, {}, (i) => {
    apply(i)
    notice.value = t('git.secret_replaced')
  })
}

function remove() {
  if (!window.confirm(t('git.remove_confirm', { folder: info.value.folder }))) return
  return act('remove', 'DELETE', base.value, undefined, async () => {
    clearJob()
    await load()
    notice.value = t('git.removed')
  })
}
</script>

<template>
  <div v-if="!info && !error" class="muted">…</div>
  <div v-else class="stack">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>

    <template v-if="info">
      <p class="muted small">{{ t('git.intro') }}</p>

      <form class="stack" @submit.prevent="save">
        <label class="field">
          <span>{{ t('git.repo') }}</span>
          <input v-model="form.repo" class="input mono" required maxlength="300" spellcheck="false" autocomplete="off" :placeholder="repoExample" />
          <small v-if="httpsToken" class="text-crit">{{ t('errors.git_url_credentials') }}</small>
          <small v-else class="muted">{{ t('git.repo_hint') }}</small>
        </label>

        <label class="field">
          <span>{{ t('git.branch') }}</span>
          <input v-model="form.branch" class="input mono input-auto" required maxlength="100" spellcheck="false" autocomplete="off" placeholder="main" />
        </label>

        <!-- an app site runs from app/: its repository always goes there -->
        <fieldset v-if="info.app_folder" class="stack">
          <legend class="field-label">{{ t('git.target') }}</legend>
          <span>{{ t('git.target_app') }} <code class="small">{{ info.app_folder }}</code></span>
          <small class="muted">{{ t('git.target_app_hint') }}</small>
          <small class="muted">{{ t('git.target_app_empty_hint') }}</small>
        </fieldset>
        <fieldset v-else class="stack">
          <legend class="field-label">{{ t('git.target') }}</legend>
          <label class="check">
            <input v-model="form.target" type="radio" value="public" />
            <span>{{ t('git.target_public') }} <code class="small">{{ info.public_folder }}</code></span>
          </label>
          <small class="muted indent">{{ t('git.target_public_hint') }}</small>
          <label class="check">
            <input v-model="form.target" type="radio" value="root" />
            <span>{{ t('git.target_root') }} <code class="small">{{ info.root_folder }}</code></span>
          </label>
          <small class="muted indent">{{ t('git.target_root_hint') }}</small>
          <small class="muted">{{ t('git.target_empty_hint') }}</small>
        </fieldset>

        <label class="field">
          <span>{{ t('git.script') }}</span>
          <textarea v-model="form.script" class="input mono" rows="4" maxlength="2000" spellcheck="false" :placeholder="info.app_folder ? appScriptExample : scriptExample"></textarea>
          <small class="muted">{{ t('git.script_hint') }}</small>
        </label>

        <div class="row">
          <button class="btn" :class="{ 'btn-primary': !info.configured }" :disabled="!!busy || running || !changed">
            {{ busy === 'save' ? t('git.saving') : t('git.save') }}
          </button>
        </div>
      </form>

      <template v-if="info.configured">
        <section class="stack">
          <h3 class="subhead">{{ t('git.deploy') }}</h3>
          <p v-if="info.last_status" class="row small">
            <span class="tag" :class="statusClass[info.last_status]">{{ t(`git.status_${info.last_status}`) }}</span>
            <span v-if="info.last_deploy_at" class="muted">{{ fmt(info.last_deploy_at) }}</span>
            <code v-if="lastCommit">{{ lastCommit }}</code>
            <span v-if="info.last_subject" class="subject">{{ info.last_subject }}</span>
          </p>
          <p v-else class="muted small">{{ t('git.never') }}</p>
          <div class="row">
            <button type="button" class="btn btn-primary" :disabled="!!busy || running || changed" @click="deploy">
              {{ running ? t('git.deploying') : t('git.deploy_now') }}
            </button>
            <small v-if="changed" class="muted">{{ t('git.save_first') }}</small>
          </div>
          <JobLog v-if="job" :job="job" @close="clearJob" />
        </section>

        <section class="stack">
          <h3 class="subhead">{{ t('git.key') }}</h3>
          <p class="muted small">{{ info.ssh ? t('git.key_hint') : t('git.key_hint_https') }}</p>
          <p class="small path-hint">{{ t('git.key_where_github') }}</p>
          <p class="small path-hint">{{ t('git.key_where_gitlab') }}</p>
          <template v-if="info.public_key">
            <CopyField :label="t('git.public_key')" :value="info.public_key" />
            <small class="muted mono">{{ info.key_fingerprint }}</small>
          </template>
          <p v-else class="alert alert-warn">{{ t('git.no_key') }}</p>
          <div class="row">
            <button type="button" class="btn btn-sm" :disabled="!!busy || running" @click="newKey">{{ t('git.new_key') }}</button>
          </div>
        </section>

        <section class="stack">
          <h3 class="subhead">{{ t('git.webhook') }}</h3>
          <p class="muted small">{{ t('git.webhook_hint', { branch: info.branch }) }}</p>
          <p class="small path-hint">{{ t('git.webhook_where_github') }}</p>
          <p class="small path-hint">{{ t('git.webhook_where_gitlab') }}</p>
          <div class="creds">
            <CopyField :label="t('git.webhook_url')" :value="info.webhook_url" />
            <CopyField :label="t('git.webhook_secret')" :value="info.webhook_secret" />
          </div>
          <p v-if="!site.ssl" class="alert alert-warn small">{{ t('git.webhook_no_ssl') }}</p>
          <div class="row">
            <button type="button" class="btn btn-sm" :disabled="!!busy" @click="newSecret">{{ t('git.new_secret') }}</button>
          </div>
        </section>

        <section class="stack">
          <h3 class="subhead">{{ t('git.remove_title') }}</h3>
          <p class="muted small">{{ t('git.remove_hint') }}</p>
          <div class="row">
            <button type="button" class="btn btn-sm btn-danger" :disabled="!!busy || running" @click="remove">{{ t('git.remove') }}</button>
          </div>
        </section>
      </template>
    </template>

    <div class="modal-actions">
      <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.close') }}</button>
    </div>
  </div>
</template>

<style scoped>
fieldset { border: 0; padding: 0; margin: 0; }
.field-label { font-size: 13px; font-weight: 600; margin-bottom: 4px; }
.indent { margin: -4px 0 4px 24px; }
.check code { word-break: break-all; }
.subject { overflow-wrap: anywhere; }
.path-hint { margin: 0; color: var(--text-2); }
textarea.input { resize: vertical; }
</style>
