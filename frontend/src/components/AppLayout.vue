<script setup>
import { ref } from 'vue'
import { RouterLink, RouterView, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { ROLE_LABELS } from '../lib/format'
import AppIcon from './AppIcon.vue'

const auth = useAuthStore()
const router = useRouter()
const sidebarOpen = ref(false)

const nav = [
  { name: 'dashboard', label: 'Обзор', icon: 'overview' },
  { name: 'disciples', label: 'Ученики', icon: 'disciples' },
  { name: 'dictionaries', label: 'Справочники', icon: 'pin' },
  { name: 'reports', label: 'Отчёты', icon: 'reports' },
  { name: 'users', label: 'Пользователи', icon: 'users', guruOnly: true },
]

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
      <div class="flex h-16 items-center gap-2.5 border-b border-parchment-200 px-6">
        <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-saffron-500 text-white shadow-sm">
          <AppIcon name="lotus" :size="22" :stroke="1.8" />
        </span>
        <span class="font-display text-xl font-semibold tracking-wide text-ink-900">Манибандха</span>
      </div>
      <nav class="p-3">
        <template v-for="item in nav" :key="item.name">
          <RouterLink
            v-if="!item.guruOnly || auth.isGuru"
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
