<script setup>
import { ref, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import client from '../api/client'
import AppSkeleton from '../components/AppSkeleton.vue'

const summary = ref(null)
const loading = ref(true)

onMounted(async () => {
  try {
    const { data } = await client.get('/reports/summary')
    summary.value = data
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div>
    <h1 class="mb-1 font-display text-3xl font-semibold text-ink-900">Обзор</h1>
    <p class="mb-8 text-ink-700/60">Сводка по ученикам</p>

    <div v-if="loading" class="space-y-8">
      <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div v-for="i in 4" :key="i" class="card space-y-3 p-5"><AppSkeleton w="w-28" h="h-3" /><AppSkeleton w="w-16" h="h-9" /></div>
      </div>
      <div class="grid gap-6 lg:grid-cols-3">
        <div v-for="i in 3" :key="i" class="card space-y-3 p-6">
          <AppSkeleton w="w-32" h="h-5" />
          <AppSkeleton v-for="j in 4" :key="j" h="h-4" />
        </div>
      </div>
    </div>

    <div v-else-if="summary" class="space-y-8">
      <!-- Top stats -->
      <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div class="card p-5">
          <div class="text-sm text-ink-700/60">Всего учеников</div>
          <div class="mt-1 font-display text-4xl font-semibold text-ink-900">{{ summary.total }}</div>
        </div>
        <div class="card p-5">
          <div class="text-sm text-ink-700/60">Готовы к инициации</div>
          <div class="mt-1 font-display text-4xl font-semibold text-saffron-600">{{ summary.ready_for_initiation }}</div>
        </div>
        <div class="card p-5">
          <div class="text-sm text-ink-700/60">Стран</div>
          <div class="mt-1 font-display text-4xl font-semibold text-ink-900">{{ summary.by_country.length }}</div>
        </div>
        <div class="card p-5">
          <div class="text-sm text-ink-700/60">Общин / храмов</div>
          <div class="mt-1 font-display text-4xl font-semibold text-ink-900">{{ summary.by_temple.length }}</div>
        </div>
      </div>

      <div class="grid gap-6 lg:grid-cols-3">
        <div class="card p-6">
          <h3 class="mb-4 font-display text-xl text-ink-900">По статусу инициации</h3>
          <ul class="space-y-2">
            <li v-for="row in summary.by_status" :key="row.key" class="flex items-center justify-between">
              <span class="text-ink-700">{{ row.key }}</span>
              <span class="badge bg-parchment-200 text-ink-800">{{ row.count }}</span>
            </li>
            <li v-if="!summary.by_status.length" class="text-sm text-ink-700/50">Нет данных</li>
          </ul>
        </div>
        <div class="card p-6">
          <h3 class="mb-4 font-display text-xl text-ink-900">По странам</h3>
          <ul class="space-y-2">
            <li v-for="row in summary.by_country" :key="row.key" class="flex items-center justify-between">
              <span class="text-ink-700">{{ row.key }}</span>
              <span class="badge bg-parchment-200 text-ink-800">{{ row.count }}</span>
            </li>
            <li v-if="!summary.by_country.length" class="text-sm text-ink-700/50">Нет данных</li>
          </ul>
        </div>
        <div class="card p-6">
          <h3 class="mb-4 font-display text-xl text-ink-900">По храмам</h3>
          <ul class="space-y-2">
            <li v-for="row in summary.by_temple" :key="row.key" class="flex items-center justify-between">
              <span class="text-ink-700">{{ row.key }}</span>
              <span class="badge bg-parchment-200 text-ink-800">{{ row.count }}</span>
            </li>
            <li v-if="!summary.by_temple.length" class="text-sm text-ink-700/50">Нет данных</li>
          </ul>
        </div>
      </div>

      <RouterLink :to="{ name: 'disciples' }" class="btn-primary">Перейти к ученикам →</RouterLink>
    </div>
  </div>
</template>
