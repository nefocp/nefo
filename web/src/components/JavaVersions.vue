<script setup>
import { ref, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { useJob } from '../composables/useJob.js'
import JobLog from './JobLog.vue'
import RowActions from './RowActions.vue'
import SoftwareRemoveModal from './SoftwareRemoveModal.vue'

// JavaVersions installs and removes the Java (OpenJDK) versions the
// distribution offers, side by side; each app website picks one. stamp
// changes when the Java card installed or removed versions.
const props = defineProps({ stamp: { type: Number, default: 0 } })
const emit = defineEmits(['changed'])
const { t } = useI18n()
const data = ref(null)
const error = ref('')
const removing = ref(null)
const { job, running, watch: watchJob, resume, clear: clearJob } = useJob(async () => {
  await load()
  emit('changed')
})

async function load() {
  try {
    const res = await api('GET', 'api/java-versions')
    data.value = { versions: res.versions || [], in_use: res.in_use || [] }
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}
onMounted(async () => {
  await load()
  resume(['java_version'])
})
watch(() => props.stamp, load)

async function act(v, install) {
  error.value = ''
  removing.value = null
  try {
    watchJob(await api(install ? 'POST' : 'DELETE', `api/java-versions/${v.major}`))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

function actions(v) {
  if (!v.installed) return [{ label: t('software.install'), icon: 'download', inline: true, disabled: running.value || !v.available, onClick: () => act(v, true) }]
  if (data.value.in_use.includes(v.major)) return []
  return [{ label: t('software.uninstall'), icon: 'trash', danger: true, disabled: running.value, onClick: () => (removing.value = v) }]
}
</script>

<template>
  <section class="card">
    <h2>{{ t('software.java_versions') }}</h2>
    <p class="muted small">{{ t('software.java_versions_hint') }}</p>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <JobLog v-if="job" :job="job" @close="clearJob" />
    <p v-if="!data && !error" class="muted small">{{ t('software.loading') }}</p>
    <div v-if="data" class="table-wrap">
      <table class="table">
        <tbody>
          <tr v-for="v in data.versions" :key="v.major">
            <td :title="v.home || undefined"><strong>Java {{ v.major }}</strong> <span class="muted small">{{ v.version }}<template v-if="v.lts"> · LTS</template></span></td>
            <td>
              <span v-if="v.installed" class="tag tag-ok">{{ t('software.installed') }}</span>
              <span v-if="data.in_use.includes(v.major)" class="tag">{{ t('software.node_used') }}</span>
            </td>
            <td class="actions"><RowActions :actions="actions(v)" /></td>
          </tr>
          <tr v-if="!data.versions.length">
            <td class="muted small" colspan="3">{{ t('software.java_none') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <SoftwareRemoveModal
      v-if="removing"
      :name="`Java ${removing.major}`"
      :body="t('software.remove_java_line', { name: `Java ${removing.major}` })"
      :busy="running"
      @confirm="act(removing, false)"
      @close="removing = null"
    />
  </section>
</template>
