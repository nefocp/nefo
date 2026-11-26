<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { attention, refreshAttention } from '../attention.js'
import { usePopover } from '../composables/usePopover.js'
import Icon from './Icon.vue'

// The bell lists what needs attention (see attention.js).
const { t } = useI18n()
const root = ref(null)
const btn = ref(null)
const { open, toggle, hide } = usePopover(root, btn)
const n = computed(() => attention.value.length)
const label = computed(() => (n.value ? t('bell.label_n', { n: n.value }) : t('bell.label')))

function onToggle() {
  if (!open.value) refreshAttention()
  toggle()
}
</script>

<template>
  <div ref="root" class="bell" style="position: relative">
    <button ref="btn" type="button" class="icon-btn" :aria-label="label" :aria-expanded="open" aria-haspopup="dialog" @click="onToggle">
      <Icon name="bell" :size="19" :stroke="1.8" />
      <span v-if="n" class="bell-dot" aria-hidden="true"></span>
    </button>
    <div v-if="open" class="popover" role="dialog" :aria-label="t('bell.title')">
      <div class="popover-head">
        <span>{{ t('bell.title') }}</span>
        <span class="count-badge" :class="n ? 'crit' : 'ok'">{{ n }}</span>
      </div>
      <div class="popover-body">
        <div v-if="!n" class="alert-row">
          <span class="sev ok"><Icon name="check" :size="16" /></span>
          <div class="alert-row-text">
            <div class="alert-row-title">{{ t('bell.all_good') }}</div>
            <div class="alert-row-sub">{{ t('bell.all_good_sub') }}</div>
          </div>
        </div>
        <RouterLink v-for="a in attention" :key="a.id" :to="a.to" class="alert-row" @click="hide">
          <span class="sev" :class="a.sev"><Icon :name="a.icon" :size="16" /></span>
          <span class="alert-row-text">
            <span class="sr-only">{{ t('bell.sev_' + a.sev) }}: </span>
            <span class="alert-row-title" style="display: block">{{ t(a.title[0], a.title[1] || {}, a.title[2] ?? 1) }}</span>
            <span class="alert-row-sub" style="display: block">{{ t(a.sub[0], a.sub[1] || {}) }}</span>
          </span>
          <Icon name="chevronRight" :size="16" class="muted" />
        </RouterLink>
      </div>
    </div>
  </div>
</template>
