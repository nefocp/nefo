<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { SERVER_NAMES } from '../webservers.js'

// server is the managed web server, "nginx", "apache" or "openlitespeed":
// the presets and the administrator's directives use its syntax.
// section shows one part (the site drawer has a tab for each): "rewrite"
// (the preset rules) or "custom" (own directives and the generated file).
// Empty shows all. Saving always sends both.
const props = defineProps({ site: { type: Object, required: true }, server: { type: String, default: 'nginx' }, section: { type: String, default: '' } })
const show = (part) => !props.section || props.section === part
const emit = defineEmits(['saved', 'close'])
const { t, te } = useI18n()
// Application names (WordPress, Laravel…) stay as the server sends them;
// descriptive ones are translated.
const presetName = (p) => (te(`nginx.preset_names.${p.id}`) ? t(`nginx.preset_names.${p.id}`) : p.name)

const cfg = ref(null)
const rewrite = ref('')
const custom = ref('')
const error = ref('')
const busy = ref(false)
const isApp = computed(() => !!props.site.proxy_port)
const presets = computed(() => (cfg.value?.presets || []).filter((p) => !p.php || props.site.php_version))
const current = computed(() => presets.value.find((p) => p.id === rewrite.value))
const kind = computed(() => cfg.value?.server || props.server || 'nginx')
const apache = computed(() => kind.value === 'apache')
const ols = computed(() => kind.value === 'openlitespeed')
const serverLabel = computed(() => SERVER_NAMES[kind.value] || 'Nginx')
const placeholder = computed(() => {
  if (apache.value) return '<Location /api/>\n    ProxyPass http://127.0.0.1:3000/api/\n</Location>'
  if (ols.value) {
    const dir = `${props.site.root || '/var/www/example.com'}/public/downloads/`
    return `context /downloads/ {\n  location ${dir}\n  allowBrowse 1\n  extraHeaders <<<END_extraHeaders\nHeader set Cache-Control "max-age=86400"\nEND_extraHeaders\n}`
  }
  return 'location /api/ {\n    proxy_pass http://127.0.0.1:3000;\n}'
})
const presetHint = computed(() =>
  apache.value ? t('webserver.preset_hint_apache') : ols.value ? t('webserver.preset_hint_ols') : t('nginx.preset_hint'),
)
const customHint = computed(() =>
  apache.value ? t('webserver.custom_hint_apache') : ols.value ? t('webserver.custom_hint_ols') : t('nginx.custom_hint'),
)

onMounted(async () => {
  try {
    cfg.value = await api('GET', `api/sites/${props.site.id}/nginx`)
    rewrite.value = cfg.value.rewrite
    custom.value = cfg.value.custom
  } catch (e) {
    error.value = errorMessage(e)
  }
})

async function save() {
  error.value = ''
  busy.value = true
  try {
    cfg.value = await api('PUT', `api/sites/${props.site.id}/nginx`, { rewrite: rewrite.value, custom: custom.value })
    emit('saved', { ...props.site, rewrite: cfg.value.rewrite, nginx_custom: !!cfg.value.custom })
    emit('close')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="!cfg && !error" class="muted">…</div>
  <form v-else class="stack" @submit.prevent="save">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <template v-if="cfg">
      <label v-if="!isApp && show('rewrite')" class="field">
        <span>{{ t('nginx.preset') }}</span>
        <select v-model="rewrite" class="input">
          <option value="">{{ t('nginx.preset_default') }}</option>
          <option v-for="p in presets" :key="p.id" :value="p.id">{{ presetName(p) }}</option>
        </select>
        <small class="muted">{{ presetHint }}</small>
      </label>
      <pre v-if="current && show('rewrite')" class="code-block small">{{ current.rules }}</pre>

      <label v-if="show('custom')" class="field">
        <span>{{ t('webserver.directives', { server: serverLabel }) }}</span>
        <textarea v-model="custom" class="input mono" rows="8" spellcheck="false" :placeholder="placeholder"></textarea>
        <small class="muted">{{ customHint }}</small>
      </label>

      <details v-if="show('custom')">
        <summary>{{ t('nginx.generated') }}</summary>
        <pre class="code-block small">{{ cfg.generated }}</pre>
      </details>
    </template>
    <div class="modal-actions">
      <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button v-if="cfg" class="btn btn-primary" :disabled="busy">{{ t('files.save') }}</button>
    </div>
  </form>
</template>

<style scoped>
.code-block { background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 10px 12px; overflow: auto; max-height: 320px; margin: 0; white-space: pre; }
</style>
