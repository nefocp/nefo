<script setup>
import { ref, computed, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { server } from '../server.js'
import Modal from './Modal.vue'
import Icon from './Icon.vue'

// PowerDialog restarts the panel (mode "panel") or reboots the server
// (mode "reboot", the administrator types the server's name). Then it
// waits until the panel answers again and reloads the page.
const props = defineProps({ mode: { type: String, required: true } })
const emit = defineEmits(['close'])
const { t } = useI18n()

const host = computed(() => server.info?.hostname || '')
const typed = ref('')
const busy = ref(false)
const error = ref('')
const waiting = ref(false)
const seconds = ref(0)
let timer = 0
let tick = 0

const reboot = computed(() => props.mode === 'reboot')
const canGo = computed(() => !busy.value && (!reboot.value || typed.value.trim().toLowerCase() === host.value.toLowerCase()))

async function go() {
  error.value = ''
  busy.value = true
  try {
    if (reboot.value) await api('POST', 'api/system/reboot', { confirm: typed.value.trim() })
    else await api('POST', 'api/panel/restart', {})
    waiting.value = true
    tick = setInterval(() => seconds.value++, 1000)
    // The panel stops a few seconds after answering; a reboot takes longer.
    timer = setTimeout(poll, reboot.value ? 20000 : 5000)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function poll() {
  try {
    await api('GET', 'api/system/info')
    window.location.reload()
  } catch {
    if (seconds.value < (reboot.value ? 900 : 180)) timer = setTimeout(poll, 3000)
  }
}

function close() {
  if (!waiting.value) emit('close')
}

onBeforeUnmount(() => {
  clearTimeout(timer)
  clearInterval(tick)
})
</script>

<template>
  <Modal :title="reboot ? t('power.reboot_title', { host }) : t('power.restart_title')" :danger="reboot" @close="close">
    <div v-if="waiting" class="power-wait" role="status" aria-live="polite">
      <span class="spinner power-spin" aria-hidden="true"></span>
      <p class="empty-lead">{{ reboot ? t('power.rebooting') : t('power.restarting') }}</p>
      <p class="muted">{{ reboot ? t('power.rebooting_hint') : t('power.restarting_hint') }}</p>
      <p class="muted small">{{ t('power.waited', { n: seconds }) }}</p>
    </div>
    <form v-else class="stack" @submit.prevent="go">
      <p v-if="error" class="alert alert-crit pre-wrap" role="alert" style="margin: 0">{{ error }}</p>
      <p style="margin: 0">{{ reboot ? t('power.reboot_body') : t('power.restart_body') }}</p>
      <label v-if="reboot" class="field">
        <span>{{ t('power.reboot_type', { host }) }}</span>
        <input v-model="typed" class="input mono" autocomplete="off" autocapitalize="off" spellcheck="false" required />
      </label>
      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button class="btn" :class="reboot ? 'btn-danger btn-solid' : 'btn-primary'" :disabled="!canGo">
          <Icon :name="reboot ? 'power' : 'refresh'" :size="15" />{{ reboot ? t('power.reboot') : t('power.restart') }}
        </button>
      </div>
    </form>
  </Modal>
</template>

<style scoped>
.power-wait { text-align: center; padding: 16px 8px; }
.power-spin { width: 34px; height: 34px; border-width: 3px; color: var(--accent); margin-bottom: 14px; }
</style>
