<script setup>
import { ref, computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import client from '../api/client'
import AppSkeleton from '../components/AppSkeleton.vue'
import DonutChart from '../components/charts/DonutChart.vue'
import BarList from '../components/charts/BarList.vue'
import { STATUS_LABELS, STATUS_ORDER } from '../lib/format'

const summary = ref(null)
const cities = ref([])
const loading = ref(true)

// Sequential warm ramp for the ordered initiation stages (aspirant → brahman).
const STATUS_COLORS = { aspirant: '#e0a24e', recommended: '#cf7d2b', harinama: '#a85e1f', brahman: '#6d3f16' }

const statusData = computed(() => {
  if (!summary.value) return []
  const byLabel = Object.fromEntries(summary.value.by_status.map((r) => [r.key, r.count]))
  return STATUS_ORDER.map((s) => ({
    label: STATUS_LABELS[s],
    value: byLabel[STATUS_LABELS[s]] || 0,
    color: STATUS_COLORS[s],
  }))
})

onMounted(async () => {
  try {
    const [s, c] = await Promise.all([
      client.get('/reports/summary'),
      client.get('/reports/group', { params: { group_by: 'city' } }),
    ])
    summary.value = s.data
    cities.value = c.data.filter((x) => x.key !== '—').slice(0, 10).map((x) => ({ label: x.key, value: x.count }))
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div>
    <h1 class="mb-1 font-display text-3xl font-semibold text-ink-900">Обзор</h1>
    <p class="mb-8 text-ink-700/60">Сводка по ученикам</p>

    <!-- Loading skeleton -->
    <div v-if="loading" class="space-y-8">
      <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div v-for="i in 4" :key="i" class="card space-y-3 p-5"><AppSkeleton w="w-28" h="h-3" /><AppSkeleton w="w-16" h="h-9" /></div>
      </div>
      <div class="grid gap-6 lg:grid-cols-2">
        <div v-for="i in 2" :key="i" class="card space-y-3 p-6"><AppSkeleton w="w-40" h="h-5" /><AppSkeleton v-for="j in 5" :key="j" /></div>
      </div>
    </div>

    <div v-else-if="summary" class="space-y-6">
      <!-- Stat tiles -->
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
          <div class="text-sm text-ink-700/60">Городов</div>
          <div class="mt-1 font-display text-4xl font-semibold text-ink-900">{{ cities.length }}</div>
        </div>
        <div class="card p-5">
          <div class="text-sm text-ink-700/60">Общин / храмов</div>
          <div class="mt-1 font-display text-4xl font-semibold text-ink-900">{{ summary.by_temple.length }}</div>
        </div>
      </div>

      <div class="grid gap-6 lg:grid-cols-2">
        <!-- Pipeline funnel -->
        <div class="card p-6">
          <h3 class="mb-1 font-display text-xl text-ink-900">Ступени инициации</h3>
          <p class="mb-5 text-sm text-ink-700/60">Путь ученика от аспиранта к брахману</p>
          <BarList :data="statusData" />
        </div>

        <!-- Status donut -->
        <div class="card p-6">
          <h3 class="mb-1 font-display text-xl text-ink-900">Распределение по статусам</h3>
          <p class="mb-5 text-sm text-ink-700/60">Доля каждой ступени</p>
          <DonutChart :data="statusData" center-label="учеников" />
        </div>
      </div>

      <!-- Top cities -->
      <div class="card p-6">
        <div class="mb-5 flex items-center justify-between">
          <div>
            <h3 class="font-display text-xl text-ink-900">Топ городов</h3>
            <p class="text-sm text-ink-700/60">Для планирования поездок гуру</p>
          </div>
          <RouterLink :to="{ name: 'disciples' }" class="text-sm text-saffron-600 hover:underline">Все ученики →</RouterLink>
        </div>
        <BarList :data="cities" />
      </div>
    </div>
  </div>
</template>
