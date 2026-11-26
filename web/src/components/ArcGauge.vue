<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

// ArcGauge draws a 270° gauge for the dark hero: the bright accent while
// usage is normal, amber from 70 %, red from 90 %. The state is also
// written under it, so colour is never the only sign.
const props = defineProps({
  name: { type: String, required: true },
  percent: { type: Number, default: null },
  inner: { type: String, default: '' }, // under the number, such as "1.4 / 1.9 GB"
  okText: { type: String, default: '' }, // state text while normal
  size: { type: Number, default: 132 },
})
const { t } = useI18n()

const R = 50
const C = 2 * Math.PI * R
const span = C * 0.75
const ok = computed(() => Number.isFinite(props.percent))
const pct = computed(() => (ok.value ? Math.max(0, Math.min(100, props.percent)) : 0))
const lvl = computed(() => (pct.value >= 90 ? 'crit' : pct.value >= 70 ? 'warn' : 'ok'))
const color = computed(() => ({ crit: '#F87171', warn: '#FBBF24', ok: 'var(--bright)' })[lvl.value])
const state = computed(() => {
  if (!ok.value) return '—'
  if (lvl.value === 'crit') return t('dash.level_crit')
  if (lvl.value === 'warn') return t('dash.level_warn')
  return props.okText || t('dash.level_ok')
})
const h = computed(() => Math.round(props.size * 0.894))
</script>

<template>
  <div class="arc" :style="{ minWidth: size + 'px' }">
    <div class="arc-box" :style="{ width: size + 'px', height: h + 'px' }">
      <svg :width="size" :height="h" viewBox="0 0 132 118" role="img" :aria-label="`${name}: ${ok ? Math.round(pct) + '%' : '—'}, ${state}`">
        <circle cx="66" cy="62" :r="R" transform="rotate(135 66 62)" class="arc-track" :stroke-dasharray="`${span.toFixed(1)} ${C.toFixed(1)}`" />
        <circle
          v-if="ok && pct > 0"
          cx="66"
          cy="62"
          :r="R"
          transform="rotate(135 66 62)"
          class="arc-val-line"
          :style="{ stroke: color }"
          :stroke-dasharray="`${((span * pct) / 100).toFixed(1)} ${C.toFixed(1)}`"
        />
      </svg>
      <div class="arc-val" aria-hidden="true">
        <span class="arc-num">{{ ok ? Math.round(pct) : '—' }}</span><span v-if="ok" class="arc-pct">%</span>
        <div class="arc-inner">{{ inner }}</div>
      </div>
    </div>
    <span class="arc-name">{{ name }}</span>
    <span class="arc-state" :style="{ color }">{{ state }}</span>
  </div>
</template>

<style scoped>
.arc { display: flex; flex-direction: column; align-items: center; gap: 2px; }
.arc-box { position: relative; }
.arc-box svg { display: block; width: 100%; height: 100%; }
.arc-track { fill: none; stroke: rgb(148 163 184 / 0.16); stroke-width: 9; stroke-linecap: round; }
.arc-val-line { fill: none; stroke-width: 9; stroke-linecap: round; transition: stroke-dasharray 0.6s ease; }
.arc-val { position: absolute; left: 0; right: 0; top: 32%; text-align: center; line-height: 1; }
.arc-num { font-size: 30px; font-weight: 800; color: #fff; letter-spacing: -0.03em; font-variant-numeric: tabular-nums; }
.arc-pct { font-size: 14px; font-weight: 700; color: var(--ink-muted); }
.arc-inner { font-size: 11px; color: var(--ink-muted); margin-top: 6px; white-space: nowrap; }
.arc-name { font-weight: 700; font-size: 13.5px; color: #fff; margin-top: -10px; text-align: center; }
.arc-state { font-size: 12px; font-weight: 600; }
@media (prefers-reduced-motion: reduce) { .arc-val-line { transition: none; } }
</style>
