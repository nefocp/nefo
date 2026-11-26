<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import Icon from '../components/Icon.vue'
import { server } from '../server.js'

const { t } = useI18n()
const el = ref(null)
const state = ref('connecting') // connecting, open, closed
const reason = ref('') // error code from the server's close frame
let term = null
let fit = null
let socket = null
let observer = null

function send(msg) {
  if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(msg))
}

function sendSize() {
  if (term) send({ t: 'r', r: term.rows, c: term.cols })
}

function connect() {
  state.value = 'connecting'
  reason.value = ''
  const url = new URL('api/terminal', window.location.href)
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
    // 4001/4002 carry the reason the server refused the shell.
    if (e.code >= 4000 && e.reason) reason.value = e.reason
    if (state.value !== 'closed') term.write(`\r\n\x1b[90m${t('terminal.closed')}\x1b[0m\r\n`)
    state.value = 'closed'
  }
}

function reconnect() {
  term.reset()
  connect()
}

onMounted(() => {
  term = new Terminal({
    cursorBlink: true,
    fontFamily: "'JetBrains Mono', ui-monospace, 'Cascadia Mono', Menlo, Consolas, monospace",
    fontSize: 14,
    scrollback: 5000,
    theme: { background: '#0b1220', foreground: '#e2e8f0', cursor: '#2dd4bf', selectionBackground: '#1f3a4d' },
  })
  fit = new FitAddon()
  term.loadAddon(fit)
  term.open(el.value)
  fit.fit()
  term.onData((d) => send({ t: 'i', d }))
  observer = new ResizeObserver(() => {
    fit.fit()
    sendSize()
  })
  observer.observe(el.value)
  connect()
})

onBeforeUnmount(() => {
  observer?.disconnect()
  state.value = 'closed'
  socket?.close()
  term?.dispose()
})
</script>

<template>
  <div class="pg terminal-page">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('terminal.title') }}</h1></div>
    <p v-if="reason" class="alert alert-crit" role="alert">{{ t(`errors.${reason}`) }}</p>
    <section class="term-frame">
      <div class="term-bar">
        <span class="term-state" :class="state" role="status">
          <span class="term-dot" aria-hidden="true"></span>{{ t(`terminal.${state}`) }}
        </span>
        <span class="mono term-host">root@{{ server.info?.hostname || 'server' }}</span>
        <span class="term-gap"></span>
        <button v-if="state === 'closed'" class="btn btn-sm btn-ink" @click="reconnect"><Icon name="refresh" :size="15" />{{ t('terminal.reconnect') }}</button>
      </div>
      <div ref="el" class="terminal-box" :aria-label="t('terminal.title')"></div>
    </section>
    <p class="muted small" style="margin: 0">{{ t('terminal.hint') }}</p>
  </div>
</template>

<style scoped>
/* One colour for the frame and xterm, the same in both themes. */
.term-frame { border-radius: var(--radius); overflow: hidden; background: #0b1220; border: 1px solid rgb(148 163 184 / 0.16); box-shadow: var(--shadow); }
.term-bar { display: flex; align-items: center; gap: 12px; padding: 9px 14px; border-bottom: 1px solid rgb(148 163 184 / 0.12); background: #0e1626; color: #cbd5e1; flex-wrap: wrap; }
.term-state { display: inline-flex; align-items: center; gap: 7px; font-size: 12.5px; font-weight: 600; }
.term-dot { width: 8px; height: 8px; border-radius: 50%; background: #fbbf24; }
.term-state.open .term-dot { background: #34d399; box-shadow: 0 0 0 3px rgb(52 211 153 / 0.2); }
.term-state.closed .term-dot { background: #f87171; }
.term-host { font-size: 12px; color: #8f9cb0; }
.term-gap { flex: 1; }
.terminal-box { border: 0; border-radius: 0; background: #0b1220; height: calc(100vh - 250px); min-height: 360px; padding: 10px 6px 6px 12px; }
.terminal-box :deep(.xterm-viewport) { background: #0b1220 !important; }
@media (max-width: 640px) {
  .terminal-box { height: calc(100vh - 230px); min-height: 300px; padding: 6px; }
}
</style>
