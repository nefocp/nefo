import { createRouter, createWebHashHistory } from 'vue-router'
import { auth, loadUser } from './auth.js'
import LoginView from './views/LoginView.vue'
import AppLayout from './views/AppLayout.vue'
import DashboardView from './views/DashboardView.vue'

export const router = createRouter({
  // Hash history: the server only ever serves one page, at /<entrance>/.
  history: createWebHashHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    {
      path: '/',
      component: AppLayout,
      children: [
        { path: '', name: 'dashboard', component: DashboardView },
        { path: 'websites', name: 'websites', component: () => import('./views/WebsitesView.vue') },
        { path: 'apps', name: 'apps', component: () => import('./views/AppsView.vue') },
        { path: 'wordpress', name: 'wordpress', component: () => import('./views/WordPressView.vue') },
        { path: 'databases', name: 'databases', component: () => import('./views/DatabasesView.vue') },
        { path: 'dns', name: 'dns', component: () => import('./views/DnsView.vue') },
        { path: 'files', name: 'files', component: () => import('./views/FilesView.vue') },
        { path: 'backups', name: 'backups', component: () => import('./views/BackupsView.vue') },
        { path: 'cron', name: 'cron', component: () => import('./views/CronView.vue') },
        { path: 'services', name: 'services', component: () => import('./views/ServicesView.vue') },
        { path: 'processes', name: 'processes', component: () => import('./views/ProcessesView.vue') },
        { path: 'daemons', name: 'daemons', component: () => import('./views/DaemonsView.vue') },
        // Redis is a tab of the Databases page.
        { path: 'redis', name: 'redis', redirect: { name: 'databases', query: { tab: 'redis' } } },
        { path: 'firewall', name: 'firewall', component: () => import('./views/FirewallView.vue') },
        { path: 'settings', name: 'settings', component: () => import('./views/SettingsView.vue') },
        { path: 'software', name: 'software', component: () => import('./views/SoftwareView.vue') },
        { path: 'system', name: 'system', component: () => import('./views/SystemView.vue') },
        { path: 'docker', name: 'docker', component: () => import('./views/DockerView.vue') },
        // Loaded on demand: the terminal library is large.
        { path: 'terminal', name: 'terminal', component: () => import('./views/TerminalView.vue') },
        { path: 'security', name: 'security', component: () => import('./views/SecurityView.vue') },
        { path: ':pathMatch(.*)*', name: 'notfound', component: () => import('./views/NotFoundView.vue') },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  if (!auth.checked) {
    try {
      await loadUser()
    } catch {
      // server unreachable: treat as signed out, the login page shows the error
    }
  }
  if (to.meta.public) {
    return auth.user && to.name === 'login' ? { name: 'dashboard' } : true
  }
  if (!auth.user) {
    return { name: 'login', query: to.fullPath !== '/' ? { next: to.fullPath } : {} }
  }
  return true
})
