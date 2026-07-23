<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { RouterLink, RouterView, useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { ROLE_LABELS } from '../lib/format'
import { pageTitle } from '../composables/pageTitle'
import { onEscape } from '../composables/useEscape'
import { navCounts, refreshNavCounts } from '../composables/navCounts'
import { chatState, initChat, teardownChat } from '../chat/store'
import { toastState } from '../composables/toast'
import { backTarget } from '../composables/backTarget'
import CallCenter from './CallCenter.vue'
import AppIcon from './AppIcon.vue'
import { prefetchSections } from '../composables/prefetch'

// Разделы, кешируемые keep-alive (по имени компонента). Чат и детальные страницы — не кешируем.
const KEEP_ALIVE_VIEWS = [
  'DashboardView', 'CalendarView', 'DisciplesView', 'ThreadsView', 'ForumView', 'ConferenceView',
  'RecordingsArchiveView', 'DictionariesView', 'ReportsView', 'UsersView', 'RolesView', 'SettingsView',
  'ProfileView', 'ApprovalsView',
]
function viewKey(r) {
  if (r.name === 'chat' || r.name === 'chat-home') return 'chat' // чат — один инстанс, параметр обрабатывает сам
  if (r.meta && r.meta.keepAlive) return r.name                  // раздел — один инстанс на раздел (вопросы/отчёты раздельно)
  return r.fullPath                                              // детальные/param — свежий инстанс на каждый параметр
}

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const sidebarOpen = ref(false)
const profileMenu = ref(false)
// свёрнутое боковое меню на десктопе (запоминаем выбор)
const collapsed = ref(localStorage.getItem('sidebarCollapsed') === '1')
watch(collapsed, (v) => localStorage.setItem('sidebarCollapsed', v ? '1' : '0'))
onEscape(() => { profileMenu.value = false; sidebarOpen.value = false })

// плавное изменение ширины бокового меню перетаскиванием (десктоп, развёрнутое)
const NAV_MIN = 200
const NAV_MAX = 440
const navWidth = ref(Number(localStorage.getItem('navWidth')) || 256)
const isDesktop = ref(typeof window !== 'undefined' && window.innerWidth >= 1024)
const navResizing = ref(false)
function onWinResize() { isDesktop.value = window.innerWidth >= 1024 }
function startNavResize(e) {
  const startX = e.clientX
  const startW = navWidth.value
  navResizing.value = true
  const move = (ev) => { navWidth.value = Math.max(NAV_MIN, Math.min(NAV_MAX, startW + (ev.clientX - startX))) }
  const up = () => {
    navResizing.value = false
    document.removeEventListener('mousemove', move); document.removeEventListener('mouseup', up)
    document.body.style.userSelect = ''
    try { localStorage.setItem('navWidth', String(navWidth.value)) } catch { /* ignore */ }
  }
  document.addEventListener('mousemove', move); document.addEventListener('mouseup', up)
  document.body.style.userSelect = 'none'
  e.preventDefault()
}
// незаапрувленный кандидат — без левого меню, только экран ожидания
const showSidebar = computed(() => !auth.isPending)

// caps — любое из перечисленных прав открывает раздел
const nav = [
  { name: 'dashboard', label: 'Обзор', icon: 'overview', caps: ['dashboard.view'] },
  { name: 'calendar', label: 'События', icon: 'calendar', caps: ['calendar.view'] },
  { name: 'disciples', label: 'Ученики', icon: 'disciples', caps: ['disciples.view_all', 'disciples.view_own'] },
  { name: 'approvals', label: 'Заявки', icon: 'shield', caps: ['disciples.approve'] },
  { name: 'chat-home', label: 'Чат', icon: 'chat', always: true },
  { name: 'questions', label: 'Вопросы', icon: 'question', caps: ['questions.ask', 'questions.answer', 'questions.view_all'] },
  { name: 'service-reports', label: 'Отчёты', icon: 'reports', caps: ['reports.write', 'reports.read_all'] },
  { name: 'forum', label: 'Форум', icon: 'forum', caps: ['forum.view'] },
  { name: 'conference', label: 'Конференция', icon: 'video', caps: ['conference.view'] },
  { name: 'dictionaries', label: 'Справочники', icon: 'pin', caps: ['dictionaries.manage'] },
  { name: 'users', label: 'Пользователи', icon: 'users', caps: ['users.manage'] },
  { name: 'roles', label: 'Роли', icon: 'key', caps: ['roles.manage'] },
  { name: 'settings', label: 'Настройки', icon: 'settings', caps: ['settings.manage'] },
]

// top-level sections (nav) — everything else is a sub-page, so show a back button
const topLevel = new Set(nav.map((n) => n.name))
const showBack = computed(() => route.name && !topLevel.has(route.name) && route.name !== 'chat')
function goBack() {
  if (backTarget.value) { router.push(backTarget.value); return }
  if (window.history.length > 1) router.back()
  else router.push({ name: 'dashboard' })
}

function canShow(item) {
  if (item.always) return !auth.isPending
  return (item.caps || []).some((c) => auth.can(c))
}
// подсветка «Чат» держится и при открытом чате (chat/:id) — это разные роуты
function navActive(item) {
  if (item.name === 'chat-home') return route.name === 'chat-home' || route.name === 'chat'
  return false
}
// на странице чата верхнюю панель кабинета убираем — чат сам во весь экран
const chatRoute = computed(() => route.name === 'chat-home' || route.name === 'chat')

// бейджи непросмотренного в меню
function badgeFor(name) {
  if (name === 'chat-home') return chatState.totalUnread
  if (name === 'questions') return navCounts.questions
  if (name === 'service-reports') return navCounts.reports
  if (name === 'approvals') return navCounts.approvals
  if (name === 'forum') return navCounts.forum
  if (name === 'conference') return navCounts.conference
  return 0
}
let countsTimer = null
onMounted(() => {
  refreshNavCounts(); countsTimer = setInterval(refreshNavCounts, 15000)
  window.addEventListener('resize', onWinResize)
  // мессенджер работает в фоне на всём кабинете (доставка + бейдж непрочитанного)
  if (!auth.isPending && auth.user) {
    initChat({ meId: auth.user.id, getToken: () => auth.token }).catch((e) => console.warn('[chat] init failed', e))
    prefetchSections((c) => auth.can(c)) // тихий idle-прогрев данных разделов в общий кеш
  }
})
onBeforeUnmount(() => { clearInterval(countsTimer); teardownChat(); window.removeEventListener('resize', onWinResize) })
// обновлять при переходах (в т.ч. после просмотра ветки — счётчик у всех уменьшается)
watch(() => route.fullPath, refreshNavCounts)

const initials = computed(() => (auth.user?.full_name || '?').trim()[0]?.toUpperCase() || '?')

function logout() {
  auth.logout()
  router.push({ name: 'home' })
}
</script>

<template>
  <div class="min-h-screen bg-parchment-100">
    <!-- Sidebar (скрыт для незаапрувленного кандидата) -->
    <aside
      v-if="showSidebar"
      class="fixed inset-y-0 left-0 z-30 flex w-64 transform select-none flex-col border-r border-parchment-200 bg-white transition-all lg:translate-x-0"
      :class="[sidebarOpen ? 'translate-x-0' : '-translate-x-full', collapsed && 'lg:w-16', navResizing && '!transition-none']"
      :style="isDesktop && !collapsed ? { width: navWidth + 'px' } : null"
    >
      <!-- перетаскивание правого края для изменения ширины (десктоп, развёрнутое) -->
      <div v-if="!collapsed" class="absolute inset-y-0 right-0 z-40 hidden w-1.5 cursor-col-resize hover:bg-saffron-300/50 lg:block" @mousedown="startNavResize"></div>
      <div class="flex h-16 shrink-0 items-center gap-2.5 overflow-hidden border-b border-parchment-200 px-5" :class="collapsed && 'lg:justify-center lg:px-0'">
        <RouterLink to="/" class="flex min-w-0 flex-1 items-center gap-2.5" title="На главную">
          <img src="/lotus-mark.png" alt="" class="h-9 w-auto shrink-0" :class="collapsed && 'lg:hidden'" />
          <span class="min-w-0 flex-1 truncate font-display text-2xl font-semibold leading-none text-ink-900" :class="collapsed && 'lg:hidden'">Манибандха</span>
        </RouterLink>
        <!-- свернуть (в развёрнутом виде) -->
        <button class="ml-auto hidden rounded-lg p-1.5 text-ink-700/60 hover:bg-parchment-100 lg:block" :class="collapsed && 'lg:hidden'" title="Свернуть меню" @click="collapsed = true">
          <AppIcon name="sidebar" :size="20" />
        </button>
        <!-- развернуть (в свёрнутом виде) -->
        <button class="hidden rounded-lg p-1.5 text-ink-700/60 hover:bg-parchment-100" :class="collapsed ? 'lg:block' : 'lg:hidden'" title="Развернуть меню" @click="collapsed = false">
          <AppIcon name="sidebar" :size="20" />
        </button>
      </div>

      <nav class="flex-1 overflow-y-auto p-3">
        <template v-for="item in nav" :key="item.name">
          <RouterLink
            v-if="canShow(item)"
            :to="{ name: item.name }"
            class="mb-1 flex items-center gap-3 rounded-lg px-3 py-2 text-[15px] font-medium text-ink-700 hover:bg-parchment-100"
            :class="[collapsed && 'lg:justify-center lg:px-2', navActive(item) && 'bg-saffron-500/10 text-saffron-700']"
            active-class="bg-saffron-500/10 text-saffron-700"
            :title="collapsed ? item.label : ''"
            @click="sidebarOpen = false"
          >
            <span class="relative shrink-0">
              <AppIcon :name="item.icon" :size="22" />
              <!-- цифра прямо на иконке, когда меню свёрнуто -->
              <span v-if="badgeFor(item.name) > 0"
                    class="absolute -right-2 -top-2 hidden h-4 min-w-[1rem] items-center justify-center rounded-full bg-saffron-500 px-1 text-[10px] font-semibold leading-none text-white"
                    :class="collapsed && 'lg:flex'">
                {{ badgeFor(item.name) }}
              </span>
            </span>
            <span class="flex-1" :class="collapsed && 'lg:hidden'">{{ item.label }}</span>
            <span v-if="badgeFor(item.name) > 0"
                  class="inline-flex h-5 min-w-[1.25rem] items-center justify-center rounded-full bg-saffron-500 px-1.5 text-xs font-semibold text-white"
                  :class="collapsed && 'lg:hidden'">
              {{ badgeFor(item.name) }}
            </span>
          </RouterLink>
        </template>
      </nav>

      <!-- Profile (bottom) -->
      <div class="relative shrink-0 border-t border-parchment-200 p-3">
        <button
          class="flex w-full items-center gap-3 rounded-lg px-2 py-2 text-left hover:bg-parchment-100"
          :class="collapsed && 'lg:justify-center'"
          :title="collapsed ? auth.user?.full_name : ''"
          @click="profileMenu = !profileMenu"
        >
          <img v-if="auth.user?.avatar_url" :src="auth.user.avatar_url" class="photo-bw h-9 w-9 shrink-0 rounded-full object-cover" />
          <span v-else class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-sm font-semibold text-white">
            {{ initials }}
          </span>
          <span class="min-w-0 flex-1" :class="collapsed && 'lg:hidden'">
            <span class="block truncate text-sm font-medium text-ink-800">{{ auth.user?.full_name }}</span>
            <span class="block text-xs text-ink-700/60">{{ ROLE_LABELS[auth.user?.role] || auth.user?.role }}</span>
          </span>
          <AppIcon name="chevron" :size="16" class="shrink-0 text-ink-700/50 transition" :class="[profileMenu && 'rotate-180', collapsed && 'lg:hidden']" />
        </button>

        <div v-if="profileMenu" class="absolute bottom-full mb-1 overflow-hidden rounded-lg border border-parchment-200 bg-white shadow-lg" :class="collapsed ? 'left-2 min-w-[12rem] lg:left-2 lg:right-auto' : 'inset-x-3'">
          <RouterLink :to="{ name: 'profile' }" class="flex w-full items-center gap-2 px-3 py-2.5 text-left text-sm text-ink-700 hover:bg-parchment-100" @click="profileMenu = false; sidebarOpen = false">
            <AppIcon name="disciples" :size="16" /> Профиль
          </RouterLink>
          <button class="flex w-full items-center gap-2 border-t border-parchment-200 px-3 py-2.5 text-left text-sm text-red-600 hover:bg-red-50" @click="logout">
            <AppIcon name="logout" :size="16" /> Выйти
          </button>
        </div>
      </div>
    </aside>

    <!-- Overlay (mobile) -->
    <div v-if="sidebarOpen" class="fixed inset-0 z-20 bg-ink-900/40 lg:hidden" @click="sidebarOpen = false"></div>
    <!-- close profile menu on outside click -->
    <div v-if="profileMenu" class="fixed inset-0 z-20" @click="profileMenu = false"></div>

    <!-- Main -->
    <div :class="[showSidebar && collapsed && 'lg:pl-16', navResizing && '!transition-none']"
         :style="isDesktop && showSidebar && !collapsed ? { paddingLeft: navWidth + 'px' } : null">
      <header v-if="!chatRoute" class="sticky top-0 z-10 flex h-16 items-center gap-3 border-b border-parchment-200 bg-parchment-50/90 px-4 backdrop-blur sm:px-6">
        <button v-if="showSidebar" class="-ml-1 shrink-0 rounded-lg p-2 text-ink-800 hover:bg-parchment-200 lg:hidden" @click="sidebarOpen = true">
          <AppIcon name="menu" :size="28" :stroke="2" />
        </button>
        <button v-if="showSidebar && showBack" class="btn-outline shrink-0" @click="goBack">
          <AppIcon name="chevron" :size="16" class="rotate-90" /> Назад
        </button>
        <RouterLink v-if="!showSidebar" to="/" class="btn-outline shrink-0">
          <AppIcon name="chevron" :size="16" class="rotate-90" /> На главную
        </RouterLink>
        <h1 class="truncate font-display text-lg font-semibold text-ink-900 sm:text-xl">{{ pageTitle }}</h1>
      </header>

      <main class="p-4 sm:p-6 lg:p-8">
        <!-- Разделы из KEEP_ALIVE_VIEWS кешируются по ИМЕНИ компонента (надёжно, без двойного
             монтажа). SPA-навигация не перерисовывает их: нет скелетонов, позиция сохраняется.
             Чат и детальные/param-страницы не кешируются; полная перезагрузка — всё с нуля. -->
        <RouterView v-slot="{ Component, route }">
          <keep-alive :include="KEEP_ALIVE_VIEWS" :max="20">
            <component :is="Component" :key="viewKey(route)" />
          </keep-alive>
        </RouterView>
      </main>
    </div>

    <!-- глобальный центр звонков (входящие приходят в любом разделе, не только в чате) -->
    <CallCenter />

    <!-- всплывающее уведомление (тост) -->
    <transition enter-active-class="transition duration-200" enter-from-class="translate-y-3 opacity-0" leave-active-class="transition duration-200" leave-to-class="translate-y-3 opacity-0">
      <div v-if="toastState.show" class="fixed bottom-6 left-1/2 z-50 -translate-x-1/2 rounded-full bg-ink-900 px-4 py-2.5 text-sm font-medium text-white shadow-lg">
        {{ toastState.msg }}
      </div>
    </transition>
  </div>
</template>
