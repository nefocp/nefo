import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// During development the Go backend runs on 127.0.0.1:8080 with entrance
// "dev" (see `nefo init --dev`). Vite proxies /api to it.
const backend = process.env.NEFO_DEV_BACKEND || 'http://127.0.0.1:8080'
const entrance = process.env.NEFO_DEV_ENTRANCE || 'dev'

export default defineConfig({
  // Relative asset URLs: the app is served under a secret path that is
  // different on every server.
  base: './',
  plugins: [vue()],
  define: {
    __VUE_I18N_FULL_INSTALL__: true,
    __VUE_I18N_LEGACY_API__: false,
    __INTLIFY_PROD_DEVTOOLS__: false,
  },
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
    // Fonts never become data: URLs (the CSP allows none for fonts), and
    // logos stay separate files, fetched only when a page shows them.
    assetsInlineLimit: (file) => (/\.(woff2?|ttf|otf|svg)$/.test(file) ? false : undefined),
  },
  server: {
    proxy: {
      '/api': {
        target: backend,
        ws: true,
        rewrite: (path) => `/${entrance}${path}`,
        // The backend scopes its cookie to /<entrance>/; the dev server is at /.
        cookiePathRewrite: { [`/${entrance}/`]: '/' },
      },
    },
  },
})
