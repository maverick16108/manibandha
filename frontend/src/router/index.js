import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/', name: 'home', component: () => import('../views/HomeView.vue'), meta: { public: true } },
  { path: '/login', name: 'login', component: () => import('../views/LoginView.vue'), meta: { public: true } },
  {
    path: '/app',
    component: () => import('../components/AppLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      { path: '', redirect: '/app/dashboard' },
      { path: 'dashboard', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
      { path: 'disciples', name: 'disciples', component: () => import('../views/DisciplesView.vue') },
      { path: 'disciples/new', name: 'disciple-new', component: () => import('../views/DiscipleFormView.vue') },
      { path: 'disciples/:id', name: 'disciple', component: () => import('../views/DiscipleDetailView.vue') },
      { path: 'disciples/:id/edit', name: 'disciple-edit', component: () => import('../views/DiscipleFormView.vue') },
      { path: 'questions', name: 'questions', component: () => import('../views/ThreadsView.vue'), meta: { kind: 'question' } },
      { path: 'questions/new', name: 'question-new', component: () => import('../views/ThreadComposeView.vue'), meta: { kind: 'question' } },
      { path: 'service-reports', name: 'service-reports', component: () => import('../views/ThreadsView.vue'), meta: { kind: 'report' } },
      { path: 'service-reports/new', name: 'report-new', component: () => import('../views/ThreadComposeView.vue'), meta: { kind: 'report' } },
      { path: 'threads/:id', name: 'thread', component: () => import('../views/ThreadView.vue') },
      { path: 'dictionaries', name: 'dictionaries', component: () => import('../views/DictionariesView.vue') },
      { path: 'reports', name: 'reports', component: () => import('../views/ReportsView.vue') },
      { path: 'users', name: 'users', component: () => import('../views/UsersView.vue'), meta: { staffOnly: true } },
    ],
  },
]

const router = createRouter({ history: createWebHistory(), routes })

// After a deploy, chunk filenames change; a stale tab may fail to import a route
// chunk. Reload once so it fetches the fresh assets instead of silently failing.
router.onError((err) => {
  if (/dynamically imported module|Importing a module script failed|Failed to fetch/i.test(err?.message || '')) {
    window.location.reload()
  }
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (auth.token && !auth.user) {
    try {
      await auth.fetchMe()
    } catch {
      auth.logout()
    }
  }
  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.guruOnly && !auth.isGuru) {
    return { name: 'dashboard' }
  }
  if (to.meta.staffOnly && !auth.isStaff) {
    return { name: 'dashboard' }
  }
  if (to.name === 'login' && auth.isAuthenticated) {
    return { name: 'dashboard' }
  }
})

export default router
