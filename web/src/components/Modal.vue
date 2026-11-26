<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import Icon from './Icon.vue'

// Modal shows its content in a native <dialog>: focus is trapped, Escape and
// the close button emit "close". Render it with v-if.
defineProps({
  title: { type: String, required: true },
  wide: { type: Boolean, default: false },
  danger: { type: Boolean, default: false },
})
const emit = defineEmits(['close'])
const dlg = ref(null)

function onCancel(e) {
  e.preventDefault()
  emit('close')
}

function onClick(e) {
  // A click on the backdrop (outside the box) closes the dialog.
  if (e.target === dlg.value) emit('close')
}

onMounted(() => dlg.value?.showModal())
onBeforeUnmount(() => dlg.value?.open && dlg.value.close())
</script>

<template>
  <dialog ref="dlg" class="modal" :class="{ 'modal-wide': wide, 'modal-danger': danger }" @cancel="onCancel" @mousedown.self="onClick">
    <div class="modal-box">
      <header class="modal-head">
        <h2>{{ title }}</h2>
        <button type="button" class="icon-btn" :aria-label="$t('common.close')" @click="emit('close')"><Icon name="x" /></button>
      </header>
      <div class="modal-body">
        <slot />
      </div>
    </div>
  </dialog>
</template>
