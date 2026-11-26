<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { store, load, list } from '../data.js'
import { toggleTheme } from '../theme.js'
import Icon from './Icon.vue'
import AppLogo from './AppLogo.vue'
import { logoSrc } from './apps/appstore.js'

// CommandPalette (Ctrl K) jumps to a page, a website or a database, or
// opens an app's install. It searches in the browser, in lists the panel
// already has.
const props = defineProps({ pages: { type: Array, required: true } }) // [{ name, label, icon }]
const emit = defineEmits(['close'])
const { t } = useI18n()
const router = useRouter()

const dlg = ref(null)
const input = ref(null)
const q = ref('')
const active = ref(0)
const listEl = ref(null)

onMounted(async () => {
  dlg.value?.showModal()
  await nextTick()
  input.value?.focus()
  load('api/sites').catch(() => {})
  load('api/databases', { ttl: 60000 }).catch(() => {})
  load('api/apps', { ttl: 300000 }).catch(() => {})
})
onBeforeUnmount(() => dlg.value?.open && dlg.value.close())

// Lower case without accents: "sao luu" finds "Sao lưu".
const fold = (s) => String(s || '').normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/đ/g, 'd').replace(/Đ/g, 'd').toLowerCase()

const all = computed(() => {
  const items = []
  for (const p of props.pages) items.push({ id: 'p:' + p.name, group: 'pages', label: p.label, icon: p.icon, to: { name: p.name } })
  items.push({ id: 'a:new', group: 'actions', label: t('palette.new_site'), icon: 'plus', to: { name: 'websites', query: { new: '1' } } })
  items.push({ id: 'a:theme', group: 'actions', label: t('palette.toggle_theme'), icon: 'moon', run: toggleTheme })
  for (const s of list(store['api/sites']?.sites)) {
    items.push({ id: 's:' + s.id, group: 'sites', label: s.domain, sub: list(s.aliases).join(', '), logo: s.app || (s.php_version ? 'php' : ''), icon: 'globe', to: { name: 'websites', query: { site: String(s.id) } }, extra: list(s.aliases).join(' ') })
  }
  for (const d of list(store['api/databases']?.databases)) {
    items.push({ id: 'd:' + d.id, group: 'databases', label: d.name, sub: d.site_domain || d.username, icon: 'database', to: { name: 'databases' }, extra: d.username })
  }
  for (const a of list(store['api/apps']?.apps)) {
    items.push({ id: 'i:' + a.id, group: 'apps', label: t('palette.install_app', { app: a.name }), src: logoSrc(a.logo), icon: 'download', to: { name: 'apps', query: { install: a.id } }, extra: a.name + ' ' + a.category })
  }
  return items
})

const results = computed(() => {
  const f = fold(q.value.trim())
  const pool = f ? all.value.filter((i) => fold(i.label + ' ' + (i.extra || '') + ' ' + (i.sub || '')).includes(f)) : all.value.filter((i) => i.group !== 'databases' && i.group !== 'apps')
  // Exact starts first, then the rest; at most 40.
  if (f) pool.sort((a, b) => Number(!fold(a.label).startsWith(f)) - Number(!fold(b.label).startsWith(f)))
  return pool.slice(0, 40)
})
const groups = computed(() => {
  const order = ['pages', 'sites', 'databases', 'apps', 'actions']
  return order.map((g) => ({ g, items: results.value.filter((i) => i.group === g) })).filter((x) => x.items.length)
})
const flat = computed(() => groups.value.flatMap((x) => x.items))
watch(q, () => (active.value = 0))

function choose(item) {
  if (!item) return
  emit('close')
  if (item.run) item.run()
  else router.push(item.to)
}

function onKey(e) {
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    active.value = Math.min(active.value + 1, flat.value.length - 1)
    scrollActive()
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    active.value = Math.max(active.value - 1, 0)
    scrollActive()
  } else if (e.key === 'Enter') {
    e.preventDefault()
    choose(flat.value[active.value])
  }
}
async function scrollActive() {
  await nextTick()
  listEl.value?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' })
}
const indexOf = (item) => flat.value.indexOf(item)
</script>

<template>
  <dialog ref="dlg" class="palette" :aria-label="t('palette.title')" @cancel.prevent="emit('close')" @mousedown.self="emit('close')">
    <div class="pal-search">
      <Icon name="search" :size="18" />
      <input
        ref="input"
        v-model="q"
        type="text"
        role="combobox"
        aria-expanded="true"
        aria-controls="pal-list"
        :aria-activedescendant="flat[active] ? 'pal-' + flat[active].id : undefined"
        :placeholder="t('palette.placeholder')"
        :aria-label="t('palette.placeholder')"
        autocomplete="off"
        spellcheck="false"
        @keydown="onKey"
      />
      <kbd>Esc</kbd>
    </div>
    <div id="pal-list" ref="listEl" class="pal-list" role="listbox" :aria-label="t('palette.results')">
      <p v-if="!flat.length" class="pal-empty muted">{{ t('palette.none') }}</p>
      <template v-for="grp in groups" :key="grp.g">
        <div class="pal-group" role="presentation">{{ t('palette.group_' + grp.g) }}</div>
        <div
          v-for="item in grp.items"
          :id="'pal-' + item.id"
          :key="item.id"
          class="pal-item"
          role="option"
          :aria-selected="indexOf(item) === active"
          @mousemove="active = indexOf(item)"
          @click="choose(item)"
        >
          <AppLogo v-if="item.group === 'sites'" :name="item.logo" size="xs" icon="globe" />
          <AppLogo v-else-if="item.group === 'apps'" :src="item.src" size="xs" />
          <span v-else class="pal-ico"><Icon :name="item.icon" :size="16" /></span>
          <span class="pal-label truncate">{{ item.label }}</span>
          <span v-if="item.sub" class="pal-sub truncate">{{ item.sub }}</span>
          <Icon v-if="indexOf(item) === active" name="return" :size="15" class="pal-enter" />
        </div>
      </template>
    </div>
    <div class="pal-foot muted">
      <span><kbd>↑</kbd> <kbd>↓</kbd> {{ t('palette.move') }}</span>
      <span><kbd>Enter</kbd> {{ t('palette.open') }}</span>
    </div>
  </dialog>
</template>

<style scoped>
.palette {
  width: min(620px, calc(100vw - 24px)); margin: 12vh auto auto; padding: 0; border: 1px solid var(--border); border-radius: 16px;
  background: var(--surface); color: var(--text); box-shadow: var(--shadow-lg); overflow: hidden;
}
.pal-search { display: flex; align-items: center; gap: 10px; padding: 0 16px; border-bottom: 1px solid var(--border); color: var(--muted); }
.pal-search input { flex: 1; min-width: 0; height: 54px; border: 0; outline: 0; background: transparent; color: var(--text); font: 500 15px var(--font); }
.pal-list { max-height: min(420px, 60vh); overflow-y: auto; padding: 6px; }
.pal-group { padding: 10px 10px 4px; font-size: 11px; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: var(--muted); }
.pal-item { display: flex; align-items: center; gap: 10px; min-height: 40px; padding: 6px 10px; border-radius: 9px; cursor: pointer; }
.pal-item[aria-selected='true'] { background: var(--accent-soft); color: var(--accent); }
.pal-ico { width: 22px; height: 22px; display: grid; place-items: center; color: var(--muted); flex: none; }
.pal-item[aria-selected='true'] .pal-ico { color: var(--accent); }
.pal-label { font-weight: 600; }
.pal-sub { color: var(--muted); font-size: 12.5px; flex: 1; }
.pal-enter { margin-left: auto; }
.pal-empty { padding: 18px 12px; margin: 0; text-align: center; }
.pal-foot { display: flex; gap: 16px; padding: 9px 16px; border-top: 1px solid var(--border); font-size: 12px; background: var(--surface-2); }
@media (max-width: 640px) {
  .palette { margin-top: 8px; }
  .pal-item { min-height: 44px; }
  .pal-foot { display: none; }
}
</style>
