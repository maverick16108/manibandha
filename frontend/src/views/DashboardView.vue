<script setup>
import { ref, computed, onMounted, onActivated } from 'vue'
defineOptions({ name: 'DashboardView' })
import { useRouter } from 'vue-router'
import { cachedGet, peekCache, TTL } from '../composables/apiCache'
import AppSkeleton from '../components/AppSkeleton.vue'
import DonutChart from '../components/charts/DonutChart.vue'
import BarList from '../components/charts/BarList.vue'
import TimeSeriesChart from '../components/charts/TimeSeriesChart.vue'
import { STATUS_LABELS, STATUS_ORDER } from '../lib/format'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Обзор')

const router = useRouter()
const summary = ref(null)
const cities = ref([])
const regions = ref([])
const timeline = ref([])
const loading = ref(true)

// Sequential warm ramp for the ordered initiation stages (кандидат → брахман).
const STATUS_COLORS = {
  recommended: '#e6b15c', aspirant: '#d98a3d', pranama: '#c8742a', harinama: '#a4551b', brahman: '#6d3f16',
}

const statusData = computed(() => {
  if (!summary.value) return []
  const byLabel = Object.fromEntries(summary.value.by_status.map((r) => [r.key, r.count]))
  return STATUS_ORDER.map((s) => ({
    key: s,
    label: STATUS_LABELS[s],
    value: byLabel[STATUS_LABELS[s]] || 0,
    color: STATUS_COLORS[s],
  }))
})

function go(query) {
  router.push({ name: 'disciples', query })
}

const topGroup = (arr) => arr.filter((x) => x.key !== '—').slice(0, 10).map((x) => ({ label: x.key, value: x.count }))
const CITY = { group_by: 'city' }; const REGION = { group_by: 'region' }

async function load(silent = false) {
  // мгновенно из общего кеша (без скелетона), если уже загружали
  const s0 = peekCache('/reports/summary'); const c0 = peekCache('/reports/group', CITY)
  const r0 = peekCache('/reports/group', REGION); const t0 = peekCache('/reports/timeline')
  if (s0) summary.value = s0
  if (c0) cities.value = topGroup(c0)
  if (r0) regions.value = topGroup(r0)
  if (t0) timeline.value = t0
  if (s0 && c0 && r0 && t0) loading.value = false
  else if (!silent && !summary.value) loading.value = true
  try {
    const [s, c, r, t] = await Promise.all([
      cachedGet('/reports/summary', { ttl: TTL.list }),
      cachedGet('/reports/group', { params: CITY, ttl: TTL.list }),
      cachedGet('/reports/group', { params: REGION, ttl: TTL.list }),
      cachedGet('/reports/timeline', { ttl: TTL.list }),
    ])
    summary.value = s
    cities.value = topGroup(c)
    regions.value = topGroup(r)
    timeline.value = t
  } finally {
    loading.value = false
  }
}
onMounted(() => load())
// keep-alive: первую активацию (сразу после mount) пропускаем, дальше — тихий рефреш без скелетона
let firstActivate = true
onActivated(() => { if (firstActivate) { firstActivate = false; return } load(true) })
</script>

<template>
  <div>
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
      <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <button class="card p-5 text-left transition hover:border-saffron-400/50 hover:shadow" @click="go({})">
          <div class="text-sm text-ink-700/60">Всего учеников</div>
          <div class="mt-1 font-display text-4xl font-semibold text-ink-900">{{ summary.total }}</div>
        </button>
        <button class="card p-5 text-left transition hover:border-saffron-400/50 hover:shadow" @click="go({ ready_pranama: 'true' })">
          <div class="text-sm text-ink-700/60">Готовы к пранаме</div>
          <div class="mt-1 font-display text-4xl font-semibold text-orange-600">{{ summary.ready_for_pranama }}</div>
        </button>
        <button class="card p-5 text-left transition hover:border-saffron-400/50 hover:shadow" @click="go({ ready: 'true' })">
          <div class="text-sm text-ink-700/60">Готовы к инициации</div>
          <div class="mt-1 font-display text-4xl font-semibold text-saffron-600">{{ summary.ready_for_initiation }}</div>
        </button>
        <div class="card p-5">
          <div class="text-sm text-ink-700/60">Городов</div>
          <div class="mt-1 font-display text-4xl font-semibold text-ink-900">{{ cities.length }}</div>
        </div>
        <div class="card p-5">
          <div class="text-sm text-ink-700/60">Регионов</div>
          <div class="mt-1 font-display text-4xl font-semibold text-ink-900">{{ regions.length }}</div>
        </div>
      </div>

      <div class="grid gap-6 lg:grid-cols-2">
        <!-- Timeline: when disciples receive pranama / initiations -->
        <div class="card p-6">
          <h3 class="mb-1 font-display text-xl text-ink-900">Пранама и инициации по времени</h3>
          <p class="mb-5 text-sm text-ink-700/60">Когда ученики получают пранаму и инициации (по месяцам) · нажмите на месяц</p>
          <TimeSeriesChart :data="timeline" @select="(period) => go({ event_month: period })" />
        </div>

        <!-- Status donut -->
        <div class="card p-6">
          <h3 class="mb-1 font-display text-xl text-ink-900">Распределение по статусам</h3>
          <p class="mb-5 text-sm text-ink-700/60">Доля каждой ступени</p>
          <DonutChart :data="statusData" center-label="учеников" clickable @select="(d) => go({ status: d.key })" />
        </div>

        <!-- Top regions -->
        <div class="card p-6">
          <div class="mb-5 flex items-center justify-between">
            <div>
              <h3 class="font-display text-xl text-ink-900">По областям</h3>
              <p class="text-sm text-ink-700/60">Топ регионов</p>
            </div>
          </div>
          <BarList :data="regions" color="#6f7a5a" wide-labels clickable @select="(d) => go({ region: d.label })" />
        </div>

        <!-- Top cities -->
        <div class="card p-6">
          <div class="mb-5 flex items-center justify-between">
            <div>
              <h3 class="font-display text-xl text-ink-900">Топ городов</h3>
              <p class="text-sm text-ink-700/60">Для планирования поездок гуру</p>
            </div>
          </div>
          <BarList :data="cities" clickable @select="(d) => go({ city: d.label })" />
        </div>
      </div>
    </div>
  </div>
</template>
