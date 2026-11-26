<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLogo from '../AppLogo.vue'
import Icon from '../Icon.vue'
import { tx, logoSrc, problemText, mb, diskMB } from './appstore.js'

// AppCard is one app of the catalog: logo, name, what it is, what it needs,
// and why it cannot be installed here (greyed, with the first refusal).
const props = defineProps({
  app: { type: Object, required: true },
  installs: { type: Number, default: 0 }, // installed copies on this server
  dockerReady: { type: Boolean, default: true },
})
const emit = defineEmits(['open', 'manage'])
const { t, te, locale } = useI18n()

const needsDocker = computed(() => props.app.kind !== 'php' && !props.dockerReady)
const refusal = computed(() => {
  if (needsDocker.value) return t('apps.needs_docker')
  const r = props.app.check?.refusals?.[0]
  return r ? problemText(r, t, te) : ''
})
const blocked = computed(() => !!refusal.value)
// A service app runs once per server; once installed it is managed, not installed again.
const serviceDone = computed(() => props.app.kind === 'service' && props.app.installed)
const ram = computed(() => {
  const r = props.app.resources || {}
  if (r.idle_ram_mb) return t('apps.idle_ram', { size: mb(r.idle_ram_mb, locale.value) })
  if (r.memory_limit_mb) return t('apps.ram_limit', { size: mb(r.memory_limit_mb, locale.value) })
  return ''
})
const kindText = computed(() => t(`apps.kind_${props.app.kind}`))
</script>

<template>
  <article class="as-card" :class="{ off: blocked }">
    <div class="as-card-top">
      <AppLogo :src="logoSrc(app.logo)" :dim="blocked" />
      <div class="as-card-head">
        <h3 class="as-card-name">{{ app.name }}</h3>
        <div class="as-card-sub">
          <span class="mono">{{ app.version }}</span><span>·&nbsp;{{ kindText }}</span>
        </div>
      </div>
    </div>
    <p class="as-desc">{{ tx(app.description, locale) }}</p>
    <div class="as-facts">
      <span v-if="ram"><Icon name="memory" :size="14" />{{ ram }}</span>
      <span v-if="app.kind === 'php'"><Icon name="code" :size="14" />PHP {{ app.min_php }}+</span>
      <span v-if="diskMB(app)"><Icon name="disk" :size="14" />{{ mb(diskMB(app), locale) }}</span>
      <span v-if="app.source_available" class="tag tag-warn as-lic" :title="t('apps.source_available')">{{ t('apps.badge_source') }}</span>
    </div>
    <p v-if="refusal" class="as-refusal"><Icon name="ban" :size="14" /><span>{{ refusal }}</span></p>
    <div class="as-card-foot">
      <span v-if="installs" class="status-pill ok as-installed">{{ installs > 1 ? t('apps.installed_n', { n: installs }) : t('apps.installed_badge') }}</span>
      <button v-if="serviceDone" type="button" class="btn btn-sm" @click="emit('manage')">
        <Icon name="settings" :size="15" />{{ t('apps.manage') }}
      </button>
      <button v-else type="button" class="btn btn-sm" :aria-label="(blocked ? t('apps.details') : t('apps.install')) + ' ' + app.name" @click="emit('open')">
        <Icon :name="blocked ? 'info' : 'download'" :size="15" />{{ blocked ? t('apps.details') : t('apps.install') }}
      </button>
    </div>
  </article>
</template>

<style scoped>
.as-card {
  display: flex; flex-direction: column; gap: 10px; padding: 16px; min-width: 0;
  background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); box-shadow: var(--shadow);
}
.as-card-top { display: flex; align-items: flex-start; gap: 12px; }
.as-card-head { flex: 1; min-width: 0; }
.as-card-name { margin: 0; font-size: 15px; font-weight: 800; letter-spacing: -0.01em; overflow-wrap: anywhere; }
.as-card-sub { display: flex; flex-wrap: wrap; gap: 0 5px; color: var(--muted); font-size: 12.5px; }
.as-card-sub .mono { font-size: 12px; }
.as-installed { flex: none; }
.as-desc {
  margin: 0; color: var(--text-2); font-size: 13px; line-height: 1.45; min-height: 2.9em;
  display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}
.as-facts { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 12px; color: var(--muted); font-size: 12.5px; }
.as-facts > span { display: inline-flex; align-items: center; gap: 5px; }
.as-lic { min-height: 20px; font-size: 11.5px; }
.as-refusal { display: flex; gap: 6px; align-items: flex-start; margin: 0; color: var(--crit); font-size: 12.5px; font-weight: 600; line-height: 1.4; }
.as-refusal .icon { flex: none; margin-top: 2px; }
.as-card-foot { display: flex; align-items: center; justify-content: flex-end; gap: 8px; margin-top: auto; }
.as-card-foot .as-installed { margin-right: auto; }
.as-card.off { background: var(--surface-2); box-shadow: none; }
.as-card.off .as-card-name, .as-card.off .as-desc, .as-card.off .as-facts { opacity: 0.7; }
</style>
