<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { useJob } from '../composables/useJob.js'
import CopyField from './CopyField.vue'
import Icon from './Icon.vue'
import Modal from './Modal.vue'

// SiteCloneModal copies a website to a new domain: PHP, static, reverse
// proxy, app (Node.js, Python…) and Docker app websites. The copy runs as a
// job: "started" hands it to the drawer, which shows its log;
// the modal follows it too, and when it is done shows the new site and the
// connection details of the copied databases.
const props = defineProps({
  site: { type: Object, required: true },
})
const emit = defineEmits(['close', 'started'])

const { t } = useI18n()
const domain = ref('')
const copyDBs = ref(true)
const busy = ref(false)
const error = ref('')
const started = ref('') // the new domain, once the job runs
const creds = ref([])
const isWP = computed(() => props.site.app === 'wordpress' && !!props.site.php_version)
const isDocker = computed(() => (props.site.app || '').startsWith('docker:'))
const isApp = computed(() => !!props.site.proxy_port && !isDocker.value && !props.site.proxy_url)
const runtimeName = computed(() => {
  const rt = props.site.app_runtime || ''
  if (rt === 'python') return 'Python'
  if (rt.startsWith('node:')) return 'Node.js ' + rt.slice(5)
  if (!rt) return 'Node.js'
  const [name, ver] = rt.split(':')
  return name.charAt(0).toUpperCase() + name.slice(1) + (ver ? ' ' + ver : '')
})
const { job, watch: watchJob } = useJob(onDone)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    const res = await api('POST', `api/sites/${props.site.id}/clone`, { domain: domain.value.trim(), copy_databases: copyDBs.value })
    started.value = res.domain
    watchJob(res.job)
    emit('started', res.job)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function onDone(j) {
  if (j.status !== 'succeeded' || isWP.value || isDocker.value) return
  try {
    const { databases } = await api('GET', 'api/databases')
    const mine = (databases || []).filter((d) => d.site_domain === started.value)
    creds.value = await Promise.all(mine.map((d) => api('GET', `api/databases/${d.id}/credentials`)))
  } catch {
    // the Databases page has them too
  }
}
</script>

<template>
  <Modal :title="t('clone.title', { domain: site.domain })" @close="emit('close')">
    <form v-if="!started" class="stack" @submit.prevent="submit">
      <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
      <p>{{ t('clone.body') }}</p>
      <label class="field">
        <span>{{ t('clone.domain') }}</span>
        <input v-model="domain" class="input mono" placeholder="copy.example.com" autocomplete="off" autocapitalize="off" spellcheck="false" required maxlength="253" />
        <small class="muted">{{ t('clone.domain_hint') }}</small>
      </label>
      <label v-if="!isWP && !isDocker" class="check"><input v-model="copyDBs" type="checkbox" /><span>{{ t('clone.copy_dbs') }}</span></label>
      <ul class="small muted clone-steps">
        <template v-if="isDocker">
          <li>{{ t('clone.step_docker') }}</li>
          <li>{{ t('clone.step_docker_data') }}</li>
          <li>{{ t('clone.step_docker_url') }}</li>
        </template>
        <template v-else>
          <li v-if="site.proxy_url">{{ t('clone.step_site_proxy', { url: site.proxy_url }) }}</li>
          <li v-else-if="isApp">{{ t('clone.step_site_app', { runtime: runtimeName }) }}</li>
          <li v-else>{{ site.php_version ? t('clone.step_site_php', { v: site.php_version }) : t('clone.step_site_static') }}</li>
          <li>{{ isWP ? t('clone.step_files_wp') : t('clone.step_files') }}</li>
          <li v-if="isWP">{{ t('clone.step_wp') }}</li>
          <li v-else-if="copyDBs">{{ t('clone.step_dbs') }}</li>
          <li v-if="isApp">{{ t('clone.step_app_daemons') }}</li>
        </template>
        <li>{{ isApp ? t('clone.step_not_app') : t('clone.step_not') }}</li>
      </ul>
      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="busy || !domain.trim()"><Icon name="copy" />{{ t('clone.start') }}</button>
      </div>
    </form>

    <div v-else class="stack">
      <p v-if="job?.status === 'running'" role="status"><span class="spinner" aria-hidden="true"></span> {{ t('clone.running', { domain: started }) }}</p>
      <p v-else-if="job?.status === 'failed'" class="alert alert-crit pre-wrap" role="alert">{{ job.error || t('errors.unknown') }}</p>
      <template v-else-if="job?.status === 'succeeded'">
        <p role="status">{{ t('clone.done', { domain: started }) }}</p>
        <template v-if="creds.length">
          <p class="alert alert-warn">{{ t('clone.config_warning') }}</p>
          <div v-for="c in creds" :key="c.id" class="stack clone-db">
            <CopyField :label="t('databases.col_name')" :value="c.name" />
            <CopyField :label="t('databases.col_user')" :value="c.username" />
            <CopyField :label="t('databases.password')" :value="c.password" />
            <CopyField :label="t('databases.host')" :value="c.host" />
          </div>
        </template>
      </template>
      <div class="modal-actions">
        <a v-if="job?.status === 'succeeded'" class="btn" :href="`http://${started}/`" target="_blank" rel="noopener noreferrer"><Icon name="external" />{{ t('websites.open_site') }}</a>
        <button type="button" class="btn btn-primary" @click="emit('close')">{{ t('common.close') }}</button>
      </div>
    </div>
  </Modal>
</template>

<style scoped>
.clone-steps { margin: 0; padding-left: 18px; }
.clone-steps li { margin: 2px 0; }
.clone-db { gap: 8px; padding-top: 12px; border-top: 1px solid var(--border); }
</style>
