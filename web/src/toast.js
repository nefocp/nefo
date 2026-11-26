import { reactive } from 'vue'

// Short confirmations ("Saved") shown at the bottom right, read out by
// screen readers. kind: "ok" or "crit".
export const toasts = reactive([])
let seq = 0

export function toast(text, kind = 'ok', ms = 3500) {
  const id = ++seq
  toasts.push({ id, text, kind })
  setTimeout(() => {
    const i = toasts.findIndex((t) => t.id === id)
    if (i >= 0) toasts.splice(i, 1)
  }, ms)
}
