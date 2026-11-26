<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'

// section shows one part of the form (the site drawer has a tab for each):
// "domains", "php", "runtime" or "proxy". Empty shows all. Saving always
// sends every field, so the parts not shown keep their values.
const props = defineProps({
  site: { type: Object, required: true },
  phpVersions: { type: Array, required: true },
  // Versions that get no security updates on this server (the web status's "eol").
  phpEol: { type: Array, default: () => [] },
  section: { type: String, default: '' },
})
const show = (part) => !props.section || props.section === part
const emit = defineEmits(['saved', 'close', 'reissue'])
const { t } = useI18n()

const form = ref({
  aliases: (props.site.aliases || []).join('\n'),
  php_version: props.site.php_version,
  php_memory: props.site.php_memory,
  php_upload: props.site.php_upload,
  php_time: props.site.php_time,
  php_workers: props.site.php_workers || 5,
  php_pm: props.site.php_pm || 'ondemand',
  php_allow: [...(props.site.php_allow || [])],
  proxy_port: props.site.proxy_port,
  app_command: props.site.app_command,
  app_runtime: props.site.app_runtime || '',
})
// Runtimes the server has: installed Node.js lines, and Python if venv works.
const runtimes = ref([])
if (props.site.proxy_port && !(props.site.app || '').startsWith('docker:')) {
  Promise.all([api('GET', 'api/node-versions').catch(() => ({ versions: [] })), api('GET', 'api/software').catch(() => ({ items: [] }))])
    .then(([n, sw]) => {
      runtimes.value = n.versions.filter((v) => v.installed && !v.default).map((v) => ({ value: `node:${v.major}`, label: `Node.js ${v.major} (${v.version})` }))
      if (sw.items.some((i) => i.id === 'python' && i.installed)) runtimes.value.push({ value: 'python', label: 'Python (app/.venv)' })
    })
}
// Disabled for every site unless allowed back (internal/hosting HardenedFunctions).
const hardened = ['exec', 'passthru', 'shell_exec', 'system', 'proc_open', 'popen', 'pcntl_exec']
const error = ref('')
const busy = ref(false)
const reissue = ref(false)

async function save() {
  error.value = ''
  busy.value = true
  try {
    const res = await api('PUT', `api/sites/${props.site.id}/settings`, {
      aliases: form.value.aliases.split(/[\s,]+/).map((a) => a.trim().toLowerCase()).filter(Boolean),
      php_version: form.value.php_version,
      php_memory: Number(form.value.php_memory),
      php_upload: Number(form.value.php_upload),
      php_time: Number(form.value.php_time),
      ...(form.value.php_version ? { php_workers: Number(form.value.php_workers), php_pm: form.value.php_pm, php_allow: form.value.php_allow } : {}),
      proxy_port: Number(form.value.proxy_port) || 0,
      app_command: (form.value.app_command || '').trim(),
      app_runtime: form.value.app_runtime,
    })
    reissue.value = res.reissue_needed
    emit('saved', res.site)
    if (!res.reissue_needed) emit('close')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <div v-if="reissue" class="alert alert-warn row">
      <span>{{ t('websites.reissue_needed') }}</span>
      <button class="btn btn-sm" @click="emit('reissue', site); emit('close')">{{ t('websites.reissue') }}</button>
    </div>
    <form class="stack" @submit.prevent="save">
      <label v-if="show('domains')" class="field">
        <span>{{ t('websites.aliases') }}</span>
        <textarea v-model="form.aliases" class="input mono" rows="3" spellcheck="false" autocapitalize="off"></textarea>
        <small class="muted">{{ t('websites.aliases_hint') }}</small>
      </label>
      <template v-if="site.proxy_port">
        <label v-if="show('proxy')" class="field">
          <span>{{ t('websites.app_port') }}</span>
          <input v-model="form.proxy_port" class="input input-auto" type="number" min="1024" max="65535" required />
          <small v-if="section === 'proxy'" class="muted">{{ t('drawer.proxy_hint', { port: form.proxy_port || '…' }) }}</small>
        </label>
        <label v-if="show('runtime')" class="field">
          <span>{{ t('websites.app_command') }}</span>
          <input v-model="form.app_command" class="input mono" maxlength="1000" />
          <small class="muted">{{ t('websites.app_hint') }}</small>
        </label>
        <label v-if="show('runtime') && !(site.app || '').startsWith('docker:')" class="field">
          <span>{{ t('websites.app_runtime') }}</span>
          <select v-model="form.app_runtime" class="input">
            <option value="">{{ t('websites.app_runtime_default') }}</option>
            <option v-for="r in runtimes" :key="r.value" :value="r.value">{{ r.label }}</option>
          </select>
          <small class="muted">{{ t('websites.app_runtime_hint') }}</small>
        </label>
      </template>
      <template v-if="site.php_version && show('php')">
        <label class="field">
          <span>{{ t('websites.php_version') }}</span>
          <span class="row php-pick">
            <select v-model="form.php_version" class="input input-auto">
              <option v-for="v in phpVersions" :key="v" :value="v">PHP {{ v }}{{ phpEol.includes(v) ? ` (${t('software.eol_short')})` : '' }}</option>
            </select>
            <span v-if="phpEol.includes(form.php_version)" class="tag tag-crit">{{ t('software.eol_badge') }}</span>
          </span>
          <small v-if="phpEol.includes(form.php_version)" class="muted">{{ t('software.eol_site_hint') }}</small>
        </label>
        <div class="row">
          <label class="field grow">
            <span>{{ t('websites.php_memory') }}</span>
            <input v-model="form.php_memory" class="input" type="number" min="32" max="4096" required />
          </label>
          <label class="field grow">
            <span>{{ t('websites.php_upload') }}</span>
            <input v-model="form.php_upload" class="input" type="number" min="1" max="4096" required />
          </label>
          <label class="field grow">
            <span>{{ t('websites.php_time') }}</span>
            <input v-model="form.php_time" class="input" type="number" min="10" max="3600" required />
          </label>
        </div>
        <div class="row">
          <label class="field grow">
            <span>{{ t('websites.php_workers') }}</span>
            <input v-model="form.php_workers" class="input" type="number" min="1" max="200" required />
          </label>
          <label class="field grow">
            <span>{{ t('websites.php_pm') }}</span>
            <select v-model="form.php_pm" class="input">
              <option value="ondemand">{{ t('websites.php_pm_ondemand') }}</option>
              <option value="dynamic">{{ t('websites.php_pm_dynamic') }}</option>
            </select>
          </label>
        </div>
        <small class="muted">{{ t('websites.php_workers_hint') }}</small>
        <fieldset class="php-allow">
          <legend class="small">{{ t('websites.php_allow') }}</legend>
          <label v-for="f in hardened" :key="f" class="check"><input v-model="form.php_allow" type="checkbox" :value="f" /><span class="mono">{{ f }}()</span></label>
          <small class="muted">{{ t('websites.php_allow_hint') }}</small>
        </fieldset>
      </template>
      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="busy">{{ t('files.save') }}</button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.php-allow { border: 0; padding: 0; margin: 0; display: flex; flex-wrap: wrap; gap: 4px 16px; }
.php-pick { align-items: center; flex-wrap: wrap; gap: 8px; }
.php-allow legend { font-weight: 600; margin-bottom: 4px; width: 100%; }
</style>
