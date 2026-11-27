<script setup>
import { ref, reactive, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { toast } from '../toast.js'
import Icon from './Icon.vue'
import Modal from './Modal.vue'

// "Set up email" on the DNS page: the MX, SPF and DMARC records a mail
// provider needs (and a DKIM record the provider generated), shown as a
// preview before anything changes. The panel merges SPF into one record
// and replaces another provider's MX records only when asked.
const props = defineProps({
  zone: { type: Object, required: true }, // { id, name }
})
const emit = defineEmits(['close', 'done'])
const { t } = useI18n()

const PROVIDERS = ['google', 'microsoft', 'zoho', 'cloudflare', 'custom']
const REGIONS = ['com', 'eu', 'in', 'com.au']
// The DKIM records each provider gives: Microsoft two CNAMEs, the others a
// TXT key; Cloudflare's Email Routing needs none here.
const DKIM_ROWS = {
  google: [{ name: 'google._domainkey', type: 'TXT' }],
  microsoft: [{ name: 'selector1._domainkey', type: 'CNAME' }, { name: 'selector2._domainkey', type: 'CNAME' }],
  zoho: [{ name: '', type: 'TXT', placeholder: 'zmail._domainkey' }],
  cloudflare: [],
  custom: [{ name: '', type: 'TXT', placeholder: 'selector._domainkey' }],
}

const f = reactive({ provider: '', region: 'com', domain: '', report: '', mx: [{ host: '', priority: 10 }], include: '', ips: '', dkim: [], replace_mx: false })
const busy = ref(false)
const error = ref('')
const plan = ref(null)
const plannedFor = ref('') // the request the plan was made for

function pick(p) {
  if (f.provider === p) return
  f.provider = p
  f.replace_mx = false
  f.dkim = DKIM_ROWS[p].map((r) => ({ ...r, value: '' }))
  plan.value = null
  error.value = ''
}

const body = computed(() => {
  const o = { provider: f.provider, domain: f.domain.trim(), dmarc_report: f.report.trim(), replace_mx: f.replace_mx }
  if (f.provider === 'zoho') o.region = f.region
  if (f.provider === 'custom') {
    o.mx = f.mx.filter((x) => x.host.trim()).map((x) => ({ host: x.host.trim(), priority: Number(x.priority) || 0 }))
    o.spf_include = f.include.trim()
    o.spf_ips = f.ips.split(/[\s,]+/).filter(Boolean)
  }
  o.dkim = f.dkim.filter((d) => d.value.trim()).map((d) => ({ name: d.name.trim(), type: d.type, value: d.value.trim() }))
  return o
})
const dirty = computed(() => !plan.value || plannedFor.value !== JSON.stringify(body.value))

async function preview(keepError = false) {
  if (!f.provider) return
  busy.value = true
  if (!keepError) error.value = ''
  const req = JSON.stringify(body.value)
  try {
    plan.value = await api('POST', `api/dns/zones/${props.zone.id}/mail/plan`, body.value)
    plannedFor.value = req
  } catch (e) {
    plan.value = null
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

function toggleReplace() {
  f.replace_mx = !f.replace_mx
  preview()
}

const changes = computed(() => plan.value?.changes || [])
const pending = computed(() => changes.value.filter((c) => c.action !== 'keep'))

// The name without the zone: "@" for the zone itself.
function relName(name) {
  const z = props.zone.name
  if (name === z) return '@'
  return name.endsWith('.' + z) ? name.slice(0, -z.length - 1) : name
}
const roleText = (c) => (c.type === 'MX' ? 'MX' : `${c.type} · ${c.role.toUpperCase()}`)
const actText = (a) => (a === 'conflict' ? t('dnsmail.act_conflict') : t('dns.act_' + a))
const actClass = (a) => ({ create: 'ok', update: 'warn', delete: 'crit', conflict: 'crit' })[a] || ''

function addMX() {
  if (f.mx.length < 5) f.mx.push({ host: '', priority: (f.mx.length + 1) * 10 })
}

async function apply() {
  busy.value = true
  error.value = ''
  try {
    const res = await api('POST', `api/dns/zones/${props.zone.id}/mail/apply`, { ...JSON.parse(plannedFor.value), hash: plan.value.hash })
    toast(t('dnsmail.done', { n: res.changed }, res.changed))
    emit('done')
  } catch (e) {
    error.value = errorMessage(e)
    busy.value = false
    // Show what is at Cloudflare now: after a refusal some changes may be made.
    await preview(true)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Modal wide :title="t('dnsmail.title', { zone: zone.name })" @close="emit('close')">
    <div class="stack">
      <p class="muted" style="margin: 0">{{ t('dnsmail.intro') }}</p>
      <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>

      <div class="field">
        <span>{{ t('dnsmail.provider') }}</span>
        <div class="prov-grid" role="radiogroup" :aria-label="t('dnsmail.provider')">
          <button v-for="p in PROVIDERS" :key="p" type="button" class="prov" role="radio" :aria-checked="f.provider === p" @click="pick(p)">
            <span class="prov-name">{{ t('dnsmail.p_' + p) }}</span>
            <span class="prov-hint">{{ t('dnsmail.p_' + p + '_hint') }}</span>
          </button>
        </div>
      </div>

      <template v-if="f.provider">
        <div class="form-grid">
          <label class="field">
            <span>{{ t('dnsmail.domain') }}</span>
            <input v-model="f.domain" class="input mono" spellcheck="false" autocomplete="off" :placeholder="zone.name" />
            <small class="muted">{{ t('dnsmail.domain_hint', { zone: zone.name }) }}</small>
          </label>
          <label v-if="f.provider === 'zoho'" class="field">
            <span>{{ t('dnsmail.region') }}</span>
            <select v-model="f.region" class="input">
              <option v-for="r in REGIONS" :key="r" :value="r">{{ t('dnsmail.region_' + r.replace('.', '_')) }}</option>
            </select>
            <small class="muted">{{ t('dnsmail.region_hint') }}</small>
          </label>
        </div>
        <p v-if="f.provider === 'microsoft'" class="alert alert-info" style="margin: 0">{{ t('dnsmail.microsoft_note') }}</p>
        <p v-if="f.provider === 'cloudflare'" class="alert alert-info" style="margin: 0">{{ t('dnsmail.cloudflare_note') }}</p>

        <template v-if="f.provider === 'custom'">
          <h3 class="sub-title">{{ t('dnsmail.mx_title') }}</h3>
          <div v-for="(x, i) in f.mx" :key="i" class="mx-row">
            <label class="field grow">
              <span :class="{ 'sr-only': i > 0 }">{{ t('dnsmail.mx_host') }}</span>
              <input v-model="x.host" class="input mono" spellcheck="false" autocomplete="off" placeholder="mx1.example.net" />
            </label>
            <label class="field mx-prio">
              <span :class="{ 'sr-only': i > 0 }">{{ t('dns.priority') }}</span>
              <input v-model="x.priority" class="input" type="number" min="0" max="65535" />
            </label>
            <button v-if="f.mx.length > 1" type="button" class="icon-btn" :aria-label="t('dnsmail.mx_remove')" :title="t('dnsmail.mx_remove')" @click="f.mx.splice(i, 1)"><Icon name="x" :size="15" /></button>
          </div>
          <div><button v-if="f.mx.length < 5" type="button" class="btn btn-sm" @click="addMX"><Icon name="plus" :size="14" />{{ t('dnsmail.mx_add') }}</button></div>
          <div class="form-grid">
            <label class="field">
              <span>{{ t('dnsmail.spf_include') }}</span>
              <input v-model="f.include" class="input mono" spellcheck="false" autocomplete="off" placeholder="spf.example.net" />
              <small class="muted">{{ t('dnsmail.spf_include_hint') }}</small>
            </label>
            <label class="field">
              <span>{{ t('dnsmail.spf_ips') }}</span>
              <input v-model="f.ips" class="input mono" spellcheck="false" autocomplete="off" placeholder="203.0.113.10 198.51.100.0/24" />
              <small class="muted">{{ t('dnsmail.spf_ips_hint') }}</small>
            </label>
          </div>
        </template>

        <label class="field">
          <span>{{ t('dnsmail.dmarc_report') }}</span>
          <input v-model="f.report" class="input" type="email" autocomplete="off" :placeholder="'dmarc@' + zone.name" />
          <small class="muted">{{ t('dnsmail.dmarc_hint') }}</small>
        </label>

        <h3 class="sub-title">{{ t('dnsmail.dkim_title') }}</h3>
        <p class="muted small" style="margin: 0">{{ t('dnsmail.dkim_' + f.provider) }}</p>
        <div v-for="(d, i) in f.dkim" :key="i" class="dkim-row">
          <div class="form-grid">
            <label class="field">
              <span>{{ t('dnsmail.dkim_name') }}</span>
              <input v-model="d.name" class="input mono" spellcheck="false" autocomplete="off" :placeholder="d.placeholder || ''" />
            </label>
            <label class="field">
              <span>{{ t('dns.type') }}</span>
              <select v-model="d.type" class="input">
                <option value="TXT">TXT</option>
                <option value="CNAME">CNAME</option>
              </select>
            </label>
          </div>
          <label class="field">
            <span>{{ t('dnsmail.dkim_value') }}</span>
            <textarea v-model="d.value" class="input mono" :rows="d.type === 'TXT' ? 3 : 1" spellcheck="false" maxlength="2048"></textarea>
          </label>
        </div>
        <small v-if="f.dkim.length" class="muted field-note">{{ t('dnsmail.dkim_later') }}</small>

        <span v-if="busy && !plan" class="skel" style="display: block; height: 120px; border-radius: 10px"></span>
        <template v-if="plan">
          <div class="table-wrap mail-table">
            <table class="table">
              <thead>
                <tr>
                  <th>{{ t('dns.col_name') }}</th>
                  <th>{{ t('dns.col_type') }}</th>
                  <th>{{ t('dns.col_now') }}</th>
                  <th>{{ t('dns.col_after') }}</th>
                  <th><span class="sr-only">{{ t('dns.col_change') }}</span></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(c, i) in changes" :key="i" :class="{ 'row-keep': c.action === 'keep' }">
                  <td class="mono c-name">{{ relName(c.name) }}</td>
                  <td class="mono nowrap small c-type">{{ roleText(c) }}</td>
                  <td class="mono small val" :data-label="t('dns.col_now')">{{ c.old || '—' }}</td>
                  <td class="mono small val" :data-label="t('dns.col_after')">{{ c.action === 'delete' ? t('dns.point_removed') : c.action === 'conflict' ? '—' : c.new }}</td>
                  <td class="nowrap c-act"><span v-if="actClass(c.action)" class="status-pill" :class="actClass(c.action)">{{ actText(c.action) }}</span><span v-else class="muted small">{{ actText(c.action) }}</span></td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-for="n in plan.notes" :key="n.code" class="alert" :class="n.code === 'spf_lookups' || n.code === 'spf_merged' ? 'alert-warn' : 'alert-info'" style="margin: 0">
            {{ t('dnsmail.note_' + n.code, { detail: n.detail || '' }) }}
          </p>
          <div v-if="plan.conflicts" class="alert alert-warn stack-sm" style="margin: 0">
            <div>{{ t('dnsmail.conflicts', { n: plan.conflicts }, plan.conflicts) }}</div>
            <label class="check"><input type="checkbox" :checked="f.replace_mx" :disabled="busy" @change="toggleReplace" /><span>{{ t('dnsmail.replace_mx') }}</span></label>
            <small>{{ t('dnsmail.replace_hint') }}</small>
          </div>
          <p v-if="!pending.length" class="muted" style="margin: 0">{{ t('dnsmail.nothing') }}</p>
        </template>
      </template>

      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button v-if="dirty" type="button" class="btn" :class="{ 'btn-primary': !plan }" :disabled="busy || !f.provider" @click="preview()">
          <span v-if="busy" class="spinner" aria-hidden="true"></span>{{ plan ? t('dns.point_preview') : t('dnsmail.preview') }}
        </button>
        <button v-if="plan" type="button" class="btn btn-primary" :disabled="busy || dirty || plan.need_confirm || !pending.length" @click="apply">
          {{ t('dnsmail.apply', { n: pending.length }, pending.length) }}
        </button>
      </div>
    </div>
  </Modal>
</template>

<style scoped>
.prov-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(170px, 1fr)); gap: 10px; }
.prov {
  display: flex; flex-direction: column; align-items: flex-start; gap: 3px; padding: 11px 13px; text-align: left; cursor: pointer;
  border: 1px solid var(--border); border-radius: 12px; background: var(--surface); color: var(--text); font: inherit;
}
.prov:hover { border-color: var(--border-strong); }
.prov[aria-checked='true'] { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.prov-name { font-weight: 700; }
.prov-hint { font-size: 12px; color: var(--muted); }
.form-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 14px; }
.sub-title { font-size: 14px; margin: 4px 0 0; }
.mx-row { display: flex; gap: 10px; align-items: flex-end; }
.mx-row .grow { flex: 1; min-width: 0; }
.mx-prio { width: 110px; flex: none; }
.dkim-row { display: flex; flex-direction: column; gap: 10px; padding: 12px; border: 1px solid var(--border); border-radius: 12px; }
.field-note { margin-top: -6px; font-size: 12.5px; }
.mail-table { margin: 0; }
.mail-table .table th:first-child, .mail-table .table td:first-child { padding-left: 0; }
.mail-table .val { overflow-wrap: anywhere; max-width: 280px; }
.row-keep td { color: var(--muted); }
.stack-sm { display: flex; flex-direction: column; gap: 8px; }
/* phones: one block per record, "Now" and "After" under the name */
@media (max-width: 640px) {
  .mail-table thead { display: none; }
  .mail-table tbody tr { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 4px 10px; padding: 10px 0; border-bottom: 1px solid var(--border); }
  .mail-table tbody td { padding: 0; border: 0; }
  .mail-table .c-act { grid-column: 2; grid-row: 1 / span 2; }
  .mail-table .val { grid-column: 1 / -1; max-width: none; }
  .mail-table .val::before { content: attr(data-label) ': '; color: var(--muted); font-family: var(--font, inherit); }
  .mx-prio { width: 90px; }
}
</style>
