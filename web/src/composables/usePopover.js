import { ref, watch, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'

// usePopover opens and closes a menu or popover: a click outside root or
// Escape closes it, and focus goes back to the button.
export function usePopover(root, button) {
  const open = ref(false)
  function onDoc(e) {
    if (root.value && !root.value.contains(e.target)) hide()
  }
  function onKey(e) {
    if (e.key === 'Escape') {
      hide()
      button?.value?.focus()
    }
  }
  function show() {
    open.value = true
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
  }
  function hide() {
    open.value = false
    document.removeEventListener('mousedown', onDoc)
    document.removeEventListener('keydown', onKey)
  }
  const toggle = () => (open.value ? hide() : show())
  // A link inside, or any other navigation, closes it.
  const route = useRoute()
  watch(() => route.fullPath, hide)
  onBeforeUnmount(hide)
  return { open, show, hide, toggle }
}
