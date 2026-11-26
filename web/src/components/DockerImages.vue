<script setup>
import { ref, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatDateTime } from '../format.js'
import RowActions from './RowActions.vue'

// DockerImages lists the local images. "Remove unused" runs as a job,
// handed to the page with the "job" event; "changed" follows a removal.
const props = defineProps({
  version: { type: Number, default: 0 },
  busy: { type: Boolean, default: false },
})
const emit = defineEmits(['job', 'changed'])
const { t, locale } = useI18n()

const images = ref(null)
const error = ref('')
const notice = ref('')

async function load() {
  try {
    images.value = (await api('GET', 'api/docker/images')).images
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const label = (i) => (i.repo && i.repo !== '<none>' ? `${i.repo}:${i.tag}` : i.id)

async function remove(i) {
  error.value = ''
  notice.value = ''
  const force = i.in_use
  const question = force
    ? t('docker.image_remove_used', { name: label(i), containers: i.used_by.join(', ') })
    : t('docker.image_remove_confirm', { name: label(i) })
  if (!window.confirm(question)) return
  try {
    await api('POST', 'api/docker/images/remove', { image: i.ref, force })
    notice.value = t('docker.image_removed', { name: label(i) })
    emit('changed')
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function prune() {
  error.value = ''
  notice.value = ''
  if (!window.confirm(t('docker.prune_confirm'))) return
  try {
    emit('job', (await api('POST', 'api/docker/images/prune', {})).job)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

watch(() => props.version, load)
onMounted(load)
defineExpose({ prune })
</script>

<template>
  <div>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <section v-if="images" class="card">
      <div class="card-head">
        <h2>{{ t('docker.images') }}</h2>
      </div>
      <p v-if="!images.length" class="muted">{{ t('docker.no_images') }}</p>
      <div v-else class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>{{ t('docker.image') }}</th>
              <th>{{ t('docker.col_size') }}</th>
              <th>{{ t('docker.col_created') }}</th>
              <th>{{ t('docker.col_used_by') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="i in images" :key="i.ref">
              <td>
                <span class="mono">{{ label(i) }}</span>
                <div class="muted small mono">{{ i.id }}</div>
              </td>
              <td class="nowrap">{{ i.size }}</td>
              <td class="nowrap small" :title="i.created">{{ i.created_at ? formatDateTime(i.created_at, locale) : i.created }}</td>
              <td class="small">
                <span v-if="i.in_use" class="tag tag-info" :title="i.used_by.join(', ')">{{ t('docker.image_in_use', { n: i.used_by.length }) }}</span>
                <span v-else class="muted">{{ t('docker.image_unused') }}</span>
              </td>
              <td class="actions">
                <RowActions :actions="[{ label: t('docker.image_remove'), icon: 'trash', danger: true, inline: true, disabled: busy, onClick: () => remove(i) }]" />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
