<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from './Icon.vue'
import AppLogo from './AppLogo.vue'

// SoftwareCard shows one item of GET /api/software: status, version, what
// it is for, roughly what it costs in memory, and Install or Uninstall.
// When it cannot be uninstalled now, the reason shows instead of a button
// that would fail. The default slot adds actions (an "Open" link) and the
// "extra" slot content under them.
const props = defineProps({
  item: { type: Object, required: true },
  icon: { type: String, default: 'package' },
  busy: { type: Boolean, default: false }, // a job is running
  working: { type: Boolean, default: false }, // the running job is this item's
  supported: { type: Boolean, default: true },
})
defineEmits(['install', 'remove'])
const { t, te } = useI18n()

const blockedText = computed(() => {
  const b = props.item.blocked
  if (!b) return ''
  const key = `errors.${b}`
  return te(key) ? t(key, { detail: props.item.blocked_detail ?? '' }) : b
})

const ram = computed(() => {
  const mb = props.item.ram_mb
  if (!mb) return t('software.ram_idle')
  return props.item.ram_max ? t('software.ram_max', { mb }) : t('software.ram_about', { mb })
})
</script>

<template>
  <section class="sw-card" :class="{ on: item.installed }" :aria-busy="working">
    <div class="sw-head">
      <AppLogo :name="item.kind" :icon="icon" :dim="!item.installed" />
      <div class="sw-name">
        <h3>{{ t(`software.${item.kind}`) }}</h3>
        <div class="muted small">
          <span v-if="item.installed && item.version" class="mono">{{ item.version }}</span>
          <span v-else-if="!item.installed">{{ ram }}</span>
        </div>
      </div>
      <span v-if="working" class="status-pill warn"><span class="spinner" aria-hidden="true"></span>{{ t('software.working') }}</span>
      <RouterLink v-else-if="item.installed && item.stopped" class="status-pill warn" :to="{ name: 'services' }" :title="t('software.stopped_hint')">{{ t('pg.svc_state_inactive') }}</RouterLink>
      <span v-else-if="item.installed" class="status-pill ok">{{ t('software.installed') }}</span>
      <span v-else class="status-pill">{{ t('software.not_installed') }}</span>
    </div>
    <p class="muted small sw-desc">{{ t(`software.${item.kind}_hint`) }}</p>
    <div class="sw-meta">
      <span><Icon name="memory" :size="13" />{{ ram }}</span>
      <span v-if="item.installed && item.usage?.sites"><Icon name="globe" :size="13" />{{ t('software.used_by', { n: item.usage.sites }, item.usage.sites) }}</span>
      <span v-if="item.installed && item.usage?.databases"><Icon name="database" :size="13" />{{ t('software.usage_databases', { n: item.usage.databases }, item.usage.databases) }}</span>
      <span v-if="item.installed && item.usage?.containers"><Icon name="box" :size="13" />{{ t('software.usage_containers', { n: item.usage.containers }, item.usage.containers) }}</span>
    </div>
    <p v-if="item.installed && blockedText" class="sw-blocked small"><Icon name="alert" :size="14" />{{ blockedText }}</p>
    <p v-if="!item.installed && item.not_offered" class="sw-blocked small">
      <Icon name="alert" :size="14" />{{ t(`errors.${item.not_offered}`, { detail: item.not_offered_detail || '' }) }}
    </p>
    <div class="sw-actions">
      <button v-if="!item.installed" class="btn btn-primary btn-sm" :disabled="busy || !supported || !!item.not_offered" @click="$emit('install', item)">
        <Icon name="download" :size="15" />{{ t('software.install') }}
      </button>
      <slot />
      <span class="sw-gap"></span>
      <button
        v-if="item.installed && (item.removable || item.blocked)"
        class="btn btn-sm btn-ghost sw-remove"
        :disabled="busy || !item.removable"
        @click="$emit('remove', item)"
      >
        <Icon name="trash" :size="15" />{{ t('software.uninstall') }}
      </button>
    </div>
    <slot name="extra" />
  </section>
</template>

<style scoped>
.sw-card {
  display: flex; flex-direction: column; gap: 10px; padding: 16px; min-width: 0;
  background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); box-shadow: var(--shadow);
}
.sw-card:not(.on) { border-style: dashed; box-shadow: none; }
.sw-head { display: flex; align-items: center; gap: 12px; }
.sw-name { flex: 1; min-width: 0; }
.sw-name h3 { margin: 0; font-size: 15px; }
.sw-desc { margin: 0; }
.sw-meta { display: flex; flex-wrap: wrap; gap: 4px 14px; color: var(--muted); font-size: 12.5px; }
.sw-meta span { display: inline-flex; align-items: center; gap: 5px; }
.sw-blocked { display: flex; align-items: flex-start; gap: 6px; margin: 0; color: var(--warn); }
.sw-blocked .icon { flex: none; margin-top: 2px; }
.sw-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-top: auto; padding-top: 4px; }
.sw-gap { flex: 1; }
.sw-remove { color: var(--crit); }
</style>
