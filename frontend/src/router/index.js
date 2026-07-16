import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/', name: 'home', component: () => import('../views/HomeView.vue'), meta: { public: true } },
  { path: '/login', name: 'login', component: () => import('../views/LoginView.vue'), meta: { public: true } },
  { path: '/calendar', name: 'public-calendar', component: () => import('../views/PublicCalendarView.vue'), meta: { public: true } },
  { path: '/events/:id', name: 'public-event', component: () => import('../views/PublicEventView.vue'), meta: { public: true } },
  { path: '/join/:room', name: 'conference-guest', component: () => import('../views/ConferenceRoomView.vue'), meta: { public: true } },
  { path: '/c/:code', name: 'conference-link', component: () => import('../views/ConferenceLinkView.vue'), meta: { public: true } },
  {
    path: '/app',
    component: () => import('../components/AppLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      { path: '', redirect: '/app/dashboard' },
      { path: 'dashboard', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
      { path: 'calendar', name: 'calendar', component: () => import('../views/CalendarView.vue') },
      { path: 'calendar/new', name: 'event-new', component: () => import('../views/EventFormView.vue') },
      { path: 'calendar/:id/edit', name: 'event-edit', component: () => import('../views/EventFormView.vue') },
      { path: 'disciples', name: 'disciples', component: () => import('../views/DisciplesView.vue') },
      { path: 'disciples/new', name: 'disciple-new', component: () => import('../views/DiscipleFormView.vue') },
      { path: 'disciples/:id', name: 'disciple', component: () => import('../views/DiscipleDetailView.vue') },
      { path: 'disciples/:id/edit', name: 'disciple-edit', component: () => import('../views/DiscipleFormView.vue') },
      { path: 'questions', name: 'questions', component: () => import('../views/ThreadsView.vue'), meta: { kind: 'question' } },
      { path: 'questions/new', name: 'question-new', component: () => import('../views/ThreadComposeView.vue'), meta: { kind: 'question' } },
      { path: 'service-reports', name: 'service-reports', component: () => import('../views/ThreadsView.vue'), meta: { kind: 'report' } },
      { path: 'service-reports/new', name: 'report-new', component: () => import('../views/ThreadComposeView.vue'), meta: { kind: 'report' } },
      { path: 'threads/:id', name: 'thread', component: () => import('../views/ThreadView.vue') },
      { path: 'chat', name: 'chat-home', component: () => import('../views/ChatView.vue') },
      { path: 'chat/:id', name: 'chat', component: () => import('../views/ChatView.vue') },
      { path: 'forum', name: 'forum', component: () => import('../views/ForumView.vue') },
      { path: 'forum/new', name: 'forum-new', component: () => import('../views/ForumNewView.vue') },
      { path: 'forum/:id', name: 'forum-topic', component: () => import('../views/ForumTopicView.vue') },
      { path: 'conference', name: 'conference', component: () => import('../views/ConferenceView.vue') },
      { path: 'conference/recordings', name: 'conference-recordings', component: () => import('../views/RecordingsArchiveView.vue') },
      { path: 'conference/:id', name: 'conference-room', component: () => import('../views/ConferenceRoomView.vue') },
      { path: 'dictionaries', name: 'dictionaries', component: () => import('../views/DictionariesView.vue') },
      { path: 'reports', name: 'reports', component: () => import('../views/ReportsView.vue') },
      { path: 'users', name: 'users', component: () => import('../views/UsersView.vue'), meta: { staffOnly: true } },
      { path: 'roles', name: 'roles', component: () => import('../views/RolesView.vue'), meta: { guruOnly: true } },
      { path: 'settings', name: 'settings', component: () => import('../views/SettingsView.vue') },
      { path: 'profile', name: 'profile', component: () => import('../views/ProfileView.vue') },
      { path: 'approvals', name: 'approvals', component: () => import('../views/ApprovalsView.vue') },
      { path: 'waiting', name: 'waiting', component: () => import('../views/WaitingView.vue') },
    ],
  },
]

// Требуемые права-действия для маршрутов (любое из списка открывает доступ)
const ROUTE_CAPS = {
  dashboard: ['dashboard.view'],
  calendar: ['calendar.view'],
  'event-new': ['calendar.manage'], 'event-edit': ['calendar.manage'],
  disciples: ['disciples.view_all', 'disciples.view_own'],
  'disciple-new': ['disciples.create'], 'disciple-edit': ['disciples.edit'],
  questions: ['questions.ask', 'questions.answer', 'questions.view_all'],
  'question-new': ['questions.ask'],
  'service-reports': ['reports.write', 'reports.read_all'],
  'report-new': ['reports.write'],
  forum: ['forum.view'], 'forum-new': ['forum.post'], 'forum-topic': ['forum.view'],
  conference: ['conference.view'], 'conference-room': ['conference.view'], 'conference-recordings': ['conference.view'],
  dictionaries: ['dictionaries.manage'],
  users: ['users.manage'],
  roles: ['roles.manage'],
  settings: ['settings.manage'],
  approvals: ['disciples.approve'],
}
const LANDING_ORDER = ['dashboard', 'calendar', 'disciples', 'questions', 'service-reports', 'dictionaries', 'users']

// запоминаем прокрутку главной, чтобы вернуться на то же место после календаря/событий/карты
let homeScroll = 0

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior(to, from, savedPosition) {
    if (to.hash) return { el: to.hash, behavior: 'smooth' }
    const pos = savedPosition || (to.name === 'home' && homeScroll ? { top: homeScroll } : null)
    if (!pos) return { top: 0 }
    // дождаться, пока страница станет достаточно высокой (ленивый чанк + раскладка), затем прокрутить — без рывка
    return new Promise((resolve) => {
      let tries = 0
      const target = pos.top || 0
      const tick = () => {
        const maxScroll = document.documentElement.scrollHeight - window.innerHeight
        if (maxScroll >= target - 2 || tries > 40) resolve(pos)
        else { tries += 1; requestAnimationFrame(tick) }
      }
      requestAnimationFrame(tick)
    })
  },
})

// After a deploy, chunk filenames change; a stale tab may fail to import a route
// chunk. Reload once so it fetches the fresh assets instead of silently failing.
router.onError((err) => {
  if (/dynamically imported module|Importing a module script failed|Failed to fetch/i.test(err?.message || '')) {
    window.location.reload()
  }
})

router.beforeEach(async (to, from) => {
  // запомнить позицию главной перед уходом (для возврата на то же место)
  if (from.name === 'home') homeScroll = window.scrollY || document.documentElement.scrollTop || 0
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
  // Незаапрувленный (самостоятельно зарегистрированный) — только экран ожидания, своя анкета и чат
  if (auth.isAuthenticated && auth.isPending) {
    if (to.name === 'login') return { name: 'waiting' }
    if (to.meta.requiresAuth) {
      const allowedForPending = ['waiting', 'profile', 'disciple-edit', 'thread']
      return allowedForPending.includes(to.name) ? true : { name: 'waiting' }
    }
  }

  // Гейтинг по правам-действиям
  const has = (caps) => (caps || []).some((c) => auth.can(c))
  const landing = () => LANDING_ORDER.find((n) => has(ROUTE_CAPS[n])) || 'profile'
  const need = ROUTE_CAPS[to.name]
  if (need && !has(need)) {
    return { name: landing() }
  }
  if (to.name === 'login' && auth.isAuthenticated) {
    return { name: landing() }
  }
})

export default router
