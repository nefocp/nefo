<script setup>
import { ref, computed } from 'vue'

// AreaChart draws one series over time as a line with a light area under
// it, a 0–max scale on the left, times below, and a tooltip on hover.
// With points2 it draws a second series (same times) as a dashed line in the
// second series color, and the tooltip names both (names: [first, second]).
const props = defineProps({
  label: { type: String, required: true }, // for screen readers
  points: { type: Array, required: true }, // [{ t, v }]
  points2: { type: Array, default: null }, // [{ t, v }], same times as points
  names: { type: Array, default: () => [] },
  max: { type: Number, default: 100 },
  format: { type: Function, default: (v) => `${Math.round(v)}%` },
  locale: { type: String, default: undefined },
  height: { type: Number, default: 180 },
})

const W = 600
const H = 170
const times = computed(() => props.points.map((p) => new Date(p.t).getTime()))
const t0 = computed(() => Math.min(...times.value))
const t1 = computed(() => Math.max(...times.value))
const x = (t) => (t1.value === t0.value ? W : ((t - t0.value) / (t1.value - t0.value)) * W)
const y = (v) => H - 1 - (Math.min(Math.max(v, 0), props.max) / props.max) * (H - 2)

// Break the line where samples are missing (the server was off).
function segmentsOf(points) {
  const out = []
  let cur = []
  const n = points.length
  const step = n > 1 ? (t1.value - t0.value) / (n - 1) : 0
  points.forEach((p, i) => {
    if (i > 0 && step > 0 && times.value[i] - times.value[i - 1] > step * 3 && cur.length) {
      out.push(cur)
      cur = []
    }
    cur.push([x(times.value[i]), y(p.v)])
  })
  if (cur.length) out.push(cur)
  return out
}
const polyline = (s) => s.map(([a, b]) => `${a.toFixed(1)},${b.toFixed(1)}`).join(' ')
const segments = computed(() => segmentsOf(props.points))
const lines = computed(() => segments.value.map(polyline))
const lines2 = computed(() => (props.points2 ? segmentsOf(props.points2).map(polyline) : []))
const areas = computed(() =>
  segments.value.map((s) => `M${s[0][0].toFixed(1)},${H} L` + s.map(([a, b]) => `${a.toFixed(1)},${b.toFixed(1)}`).join(' L') + ` L${s.at(-1)[0].toFixed(1)},${H} Z`),
)
const yTicks = computed(() => [props.max, props.max / 2, 0])
const long = computed(() => t1.value - t0.value > 20 * 3600 * 1000)
const fmtTime = (t) =>
  new Date(t).toLocaleString(props.locale, long.value ? { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' } : { hour: '2-digit', minute: '2-digit' })
const xTicks = computed(() => {
  if (!props.points.length) return []
  return [0, 0.25, 0.5, 0.75, 1].map((f) => fmtTime(t0.value + (t1.value - t0.value) * f))
})

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
  const p2 = props.points2?.[hover.value]
  const hx = x(times.value[hover.value])
  const rows = p2 ? [{ name: props.names[0], text: props.format(p.v) }, { name: props.names[1], text: props.format(p2.v), second: true }] : null
  return {
    x: hx,
    left: (hx / W) * 100,
    dots: [{ top: (y(p.v) / H) * 100 }, ...(p2 ? [{ top: (y(p2.v) / H) * 100, second: true }] : [])],
    text: props.format(p.v),
    rows,
    time: fmtTime(times.value[hover.value]),
  }
})
</script>

<template>
  <div class="area" :class="{ multi: !!points2 }">
    <div class="area-y" aria-hidden="true" :style="{ height: height + 'px' }">
      <span v-for="v in yTicks" :key="v">{{ format(v) }}</span>
    </div>
    <div class="area-plot">
      <svg
        ref="svg"
        :viewBox="`0 0 ${W} ${H}`"
        preserveAspectRatio="none"
        :style="{ height: height + 'px' }"
        role="img"
        :aria-label="label"
        @pointermove="onMove"
        @pointerleave="hover = -1"
      >
        <line x1="0" y1="1" :x2="W" y2="1" class="g" />
        <line x1="0" :y1="H / 2" :x2="W" :y2="H / 2" class="g" />
        <line x1="0" :y1="H - 1" :x2="W" :y2="H - 1" class="g base" />
        <path v-for="(d, i) in areas" :key="'a' + i" :d="d" class="fill" />
        <polyline v-for="(p, i) in lines" :key="'l' + i" :points="p" class="line" />
        <polyline v-for="(p, i) in lines2" :key="'m' + i" :points="p" class="line line2" />
        <line v-if="tip" :x1="tip.x" :x2="tip.x" y1="0" :y2="H" class="cross" />
      </svg>
      <template v-if="tip">
        <span v-for="(d, i) in tip.dots" :key="i" class="area-dot" :class="{ second: d.second }" :style="{ left: tip.left + '%', top: d.top + '%' }" aria-hidden="true"></span>
      </template>
      <div v-if="tip" class="chart-tip" :style="{ left: tip.left + '%' }" aria-hidden="true">
        <template v-if="tip.rows">
          <span v-for="r in tip.rows" :key="r.name" class="tip-row"><span class="tip-key" :class="{ second: r.second }"></span>{{ r.name }} <strong>{{ r.text }}</strong></span>
        </template>
        <strong v-else>{{ tip.text }}</strong>
        <span>{{ tip.time }}</span>
      </div>
      <div class="area-x" aria-hidden="true"><span v-for="(l, i) in xTicks" :key="i">{{ l }}</span></div>
    </div>
  </div>
</template>

<style scoped>
.area { display: flex; gap: 8px; }
.area-y { display: flex; flex-direction: column; justify-content: space-between; min-width: 46px; flex: none; text-align: right; font-size: 11px; color: var(--muted); font-variant-numeric: tabular-nums; white-space: nowrap; }
.area-y span { transform: translateY(-50%); }
.area-y span:first-child { transform: none; }
.area-y span:last-child { transform: translateY(0); }
.area-plot { flex: 1; min-width: 0; position: relative; }
svg { display: block; width: 100%; overflow: visible; touch-action: none; }
.g { stroke: var(--border); stroke-dasharray: 3 4; vector-effect: non-scaling-stroke; }
.g.base { stroke: var(--border-strong); stroke-dasharray: none; }
.fill { fill: var(--accent); fill-opacity: 0.1; }
.line { fill: none; stroke: var(--accent); stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; vector-effect: non-scaling-stroke; }
.cross { stroke: var(--muted); stroke-dasharray: 3 3; vector-effect: non-scaling-stroke; }
.area-dot { position: absolute; width: 10px; height: 10px; margin: -5px 0 0 -5px; border-radius: 50%; background: var(--accent); border: 2px solid var(--surface); pointer-events: none; }
.area-x { display: flex; justify-content: space-between; font-size: 11px; color: var(--muted); margin-top: 6px; font-variant-numeric: tabular-nums; }
.chart-tip { top: 0; }
.multi .fill { fill: var(--s1); fill-opacity: 0.08; }
.multi .line { stroke: var(--s1); }
.multi .line2 { stroke: var(--s2); stroke-dasharray: 5 3; }
.multi .area-dot { background: var(--s1); }
.multi .area-dot.second { background: var(--s2); }
.tip-row { display: flex; align-items: center; gap: 6px; white-space: nowrap; }
.tip-row strong { margin-left: auto; padding-left: 8px; }
.tip-key { width: 12px; height: 0; border-top: 2px solid var(--s1); flex: none; }
.tip-key.second { border-top: 2px dashed var(--s2); }
@media (max-width: 640px) {
  .area-x span:nth-child(2), .area-x span:nth-child(4) { display: none; }
}
</style>
