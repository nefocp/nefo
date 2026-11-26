<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Modal from './Modal.vue'
import Icon from './Icon.vue'

// SoftwareRemoveModal asks before an uninstall and says what it does.
// dataLabel offers to delete the data folder too, which then needs dataWord
// typed; confirmWord, when set, must always be typed.
const props = defineProps({
  name: { type: String, required: true },
  body: { type: String, required: true },
  dataLabel: { type: String, default: '' },
  dataWord: { type: String, default: '' },
  confirmWord: { type: String, default: '' },
  busy: { type: Boolean, default: false },
})
const emit = defineEmits(['confirm', 'close'])
const { t } = useI18n()

const deleteData = ref(false)
const typed = ref('')
const word = computed(() => props.confirmWord || (deleteData.value ? props.dataWord : ''))
const ok = computed(() => !word.value || typed.value.trim() === word.value)

function submit() {
  if (!ok.value) return
  emit('confirm', { delete_data: deleteData.value, confirm: word.value ? typed.value.trim() : '' })
}
</script>

<template>
  <Modal danger :title="t('software.uninstall_title', { name })" @close="emit('close')">
    <form class="stack" @submit.prevent="submit">
      <p class="pre-wrap sw-body">{{ body }}</p>
      <label v-if="dataLabel" class="check">
        <input v-model="deleteData" type="checkbox" />
        <span>{{ dataLabel }}</span>
      </label>
      <p v-if="deleteData" class="alert alert-crit sw-warn" role="alert"><Icon name="alert" :size="15" />{{ t('software.delete_data_warn') }}</p>
      <label v-if="word" class="field">
        <span>{{ t('software.type_to_confirm', { word }) }}</span>
        <input v-model="typed" class="input mono" autocomplete="off" spellcheck="false" autocapitalize="off" required />
      </label>
      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button class="btn btn-danger btn-solid" :disabled="busy || !ok"><Icon name="trash" :size="15" />{{ t('software.uninstall') }}</button>
      </div>
    </form>
  </Modal>
</template>

<style scoped>
.sw-body { margin: 0; line-height: 1.55; }
.sw-warn { display: flex; gap: 8px; align-items: flex-start; margin: 0; }
.sw-warn .icon { flex: none; margin-top: 2px; }
</style>
