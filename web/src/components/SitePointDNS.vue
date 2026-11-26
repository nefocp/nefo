<script setup>
import { ref, reactive, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { errorMessage } from '../errors.js'
import { toast } from '../toast.js'
import { PROXIABLE, panelHost } from '../dns.js'
import Icon from './Icon.vue'
import Modal from './Modal.vue'

// "Point DNS here": the A (and AAAA) records of the site's names at
// Cloudflare, shown as a preview (now → after) before anything changes.
const props = defineProps({
  site: { type: Object, required: true },
})
const { t } = useI18n()

const open = ref(false)
const busy = ref(false)
const error = ref('')
const needIPv4 = ref(false)
const plan = ref(null)
const opts = reactive({ ipv4: '', ipv6: '', no_ipv6: false, proxied: false })
const planned = ref(null) // the options the plan was made with

function start() {
  Object.assign(opts, { ipv4: '', ipv6: '', no_ipv6: false, proxied: false })
  plan.value = null
  needIPv4.value = false
  error.value = ''
  open.value = true
  preview()
}

async function preview(keepError = false) {
  busy.value = true
  if (!keepError) error.value = ''
  try {
    const p = await api('POST', `api/sites/${props.site.id}/dns/plan`, { ...opts })
    plan.value = p
    opts.ipv4 = p.ipv4
    if (!opts.no_ipv6) opts.ipv6 = p.ipv6
    planned.value = { ipv4: p.ipv4, ipv6: opts.no_ipv6 ? '' : p.ipv6, no_ipv6: opts.no_ipv6, proxied: opts.proxied }
    needIPv4.value = false
  } catch (e) {
    plan.value = null
    if (e.code === 'dns_need_ipv4') needIPv4.value = true
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

// The inputs changed since the preview: preview again before applying.
const dirty = computed(() => {
  const p = planned.value
  if (!p) return true
  return opts.ipv4.trim() !== p.ipv4 || opts.no_ipv6 !== p.no_ipv6 || opts.proxied !== p.proxied || (!opts.no_ipv6 && opts.ipv6.trim() !== p.ipv6)
})
const changes = computed(() => plan.value?.changes || [])
const pending = computed(() => changes.value.filter((c) => c.action !== 'keep'))
const panelWarn = computed(() => opts.proxied && panelHost && (plan.value?.zones || []).some((z) => panelHost === z.name || panelHost.endsWith('.' + z.name)))
const inactive = computed(() => (plan.value?.zones || []).filter((z) => z.status !== 'active'))

function toggle(key) {
  opts[key] = !opts[key]
  preview()
}

const cell = (content, proxied, type) => (content ? content + (proxied && PROXIABLE.includes(type) ? ` · ${t('dns.proxied')}` : '') : '—')

async function apply() {
  busy.value = true
  error.value = ''
  try {
    const res = await api('POST', `api/sites/${props.site.id}/dns/apply`, { ...planned.value, hash: plan.value.hash })
    toast(t('dns.point_done', { n: res.changed }, res.changed))
    open.value = false
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
  <div class="point-dns">
    <h3 class="sub-title">{{ t('dns.point_title') }}</h3>
    <p class="muted small">{{ t('dns.point_hint') }}</p>
    <button type="button" class="btn" @click="start"><Icon name="cloud" :size="15" />{{ t('dns.point_button') }}</button>

    <Modal v-if="open" wide :title="t('dns.point_modal', { domain: site.domain })" @close="open = false">
      <div class="stack">
        <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>

        <div class="ip-grid">
          <label class="field">
            <span>{{ t('dns.point_ipv4') }}</span>
            <input v-model="opts.ipv4" class="input mono" spellcheck="false" autocomplete="off" placeholder="203.0.113.10" @change="preview()" @keydown.enter.prevent="preview()" />
            <small class="muted">{{ needIPv4 ? t('dns.point_ipv4_type') : t('dns.point_ipv4_hint') }}</small>
          </label>
          <div class="field">
            <label class="check"><input type="checkbox" :checked="!opts.no_ipv6" @change="toggle('no_ipv6')" />{{ t('dns.point_ipv6') }}</label>
            <input v-if="!opts.no_ipv6" v-model="opts.ipv6" class="input mono" spellcheck="false" autocomplete="off" placeholder="2001:db8::10" :aria-label="t('dns.point_ipv6')" @change="preview()" @keydown.enter.prevent="preview()" />
            <small v-if="!opts.no_ipv6 && plan && !plan.detected_ipv6" class="muted">{{ t('dns.point_ipv6_none') }}</small>
          </div>
        </div>
        <div class="opt">
          <div class="opt-text">
            <div class="opt-title">{{ t('dns.proxy') }}</div>
            <div class="opt-hint">{{ t('dns.point_proxy_hint') }}</div>
          </div>
          <button type="button" class="switch" role="switch" :aria-checked="opts.proxied" :aria-label="t('dns.proxy')" :disabled="busy" @click="toggle('proxied')"></button>
        </div>

        <p v-if="panelWarn" class="alert alert-warn">{{ t('dns.warn_panel', { host: panelHost }) }}</p>
        <p v-if="opts.proxied && plan && !plan.ssl_dns" class="alert alert-warn">{{ t('dns.warn_http01') }}</p>
        <p v-for="z in inactive" :key="z.id" class="alert alert-warn">{{ t('dns.zone_not_active', { zone: z.name, status: z.status }) }}</p>

        <span v-if="busy && !plan" class="skel" style="display: block; height: 120px; border-radius: 10px"></span>
        <template v-if="plan">
          <div class="table-wrap point-table">
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
                  <td class="mono">{{ c.name }}</td>
                  <td class="mono">{{ c.type }}</td>
                  <td class="mono small">{{ cell(c.old, c.old_proxied, c.type) }}</td>
                  <td class="mono small">{{ c.action === 'delete' ? t('dns.point_removed') : cell(c.new, c.proxied, c.type) }}</td>
                  <td class="small nowrap" :class="{ strong: c.action !== 'keep' }">{{ t('dns.act_' + c.action) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-if="plan.skipped.length" class="muted small" style="margin: 0">{{ t('dns.point_skipped', { names: plan.skipped.map((s) => s.name).join(', ') }) }}</p>
          <p v-if="!pending.length" class="muted" style="margin: 0">{{ t('dns.point_nothing') }}</p>
        </template>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="open = false">{{ t('common.cancel') }}</button>
          <button v-if="dirty && (plan || needIPv4)" type="button" class="btn" :disabled="busy" @click="preview()">{{ t('dns.point_preview') }}</button>
          <button type="button" class="btn btn-primary" :disabled="busy || dirty || !plan || !pending.length" @click="apply">
            {{ t('dns.point_apply', { n: pending.length }, pending.length) }}
          </button>
        </div>
      </div>
    </Modal>
  </div>
</template>

<style scoped>
.point-dns { margin-top: 20px; padding-top: 16px; border-top: 1px solid var(--border); }
.point-dns .sub-title { font-size: 14px; margin: 0 0 6px; }
.point-dns > p { margin: 0 0 12px; }
.ip-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 14px; }
.point-table { margin: 0; }
.point-table .table th:first-child, .point-table .table td:first-child { padding-left: 0; }
.row-keep td { color: var(--muted); }
.strong { font-weight: 700; }
</style>
