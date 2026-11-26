<script setup>
import { ref, computed } from 'vue'

// LineChart draws one series over time on a 0–max scale, with a crosshair
// and tooltip on hover. The title names the series, so it has no legend.
const props = defineProps({
  title: { type: String, required: true },
  points: { type: Array, required: true }, // [{ t: Date|string, v: number }]
  max: { type: Number, default: 100 },
  unit: { type: String, default: '%' },
  locale: { type: String, default: undefined },
})

const W = 600
const H = 150
const pad = { l: 34, r: 8, t: 8, b: 22 }
const iw = W - pad.l - pad.r
const ih = H - pad.t - pad.b

const times = computed(() => props.points.map((p) => new Date(p.t).getTime()))
const t0 = computed(() => Math.min(...times.value))
const t1 = computed(() => Math.max(...times.value))
const x = (t) => pad.l + (t1.value === t0.value ? iw : ((t - t0.value) / (t1.value - t0.value)) * iw)
const y = (v) => pad.t + ih - (Math.min(Math.max(v, 0), props.max) / props.max) * ih

// Break the line where samples are missing (the server was off).
const paths = computed(() => {
  const segs = []
  let cur = ''
  const step = props.points.length > 1 ? (t1.value - t0.value) / (props.points.length - 1) : 0
  props.points.forEach((p, i) => {
    const gap = i > 0 && step > 0 && times.value[i] - times.value[i - 1] > step * 3
    if (gap && cur) {
      segs.push(cur)
      cur = ''
    }
    cur += `${cur ? 'L' : 'M'}${x(times.value[i]).toFixed(1)},${y(p.v).toFixed(1)}`
  })
  if (cur) segs.push(cur)
  return segs
})

const grid = computed(() => [0, 0.5, 1].map((f) => ({ v: Math.round(props.max * f), y: y(props.max * f) })))
const fmtTime = (t, withDate) =>
  new Date(t).toLocaleString(props.locale, withDate ? { dateStyle: 'short', timeStyle: 'short' } : { timeStyle: 'short' })
const long = computed(() => t1.value - t0.value > 20 * 3600 * 1000)

const latest = computed(() => props.points.at(-1)?.v)
const peak = computed(() => Math.max(...props.points.map((p) => p.v)))
const summary = computed(() =>
  props.points.length
    ? `${props.title}: ${latest.value.toFixed(0)}${props.unit} now, highest ${peak.value.toFixed(0)}${props.unit}`
    : props.title,
)

// ---- hover ----
const svg = ref(null)
const hover = ref(-1)

function onMove(ev) {
  const r = svg.value.getBoundingClientRect()
  const px = ((ev.clientX - r.left) / r.width) * W
  let best = -1
  let dist = Infinity
  times.value.forEach((t, i) => {
    const d = Math.abs(x(t) - px)
    if (d < dist) {
      dist = d
      best = i
    }
  })
  hover.value = best
}

const tip = computed(() => {
  if (hover.value < 0) return null
  const p = props.points[hover.value]
  const hx = x(times.value[hover.value])
  return { x: hx, y: y(p.v), left: (hx / W) * 100, text: `${p.v.toFixed(1)}${props.unit}`, time: fmtTime(times.value[hover.value], long.value) }
})
</script>

<template>
  <figure class="chart">
    <figcaption class="chart-title">
      <span>{{ title }}</span>
      <span v-if="points.length" class="chart-latest">{{ latest.toFixed(0) }}{{ unit }}</span>
    </figcaption>
    <div class="chart-plot">
      <svg
        ref="svg"
        :viewBox="`0 0 ${W} ${H}`"
        preserveAspectRatio="none"
        role="img"
        :aria-label="summary"
        @pointermove="onMove"
        @pointerleave="hover = -1"
      >
        <g class="chart-grid">
          <line v-for="g in grid" :key="g.v" :x1="pad.l" :x2="W - pad.r" :y1="g.y" :y2="g.y" />
        </g>
        <path v-for="(d, i) in paths" :key="i" :d="d" class="chart-line" />
        <template v-if="tip">
          <line class="chart-cross" :x1="tip.x" :x2="tip.x" :y1="pad.t" :y2="pad.t + ih" />
          <circle class="chart-dot" :cx="tip.x" :cy="tip.y" r="4" />
        </template>
      </svg>
      <span v-for="g in grid" :key="'l' + g.v" class="chart-ylabel" :style="{ top: (g.y / H) * 100 + '%' }">{{ g.v }}{{ unit }}</span>
      <div v-if="tip" class="chart-tip" :style="{ left: tip.left + '%' }">
        <strong>{{ tip.text }}</strong>
        <span>{{ tip.time }}</span>
      </div>
    </div>
    <div v-if="points.length" class="chart-x">
      <span>{{ fmtTime(t0, long) }}</span>
      <span>{{ fmtTime(t1, long) }}</span>
    </div>
  </figure>
</template>
