<script setup>
import { computed } from 'vue'
import Icon from './Icon.vue'

const props = defineProps({
  label: { type: String, required: true },
  percent: { type: Number, default: null },
  detail: { type: String, default: '' },
  available: { type: Boolean, default: false },
  icon: { type: String, default: '' },
})

const shown = computed(() => props.available && Number.isFinite(props.percent))
const level = computed(() => (props.percent >= 90 ? 'crit' : props.percent >= 75 ? 'warn' : 'ok'))
</script>

<template>
  <div class="card stat">
    <div class="stat-top">
      <div class="stat-label">{{ label }}</div>
      <span v-if="icon" class="stat-icon"><Icon :name="icon" /></span>
    </div>
    <div class="stat-value">{{ shown ? `${Math.round(percent)}%` : '—' }}</div>
    <div
      class="meter"
      role="meter"
      :aria-label="label"
      aria-valuemin="0"
      aria-valuemax="100"
      :aria-valuenow="shown ? Math.round(percent) : undefined"
    >
      <div class="meter-fill" :class="`meter-${level}`" :style="{ width: `${shown ? Math.min(percent, 100) : 0}%` }"></div>
    </div>
    <div class="stat-detail">{{ detail || ' ' }}</div>
  </div>
</template>
