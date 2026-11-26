<script setup>
import { BRAND } from '../brand.js'

defineProps({ size: { type: Number, default: 34 } })
</script>

<template>
  <span class="brand-mark" :style="{ width: size + 'px', height: size + 'px', borderRadius: Math.round(size * 0.3) + 'px' }" aria-hidden="true">
    <svg :width="Math.round(size * 0.56)" :height="Math.round(size * 0.56)" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
      <path v-for="d in BRAND.mark" :key="d" :d="d" />
    </svg>
  </span>
</template>
