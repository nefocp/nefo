<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

defineProps({
  label: { type: String, required: true },
  value: { type: String, required: true },
})

const { t } = useI18n()
const copied = ref(false)

async function copy(value) {
  try {
    await navigator.clipboard.writeText(value)
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    // clipboard blocked: the value is still visible to copy by hand
  }
}
</script>

<template>
  <div class="copy-field">
    <span class="muted small">{{ label }}</span>
    <div class="row">
      <code class="secret">{{ value }}</code>
      <button type="button" class="btn btn-sm" @click="copy(value)">{{ copied ? t('common.copied') : t('common.copy') }}</button>
    </div>
  </div>
</template>
