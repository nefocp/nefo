<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import Icon from './Icon.vue'

// DockerTerminal is a shell inside a running container, for a wide Modal.
// Import it with defineAsyncComponent: the terminal library is large.
const props = defineProps({
  container: { type: String, required: true }, // ID or name
})
const { t } = useI18n()
const el = ref(null)
const state = ref('connecting') // connecting, open, closed
const reason = ref('') // error code from the server's close frame
let term = null
let fit = null
let socket = null
let observer = null
let dialog = null

function send(msg) {
  if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(msg))
}

function sendSize() {
  if (term) send({ t: 'r', r: term.rows, c: term.cols })
}

function connect() {
  state.value = 'connecting'
  reason.value = ''
  const url = new URL(`api/docker/containers/${encodeURIComponent(props.container)}/terminal`, window.location.href)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  url.hash = ''
  socket = new WebSocket(url)
  socket.binaryType = 'arraybuffer'
  socket.onopen = () => {
    state.value = 'open'
    sendSize()
    term.focus()
  }
  socket.onmessage = (e) => term.write(new Uint8Array(e.data))
  socket.onclose = (e) => {
    // Codes from 4000 carry the reason the server refused the shell.
    if (e.code >= 4000 && e.reason) reason.value = e.reason
    if (state.value !== 'closed') term.write(`\r\n\x1b[90m${t('terminal.closed')}\x1b[0m\r\n`)
    state.value = 'closed'
  }
}

function reconnect() {
  term.reset()
  connect()
}

// Escape belongs to the shell (vi, less…): it must not close the dialog
// while the terminal has the focus. The close button still works.
function keepEscape(e) {
  if (el.value?.contains(document.activeElement)) {
    e.preventDefault()
    e.stopImmediatePropagation()
  }
}

onMounted(() => {
  term = new Terminal({
    cursorBlink: true,
    fontFamily: "ui-monospace, 'Cascadia Mono', Menlo, Consolas, monospace",
    fontSize: 14,
    scrollback: 5000,
    theme: { background: '#0b1020', foreground: '#e2e8f0', cursor: '#a5b4fc', selectionBackground: '#334155' },
  })
  fit = new FitAddon()
  term.loadAddon(fit)
  term.open(el.value)
  term.onData((d) => send({ t: 'i', d }))
  // The dialog may still be opening: fit once it has a size.
  observer = new ResizeObserver(() => {
    if (!el.value?.clientWidth) return
    fit.fit()
    sendSize()
  })
  observer.observe(el.value)
  dialog = el.value.closest('dialog')
  dialog?.addEventListener('cancel', keepEscape, true)
  connect()
})

onBeforeUnmount(() => {
  observer?.disconnect()
  dialog?.removeEventListener('cancel', keepEscape, true)
  state.value = 'closed'
  socket?.close()
  term?.dispose()
})
</script>

<template>
  <div>
    <div class="row toolbar">
      <span class="tag" :class="state === 'open' ? 'tag-ok' : state === 'closed' ? 'tag-crit' : 'tag-warn'">
        <span class="dot" :class="state === 'open' ? 'dot-ok' : state === 'closed' ? 'dot-crit' : 'dot-warn'"></span>{{ t(`terminal.${state}`) }}
      </span>
      <span class="toolbar-gap"></span>
      <button v-if="state === 'closed'" class="btn btn-sm" @click="reconnect"><Icon name="refresh" :size="15" />{{ t('terminal.reconnect') }}</button>
    </div>
    <p v-if="reason" class="alert alert-crit" role="alert">{{ t(`errors.${reason}`, { detail: container }) }}</p>
    <div ref="el" class="terminal-box docker-terminal" :aria-label="t('docker.terminal_title', { name: container })"></div>
    <p class="muted small note">{{ t('docker.terminal_hint') }}</p>
  </div>
</template>

<style scoped>
.docker-terminal { height: min(60vh, 560px); min-height: 280px; }
.note { margin: 10px 0 0; }
</style>
