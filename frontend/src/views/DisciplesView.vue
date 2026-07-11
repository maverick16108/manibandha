<script setup>
import { ref, reactive, onMounted, watch, computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppSelect from '../components/AppSelect.vue'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { STATUS_LABELS, STATUS_ORDER, STATUS_BADGE } from '../lib/format'

const auth = useAuthStore()
const route = useRoute()
const items = ref([])
const total = ref(0)
const loading = ref(true)
const temples = ref([])
const mentors = ref([])
const regions = ref([])
const cities = ref([])

const filters = reactive({ q: '', status: '', region: '', city: '', temple_id: '', mentor_id: '', ready: '' })

const statusOptions = [{ value: '', label: 'Все статусы' }, ...STATUS_ORDER.map((s) => ({ value: s, label: STATUS_LABELS[s] }))]
const templeOptions = computed(() => [{ value: '', label: 'Все храмы' }, ...temples.value.map((t) => ({ value: t.id, label: t.name }))])
const mentorOptions = computed(() => [{ value: '', label: 'Все наставники' }, ...mentors.value.map((m) => ({ value: m.id, label: m.full_name }))])
const regionOptions = computed(() => [{ value: '', label: 'Все области' }, ...regions.value.map((r) => ({ value: r.name, label: r.name }))])
const cityOptions = computed(() => [{ value: '', label: 'Все города' }, ...cities.value.map((c) => ({ value: c.name, label: c.name }))])

let debounce
watch(filters, () => {
  clearTimeout(debounce)
  debounce = setTimeout(load, 300)
})

function params() {
  const p = {}
  for (const [k, v] of Object.entries(filters)) if (v !== '' && v !== null) p[k] = v
  return p
}

async function load() {
  loading.value = true
  try {
    const { data } = await client.get('/disciples', { params: params() })
    items.value = data.items
    total.value = data.total
  } finally {
    loading.value = false
  }
}

async function exportFile(kind) {
  const res = await client.get(`/reports/disciples.${kind}`, { params: params(), responseType: 'blob' })
  const url = URL.createObjectURL(res.data)
  const a = document.createElement('a')
  a.href = url
  a.download = `disciples.${kind}`
  a.click()
  URL.revokeObjectURL(url)
}

function reset() {
  Object.keys(filters).forEach((k) => (filters[k] = ''))
}

onMounted(async () => {
  // seed filters from URL query (deep links from the dashboard)
  for (const k of Object.keys(filters)) if (route.query[k] != null) filters[k] = String(route.query[k])
  const [t, m, r, c] = await Promise.all([
    client.get('/temples'), client.get('/users/mentors'), client.get('/regions'), client.get('/cities'),
  ])
  temples.value = t.data
  mentors.value = m.data
  regions.value = r.data
  cities.value = c.data
  await load()
})
</script>

<template>
  <div>
    <div class="mb-6 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="font-display text-3xl font-semibold text-ink-900">Ученики</h1>
        <p class="text-ink-700/60">Найдено: {{ total }}</p>
      </div>
      <div class="flex gap-2">
        <button class="btn-outline" @click="exportFile('xlsx')"><AppIcon name="download" :size="16" /> Excel</button>
        <button class="btn-outline" @click="exportFile('pdf')"><AppIcon name="download" :size="16" /> PDF</button>
        <RouterLink v-if="auth.isStaff" :to="{ name: 'disciple-new' }" class="btn-primary">+ Добавить</RouterLink>
      </div>
    </div>

    <!-- Filters -->
    <div class="card mb-5 p-4">
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div class="relative lg:col-span-1">
          <AppIcon name="search" :size="16" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-700/40" />
          <input v-model="filters.q" class="input pl-9" placeholder="Поиск по имени…" />
        </div>
        <AppSelect v-model="filters.status" :options="statusOptions" placeholder="Все статусы" />
        <AppSelect v-model="filters.region" :options="regionOptions" placeholder="Все области" />
        <AppSelect v-model="filters.city" :options="cityOptions" placeholder="Все города" />
        <AppSelect v-model="filters.temple_id" :options="templeOptions" placeholder="Все храмы" />
        <AppSelect v-model="filters.mentor_id" :options="mentorOptions" placeholder="Все наставники" />
        <label class="flex items-center gap-2 px-1 text-sm text-ink-700">
          <input type="checkbox" v-model="filters.ready" true-value="true" false-value="" /> Готовые к инициации
        </label>
      </div>
      <div class="mt-3">
        <button class="text-sm text-saffron-600 hover:underline" @click="reset">Сбросить фильтры</button>
      </div>
    </div>

    <!-- Table -->
    <div class="card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead class="border-b border-parchment-200 bg-parchment-50 text-left text-xs uppercase tracking-wide text-ink-700/60">
            <tr>
              <th class="px-4 py-3">Имя</th>
              <th class="px-4 py-3">Статус</th>
              <th class="px-4 py-3">Страна / Город</th>
              <th class="px-4 py-3">Храм</th>
              <th class="px-4 py-3">Наставник</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-parchment-100">
            <template v-if="loading">
              <tr v-for="i in 8" :key="i">
                <td class="px-4 py-3">
                  <div class="flex items-center gap-3"><AppSkeleton w="w-9" h="h-9" rounded="rounded-full" /><AppSkeleton w="w-40" /></div>
                </td>
                <td class="px-4 py-3"><AppSkeleton w="w-20" h="h-5" rounded="rounded-full" /></td>
                <td class="px-4 py-3"><AppSkeleton w="w-28" /></td>
                <td class="px-4 py-3"><AppSkeleton w="w-24" /></td>
                <td class="px-4 py-3"><AppSkeleton w="w-24" /></td>
              </tr>
            </template>
            <template v-else>
              <tr v-for="d in items" :key="d.id" class="hover:bg-parchment-50">
                <td class="px-4 py-3">
                  <RouterLink :to="{ name: 'disciple', params: { id: d.id } }" class="flex items-center gap-3">
                    <img v-if="d.photo_url" :src="d.photo_url" class="photo-bw h-9 w-9 rounded-full object-cover" />
                    <span v-else class="flex h-9 w-9 items-center justify-center rounded-full bg-parchment-200 text-ink-700">
                      {{ (d.spiritual_name || d.material_name || '?')[0] }}
                    </span>
                    <span>
                      <span class="block font-medium text-ink-900">{{ d.spiritual_name || d.material_name }}</span>
                      <span v-if="d.spiritual_name" class="block text-xs text-ink-700/60">{{ d.material_name }}</span>
                    </span>
                  </RouterLink>
                </td>
                <td class="px-4 py-3"><span class="badge" :class="STATUS_BADGE[d.initiation_status]">{{ STATUS_LABELS[d.initiation_status] }}</span></td>
                <td class="px-4 py-3 text-ink-700">{{ d.country || '—' }}<span v-if="d.city">, {{ d.city }}</span></td>
                <td class="px-4 py-3 text-ink-700">{{ d.temple?.name || '—' }}</td>
                <td class="px-4 py-3 text-ink-700">{{ d.mentor?.full_name || '—' }}</td>
              </tr>
              <tr v-if="!items.length"><td colspan="5" class="px-4 py-10 text-center text-ink-700/50">Ученики не найдены</td></tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
