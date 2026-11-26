<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import Icon from './Icon.vue'

// FilesDelete confirms the deletion of `paths`. Items go to the trash
// unless "Delete permanently" is ticked. v-model:permanent lets the parent
// colour its modal. It emits "done" with { n, permanent } on success.
const props = defineProps({
  paths: { type: Array, required: true },
  permanent: { type: Boolean, default: false },
})
const emit = defineEmits(['done', 'close', 'update:permanent'])
const { t } = useI18n()

const busy = ref(false)
const error = ref('')
const shown = computed(() => props.paths.slice(0, 8).map((p) => p.slice(p.lastIndexOf('/') + 1)))
const more = computed(() => Math.max(0, props.paths.length - shown.value.length))
const permanentModel = computed({
  get: () => props.permanent,
  set: (v) => emit('update:permanent', v),
})

async function remove() {
  error.value = ''
  busy.value = true
  const permanent = props.permanent
  try {
    await api('POST', permanent ? 'api/files/delete-permanently' : 'api/files/delete', { paths: props.paths })
    emit('done', { n: props.paths.length, permanent })
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="stack" @submit.prevent="remove">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <ul class="names mono small">
      <li v-for="n in shown" :key="n">{{ n }}</li>
      <li v-if="more" class="muted">{{ t('files.delete_more', { n: more }) }}</li>
    </ul>
    <p v-if="!permanent" class="muted small">{{ t('files.delete_to_trash') }}</p>
    <label class="check"><input v-model="permanentModel" type="checkbox" /><span>{{ t('files.delete_permanent') }}</span></label>
    <p v-if="permanent" class="alert alert-warn">{{ t('files.delete_permanent_warn') }}</p>
    <div class="modal-actions">
      <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button class="btn btn-danger" :class="{ 'btn-solid': permanent }" :disabled="busy">
        <Icon name="trash" :size="15" />{{ permanent ? t('files.delete_now') : t('files.delete_move') }}
      </button>
    </div>
  </form>
</template>

<style scoped>
.names { margin: 0; padding-left: 18px; max-height: 180px; overflow: auto; overflow-wrap: anywhere; }
</style>
