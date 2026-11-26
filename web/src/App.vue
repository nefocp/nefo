<script setup>
import { toasts } from './toast.js'
import Icon from './components/Icon.vue'
</script>

<template>
  <RouterView />
  <div class="toasts" role="status" aria-live="polite">
    <div v-for="m in toasts" :key="m.id" class="toast" :class="m.kind">
      <Icon :name="m.kind === 'crit' ? 'alert' : 'checkCircle'" />
      <span>{{ m.text }}</span>
    </div>
  </div>
</template>
