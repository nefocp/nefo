<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from './Icon.vue'
import AppLogo from './AppLogo.vue'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytes, formatUptime, formatNumber } from '../format.js'

// DbMemcached is the Memcached tab of the Databases page: a few numbers
// from its "stats" command, and a restart (which empties it).
const { t, locale } = useI18n()

const info = ref(null)
const error = ref('')
const notice = ref('')
const loading = ref(true)
const busy = ref(false)

async function load() {
  try {
    info.value = await api('GET', 'api/memcached')
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

async function restart() {
  error.value = ''
  notice.value = ''
  if (!window.confirm(t('memcached.restart_confirm'))) return
  busy.value = true
  try {
    info.value = await api('POST', 'api/memcached/restart', {})
    notice.value = t('memcached.restarted')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

const memPct = computed(() => (info.value?.limit_bytes ? Math.min(100, (info.value.bytes / info.value.limit_bytes) * 100) : null))
const hitRate = computed(() => (info.value?.hit_rate >= 0 ? `${formatNumber(info.value.hit_rate, locale.value, 1)} %` : '—'))

onMounted(load)
</script>

<template>
  <div class="pg">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <span v-if="loading" class="skel" style="height: 160px; border-radius: 14px"></span>

    <template v-if="info && !loading">
      <div v-if="!info.installed" class="card empty">
        <AppLogo name="memcached" icon="memory" size="lg" style="margin: 0 auto 14px" />
        <p class="empty-lead">{{ t('memcached.not_installed') }}</p>
        <p class="muted">{{ t('memcached.not_installed_hint') }}</p>
        <RouterLink class="btn btn-primary" :to="{ name: 'software' }">{{ t('nav.software') }}</RouterLink>
      </div>

      <div v-else-if="!info.running" class="card empty">
        <AppLogo name="memcached" icon="memory" size="lg" dim style="margin: 0 auto 14px" />
        <p class="empty-lead">{{ t('memcached.not_running') }}</p>
        <p v-if="info.error" class="muted mono small">{{ info.error }}</p>
        <RouterLink class="btn" :to="{ name: 'services' }">{{ t('nav.services') }}</RouterLink>
      </div>

      <template v-else>
        <div class="kpis">
          <div class="kpi">
            <AppLogo name="memcached" icon="memory" />
            <div>
              <div class="kpi-val">Memcached {{ info.version }}</div>
              <div class="kpi-label">{{ t('pg.redis_up', { d: formatUptime(info.uptime, t) }) }}</div>
            </div>
          </div>
          <div class="kpi">
            <span class="kpi-ico c-violet"><Icon name="memory" :size="20" /></span>
            <div style="flex: 1; min-width: 0">
              <div class="kpi-val">{{ formatBytes(info.bytes) }}<span v-if="info.limit_bytes" class="muted kpi-of"> / {{ formatBytes(info.limit_bytes) }}</span></div>
              <div class="kpi-label">{{ t('memcached.memory') }}</div>
              <div v-if="memPct !== null" class="ubar" style="width: 100%" aria-hidden="true"><span :style="{ width: Math.max(1, memPct) + '%' }"></span></div>
            </div>
          </div>
          <div class="kpi">
            <span class="kpi-ico c-blue"><Icon name="layers" :size="20" /></span>
            <div>
              <div class="kpi-val">{{ formatNumber(info.items, locale) }}</div>
              <div class="kpi-label">{{ t('memcached.items') }}</div>
            </div>
          </div>
          <div class="kpi">
            <span class="kpi-ico c-teal"><Icon name="check" :size="20" /></span>
            <div>
              <div class="kpi-val">{{ hitRate }}</div>
              <div class="kpi-label">{{ info.hit_rate >= 0 ? t('memcached.hit_rate', { hits: formatNumber(info.hits, locale), misses: formatNumber(info.misses, locale) }) : t('memcached.no_reads') }}</div>
            </div>
          </div>
        </div>

        <section class="card">
          <div class="card-head">
            <h2>{{ t('memcached.settings') }}</h2>
            <button type="button" class="btn btn-sm" :disabled="busy" @click="restart"><Icon name="refresh" :size="15" />{{ t('pg.restart') }}</button>
          </div>
          <ul class="mc-facts">
            <li><Icon name="lock" :size="14" />{{ t('memcached.listen', { addr: info.listen || '127.0.0.1' }) }}</li>
            <li v-if="info.udp_port" class="text-crit"><Icon name="alert" :size="14" />{{ t('memcached.udp_on', { port: info.udp_port }) }}</li>
            <li v-else><Icon name="shield" :size="14" />{{ t('memcached.udp_off') }}</li>
            <li><Icon name="link" :size="14" />{{ t('memcached.connections', { n: formatNumber(info.connections, locale), max: formatNumber(info.max_conns, locale) }) }}</li>
            <li v-if="info.evictions"><Icon name="alert" :size="14" />{{ t('memcached.evictions', { n: formatNumber(info.evictions, locale) }) }}</li>
          </ul>
          <p class="muted small" style="margin: 0">{{ t('memcached.php_hint') }}</p>
        </section>
      </template>
    </template>
  </div>
</template>

<style scoped>
.mc-facts { list-style: none; margin: 0 0 12px; padding: 0; display: flex; flex-direction: column; gap: 6px; }
.mc-facts li { display: flex; align-items: center; gap: 8px; }
</style>
