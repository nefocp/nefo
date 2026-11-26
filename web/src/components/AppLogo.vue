<script setup>
import { computed, ref, watch } from 'vue'
import { logoURL } from '../logos.js'
import Icon from './Icon.vue'

// AppLogo shows a software or application logo on a white rounded tile, so
// dark logos stay visible in the dark theme. Without a logo it draws the
// fallback icon. size: xs (22px), sm (28px), md (42px) or lg (52px).
const props = defineProps({
  name: { type: String, default: '' },
  size: { type: String, default: 'md' },
  icon: { type: String, default: 'package' },
  dim: { type: Boolean, default: false },
  label: { type: String, default: '' }, // alt text; empty when the name is written next to it
  src: { type: String, default: '' }, // a logo address instead of a name (the app store's logos)
})
const url = computed(() => props.src || logoURL(props.name))
const broken = ref(false) // the file did not load: show the icon
watch(url, () => (broken.value = false))
const iconSize = computed(() => ({ xs: 13, sm: 16, md: 22, lg: 26 })[props.size] || 22)
</script>

<template>
  <span class="logo-tile" :class="[size === 'md' ? '' : size, { dim }]" :role="label ? 'img' : undefined" :aria-label="label || undefined">
    <img v-if="url && !broken" :src="url" alt="" @error="broken = true" />
    <Icon v-else :name="icon" :size="iconSize" />
  </span>
</template>
