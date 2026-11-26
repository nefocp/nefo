import { reactive, watchEffect, onBeforeUnmount } from 'vue'

// The top bar shows the page's title (from its menu entry) and, when the
// page gives one, a subtitle. Pages call usePageMeta(() => ({ sub, title })).
export const pageMeta = reactive({ title: '', sub: '' })

export function usePageMeta(fn) {
  const stop = watchEffect(() => {
    const m = fn() || {}
    pageMeta.title = m.title || ''
    pageMeta.sub = m.sub || ''
  })
  onBeforeUnmount(() => {
    stop()
    pageMeta.title = ''
    pageMeta.sub = ''
  })
}
