<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import RowActions from '../components/RowActions.vue'
import FilesFetch from '../components/FilesFetch.vue'
import FilesSearch from '../components/FilesSearch.vue'
import FilesTrash from '../components/FilesTrash.vue'
import FilesDelete from '../components/FilesDelete.vue'
import { useRoute, useRouter } from 'vue-router'
import { api, ApiError } from '../api.js'
import { errorMessage } from '../errors.js'
import { formatBytes } from '../format.js'
import JobLog from '../components/JobLog.vue'
import { useJob } from '../composables/useJob.js'
import { usePageMeta } from '../page.js'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()

const HOME = '/var/www'
const listing = ref(null)
const loading = ref(false)
const error = ref('')
const notice = ref('')
const selected = ref(new Set())
const pathInput = ref('')

const { job, running: jobRunning, watch: watchJob, clear: clearJob } = useJob(() => load())

const current = computed(() => (typeof route.query.path === 'string' && route.query.path.startsWith('/') ? route.query.path : HOME))

const crumbs = computed(() => {
  const parts = current.value.split('/').filter(Boolean)
  return [{ name: '/', path: '/' }, ...parts.map((p, i) => ({ name: p, path: '/' + parts.slice(0, i + 1).join('/') }))]
})

const join = (dir, name) => (dir === '/' ? `/${name}` : `${dir}/${name}`)
const parentOf = (p) => (p === '/' ? '/' : p.slice(0, p.lastIndexOf('/')) || '/')

function go(p) {
  router.push({ name: 'files', query: { path: p } })
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    listing.value = await api('GET', `api/files?path=${encodeURIComponent(current.value)}`)
    selected.value = new Set()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
    pathInput.value = current.value
  }
}

watch(current, () => {
  closeEditor(true)
  action.value = null
  load()
})

function open(e) {
  if (e.type === 'dir' || e.target_dir) go(join(current.value, e.name))
  else if (e.type === 'file') edit(e)
}

function toggle(name) {
  const s = new Set(selected.value)
  s.has(name) ? s.delete(name) : s.add(name)
  selected.value = s
}

const allSelected = computed(() => (listing.value?.entries || []).length > 0 && selected.value.size === listing.value.entries.length)
function toggleAll() {
  selected.value = allSelected.value ? new Set() : new Set((listing.value.entries || []).map((e) => e.name))
}

function clearMessages() {
  error.value = ''
  notice.value = ''
}

// ---- simple actions with one input ----
// action = { kind, value, entry? }
const action = ref(null)
const actionBusy = ref(false)

function startAction(kind, entry) {
  clearMessages()
  const defaults = {
    mkdir: '',
    newfile: '',
    rename: entry?.name ?? '',
    chmod: entry?.mode ?? '0644',
    compress: `${selected.value.size === 1 ? [...selected.value][0] : 'archive'}.zip`,
    extract: entry ? current.value : '',
  }
  action.value = { kind, entry, value: defaults[kind] }
}

async function runAction() {
  const a = action.value
  const dir = current.value
  actionBusy.value = true
  try {
    switch (a.kind) {
      case 'mkdir':
        await api('POST', 'api/files/mkdir', { path: join(dir, a.value.trim()) })
        break
      case 'newfile':
        await api('PUT', 'api/files/content', { path: join(dir, a.value.trim()), content: '', create: true })
        break
      case 'rename':
        await api('POST', 'api/files/rename', { from: join(dir, a.entry.name), to: join(dir, a.value.trim()) })
        break
      case 'chmod':
        await api('POST', 'api/files/chmod', { path: join(dir, a.entry.name), mode: a.value.trim() })
        break
      case 'compress':
        watchJob(await api('POST', 'api/files/compress', { paths: [...selected.value].map((n) => join(dir, n)), dest: join(dir, a.value.trim()) }))
        break
      case 'extract':
        watchJob(await api('POST', 'api/files/extract', { path: join(dir, a.entry.name), dest: a.value.trim() }))
        break
    }
    action.value = null
    if (a.kind !== 'compress' && a.kind !== 'extract') await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    actionBusy.value = false
  }
}

// ---- delete: to the trash, or permanently ----
const deleting = ref(null) // { paths, permanent }

function removeSelected(names) {
  clearMessages()
  const list = names ?? [...selected.value]
  if (list.length) deleting.value = { paths: list.map((n) => join(current.value, n)), permanent: false }
}

async function onDeleted({ n, permanent }) {
  deleting.value = null
  notice.value = t(permanent ? 'files.deleted_permanently' : 'files.moved_to_trash', { n }, n)
  await load()
}

// ---- download from a URL, search and trash (in modals) ----
const showFetch = ref(false)
const showTrash = ref(false)
const searchText = ref('')
const searching = ref(null) // { name }

function openFetch() {
  clearMessages()
  showFetch.value = true
}

function openSearch() {
  clearMessages()
  searching.value = { name: searchText.value.trim() }
}

// A search result opens its folder, or the folder holding it; a text file
// also opens in the editor.
async function openHit(hit) {
  searching.value = null
  if (hit.type === 'dir') return go(hit.path)
  const dir = parentOf(hit.path)
  if (dir !== current.value) {
    await router.push({ name: 'files', query: { path: dir } })
    await nextTick()
  }
  if (hit.type === 'file') await editPath(hit.path)
}

const download = (e) => `api/files/download?path=${encodeURIComponent(join(current.value, e.name))}`
const isArchive = (n) => /\.(zip|tar\.gz|tgz|tar)$/i.test(n)

// ---- upload with progress ----
const uploadInput = ref(null)
const upload = ref(null) // { percent, names }

function chooseFiles() {
  clearMessages()
  uploadInput.value.value = ''
  uploadInput.value.click()
}

function sendFiles(list, overwrite) {
  return new Promise((resolve, reject) => {
    const body = new FormData()
    for (const f of list) body.append('file', f, f.name)
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `api/files/upload?dir=${encodeURIComponent(current.value)}&overwrite=${overwrite ? 1 : 0}`)
    xhr.setRequestHeader('X-Requested-With', 'panel')
    xhr.upload.onprogress = (ev) => {
      if (ev.lengthComputable) upload.value.percent = Math.round((ev.loaded / ev.total) * 100)
    }
    xhr.onload = () => {
      let data = null
      try {
        data = JSON.parse(xhr.responseText)
      } catch {
        // not JSON
      }
      if (xhr.status >= 200 && xhr.status < 300) resolve(data)
      else reject(new ApiError(xhr.status, data?.error || 'unknown', data))
    }
    xhr.onerror = () => reject(new ApiError(0, 'network', null))
    xhr.send(body)
  })
}

async function onFilesChosen(ev) {
  const list = [...ev.target.files]
  if (!list.length) return
  const existing = new Set((listing.value?.entries || []).map((e) => e.name))
  const clash = list.filter((f) => existing.has(f.name)).map((f) => f.name)
  const overwrite = clash.length > 0 && window.confirm(t('files.overwrite_confirm', { names: clash.join(', ') }))
  if (clash.length && !overwrite) return
  upload.value = { percent: 0, names: list.map((f) => f.name).join(', ') }
  try {
    await sendFiles(list, overwrite)
    notice.value = t('files.uploaded', { n: list.length }, list.length)
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    upload.value = null
  }
}

// ---- editor ----
const editor = ref(null) // { path, content, original, saving }

const edit = (e) => editPath(join(current.value, e.name))

async function editPath(p) {
  clearMessages()
  try {
    const res = await api('GET', `api/files/content?path=${encodeURIComponent(p)}`)
    editor.value = { path: p, content: res.content, original: res.content, saving: false }
  } catch (err) {
    error.value = errorMessage(err)
  }
}

const dirty = computed(() => editor.value && editor.value.content !== editor.value.original)

async function save() {
  if (!editor.value || editor.value.saving) return
  editor.value.saving = true
  try {
    await api('PUT', 'api/files/content', { path: editor.value.path, content: editor.value.content })
    editor.value.original = editor.value.content
    notice.value = t('files.saved', { name: editor.value.path })
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    if (editor.value) editor.value.saving = false
  }
}

function closeEditor(force) {
  if (!force && dirty.value && !window.confirm(t('files.discard_changes'))) return
  editor.value = null
}

function onEditorKey(ev) {
  if ((ev.ctrlKey || ev.metaKey) && ev.key === 's') {
    ev.preventDefault()
    save()
  }
  // Tab inserts a tab instead of leaving the editor.
  if (ev.key === 'Tab' && !ev.shiftKey) {
    ev.preventDefault()
    const el = ev.target
    const { selectionStart: s, selectionEnd: e } = el
    editor.value.content = editor.value.content.slice(0, s) + '\t' + editor.value.content.slice(e)
    requestAnimationFrame(() => el.setSelectionRange(s + 1, s + 1))
  }
}

function beforeUnload(ev) {
  if (dirty.value) ev.preventDefault()
}

const formatDate = (v) => new Date(v).toLocaleString(locale.value, { dateStyle: 'short', timeStyle: 'short' })
const pathEdit = ref(false) // the typed path field is open

// kindOf picks the tile colour of an entry; iconOf its icon.
const CODE = /\.(php|js|ts|css|html?|json|ya?ml|conf|ini|env|sh|py|go|rb|md|xml|sql|txt|vue)$/i
function kindOf(e) {
  if (e.type === 'dir' || e.target_dir) return 'dir'
  if (e.type === 'link') return 'link'
  if (isArchive(e.name)) return 'archive'
  if (CODE.test(e.name)) return 'code'
  return ''
}
const iconOf = (e) => ({ dir: 'folder', link: 'link', archive: 'zip', code: 'code' })[kindOf(e)] || 'file'
usePageMeta(() => ({ sub: current.value }))

onMounted(() => {
  load()
  window.addEventListener('beforeunload', beforeUnload)
})
onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
</script>

<template>
  <div class="pg">
    <div class="page-head sr-only"><h1 class="page-title">{{ t('files.title') }}</h1></div>
    <Teleport defer to="#topbar-actions">
      <template v-if="!editor">
        <button class="btn files-top-sec" @click="startAction('mkdir')"><Icon name="folder" :size="15" />{{ t('files.new_folder') }}</button>
        <button class="btn btn-primary" :disabled="!!upload" @click="chooseFiles"><Icon name="upload" :size="15" />{{ t('files.upload') }}</button>
      </template>
    </Teleport>
    <input ref="uploadInput" type="file" multiple hidden @change="onFilesChosen" />

    <p v-if="error" class="alert alert-crit pre-wrap" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert alert-ok" role="status">{{ notice }}</p>
    <JobLog v-if="job && !showFetch" :job="job" @close="clearJob" />

    <!-- editor -->
    <section v-if="editor" class="panel">
      <div class="panel-head">
        <h2 class="panel-title editor-title">
          <Icon name="file" :size="17" /><span class="mono editor-path">{{ editor.path }}</span>
          <span v-if="dirty" class="status-pill warn">{{ t('pg.files_unsaved') }}</span>
        </h2>
        <div class="row">
          <button class="btn btn-sm" @click="closeEditor(false)">{{ t('common.close') }}</button>
          <button class="btn btn-primary btn-sm" :disabled="editor.saving || !dirty" @click="save"><Icon name="check" :size="15" />{{ t('files.save') }}</button>
        </div>
      </div>
      <div class="panel-body">
        <textarea v-model="editor.content" class="editor" spellcheck="false" autocapitalize="off" autocomplete="off" :aria-label="editor.path" @keydown="onEditorKey"></textarea>
        <p class="muted small" style="margin: 0">{{ t('files.editor_hint') }}</p>
      </div>
    </section>

    <section v-show="!editor" class="panel">
      <!-- location -->
      <div class="panel-tools files-loc">
        <nav class="fcrumbs" :aria-label="t('files.location')">
          <button type="button" class="fcrumb home" :aria-label="t('files.websites')" :title="HOME" @click="go(HOME)"><Icon name="home" :size="15" /></button>
          <template v-for="(c, i) in crumbs" :key="c.path">
            <Icon v-if="i > 0" name="chevronRight" :size="14" class="muted" />
            <button type="button" class="fcrumb mono" :aria-current="i === crumbs.length - 1 ? 'location' : undefined" @click="go(c.path)">{{ c.name }}</button>
          </template>
          <span v-if="listing?.site" class="status-pill ok site-pill"><Icon name="globe" :size="12" />{{ listing.site }}</span>
        </nav>
        <button type="button" class="icon-btn icon-btn-sm" :aria-label="t('pg.files_edit_path')" :title="t('pg.files_edit_path')" :aria-expanded="pathEdit" @click="pathEdit = !pathEdit"><Icon name="edit" :size="15" /></button>
      </div>
      <form v-if="pathEdit" class="panel-tools" @submit.prevent="go(pathInput.trim() || '/'); pathEdit = false">
        <input v-model="pathInput" class="input mono grow" spellcheck="false" :aria-label="t('files.location')" />
        <button class="btn btn-sm">{{ t('files.go') }}</button>
      </form>

      <!-- toolbar: icon buttons, words on wide screens -->
      <div class="panel-tools files-bar">
        <button class="btn btn-sm" :disabled="current === '/'" :aria-label="t('files.up')" @click="go(parentOf(current))"><Icon name="up" :size="15" /><span class="lbl">{{ t('files.up') }}</span></button>
        <button class="btn btn-sm" :aria-label="t('files.refresh')" @click="load"><Icon name="refresh" :size="15" /><span class="lbl">{{ t('files.refresh') }}</span></button>
        <span class="bar-sep" aria-hidden="true"></span>
        <button class="btn btn-sm" :aria-label="t('files.new_file')" @click="startAction('newfile')"><Icon name="file" :size="15" /><span class="lbl">{{ t('files.new_file') }}</span></button>
        <button class="btn btn-sm files-mkdir-sm" :aria-label="t('files.new_folder')" @click="startAction('mkdir')"><Icon name="folder" :size="15" /><span class="lbl">{{ t('files.new_folder') }}</span></button>
        <button class="btn btn-sm" :disabled="jobRunning" :aria-label="t('files.fetch')" @click="openFetch"><Icon name="download" :size="15" /><span class="lbl">{{ t('files.fetch') }}</span></button>
        <button class="btn btn-sm" :aria-label="t('files.trash')" @click="showTrash = true"><Icon name="trash" :size="15" /><span class="lbl">{{ t('files.trash') }}</span></button>
        <span class="bar-gap"></span>
        <form class="search-field files-search" role="search" @submit.prevent="openSearch">
          <Icon name="search" :size="15" />
          <input v-model="searchText" type="search" maxlength="200" :placeholder="t('files.search_placeholder')" :aria-label="t('files.search')" spellcheck="false" autocomplete="off" />
        </form>
      </div>

      <div v-if="selected.size" class="panel-tools sel-bar" role="status">
        <span class="count-badge">{{ selected.size }}</span>
        <span class="small">{{ t('files.selected', { n: selected.size }, selected.size) }}</span>
        <span class="bar-gap"></span>
        <button class="btn btn-sm" :disabled="jobRunning" @click="startAction('compress')"><Icon name="zip" :size="15" />{{ t('files.compress') }}</button>
        <button class="btn btn-sm btn-danger" @click="removeSelected()"><Icon name="trash" :size="15" />{{ t('files.delete') }}</button>
        <button class="icon-btn icon-btn-sm" :aria-label="t('pg.files_clear_sel')" @click="selected = new Set()"><Icon name="x" :size="15" /></button>
      </div>

      <div v-if="upload" class="panel-tools upload-bar" role="status">
        <span class="small">{{ t('files.uploading', { names: upload.names }) }} {{ upload.percent }}%</span>
        <div class="meter" style="width: 100%"><div class="meter-fill meter-ok" :style="{ width: upload.percent + '%' }"></div></div>
      </div>

      <!-- one-field actions -->
      <form v-if="action" class="panel-tools action-form" @submit.prevent="runAction">
        <label class="field action-field">
          <span>{{ t(`files.action_${action.kind}`, { name: action.entry?.name ?? '' }) }}</span>
          <input v-model="action.value" class="input mono" spellcheck="false" autocomplete="off" required />
        </label>
        <button type="button" class="btn btn-ghost btn-sm" @click="action = null">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary btn-sm" :disabled="actionBusy">{{ t('files.ok') }}</button>
      </form>

      <div v-if="loading && !listing" class="panel-body"><span class="skel" style="height: 220px"></span></div>
      <div v-else-if="listing && !(listing.entries || []).length" class="empty">
        <div class="empty-icon"><Icon name="folder" :size="26" /></div>
        <p class="empty-lead">{{ t('files.empty') }}</p>
        <button class="btn btn-primary" @click="chooseFiles"><Icon name="upload" />{{ t('files.upload') }}</button>
      </div>
      <div v-else-if="listing" class="table-wrap">
        <table class="table files-table">
          <thead>
            <tr>
              <th class="col-check"><input type="checkbox" :checked="allSelected" :aria-label="t('files.select_all')" @change="toggleAll" /></th>
              <th>{{ t('files.col_name') }}</th>
              <th>{{ t('files.col_size') }}</th>
              <th>{{ t('files.col_mode') }}</th>
              <th>{{ t('files.col_owner') }}</th>
              <th>{{ t('files.col_modified') }}</th>
              <th class="actions"><span class="sr-only">{{ t('databases.actions') }}</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="e in listing.entries" :key="e.name" :class="{ 'row-selected': selected.has(e.name) }">
              <td class="col-check"><input type="checkbox" :checked="selected.has(e.name)" :aria-label="e.name" @change="toggle(e.name)" /></td>
              <td class="name-cell">
                <span class="fname">
                  <span class="ftile" :class="kindOf(e)" aria-hidden="true"><Icon :name="iconOf(e)" :size="16" /></span>
                  <a v-if="e.type === 'dir' || e.target_dir || e.type === 'file'" href="#" @click.prevent="open(e)">{{ e.name }}</a>
                  <span v-else>{{ e.name }}</span>
                  <span v-if="e.target" class="muted small"> → {{ e.target }}</span>
                </span>
              </td>
              <td class="nowrap muted">{{ e.type === 'file' ? formatBytes(e.size) : '—' }}</td>
              <td class="mono small"><button type="button" class="link-btn mono" :aria-label="t('pg.files_chmod', { name: e.name })" @click="startAction('chmod', e)">{{ e.mode }}</button></td>
              <td class="small owner-cell">{{ e.owner === e.group ? e.owner : `${e.owner}:${e.group}` }}</td>
              <td class="nowrap small muted">{{ formatDate(e.modified) }}</td>
              <td class="actions">
                <RowActions :actions="[
                  e.type === 'file' ? { label: t('files.download'), icon: 'download', href: download(e) } : null,
                  e.type === 'file' && isArchive(e.name) ? { label: t('files.extract'), icon: 'zip', disabled: jobRunning, onClick: () => startAction('extract', e) } : null,
                  { label: t('files.rename'), icon: 'edit', onClick: () => startAction('rename', e) },
                  { label: t('files.col_mode'), icon: 'lock', onClick: () => startAction('chmod', e) },
                  { label: t('files.delete'), icon: 'trash', danger: true, onClick: () => removeSelected([e.name]) },
                ].filter(Boolean)" />
              </td>
            </tr>
          </tbody>
        </table>
        <p v-if="listing.truncated" class="muted small note" style="padding: 0 18px 14px">{{ t('files.truncated') }}</p>
      </div>
    </section>

    <Modal v-if="deleting" :danger="deleting.permanent" :title="t('files.delete_title', { n: deleting.paths.length }, deleting.paths.length)" @close="deleting = null">
      <FilesDelete v-model:permanent="deleting.permanent" :paths="deleting.paths" @done="onDeleted" @close="deleting = null" />
    </Modal>
    <Modal v-if="showFetch" :title="t('files.fetch_title')" @close="showFetch = false">
      <FilesFetch :dir="current" :job="job" @started="watchJob" @close="showFetch = false" />
    </Modal>
    <Modal v-if="searching" wide :title="t('files.search_title', { dir: current })" @close="searching = null">
      <FilesSearch :dir="current" :initial-name="searching.name" @open="openHit" @close="searching = null" />
    </Modal>
    <Modal v-if="showTrash" wide :title="t('files.trash_title')" @close="showTrash = false">
      <FilesTrash :path="current" @changed="load" @close="showTrash = false" />
    </Modal>
  </div>
</template>

<style scoped>
.files-loc { gap: 6px; }
.fcrumbs { display: flex; flex-wrap: wrap; align-items: center; gap: 2px; flex: 1; min-width: 0; }
.fcrumb {
  min-height: 28px; padding: 0 7px; border: 0; border-radius: 7px; background: none; color: var(--text-2);
  font-size: 13px; cursor: pointer; display: inline-flex; align-items: center;
}
.fcrumb:hover { background: var(--hover); color: var(--text); }
.fcrumb[aria-current] { color: var(--text); font-weight: 600; }
.fcrumb.home { color: var(--accent); }
.site-pill { margin-left: 8px; }
.files-bar { gap: 6px; }
.bar-sep { width: 1px; height: 22px; background: var(--border); margin: 0 4px; }
.bar-gap { flex: 1; }
.files-search { flex: 0 1 240px; }
.sel-bar { background: var(--accent-soft); }
.upload-bar { flex-direction: column; align-items: stretch; gap: 6px; }
.action-form { align-items: flex-end; background: var(--surface-2); }
.action-field { flex: 1; min-width: 220px; }
.fname { display: inline-flex; align-items: center; gap: 10px; }
.ftile { width: 28px; height: 28px; border-radius: 8px; display: grid; place-items: center; flex: none; background: var(--surface-3); color: var(--text-2); }
.ftile.dir { background: rgb(217 154 11 / 0.14); color: #b7800a; }
.ftile.link { background: var(--info-soft); color: var(--info); }
.ftile.archive { background: rgb(116 84 224 / 0.12); color: #7454e0; }
.ftile.code { background: var(--accent-soft); color: var(--accent); }
.owner-cell { overflow-wrap: anywhere; max-width: 220px; }
.files-table td.name-cell a { color: var(--text); font-weight: 600; }
.editor-title { min-width: 0; flex: 1; }
.editor-path { font-size: 13px; font-weight: 500; overflow-wrap: anywhere; }
@media (max-width: 900px) {
  .files-bar .lbl { display: none; }
  .files-bar .btn { padding: 0; width: 36px; }
  .files-search { flex: 1 1 100%; order: 10; }
  .bar-gap { display: none; }
}
@media (max-width: 640px) {
  .files-top-sec { display: none; }
  .files-bar .btn { width: 44px; }
  .bar-sep { display: none; }
}
@media (min-width: 641px) {
  .files-mkdir-sm { display: none; }
}
</style>
