<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { toast } from '../toast.js'
import { usePageMeta } from '../page.js'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import RowActions from '../components/RowActions.vue'
import DnsMailSetup from '../components/DnsMailSetup.vue'
import { EDITABLE, PROXIABLE, TTLS, ttlText, panelHost } from '../dns.js'

// The DNS page: the zones the Cloudflare token can see, a zone's records
// (add, edit, delete the common types), and purging Cloudflare's cache.
const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const zones = ref(null) // null while loading
const noToken = ref(false)
const error = ref('')
const zoneId = ref('')
const zone = computed(() => (zones.value || []).find((z) => z.id === zoneId.value) || null)

const list = ref(null)
const loading = ref(false)
const recError = ref('')
const type = ref('all')
const q = ref('')
const limit = ref(200)

async function loadZones() {
  error.value = ''
  try {
    const res = await api('GET', 'api/dns/zones')
    zones.value = res.zones || []
    const z = zones.value.find((x) => x.name === route.query.zone) || zones.value[0]
    if (z) zoneId.value = z.id
  } catch (e) {
    zones.value = []
    if (e.code === 'cloudflare_not_configured') noToken.value = true
    else error.value = errorMessage(e)
  }
}

async function loadRecords() {
  if (!zoneId.value) return
  loading.value = true
  recError.value = ''
  try {
    list.value = await api('GET', `api/dns/zones/${zoneId.value}/records`)
  } catch (e) {
    list.value = null
    recError.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

watch(zoneId, () => {
  list.value = null
  limit.value = 200
  type.value = 'all'
  if (zone.value && route.query.zone !== zone.value.name) router.replace({ query: { ...route.query, zone: zone.value.name } })
  loadRecords()
})

const records = computed(() => list.value?.records || [])
const typeTabs = computed(() => {
  const have = new Set(records.value.map((r) => r.type))
  const tabs = EDITABLE.filter((x) => have.has(x))
  if ([...have].some((x) => !EDITABLE.includes(x))) tabs.push('other')
  return ['all', ...tabs]
})
const filtered = computed(() => {
  const f = q.value.trim().toLowerCase()
  return records.value.filter((r) => {
    const okType = type.value === 'all' || (type.value === 'other' ? !EDITABLE.includes(r.type) : r.type === type.value)
    return okType && (!f || `${r.name} ${r.content}`.toLowerCase().includes(f))
  })
})
const shown = computed(() => filtered.value.slice(0, limit.value))
function setType(x) {
  type.value = x
  limit.value = 200
}
usePageMeta(() => ({ sub: list.value ? `${zone.value?.name || ''} · ${t('dns.sub', { n: records.value.length }, records.value.length)}` : '' }))

// The name without the zone: "@" for the zone itself.
function relName(name) {
  const z = zone.value?.name || ''
  if (name === z) return '@'
  return z && name.endsWith('.' + z) ? name.slice(0, -z.length - 1) : name
}
const contentText = (r) => (r.type === 'MX' && r.priority != null ? `${r.priority} ${r.content}` : r.content)
const zoneStatus = (s) => (s === 'active' ? t('dns.zone_active') : s === 'pending' ? t('dns.zone_pending') : s)

// ---- add and edit ----
const form = ref(null)
const formError = ref('')
const busy = ref(false)

function openForm(r) {
  formError.value = ''
  form.value = r
    ? { id: r.id, type: r.type, name: relName(r.name), content: r.type === 'CAA' ? '' : r.content, proxied: r.proxied, ttl: r.ttl || 1,
        priority: r.priority ?? 10, flags: r.data?.flags ?? 0, tag: r.data?.tag || 'issue', value: r.data?.value || '' }
    : { id: '', type: 'A', name: '', content: '', proxied: false, ttl: 1, priority: 10, flags: 0, tag: 'issue', value: 'letsencrypt.org' }
}
const canProxy = computed(() => form.value && PROXIABLE.includes(form.value.type))
const ttlOptions = computed(() => (form.value && !TTLS.includes(Number(form.value.ttl)) ? [...TTLS, Number(form.value.ttl)].sort((a, b) => a - b) : TTLS))
const formFullName = computed(() => {
  if (!form.value || !zone.value) return ''
  const n = form.value.name.trim().toLowerCase().replace(/\.$/, '')
  const z = zone.value.name
  return !n || n === '@' ? z : n === z || n.endsWith('.' + z) ? n : `${n}.${z}`
})
const formPanelWarn = computed(() => canProxy.value && form.value.proxied && panelHost && formFullName.value === panelHost)

async function save() {
  const f = form.value
  busy.value = true
  formError.value = ''
  const proxied = canProxy.value && f.proxied
  const body = { type: f.type, name: f.name, content: f.content, proxied, ttl: proxied ? 1 : Number(f.ttl), priority: Number(f.priority) || 0 }
  if (f.type === 'CAA') {
    body.content = ''
    body.data = { flags: Number(f.flags), tag: f.tag, value: f.value }
  }
  try {
    await api(f.id ? 'PUT' : 'POST', f.id ? `api/dns/zones/${zoneId.value}/records/${f.id}` : `api/dns/zones/${zoneId.value}/records`, body)
    toast(t(f.id ? 'dns.updated' : 'dns.created'))
    form.value = null
    await loadRecords()
  } catch (e) {
    formError.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function remove(r) {
  if (!window.confirm(t('dns.delete_confirm', { type: r.type, name: r.name }))) return
  try {
    await api('DELETE', `api/dns/zones/${zoneId.value}/records/${r.id}`)
    toast(t('dns.deleted'))
    await loadRecords()
  } catch (e) {
    toast(errorMessage(e), 'crit', 9000)
  }
}

// ---- e-mail records (DnsMailSetup) ----
const mailSetup = ref(false)
async function mailDone() {
  mailSetup.value = false
  await loadRecords()
}

// ---- purge ----
const purge = ref(null)
const purgeError = ref('')

function openPurge() {
  purgeError.value = ''
  purge.value = { mode: 'all', urls: '' }
}

async function doPurge() {
  const p = purge.value
  busy.value = true
  purgeError.value = ''
  try {
    const res = await api('POST', `api/dns/zones/${zoneId.value}/purge`, p.mode === 'all' ? { everything: true } : { everything: false, urls: p.urls.split('\n') })
    toast(p.mode === 'all' ? t('dns.purged_all', { zone: res.zone }) : t('dns.purged_urls', { n: res.urls }, res.urls))
    purge.value = null
  } catch (e) {
    purgeError.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

onMounted(loadZones)
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('dns.title') }}</h1></div>
    <Teleport defer to="#topbar-actions">
      <template v-if="zone">
        <button class="btn" @click="mailSetup = true"><Icon name="mail" :size="15" />{{ t('dnsmail.button') }}</button>
        <button class="btn" @click="openPurge"><Icon name="refresh" :size="15" />{{ t('dns.purge') }}</button>
        <button class="btn btn-primary" @click="openForm(null)"><Icon name="plus" :size="15" />{{ t('dns.add') }}</button>
      </template>
    </Teleport>
    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>

    <span v-if="zones === null" class="skel" style="display: block; height: 260px; border-radius: 14px"></span>

    <section v-else-if="noToken" class="panel">
      <div class="empty">
        <div class="empty-icon"><Icon name="cloud" :size="26" /></div>
        <p class="empty-lead">{{ t('dns.no_token') }}</p>
        <p class="muted">{{ t('dns.no_token_hint') }}</p>
        <RouterLink class="btn btn-primary" :to="{ name: 'settings', query: { tab: 'cloudflare' } }">{{ t('dns.open_settings') }}</RouterLink>
      </div>
    </section>

    <section v-else-if="!zones.length && !error" class="panel">
      <div class="empty">
        <div class="empty-icon"><Icon name="cloud" :size="26" /></div>
        <p class="empty-lead">{{ t('dns.no_zones') }}</p>
        <p class="muted">{{ t('dns.no_zones_hint') }}</p>
      </div>
    </section>

    <section v-else-if="zone" class="panel">
      <div class="panel-head">
        <div class="row zone-line">
          <select v-if="zones.length > 1" v-model="zoneId" class="input input-sm input-auto" :aria-label="t('dns.zone')">
            <option v-for="z in zones" :key="z.id" :value="z.id">{{ z.name }}</option>
          </select>
          <h2 v-else class="panel-title">{{ zone.name }}</h2>
          <span class="muted small">{{ zoneStatus(zone.status) }}<template v-if="zone.plan"> · {{ zone.plan }}</template></span>
        </div>
      </div>
      <p v-if="zone.status !== 'active'" class="alert alert-warn zone-alert">{{ t('dns.zone_not_active', { zone: zone.name, status: zone.status }) }}</p>

      <div class="panel-tools">
        <div class="tabs" role="group" :aria-label="t('dns.col_type')">
          <button v-for="x in typeTabs" :key="x" type="button" :aria-pressed="type === x" @click="setType(x)">
            {{ x === 'all' ? t('dns.all') : x === 'other' ? t('dns.other') : x }}
          </button>
        </div>
        <span class="grow"></span>
        <label class="search-field">
          <Icon name="search" :size="15" />
          <input v-model="q" type="search" :placeholder="t('dns.filter')" :aria-label="t('dns.filter')" @input="limit = 200" />
        </label>
      </div>

      <p v-if="recError" class="alert alert-crit pre-wrap zone-alert" role="alert">{{ recError }}</p>
      <div v-if="loading && !list" class="panel-body"><span class="skel" style="height: 180px; border-radius: 10px; display: block"></span></div>
      <p v-else-if="list && !records.length" class="muted panel-body" style="margin: 0">{{ t('dns.empty') }}</p>
      <p v-else-if="list && !filtered.length" class="muted panel-body" style="margin: 0">{{ t('pg.no_match') }}</p>
      <div v-else-if="list" class="table-wrap">
        <table class="table dns-table">
          <thead>
            <tr>
              <th>{{ t('dns.col_type') }}</th>
              <th>{{ t('dns.col_name') }}</th>
              <th>{{ t('dns.col_content') }}</th>
              <th>{{ t('dns.col_proxy') }}</th>
              <th>{{ t('dns.col_ttl') }}</th>
              <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in shown" :key="r.id">
              <td class="mono nowrap">{{ r.type }}</td>
              <td class="mono dns-name" :title="r.name">{{ relName(r.name) }}</td>
              <td class="mono dns-content" :title="contentText(r)">{{ contentText(r) }}</td>
              <td class="nowrap small">
                <template v-if="PROXIABLE.includes(r.type)">
                  <span v-if="r.proxied" class="proxy-on"><Icon name="cloud" :size="14" />{{ t('dns.proxied') }}</span>
                  <span v-else class="muted">{{ t('dns.dns_only') }}</span>
                  <div v-if="r.proxied && panelHost && r.name === panelHost" class="text-warn">{{ t('dns.panel_short') }}</div>
                </template>
              </td>
              <td class="nowrap small">{{ ttlText(r.ttl, t) }}</td>
              <td class="actions">
                <RowActions
                  v-if="EDITABLE.includes(r.type)"
                  :actions="[
                    { label: t('dns.edit'), icon: 'edit', inline: true, onClick: () => openForm(r) },
                    { label: t('dns.delete'), icon: 'trash', danger: true, onClick: () => remove(r) },
                  ]"
                />
                <span v-else class="muted small" :title="t('dns.read_only', { type: r.type })">{{ t('dns.read_only_short') }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="filtered.length > limit" class="panel-foot">
        <button type="button" class="btn btn-sm" @click="limit += 500">{{ t('dns.show_more', { n: Math.min(500, filtered.length - limit) }) }}</button>
        <span class="muted">{{ t('dns.showing', { n: limit, total: filtered.length }) }}</span>
      </div>
      <div v-if="list?.more" class="panel-foot muted">{{ t('dns.more', { n: list.limit }) }}</div>
    </section>

    <Modal v-if="form" :title="form.id ? t('dns.edit_title') : t('dns.new_title')" @close="form = null">
      <form class="stack" @submit.prevent="save">
        <p v-if="formError" class="alert alert-crit pre-wrap" role="alert">{{ formError }}</p>
        <div class="form-grid">
          <label class="field">
            <span>{{ t('dns.type') }}</span>
            <select v-model="form.type" class="input" :disabled="!!form.id">
              <option v-for="x in EDITABLE" :key="x" :value="x">{{ x }}</option>
            </select>
          </label>
          <label class="field">
            <span>{{ t('dns.name') }}</span>
            <input v-model="form.name" class="input mono" spellcheck="false" autocomplete="off" placeholder="@" />
          </label>
        </div>
        <small class="muted field-note">{{ t('dns.name_hint', { zone: zone.name }) }} <span class="mono">{{ formFullName }}</span></small>

        <label v-if="form.type === 'A' || form.type === 'AAAA'" class="field">
          <span>{{ form.type === 'A' ? t('dns.ipv4') : t('dns.ipv6') }}</span>
          <input v-model="form.content" class="input mono" spellcheck="false" autocomplete="off" required :placeholder="form.type === 'A' ? '203.0.113.10' : '2001:db8::10'" />
        </label>
        <label v-else-if="form.type === 'CNAME'" class="field">
          <span>{{ t('dns.target') }}</span>
          <input v-model="form.content" class="input mono" spellcheck="false" autocomplete="off" required :placeholder="zone.name" />
          <small class="muted">{{ t('dns.target_hint', { zone: zone.name }) }}</small>
        </label>
        <label v-else-if="form.type === 'TXT'" class="field">
          <span>{{ t('dns.txt') }}</span>
          <textarea v-model="form.content" class="input mono" rows="4" spellcheck="false" maxlength="2048" required></textarea>
          <small class="muted">{{ t('dns.txt_hint') }} {{ form.content.length }}/2048</small>
        </label>
        <div v-else-if="form.type === 'MX'" class="form-grid">
          <label class="field">
            <span>{{ t('dns.mail_server') }}</span>
            <input v-model="form.content" class="input mono" spellcheck="false" autocomplete="off" required :placeholder="'mail.' + zone.name" />
          </label>
          <label class="field">
            <span>{{ t('dns.priority') }}</span>
            <input v-model="form.priority" class="input" type="number" min="0" max="65535" required />
            <small class="muted">{{ t('dns.priority_hint') }}</small>
          </label>
        </div>
        <template v-else-if="form.type === 'CAA'">
          <div class="form-grid">
            <label class="field">
              <span>{{ t('dns.caa_tag') }}</span>
              <select v-model="form.tag" class="input">
                <option value="issue">{{ t('dns.caa_issue') }}</option>
                <option value="issuewild">{{ t('dns.caa_issuewild') }}</option>
                <option value="iodef">{{ t('dns.caa_iodef') }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ t('dns.caa_flags') }}</span>
              <select v-model="form.flags" class="input">
                <option :value="0">{{ t('dns.caa_flags_0') }}</option>
                <option :value="128">{{ t('dns.caa_flags_128') }}</option>
              </select>
            </label>
          </div>
          <label class="field">
            <span>{{ t('dns.caa_value') }}</span>
            <input v-model="form.value" class="input mono" spellcheck="false" autocomplete="off" required :placeholder="form.tag === 'iodef' ? 'mailto:security@' + zone.name : 'letsencrypt.org'" />
            <small class="muted">{{ t('dns.caa_value_hint') }}</small>
          </label>
        </template>

        <div v-if="canProxy" class="opt">
          <div class="opt-text">
            <div class="opt-title">{{ t('dns.proxy') }}</div>
            <div class="opt-hint">{{ t('dns.proxy_hint') }}</div>
          </div>
          <button type="button" class="switch" role="switch" :aria-checked="form.proxied" :aria-label="t('dns.proxy')" @click="form.proxied = !form.proxied"></button>
        </div>
        <p v-if="formPanelWarn" class="alert alert-warn">{{ t('dns.panel_proxied', { host: panelHost }) }}</p>

        <label class="field">
          <span>{{ t('dns.ttl') }}</span>
          <select v-model="form.ttl" class="input input-auto" :disabled="canProxy && form.proxied">
            <option v-for="x in ttlOptions" :key="x" :value="x">{{ ttlText(x, t) }}</option>
          </select>
          <small v-if="canProxy && form.proxied" class="muted">{{ t('dns.ttl_proxied') }}</small>
        </label>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="form = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="busy">{{ t('dns.save') }}</button>
        </div>
      </form>
    </Modal>

    <DnsMailSetup v-if="mailSetup && zone" :zone="zone" @close="mailSetup = false" @done="mailDone" />

    <Modal v-if="purge" :title="t('dns.purge_title', { zone: zone.name })" @close="purge = null">
      <form class="stack" @submit.prevent="doPurge">
        <p v-if="purgeError" class="alert alert-crit pre-wrap" role="alert">{{ purgeError }}</p>
        <label class="check"><input v-model="purge.mode" type="radio" value="all" />{{ t('dns.purge_all') }}</label>
        <small v-if="purge.mode === 'all'" class="muted field-note">{{ t('dns.purge_all_hint') }}</small>
        <label class="check"><input v-model="purge.mode" type="radio" value="urls" />{{ t('dns.purge_urls') }}</label>
        <label v-if="purge.mode === 'urls'" class="field">
          <span class="sr-only">{{ t('dns.purge_urls') }}</span>
          <textarea v-model="purge.urls" class="input mono" rows="5" spellcheck="false" required :placeholder="`https://${zone.name}/style.css`"></textarea>
          <small class="muted">{{ t('dns.purge_urls_hint', { zone: zone.name }) }}</small>
        </label>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="purge = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="busy">{{ t('dns.purge_go') }}</button>
        </div>
      </form>
    </Modal>
  </div>
</template>

<style scoped>
.zone-line { gap: 12px; }
.zone-alert { margin: 12px 18px; }
.dns-name { max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dns-content { max-width: 420px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12.5px; }
.proxy-on { display: inline-flex; align-items: center; gap: 5px; }
.form-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 14px; }
.field-note { margin-top: -8px; font-size: 12.5px; }
</style>
