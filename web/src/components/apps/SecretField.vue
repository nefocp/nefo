<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '../Icon.vue'

// SecretField shows one sign-in value with a copy button; a secret one is
// masked until "Show" is pressed.
const props = defineProps({
  label: { type: String, required: true },
  value: { type: String, required: true },
  secret: { type: Boolean, default: false },
  link: { type: Boolean, default: false }, // an address that can be opened
})
const { t } = useI18n()
const shown = ref(!props.secret)
const copied = ref(false)

async function copy() {
  try {
    await navigator.clipboard.writeText(props.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    // the clipboard is blocked: Show lets the value be copied by hand
    shown.value = true
  }
}
</script>

<template>
  <div class="sf">
    <div class="sf-label">{{ label }}</div>
    <div class="sf-row">
      <a v-if="link && shown" class="sf-value mono" :href="value" target="_blank" rel="noopener noreferrer">{{ value }}</a>
      <code v-else class="sf-value mono" :class="{ masked: !shown }">{{ shown ? value : '••••••••••••' }}</code>
      <button v-if="secret" type="button" class="icon-btn icon-btn-sm bordered" :aria-label="shown ? t('apps.hide') : t('apps.show')" :title="shown ? t('apps.hide') : t('apps.show')" :aria-pressed="shown" @click="shown = !shown">
        <Icon :name="shown ? 'eyeOff' : 'eye'" :size="15" />
      </button>
      <button type="button" class="icon-btn icon-btn-sm bordered" :aria-label="t('common.copy') + ': ' + label" :title="copied ? t('common.copied') : t('common.copy')" @click="copy">
        <Icon :name="copied ? 'check' : 'copy'" :size="15" />
      </button>
      <span class="sr-only" aria-live="polite">{{ copied ? t('common.copied') : '' }}</span>
    </div>
  </div>
</template>

<style scoped>
.sf { min-width: 0; }
.sf-label { font-size: 12px; color: var(--muted); font-weight: 600; margin-bottom: 4px; }
.sf-row { display: flex; align-items: center; gap: 6px; }
.sf-value {
  flex: 1; min-width: 0; padding: 7px 10px; border-radius: var(--radius-sm); background: var(--surface-2); border: 1px solid var(--border);
  font-size: 12.5px; overflow-wrap: anywhere; color: var(--text);
}
a.sf-value { color: var(--accent); }
.sf-value.masked { letter-spacing: 0.1em; color: var(--muted); }
@media (max-width: 640px) {
  .sf-row .icon-btn { width: 44px; height: 44px; }
}
</style>
