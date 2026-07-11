<script setup>
import { ref, computed } from 'vue'
import { RouterLink, RouterView, useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { ROLE_LABELS } from '../lib/format'
import AppIcon from './AppIcon.vue'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const sidebarOpen = ref(false)

const nav = [
  { name: 'dashboard', label: 'Обзор', icon: 'overview' },
  { name: 'calendar', label: 'Календарь', icon: 'calendar' },
  { name: 'disciples', label: 'Ученики', icon: 'disciples' },
  { name: 'questions', label: 'Вопросы', icon: 'chat' },
  { name: 'service-reports', label: 'Отчёты', icon: 'reports' },
  { name: 'dictionaries', label: 'Справочники', icon: 'pin' },
  { name: 'users', label: 'Пользователи', icon: 'users' },
  { name: 'roles', label: 'Роли', icon: 'shield', guruOnly: true },
]

// top-level sections (nav) — everything else is a sub-page, so show a back button
const topLevel = new Set(nav.map((n) => n.name))
const showBack = computed(() => route.name && !topLevel.has(route.name))
function goBack() {
  if (window.history.length > 1) router.back()
  else router.push({ name: 'dashboard' })
}

function canShow(item) {
  if (item.guruOnly) return auth.isGuru
  return auth.canSee(item.name)
}

function logout() {
  auth.logout()
  router.push({ name: 'home' })
}
</script>

<template>
  <div class="min-h-screen bg-parchment-100">
    <!-- Sidebar -->
    <aside
      class="fixed inset-y-0 left-0 z-30 w-64 transform border-r border-parchment-200 bg-white transition-transform lg:translate-x-0"
      :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <div class="flex h-16 items-center gap-2.5 border-b border-parchment-200 px-5">
        <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-saffron-400 to-saffron-600 text-white shadow">
          <AppIcon name="lotus-solid" :size="32" />
        </span>
        <span class="font-script text-3xl font-bold leading-none text-ink-900">Манибандха</span>
      </div>
      <nav class="p-3">
        <template v-for="item in nav" :key="item.name">
          <RouterLink
            v-if="canShow(item)"
            :to="{ name: item.name }"
            class="mb-1 flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-ink-700 hover:bg-parchment-100"
            active-class="bg-saffron-500/10 text-saffron-700"
            @click="sidebarOpen = false"
          >
            <AppIcon :name="item.icon" :size="18" />{{ item.label }}
          </RouterLink>
        </template>
      </nav>
    </aside>

    <!-- Overlay (mobile) -->
    <div v-if="sidebarOpen" class="fixed inset-0 z-20 bg-ink-900/40 lg:hidden" @click="sidebarOpen = false"></div>

    <!-- Main -->
    <div class="lg:pl-64">
      <header class="sticky top-0 z-10 flex h-16 items-center justify-between border-b border-parchment-200 bg-parchment-50/90 px-4 backdrop-blur sm:px-6">
        <button class="-ml-1 rounded-lg p-2 text-ink-800 hover:bg-parchment-200 lg:hidden" @click="sidebarOpen = true">
          <AppIcon name="menu" :size="28" :stroke="2" />
        </button>
        <button v-if="showBack" class="btn-outline" @click="goBack">
          <AppIcon name="chevron" :size="16" class="rotate-90" /> Назад
        </button>
        <div class="flex-1"></div>
        <div class="flex items-center gap-3">
          <div class="text-right">
            <div class="text-sm font-medium text-ink-800">{{ auth.user?.full_name }}</div>
            <div class="text-xs text-ink-700/60">{{ ROLE_LABELS[auth.user?.role] || auth.user?.role }}</div>
          </div>
          <button class="btn-outline" @click="logout">Выйти</button>
        </div>
      </header>

      <main class="p-4 sm:p-6 lg:p-8">
        <RouterView />
      </main>
    </div>
  </div>
</template>
