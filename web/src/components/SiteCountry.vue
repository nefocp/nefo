<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { COUNTRY_CODES, countryName } from '../countries.js'

// SiteCountry is a website's country blocking (internal/hosting/geo.go):
// off, block a list of countries, or let in only a list. The country names
// come from the browser in the panel's language; the data is DB-IP's, whose
// licence asks for the credit shown at the bottom.
const props = defineProps({ site: { type: Object, required: true }, server: { type: String, default: 'nginx' } })
const emit = defineEmits(['saved'])
const { t, locale } = useI18n()

const info = ref(null)
const error = ref('')
const busy = ref(false)
const mode = ref('')
const chosen = ref([])
const query = ref('')

const name = (c) => countryName(c, locale.value)
const fmtDate = (v) => new Date(v).toLocaleDateString(locale.value, { year: 'numeric', month: 'short', day: 'numeric' })

function apply(res) {
  info.value = res
  mode.value = res.mode || ''
  chosen.value = [...(res.countries || [])]
}

onMounted(async () => {
  try {
    apply(await api('GET', `api/sites/${props.site.id}/country`))
  } catch (e) {
    error.value = errorMessage(e)
  }
})

const sortedChosen = computed(() => [...chosen.value].sort((a, b) => name(a).localeCompare(name(b), locale.value)))

// The countries that match the search and are not chosen yet, by name.
const matches = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return []
  return COUNTRY_CODES.filter((c) => !chosen.value.includes(c) && (c.toLowerCase() === q || name(c).toLowerCase().includes(q)))
    .sort((a, b) => name(a).localeCompare(name(b), locale.value))
    .slice(0, 8)
})

function add(c) {
  if (!chosen.value.includes(c)) chosen.value.push(c)
  query.value = ''
}
function remove(c) {
  chosen.value = chosen.value.filter((x) => x !== c)
}
function onEnter() {
  if (matches.value.length) add(matches.value[0])
}

async function save() {
  error.value = ''
  busy.value = true
  try {
    const res = await api('PUT', `api/sites/${props.site.id}/country`, { mode: mode.value, countries: mode.value ? chosen.value : [] })
    apply(res)
    emit('saved', { ...props.site, geo_mode: res.mode })
  } catch (e) {
    error.value = errorMessage(e)
    // Apache's limit: name the countries that are too large ("US:271881,DE:55789").
    if (e?.code === 'geo_too_large' && /^[A-Z]{2}:[0-9]+(,[A-Z]{2}:[0-9]+)*$/.test(e.data?.detail || '')) {
      const list = e.data.detail.split(',').map((x) => {
        const [c, n] = x.split(':')
        return t('country.too_large_item', { name: name(c), n: Number(n).toLocaleString(locale.value) })
      })
      error.value = t('errors.geo_too_large', { detail: list.join(', ') })
    }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="!info && !error" class="muted">…</div>
  <form v-else class="stack" @submit.prevent="save">
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <template v-if="info">
      <fieldset class="stack">
        <legend class="sr-only">{{ t('drawer.tab_country') }}</legend>
        <label class="check"><input v-model="mode" type="radio" value="" /><span>{{ t('country.mode_off') }}</span></label>
        <label class="check"><input v-model="mode" type="radio" value="block" /><span>{{ t('country.mode_block') }}</span></label>
        <label class="check"><input v-model="mode" type="radio" value="allow" /><span>{{ t('country.mode_allow') }}</span></label>
      </fieldset>

      <div v-if="mode" class="field">
        <span>{{ t('country.countries') }}</span>
        <div class="cc-chips">
          <span v-for="c in sortedChosen" :key="c" class="chip cc-chip" :class="mode === 'block' ? 'cc-block' : 'cc-allow'">
            <span class="mono cc-code">{{ c }}</span>{{ name(c) }}
            <button type="button" class="cc-x" :aria-label="t('country.remove', { name: name(c) })" @click="remove(c)">×</button>
          </span>
          <span v-if="!chosen.length" class="muted small">{{ t('country.none_selected') }}</span>
        </div>
        <div class="cc-search">
          <input v-model="query" class="input" type="search" :placeholder="t('country.search')" :aria-label="t('country.search')" autocomplete="off" @keydown.enter.prevent="onEnter" />
          <ul v-if="query.trim()" class="cc-list" role="listbox">
            <li v-for="c in matches" :key="c">
              <button type="button" class="cc-item" @click="add(c)"><span class="mono cc-code">{{ c }}</span>{{ name(c) }}</button>
            </li>
            <li v-if="!matches.length" class="muted small cc-none">{{ t('country.no_match') }}</li>
          </ul>
        </div>
      </div>

      <div class="cc-notes small">
        <p class="muted">{{ t('country.always') }}</p>
        <p class="muted">{{ t('country.page_403') }}</p>
        <p v-if="server === 'openlitespeed'" class="muted">{{ t('country.cloudflare_ols') }}</p>
        <p v-else class="muted">{{ t('country.cloudflare') }}<template v-if="server === 'apache'"> {{ t('country.cloudflare_apache') }}</template></p>
        <p v-if="server === 'apache'" class="muted">
          <template v-if="info.prefixes">{{ t('country.apache_prefixes', { n: info.prefixes.toLocaleString(locale), max: (info.max_prefixes || 0).toLocaleString(locale) }) }}</template>
          <template v-else>{{ t('country.apache_limit', { max: (info.max_prefixes || 0).toLocaleString(locale) }) }}</template>
        </p>
        <p v-if="server === 'nginx'" class="muted">{{ t('country.memory') }}</p>
        <p class="muted">{{ info.month ? t('country.db', { month: info.month, date: fmtDate(info.fetched) }) : t('country.db_none') }}</p>
        <p class="muted">
          {{ t('country.data') }} <a :href="info.attribution_url" target="_blank" rel="noopener noreferrer">{{ info.attribution }}</a> ({{ info.license }})
        </p>
      </div>
    </template>
    <div class="modal-actions">
      <span v-if="busy" class="muted small">{{ t('country.downloading') }}</span>
      <button v-if="info" class="btn btn-primary" :disabled="busy || (mode && !chosen.length)">{{ t('files.save') }}</button>
    </div>
  </form>
</template>

<style scoped>
fieldset { border: 0; padding: 0; margin: 0; }
.cc-chips { display: flex; flex-wrap: wrap; gap: 6px; margin: 4px 0 8px; }
.cc-chip { gap: 6px; padding-right: 4px; }
.cc-block { background: var(--crit-soft); color: var(--crit); }
.cc-allow { background: var(--ok-soft); color: var(--ok); }
.cc-code { font-size: 11.5px; font-weight: 700; opacity: 0.8; }
.cc-x { border: 0; background: transparent; color: inherit; cursor: pointer; font-size: 15px; line-height: 1; padding: 0 4px; border-radius: 4px; }
.cc-x:hover { background: color-mix(in srgb, currentColor 15%, transparent); }
.cc-search { position: relative; max-width: 360px; }
.cc-list { list-style: none; margin: 4px 0 0; padding: 4px; border: 1px solid var(--border); border-radius: 8px; background: var(--surface); box-shadow: var(--shadow-md, 0 4px 14px rgba(0, 0, 0, 0.08)); }
.cc-item { display: flex; gap: 8px; align-items: center; width: 100%; text-align: left; border: 0; background: transparent; color: var(--text); padding: 6px 8px; border-radius: 6px; cursor: pointer; font: inherit; }
.cc-item:hover, .cc-item:focus-visible { background: var(--accent-soft); }
.cc-none { padding: 6px 8px; }
.cc-notes p { margin: 0 0 6px; max-width: 70ch; }
</style>
