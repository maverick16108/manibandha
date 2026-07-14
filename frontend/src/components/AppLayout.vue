<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { RouterLink, RouterView, useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { ROLE_LABELS } from '../lib/format'
import { pageTitle } from '../composables/pageTitle'
import { onEscape } from '../composables/useEscape'
import { navCounts, refreshNavCounts } from '../composables/navCounts'
import { backTarget } from '../composables/backTarget'
import AppIcon from './AppIcon.vue'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const sidebarOpen = ref(false)
const profileMenu = ref(false)
onEscape(() => { profileMenu.value = false; sidebarOpen.value = false })
// незаапрувленный кандидат — без левого меню, только экран ожидания
const showSidebar = computed(() => !auth.isPending)

// caps — любое из перечисленных прав открывает раздел
const nav = [
  { name: 'dashboard', label: 'Обзор', icon: 'overview', caps: ['dashboard.view'] },
  { name: 'calendar', label: 'Календарь', icon: 'calendar', caps: ['calendar.view'] },
  { name: 'disciples', label: 'Ученики', icon: 'disciples', caps: ['disciples.view_all', 'disciples.view_own'] },
  { name: 'approvals', label: 'Заявки', icon: 'shield', caps: ['disciples.approve'] },
  { name: 'questions', label: 'Вопросы', icon: 'chat', caps: ['questions.ask', 'questions.answer', 'questions.view_all'] },
  { name: 'service-reports', label: 'Отчёты', icon: 'reports', caps: ['reports.write', 'reports.read_all'] },
  { name: 'forum', label: 'Форум', icon: 'forum', caps: ['forum.view'] },
  { name: 'dictionaries', label: 'Справочники', icon: 'pin', caps: ['dictionaries.manage'] },
  { name: 'users', label: 'Пользователи', icon: 'users', caps: ['users.manage'] },
  { name: 'roles', label: 'Роли', icon: 'shield', caps: ['roles.manage'] },
]

// top-level sections (nav) — everything else is a sub-page, so show a back button
const topLevel = new Set(nav.map((n) => n.name))
const showBack = computed(() => route.name && !topLevel.has(route.name))
function goBack() {
  if (backTarget.value) { router.push(backTarget.value); return }
  if (window.history.length > 1) router.back()
  else router.push({ name: 'dashboard' })
}

function canShow(item) {
  return (item.caps || []).some((c) => auth.can(c))
}

// бейджи непросмотренного в меню
function badgeFor(name) {
  if (name === 'questions') return navCounts.questions
  if (name === 'service-reports') return navCounts.reports
  if (name === 'approvals') return navCounts.approvals
  return 0
}
let countsTimer = null
onMounted(() => { refreshNavCounts(); countsTimer = setInterval(refreshNavCounts, 15000) })
onBeforeUnmount(() => clearInterval(countsTimer))
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
      class="fixed inset-y-0 left-0 z-30 flex w-64 transform flex-col border-r border-parchment-200 bg-white transition-transform lg:translate-x-0"
      :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <div class="flex h-16 shrink-0 items-center gap-2.5 border-b border-parchment-200 px-5">
        <img src="/lotus-mark.png" alt="" class="h-9 w-auto" />
        <span class="font-display text-2xl font-semibold leading-none text-ink-900">Манибандха</span>
      </div>

      <nav class="flex-1 overflow-y-auto p-3">
        <template v-for="item in nav" :key="item.name">
          <RouterLink
            v-if="canShow(item)"
            :to="{ name: item.name }"
            class="mb-1 flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-ink-700 hover:bg-parchment-100"
            active-class="bg-saffron-500/10 text-saffron-700"
            @click="sidebarOpen = false"
          >
            <AppIcon :name="item.icon" :size="18" />
            <span class="flex-1">{{ item.label }}</span>
            <span v-if="badgeFor(item.name) > 0"
                  class="inline-flex h-5 min-w-[1.25rem] items-center justify-center rounded-full bg-saffron-500 px-1.5 text-xs font-semibold text-white">
              {{ badgeFor(item.name) }}
            </span>
          </RouterLink>
        </template>
      </nav>

      <!-- Profile (bottom) -->
      <div class="relative shrink-0 border-t border-parchment-200 p-3">
        <button
          class="flex w-full items-center gap-3 rounded-lg px-2 py-2 text-left hover:bg-parchment-100"
          @click="profileMenu = !profileMenu"
        >
          <img v-if="auth.user?.avatar_url" :src="auth.user.avatar_url" class="photo-bw h-9 w-9 shrink-0 rounded-full object-cover" />
          <span v-else class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-sm font-semibold text-white">
            {{ initials }}
          </span>
          <span class="min-w-0 flex-1">
            <span class="block truncate text-sm font-medium text-ink-800">{{ auth.user?.full_name }}</span>
            <span class="block text-xs text-ink-700/60">{{ ROLE_LABELS[auth.user?.role] || auth.user?.role }}</span>
          </span>
          <AppIcon name="chevron" :size="16" class="shrink-0 text-ink-700/50 transition" :class="profileMenu && 'rotate-180'" />
        </button>

        <div v-if="profileMenu" class="absolute inset-x-3 bottom-full mb-1 overflow-hidden rounded-lg border border-parchment-200 bg-white shadow-lg">
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
    <div :class="showSidebar && 'lg:pl-64'">
      <header class="sticky top-0 z-10 flex h-16 items-center gap-3 border-b border-parchment-200 bg-parchment-50/90 px-4 backdrop-blur sm:px-6">
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
        <RouterView />
      </main>
    </div>
  </div>
</template>
