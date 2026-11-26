<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { toast } from '../toast.js'
import Icon from './Icon.vue'
import WpPasswordModal from './WpPasswordModal.vue'
import WpStagingModal from './WpStagingModal.vue'

// SiteWordPress is the WordPress tab of the site drawer: the same checks
// and switches as the WordPress page, for one site. Jobs (updates,
// staging copies) are started through "job" so the drawer can follow them.
const props = defineProps({ site: { type: Object, required: true }, busy: { type: Boolean, default: false } })
const emit = defineEmits(['job'])
const { t } = useI18n()

const st = ref(null)
const loading = ref(true)
const error = ref('')
const pw = ref(false)
const staging = ref(false)
const url = () => `${props.site.ssl ? 'https' : 'http'}://${props.site.domain}`

async function load() {
  loading.value = true
  error.value = ''
  try {
    st.value = await api('GET', `api/wordpress/${props.site.id}`)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function toggle(what, on) {
  if (what === 'maintenance' && on && !window.confirm(t('wp.maintenance_confirm', { domain: props.site.domain }))) return
  const path = { maintenance: 'maintenance', debug: 'debug', auto_updates: 'auto-updates' }[what]
  try {
    await api('PUT', `api/wordpress/${props.site.id}/${path}`, { on })
    toast(t(`wp.done_${what}_${on ? 'on' : 'off'}`, { domain: props.site.domain }))
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  }
  await load()
}

async function update() {
  if (!window.confirm(t('wp.update_confirm', { domain: props.site.domain }))) return
  try {
    const res = await api('POST', `api/wordpress/${props.site.id}/update`)
    emit('job', res.job)
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  }
}
const updatesOf = () => st.value?.updates || []
</script>

<template>
  <div class="stack">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <section class="card">
      <div class="wp-head">
        <div>
          <div class="wp-version">
            <template v-if="st?.version">WordPress {{ st.version }}</template>
            <span v-else-if="loading" class="skel" style="width: 140px; height: 20px"></span>
            <template v-else>WordPress</template>
          </div>
          <div class="muted small">
            <span v-if="loading"><span class="spinner" aria-hidden="true"></span> {{ t('wp.checking') }}</span>
            <template v-else-if="st && !st.error">
              <span v-if="st.core_update" class="text-warn">{{ t('wp.core_update', { version: st.core_update }) }}</span>
              <span v-else-if="!updatesOf().length" class="text-ok">{{ t('wp.up_to_date') }}</span>
              <span v-else class="text-warn">{{ t('wp.show_updates', { n: updatesOf().length }, updatesOf().length) }}</span>
              · {{ t('wp.plugins', { n: st.plugins }) }} · {{ t('wp.themes', { n: st.themes }) }}
            </template>
            <span v-if="st?.error" class="text-crit">{{ t('wp.check_failed', { detail: st.error }) }}</span>
          </div>
        </div>
        <div class="row">
          <a class="btn" :href="url() + '/wp-admin/'" target="_blank" rel="noopener noreferrer"><Icon name="external" :size="15" />WP Admin</a>
          <button type="button" class="btn btn-primary" :disabled="busy || !st || !!st.error" @click="update"><Icon name="download" :size="15" />{{ t('wp.update') }}</button>
        </div>
      </div>
      <ul v-if="updatesOf().length" class="wp-list">
        <li v-for="u in updatesOf()" :key="u.type + u.name">
          <span class="tag">{{ t(`wp.type_${u.type}`) }}</span> {{ u.title || u.name }}
          <span class="mono small muted">{{ u.version }} → {{ u.new_version }}</span>
        </li>
      </ul>
    </section>

    <div class="opts">
      <div v-for="o in ['maintenance', 'debug', 'auto_updates']" :key="o" class="opt">
        <div class="opt-text">
          <div class="opt-title">{{ t(`drawer.wp_${o}`) }}</div>
          <div class="opt-hint">{{ t(`drawer.wp_${o}_hint`) }}</div>
        </div>
        <button
          type="button"
          class="switch"
          role="switch"
          :aria-checked="!!st?.[o]"
          :aria-label="t(`drawer.wp_${o}`)"
          :disabled="!st || !!st.error"
          @click="toggle(o, !st[o])"
        ></button>
      </div>
    </div>

    <section class="card">
      <div class="row wp-more">
        <button type="button" class="btn" @click="pw = true"><Icon name="key" :size="15" />{{ t('wp.reset_password') }}</button>
        <button type="button" class="btn" :disabled="busy" @click="staging = true"><Icon name="copy" :size="15" />{{ t('wp.clone') }}</button>
        <button type="button" class="btn btn-ghost" :disabled="loading" @click="load"><Icon name="refresh" :size="15" />{{ t('wp.recheck') }}</button>
        <RouterLink :to="{ name: 'wordpress' }" class="btn btn-ghost">{{ t('drawer.wp_toolkit') }}<Icon name="chevronRight" :size="15" /></RouterLink>
      </div>
    </section>

    <WpPasswordModal v-if="pw" :site="site" @close="pw = false" />
    <WpStagingModal v-if="staging" :site="site" @close="staging = false" @started="(j) => { staging = false; emit('job', j) }" />
  </div>
</template>

<style scoped>
.wp-head { display: flex; align-items: center; justify-content: space-between; gap: 14px; flex-wrap: wrap; }
.wp-version { font-size: 16px; font-weight: 800; margin-bottom: 2px; }
.wp-list { margin: 14px 0 0; padding: 12px 0 0; list-style: none; border-top: 1px solid var(--border); display: grid; gap: 6px; font-size: 13px; }
.opts { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 12px; }
.wp-more { gap: 8px; }
</style>
