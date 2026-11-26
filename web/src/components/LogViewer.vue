<script setup>
import { ref, watch, nextTick, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import Icon from './Icon.vue'

// LogViewer shows the end of a log; put it inside a Modal. url(kind) returns
// the API path for the chosen kind; kinds is a list of { value, label }.
const props = defineProps({
  url: { type: Function, required: true },
  kinds: { type: Array, default: () => [] },
})
const { t } = useI18n()

const kind = ref(props.kinds[0]?.value ?? '')
const text = ref('')
const error = ref('')
const loading = ref(false)
const pre = ref(null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    text.value = (await api('GET', props.url(kind.value))).log
    await nextTick()
    if (pre.value) pre.value.scrollTop = pre.value.scrollHeight
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

watch(kind, load)
onMounted(load)
</script>

<template>
  <div>
    <div class="row toolbar">
      <select v-if="kinds.length" v-model="kind" class="input input-sm">
        <option v-for="k in kinds" :key="k.value" :value="k.value">{{ k.label }}</option>
      </select>
      <span class="toolbar-gap"></span>
      <button class="btn btn-sm" :disabled="loading" @click="load"><Icon name="refresh" :size="15" />{{ t('files.refresh') }}</button>
    </div>
    <p v-if="error" class="alert alert-crit" role="alert">{{ error }}</p>
    <pre ref="pre" class="log log-tall" tabindex="0">{{ text || t('logs.empty') }}</pre>
  </div>
</template>
