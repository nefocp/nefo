<script setup>
import { ref, onBeforeUnmount } from 'vue'
import Icon from './Icon.vue'

// RowActions shows the first "inline" actions as buttons and the rest in a
// "⋯" menu. Each action: { label, icon, onClick, href, danger, disabled, inline }.
const props = defineProps({
  actions: { type: Array, required: true },
})

const open = ref(false)
const root = ref(null)
const btn = ref(null)
const menuStyle = ref({})

function onDoc(e) {
  if (root.value && !root.value.contains(e.target)) close()
}
function onKey(e) {
  if (e.key === 'Escape') close()
}
// The menu is fixed to the viewport, so tables that scroll sideways do not
// clip it. Near the bottom of the window it opens upwards.
function place() {
  const r = btn.value.getBoundingClientRect()
  const right = window.innerWidth - r.right + 'px'
  menuStyle.value = r.bottom + 280 > window.innerHeight
    ? { position: 'fixed', right, bottom: window.innerHeight - r.top + 4 + 'px', top: 'auto' }
    : { position: 'fixed', right, top: r.bottom + 4 + 'px' }
}
function toggle() {
  if (open.value) return close()
  place()
  open.value = true
  document.addEventListener('mousedown', onDoc)
  document.addEventListener('keydown', onKey)
  window.addEventListener('scroll', place, true)
  window.addEventListener('resize', close)
}
function close() {
  open.value = false
  document.removeEventListener('mousedown', onDoc)
  document.removeEventListener('keydown', onKey)
  window.removeEventListener('scroll', place, true)
  window.removeEventListener('resize', close)
}
function run(a) {
  close()
  a.onClick?.()
}
onBeforeUnmount(close)

const visible = () => props.actions.filter((a) => a && !a.hidden)
</script>

<template>
  <div ref="root" class="row-actions">
    <template v-for="a in visible().filter((x) => x.inline)" :key="a.label">
      <a v-if="a.href" class="btn btn-sm" :href="a.href" target="_blank" rel="noopener noreferrer">
        <Icon v-if="a.icon" :name="a.icon" :size="15" />{{ a.label }}
      </a>
      <button v-else type="button" class="btn btn-sm" :class="{ 'btn-danger': a.danger }" :disabled="a.disabled" @click="run(a)">
        <Icon v-if="a.icon" :name="a.icon" :size="15" />{{ a.label }}
      </button>
    </template>
    <template v-if="visible().some((x) => !x.inline)">
      <button ref="btn" type="button" class="icon-btn" :aria-expanded="open" aria-haspopup="menu" :aria-label="$t('common.more')" @click="toggle">
        <Icon name="more" />
      </button>
      <div v-if="open" class="menu" role="menu" :style="menuStyle">
        <template v-for="a in visible().filter((x) => !x.inline)" :key="a.label">
          <a v-if="a.href" class="menu-item" role="menuitem" :href="a.href" target="_blank" rel="noopener noreferrer" @click="close">
            <Icon :name="a.icon || 'external'" :size="16" />{{ a.label }}
          </a>
          <button v-else type="button" class="menu-item" :class="{ danger: a.danger }" role="menuitem" :disabled="a.disabled" @click="run(a)">
            <Icon :name="a.icon || 'file'" :size="16" />{{ a.label }}
          </button>
        </template>
      </div>
    </template>
  </div>
</template>
