<script setup>
import { ref, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { useJob } from '../composables/useJob.js'
import JobLog from './JobLog.vue'
import RowActions from './RowActions.vue'
import SoftwareRemoveModal from './SoftwareRemoveModal.vue'

// NodeVersions installs and removes extra Node.js release lines, which app
// websites can choose instead of the default node. stamp changes when the
// default was installed or removed elsewhere on the page.
const props = defineProps({ stamp: { type: Number, default: 0 } })
const emit = defineEmits(['changed'])
const { t } = useI18n()
const data = ref(null)
const error = ref('')
const removing = ref(null)
const { job, running, watch: watchJob, clear: clearJob } = useJob(async () => {
  await load()
  emit('changed')
})

async function load() {
  try {
    const res = await api('GET', 'api/node-versions')
    data.value = { ...res, versions: res.versions || [], in_use: res.in_use || [] }
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}
onMounted(load)
watch(() => props.stamp, load)

async function act(v, install) {
  error.value = ''
  removing.value = null
  try {
    watchJob(await api(install ? 'POST' : 'DELETE', `api/node-versions/${v.major}`))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

function actions(v) {
  if (!v.installed) return [{ label: t('software.install'), icon: 'download', inline: true, disabled: running.value, onClick: () => act(v, true) }]
  if (v.default || data.value.in_use.includes(v.major)) return []
  return [{ label: t('software.uninstall'), icon: 'trash', danger: true, disabled: running.value, onClick: () => (removing.value = v) }]
}
</script>

<template>
  <section class="card">
    <h2>{{ t('software.node_versions') }}</h2>
    <p class="muted small">{{ t('software.node_versions_hint') }}</p>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />
    <div v-if="data" class="table-wrap">
      <table class="table">
        <tbody>
          <tr v-for="v in data.versions" :key="v.major">
            <td><strong>Node.js {{ v.major }}</strong> <span class="muted small">{{ v.version }}<template v-if="v.lts"> · LTS {{ v.lts }}</template></span></td>
            <td>
              <span v-if="v.default" class="tag tag-info">{{ t('software.node_default') }}</span>
              <span v-else-if="v.installed" class="tag tag-ok">{{ t('software.installed') }}</span>
              <span v-if="data.in_use.includes(v.major)" class="tag">{{ t('software.node_used') }}</span>
            </td>
            <td class="actions"><RowActions :actions="actions(v)" /></td>
          </tr>
        </tbody>
      </table>
    </div>
    <SoftwareRemoveModal
      v-if="removing"
      :name="`Node.js ${removing.major}`"
      :body="t('software.remove_node_line', { name: `Node.js ${removing.major}`, dir: (removing.path || '').replace(/\/bin$/, '') })"
      :busy="running"
      @confirm="act(removing, false)"
      @close="removing = null"
    />
  </section>
</template>
