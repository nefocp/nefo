<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'

// Drawer is a panel that slides in from the right, in a native modal
// <dialog>: focus stays inside, Escape and a click on the backdrop emit
// "close". Render it with v-if. The "head" slot is the top bar, the default
// slot the rest.
defineProps({ label: { type: String, required: true } })
const emit = defineEmits(['close'])
const dlg = ref(null)

function onCancel(e) {
  e.preventDefault()
  emit('close')
}

onMounted(() => {
  dlg.value?.showModal()
  document.documentElement.style.overflow = 'hidden'
})
onBeforeUnmount(() => {
  document.documentElement.style.overflow = ''
  if (dlg.value?.open) dlg.value.close()
})
</script>

<template>
  <dialog ref="dlg" class="drawer" :aria-label="label" @cancel="onCancel" @mousedown.self="emit('close')">
    <header class="drawer-head"><slot name="head" /></header>
    <slot />
  </dialog>
</template>
