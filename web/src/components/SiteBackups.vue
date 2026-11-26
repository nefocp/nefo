<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { store, load, list } from '../data.js'
import { formatBytesL } from '../format.js'
import { toast } from '../toast.js'
import Icon from './Icon.vue'

// SiteBackups is the backups tab of the site drawer: this site's backups and
// schedules, "back up now" and a daily schedule in one click. Restoring and
// remote storage stay on the Backups page.
const props = defineProps({ site: { type: Object, required: true }, busy: { type: Boolean, default: false } })
const emit = defineEmits(['job'])
const { t, locale } = useI18n()
const error = ref('')
const loading = ref(true)

async function refresh() {
  loading.value = true
  try {
    await load('api/backups', { force: true })
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}
onMounted(refresh)
defineExpose({ refresh })

const mine = computed(() =>
  list(store['api/backups']?.backups)
    .filter((b) => b.kind === 'site' && b.target === props.site.domain)
    .sort((a, b) => new Date(b.created_at) - new Date(a.created_at)),
)
const schedules = computed(() => list(store['api/backups']?.schedules).filter((s) => s.kind === 'site' && s.target === props.site.domain))
const hh = (h) => String(h).padStart(2, '0') + ':00'
const weekday = (d) => new Date(2026, 10, 1 + d).toLocaleDateString(locale.value, { weekday: 'long' })
const describe = (sc) => (sc.frequency === 'daily' ? t('backups.every_day', { time: hh(sc.hour) }) : t('backups.every_week', { day: weekday(sc.weekday), time: hh(sc.hour) }))
const fmt = (v) => new Date(v).toLocaleString(locale.value, { dateStyle: 'medium', timeStyle: 'short' })

async function backupNow() {
  try {
    emit('job', await api('POST', 'api/backups', { kind: 'site', target: props.site.domain }))
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  }
}

async function addDaily() {
  try {
    await api('POST', 'api/backups/schedules', { kind: 'site', target: props.site.domain, frequency: 'daily', hour: 3, weekday: 0, keep: 7, remote: false })
    toast(t('drawer.backup_scheduled'))
    await refresh()
  } catch (e) {
    toast(errorMessage(e), 'crit', 6000)
  }
}
</script>

<template>
  <div class="stack">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <section class="card">
      <div class="card-head">
        <h3>{{ t('drawer.backup_schedule') }}</h3>
        <button type="button" class="btn btn-primary" :disabled="busy" @click="backupNow"><Icon name="archive" :size="15" />{{ t('drawer.backup_now') }}</button>
      </div>
      <div v-if="schedules.length" class="stack-tight">
        <div v-for="sc in schedules" :key="sc.id" class="row">
          <Icon name="clock" :size="16" class="muted" />
          <strong>{{ describe(sc) }}</strong>
          <span class="muted small">{{ t('drawer.backup_keep', { n: sc.keep }) }}<template v-if="sc.remote"> · {{ t('drawer.backup_remote') }}</template></span>
          <span v-if="sc.last_error" class="tag tag-crit">{{ t('jobs.failed') }}</span>
        </div>
      </div>
      <div v-else class="row">
        <span class="muted">{{ t('drawer.backup_none_scheduled') }}</span>
        <button type="button" class="btn btn-sm" @click="addDaily"><Icon name="plus" :size="14" />{{ t('drawer.backup_add_daily') }}</button>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3 class="panel-title">{{ t('drawer.backup_list', { n: mine.length }) }}</h3>
        <RouterLink :to="{ name: 'backups' }" class="btn btn-sm btn-ghost">{{ t('drawer.backup_manage') }}<Icon name="chevronRight" :size="14" /></RouterLink>
      </div>
      <div v-if="loading && !mine.length" class="panel-body"><span class="skel" style="height: 40px"></span></div>
      <p v-else-if="!mine.length" class="muted panel-body" style="margin: 0">{{ t('drawer.backup_empty') }}</p>
      <div v-else class="table-wrap">
        <table class="table">
          <thead>
            <tr><th>{{ t('drawer.backup_date') }}</th><th>{{ t('drawer.backup_size') }}</th><th>{{ t('drawer.backup_where') }}</th><th class="actions"><span class="sr-only">{{ t('common.more') }}</span></th></tr>
          </thead>
          <tbody>
            <tr v-for="b in mine.slice(0, 20)" :key="b.id">
              <td class="nowrap">{{ fmt(b.created_at) }} <span v-if="b.scheduled" class="tag">{{ t('drawer.backup_auto') }}</span></td>
              <td class="nowrap">{{ formatBytesL(b.size, locale) }}</td>
              <td class="nowrap"><span v-if="b.local" class="tag">{{ t('drawer.backup_local') }}</span> <span v-for="r in b.remotes || []" :key="r" class="tag tag-info">{{ r.toUpperCase() }}</span></td>
              <td class="actions">
                <a v-if="b.local" class="btn btn-sm" :href="`api/backups/${b.id}/download`"><Icon name="download" :size="14" />{{ t('files.download') }}</a>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
