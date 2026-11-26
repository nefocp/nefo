<script setup>
// Line icons drawn for this project (24×24 grid, round caps). Each entry
// lists SVG elements as [tag, attributes]. Unknown names draw "file".
const props = defineProps({
  name: { type: String, required: true },
  size: { type: [Number, String], default: 18 },
  stroke: { type: [Number, String], default: 2 },
})

const ICONS = {
  dashboard: [['rect', { x: 3.5, y: 3.5, width: 7, height: 9, rx: 1.8 }], ['rect', { x: 13.5, y: 3.5, width: 7, height: 5, rx: 1.8 }], ['rect', { x: 13.5, y: 11.5, width: 7, height: 9, rx: 1.8 }], ['rect', { x: 3.5, y: 15.5, width: 7, height: 5, rx: 1.8 }]],
  globe: [['circle', { cx: 12, cy: 12, r: 8.5 }], ['path', { d: 'M3.5 12h17M12 3.5c2.5 2.6 3.6 5.5 3.6 8.5s-1.1 5.9-3.6 8.5M12 3.5C9.5 6.1 8.4 9 8.4 12s1.1 5.9 3.6 8.5' }]],
  database: [['ellipse', { cx: 12, cy: 5.5, rx: 7.5, ry: 2.8 }], ['path', { d: 'M4.5 5.5v13c0 1.5 3.4 2.8 7.5 2.8s7.5-1.3 7.5-2.8v-13M4.5 12c0 1.5 3.4 2.8 7.5 2.8s7.5-1.3 7.5-2.8' }]],
  folder: [['path', { d: 'M3.5 7A1.5 1.5 0 0 1 5 5.5h4l2 2h8A1.5 1.5 0 0 1 20.5 9v8.5A1.5 1.5 0 0 1 19 19H5a1.5 1.5 0 0 1-1.5-1.5z' }]],
  file: [['path', { d: 'M14 3.5H7A1.5 1.5 0 0 0 5.5 5v14A1.5 1.5 0 0 0 7 20.5h10a1.5 1.5 0 0 0 1.5-1.5V8z' }], ['path', { d: 'M14 3.5V8h4.5' }]],
  archive: [['rect', { x: 3.5, y: 4, width: 17, height: 5, rx: 1.2 }], ['path', { d: 'M5.5 9v9A1.5 1.5 0 0 0 7 19.5h10a1.5 1.5 0 0 0 1.5-1.5V9M10 13h4' }]],
  clock: [['circle', { cx: 12, cy: 12, r: 8.5 }], ['path', { d: 'M12 7.5V12l3 2' }]],
  server: [['rect', { x: 3.5, y: 4, width: 17, height: 7, rx: 1.6 }], ['rect', { x: 3.5, y: 13, width: 17, height: 7, rx: 1.6 }], ['path', { d: 'M7 7.5h.01M7 16.5h.01' }]],
  package: [['path', { d: 'M3.5 7.5L12 3.5l8.5 4-8.5 4-8.5-4z' }], ['path', { d: 'M3.5 7.5v9l8.5 4 8.5-4v-9M12 11.5v9' }]],
  layers: [['path', { d: 'M12 3.5l8.5 4.5-8.5 4.5L3.5 8z' }], ['path', { d: 'M3.5 12.5l8.5 4.5 8.5-4.5M3.5 16.5l8.5 4.5 8.5-4.5' }]],
  box: [['rect', { x: 3, y: 10, width: 18, height: 9, rx: 1.8 }], ['path', { d: 'M6.5 10V6.5h4V10M10.5 10V6.5h4V10' }]],
  shield: [['path', { d: 'M12 3.5l7.5 3v5.5c0 4.3-3.2 7.8-7.5 8.5-4.3-.7-7.5-4.2-7.5-8.5V6.5z' }]],
  shieldCheck: [['path', { d: 'M12 3.5l7.5 3v5.5c0 4.3-3.2 7.8-7.5 8.5-4.3-.7-7.5-4.2-7.5-8.5V6.5z' }], ['path', { d: 'M9 12l2 2 4-4' }]],
  shieldAlert: [['path', { d: 'M12 3.5l7.5 3v5.5c0 4.3-3.2 7.8-7.5 8.5-4.3-.7-7.5-4.2-7.5-8.5V6.5z' }], ['path', { d: 'M12 9v4M12 16h.01' }]],
  lock: [['rect', { x: 5, y: 11, width: 14, height: 9, rx: 2 }], ['path', { d: 'M8 11V8a4 4 0 0 1 8 0v3' }]],
  lockOpen: [['rect', { x: 5, y: 11, width: 14, height: 9, rx: 2 }], ['path', { d: 'M8 11V8a4 4 0 0 1 7.5-1.9' }]],
  settings: [['path', { d: 'M4 7h9M17 7h3M4 12h3M11 12h9M4 17h11M19 17h1' }], ['circle', { cx: 15, cy: 7, r: 2 }], ['circle', { cx: 9, cy: 12, r: 2 }], ['circle', { cx: 17, cy: 17, r: 2 }]],
  sliders: [['path', { d: 'M4 7h9M17 7h3M4 12h3M11 12h9M4 17h11M19 17h1' }], ['circle', { cx: 15, cy: 7, r: 2 }], ['circle', { cx: 9, cy: 12, r: 2 }], ['circle', { cx: 17, cy: 17, r: 2 }]],
  gear: [['circle', { cx: 12, cy: 12, r: 3 }], ['path', { d: 'M12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3M5.3 5.3l2.1 2.1M16.6 16.6l2.1 2.1M5.3 18.7l2.1-2.1M16.6 7.4l2.1-2.1' }]],
  logout: [['path', { d: 'M15 4h3a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-3M10 17l-5-5 5-5M5 12h11' }]],
  plus: [['path', { d: 'M12 5v14M5 12h14' }]],
  more: [['circle', { cx: 5, cy: 12, r: 1.2 }], ['circle', { cx: 12, cy: 12, r: 1.2 }], ['circle', { cx: 19, cy: 12, r: 1.2 }]],
  external: [['path', { d: 'M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5' }]],
  refresh: [['path', { d: 'M20 12a8 8 0 1 1-2.3-5.7M20 4v4.5h-4.5' }]],
  play: [['path', { d: 'M7.5 5l11 7-11 7z' }]],
  stop: [['rect', { x: 6, y: 6, width: 12, height: 12, rx: 1.5 }]],
  pause: [['rect', { x: 6.5, y: 5, width: 3.5, height: 14, rx: 1 }], ['rect', { x: 14, y: 5, width: 3.5, height: 14, rx: 1 }]],
  power: [['path', { d: 'M12 3.5v8M7 6.2a7.5 7.5 0 1 0 10 0' }]],
  trash: [['path', { d: 'M5 7h14M10 7V5h4v2M7 7l1 12.5h8L17 7M10.5 11v5M13.5 11v5' }]],
  download: [['path', { d: 'M12 4v11M7.5 10.5L12 15l4.5-4.5M5 19.5h14' }]],
  upload: [['path', { d: 'M12 19V8M7.5 12.5L12 8l4.5 4.5M5 4.5h14' }]],
  logs: [['path', { d: 'M8.5 6.5h12M8.5 12h12M8.5 17.5h12M4 6.5h.01M4 12h.01M4 17.5h.01' }]],
  key: [['circle', { cx: 8, cy: 15, r: 4 }], ['path', { d: 'M11 12l9-9M17 6l3 3M15 8l2 2' }]],
  copy: [['rect', { x: 8, y: 8, width: 12, height: 12, rx: 2 }], ['path', { d: 'M16 8V5.5A1.5 1.5 0 0 0 14.5 4h-9A1.5 1.5 0 0 0 4 5.5v9A1.5 1.5 0 0 0 5.5 16H8' }]],
  x: [['path', { d: 'M6 6l12 12M18 6L6 18' }]],
  menu: [['path', { d: 'M4 7h16M4 12h16M4 17h16' }]],
  check: [['path', { d: 'M5 12.5l4.5 4.5L19 7.5' }]],
  alert: [['path', { d: 'M12 4l9 16H3z' }], ['path', { d: 'M12 10v4M12 17h.01' }]],
  info: [['circle', { cx: 12, cy: 12, r: 8.5 }], ['path', { d: 'M12 11v5M12 8h.01' }]],
  checkCircle: [['circle', { cx: 12, cy: 12, r: 8.5 }], ['path', { d: 'M8.5 12.2l2.4 2.4 4.6-4.8' }]],
  xCircle: [['circle', { cx: 12, cy: 12, r: 8.5 }], ['path', { d: 'M9.5 9.5l5 5M14.5 9.5l-5 5' }]],
  cpu: [['rect', { x: 6, y: 6, width: 12, height: 12, rx: 2 }], ['path', { d: 'M9.5 2.5v3.5M14.5 2.5v3.5M9.5 18v3.5M14.5 18v3.5M2.5 9.5H6M2.5 14.5H6M18 9.5h3.5M18 14.5h3.5' }]],
  memory: [['rect', { x: 3, y: 7, width: 18, height: 10, rx: 1.5 }], ['path', { d: 'M7 11v2M11 11v2M15 11v2M7 17v3M17 17v3' }]],
  disk: [['rect', { x: 3.5, y: 5, width: 17, height: 14, rx: 2 }], ['path', { d: 'M3.5 13h17M7 16h.01M11 16h2' }]],
  activity: [['path', { d: 'M3.5 12h4l2.5-6 4 12 2.5-6h4' }]],
  bars: [['path', { d: 'M5 20V11M12 20V5M19 20v-6' }]],
  git: [['circle', { cx: 6, cy: 5, r: 2 }], ['circle', { cx: 6, cy: 19, r: 2 }], ['circle', { cx: 18, cy: 7, r: 2 }], ['path', { d: 'M6 7v10M18 9c0 5-8 3-11 8.5' }]],
  wordpress: [['circle', { cx: 12, cy: 12, r: 8.5 }], ['path', { d: 'M5.5 8.5l4 11M9.5 7l4.5 12.5M14.5 7.5l3 9 2-6' }]],
  code: [['path', { d: 'M8 8l-4 4 4 4M16 8l4 4-4 4M13.5 5l-3 14' }]],
  up: [['path', { d: 'M12 19V5M6 11l6-6 6 6' }]],
  down: [['path', { d: 'M12 5v14M6 13l6 6 6-6' }]],
  chevronDown: [['path', { d: 'M7 10l5 5 5-5' }]],
  chevronRight: [['path', { d: 'M10 7l5 5-5 5' }]],
  chevronLeft: [['path', { d: 'M14 7l-5 5 5 5' }]],
  home: [['path', { d: 'M4 11l8-7 8 7v8.5A1.5 1.5 0 0 1 18.5 21h-13A1.5 1.5 0 0 1 4 19.5z' }], ['path', { d: 'M10 21v-6h4v6' }]],
  zip: [['path', { d: 'M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z' }], ['path', { d: 'M11 3v2M11 7v2M11 11v2M10 15h2v3h-2z' }]],
  link: [['path', { d: 'M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1' }]],
  terminal: [['rect', { x: 3.5, y: 4.5, width: 17, height: 15, rx: 2.2 }], ['path', { d: 'M7.5 9.5l3 2.5-3 2.5M13 15h3.5' }]],
  bell: [['path', { d: 'M6 16v-5a6 6 0 1 1 12 0v5l1.5 2h-15z' }], ['path', { d: 'M10 20.5a2 2 0 0 0 4 0' }]],
  search: [['circle', { cx: 11, cy: 11, r: 6.5 }], ['path', { d: 'M20 20l-4-4' }]],
  sun: [['circle', { cx: 12, cy: 12, r: 4 }], ['path', { d: 'M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M5.3 18.7l1.4-1.4M17.3 6.7l1.4-1.4' }]],
  moon: [['path', { d: 'M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5z' }]],
  monitor: [['rect', { x: 3, y: 4.5, width: 18, height: 12, rx: 1.8 }], ['path', { d: 'M8.5 20h7M12 16.5V20' }]],
  user: [['circle', { cx: 12, cy: 8.5, r: 3.8 }], ['path', { d: 'M4.5 20c1.2-3.6 4-5.5 7.5-5.5s6.3 1.9 7.5 5.5' }]],
  image: [['rect', { x: 3.5, y: 5, width: 17, height: 14, rx: 2 }], ['circle', { cx: 9, cy: 10, r: 1.6 }], ['path', { d: 'M20.5 16l-5-5-8 8' }]],
  ban: [['circle', { cx: 12, cy: 12, r: 8.5 }], ['path', { d: 'M6 18L18 6' }]],
  arrowRight: [['path', { d: 'M5 12h12M13 7l5 5-5 5' }]],
  swap: [['path', { d: 'M4 8h12l-3-3M20 16H8l3 3' }]],
  calendar: [['rect', { x: 4, y: 5, width: 16, height: 15, rx: 2 }], ['path', { d: 'M4 10h16M8.5 3v4M15.5 3v4' }]],
  fileX: [['path', { d: 'M14 3.5H7A1.5 1.5 0 0 0 5.5 5v14A1.5 1.5 0 0 0 7 20.5h10a1.5 1.5 0 0 0 1.5-1.5V8z' }], ['path', { d: 'M14 3.5V8h4.5M10 12.5l4 4M14 12.5l-4 4' }]],
  bug: [['rect', { x: 7.5, y: 8, width: 9, height: 12, rx: 4.5 }], ['path', { d: 'M9.5 8V6.5a2.5 2.5 0 0 1 5 0V8M3.5 13h4M16.5 13h4M5 7.5l3 2M19 7.5l-3 2M5 18.5l3-2M19 18.5l-3-2M12 11v9' }]],
  rocket: [['path', { d: 'M14 4.5c3-1 5.5-1 5.5-1s0 2.5-1 5.5L13 14.5 9.5 11z' }], ['path', { d: 'M9.5 11L6 11.5 4 14l4 .5M13 14.5l-.5 3.5-2.5 2-.5-4M6.5 17.5c-1 1-1.5 2.5-1.5 2.5s1.5-.5 2.5-1.5' }]],
  cloud: [['path', { d: 'M7 18.5a4 4 0 0 1-.5-8A5.5 5.5 0 0 1 17 9a4.8 4.8 0 0 1 .5 9.5z' }]],
  zap: [['path', { d: 'M13 3L5 13.5h6L10 21l8-10.5h-6z' }]],
  filter: [['path', { d: 'M4 5.5h16l-6.2 7.3v5.2l-3.6 1.8v-7z' }]],
  eye: [['path', { d: 'M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12z' }], ['circle', { cx: 12, cy: 12, r: 3 }]],
  eyeOff: [['path', { d: 'M3.5 3.5l17 17M10.6 5.6A9.6 9.6 0 0 1 12 5.5c6 0 9.5 6.5 9.5 6.5a17 17 0 0 1-2.6 3.4M6.4 6.9C3.9 8.6 2.5 12 2.5 12S6 18.5 12 18.5a9 9 0 0 0 4.2-1.1M9.9 9.9a3 3 0 0 0 4.2 4.2' }]],
  edit: [['path', { d: 'M4 20h4L19 9a2.8 2.8 0 0 0-4-4L4 16z' }], ['path', { d: 'M13.5 6.5l4 4' }]],
  grid: [['rect', { x: 4, y: 4, width: 6.5, height: 6.5, rx: 1.5 }], ['rect', { x: 13.5, y: 4, width: 6.5, height: 6.5, rx: 1.5 }], ['rect', { x: 4, y: 13.5, width: 6.5, height: 6.5, rx: 1.5 }], ['rect', { x: 13.5, y: 13.5, width: 6.5, height: 6.5, rx: 1.5 }]],
  return: [['path', { d: 'M9 10l-4 4 4 4' }], ['path', { d: 'M5 14h10a4 4 0 0 0 4-4V6' }]],
}
</script>

<template>
  <svg
    class="icon"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    :stroke-width="stroke"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <component :is="el[0]" v-for="(el, i) in ICONS[props.name] || ICONS.file" :key="i" v-bind="el[1]" />
  </svg>
</template>
