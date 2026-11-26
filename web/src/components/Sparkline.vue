<script setup>
import { computed } from 'vue'

// Sparkline: a tiny trend line (no axes). values: numbers, oldest first.
const props = defineProps({
  values: { type: Array, default: () => [] },
  width: { type: Number, default: 64 },
  height: { type: Number, default: 22 },
})
const points = computed(() => {
  const v = props.values.map((x) => Number(x) || 0)
  if (v.length < 2) return ''
  const max = Math.max(...v)
  const min = Math.min(...v)
  const span = max - min || 1
  const w = props.width - 4
  const h = props.height - 6
  return v.map((x, i) => `${(2 + (i * w) / (v.length - 1)).toFixed(1)},${(3 + h - ((x - min) / span) * h).toFixed(1)}`).join(' ')
})
</script>

<template>
  <svg :width="width" :height="height" :viewBox="`0 0 ${width} ${height}`" aria-hidden="true" class="spark">
    <polyline v-if="points" :points="points" />
    <line v-else x1="2" :x2="width - 2" :y1="height / 2" :y2="height / 2" class="flat" />
  </svg>
</template>

<style scoped>
.spark { display: block; flex: none; }
polyline { fill: none; stroke: var(--accent); stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
.flat { stroke: var(--border-strong); stroke-width: 2; stroke-dasharray: 2 4; stroke-linecap: round; }
</style>
