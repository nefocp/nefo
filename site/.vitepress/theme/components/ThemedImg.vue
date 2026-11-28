<script setup lang="ts">
// A panel screenshot in its light and dark version, each at two widths.
// Before the page is interactive the browser picks by the system theme (as
// the site itself does on a first visit); after that it follows the site's
// theme switch, or `mode` when the caller sets one.
import { computed, onMounted, ref } from 'vue'
import { useData } from 'vitepress'

const props = defineProps<{
  src: string // '/screens/en/dashboard': -light.webp, -dark.webp and their -sm halves
  w: number
  h: number
  alt: string
  sizes: string
  lazy?: boolean
  high?: boolean
  mode?: 'light' | 'dark' | null
}>()

const { isDark } = useData()
const mounted = ref(false)
onMounted(() => (mounted.value = true))
const dark = computed(() => (props.mode ? props.mode === 'dark' : isDark.value))
const media = computed(() => (mounted.value ? (dark.value ? 'all' : 'not all') : '(prefers-color-scheme: dark)'))
const set = (t: string) => `${props.src}-${t}-sm.webp ${Math.round(props.w / 2)}w, ${props.src}-${t}.webp ${props.w}w`
</script>

<template>
  <picture>
    <source type="image/webp" :media="media" :srcset="set('dark')" :sizes="sizes" />
    <img
      :src="`${src}-light.webp`"
      :srcset="set('light')"
      :sizes="sizes"
      :width="w"
      :height="h"
      :alt="alt"
      :loading="lazy ? 'lazy' : undefined"
      :fetchpriority="high ? 'high' : undefined"
      decoding="async"
    />
  </picture>
</template>
