<script setup>
import { RouterLink } from 'vue-router'
import AppIcon from './AppIcon.vue'
</script>

<template>
  <div class="min-h-screen bg-parchment-100 text-ink-800">
    <header class="sticky top-0 z-20 border-b border-parchment-200 bg-parchment-50/90 backdrop-blur">
      <div class="mx-auto flex max-w-5xl items-center justify-between gap-3 px-4 py-3 sm:px-6">
        <RouterLink to="/" class="inline-flex items-center gap-1.5 text-sm font-medium text-ink-700 hover:text-saffron-700">
          <AppIcon name="chevron" :size="18" class="rotate-90" /> На главную
        </RouterLink>
        <RouterLink to="/login" class="btn-outline">Кабинет учеников</RouterLink>
      </div>
    </header>
    <main class="mx-auto max-w-5xl px-4 py-8 sm:px-6 sm:py-12"><slot /></main>
  </div>
</template>
