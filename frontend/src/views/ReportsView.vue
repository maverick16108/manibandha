<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import { RouterLink } from 'vue-router'
import client from '../api/client'
import AppSelect from '../components/AppSelect.vue'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { STATUS_LABELS, STATUS_ORDER, STATUS_BADGE } from '../lib/format'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Отчёты')

const mentors = ref([])
const regionsOpt = ref([])
const citiesOpt = ref([])
const countriesOpt = ref([])
const groups = ref([])
const rows = ref([])
const total = ref(0)
const loading = ref(true)

const filters = reactive({ q: '', status: '', country: '', region: '', city: '', mentor_id: '', ready: '' })
const groupBy = ref('status')

const statusOptions = [{ value: '', label: 'Все статусы' }, ...STATUS_ORDER.map((s) => ({ value: s, label: STATUS_LABELS[s] }))]
const groupOptions = [
  { value: 'status', label: 'По статусу инициации' },
  { value: 'region', label: 'По области' },
  { value: 'city', label: 'По городу' },
  { value: 'country', label: 'По стране' },
  { value: 'mentor', label: 'По куратору' },
]

function params() {
  const p = {}
  for (const [k, v] of Object.entries(filters)) if (v !== '' && v !== null) p[k] = v
  return p
}

let deb
watch([filters, groupBy], () => {
  clearTimeout(deb)
  deb = setTimeout(load, 300)
})

async function load() {
  loading.value = true
  try {
    const [g, l] = await Promise.all([
      client.get('/reports/group', { params: { ...params(), group_by: groupBy.value } }),
      client.get('/disciples', { params: { ...params(), named: true, limit: 500 } }),
    ])
    groups.value = g.data
    rows.value = l.data.items
    total.value = l.data.total
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

const maxCount = () => Math.max(1, ...groups.value.map((g) => g.count))

onMounted(async () => {
  const [m, r, c, co] = await Promise.all([
    client.get('/disciples', { params: { is_mentor: true, limit: 500 } }),
    client.get('/regions'), client.get('/cities'), client.get('/countries'),
  ])
  mentors.value = [{ value: '', label: 'Все кураторы' }, ...m.data.items.map((x) => ({ value: x.id, label: x.spiritual_name || x.material_name }))]
  regionsOpt.value = [{ value: '', label: 'Все области' }, ...r.data.map((x) => ({ value: x.name, label: x.name }))]
  citiesOpt.value = [{ value: '', label: 'Все города' }, ...c.data.map((x) => ({ value: x.name, label: x.name }))]
  countriesOpt.value = [{ value: '', label: 'Все страны' }, ...co.data.map((x) => ({ value: x.name, label: x.name }))]
  await load()
})
</script>

<template>
  <div>
    <div class="mb-6 flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="text-ink-700/60">Конструктор: фильтры, группировка и экспорт</p>
      </div>
      <div class="flex gap-2">
        <button class="btn-outline" @click="exportFile('xlsx')"><AppIcon name="download" :size="16" /> Excel</button>
        <button class="btn-outline" @click="exportFile('pdf')"><AppIcon name="download" :size="16" /> PDF</button>
      </div>
    </div>

    <!-- Filters -->
    <div class="card mb-5 p-4">
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div class="relative">
          <AppIcon name="search" :size="16" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-700/40" />
          <input v-model="filters.q" class="input pl-9" placeholder="Поиск по имени…" />
        </div>
        <AppSelect v-model="filters.status" :options="statusOptions" placeholder="Все статусы" />
        <AppSelect v-model="filters.region" :options="regionsOpt" placeholder="Все области" />
        <AppSelect v-model="filters.city" :options="citiesOpt" placeholder="Все города" />
        <AppSelect v-model="filters.country" :options="countriesOpt" placeholder="Все страны" />
        <AppSelect v-model="filters.mentor_id" :options="mentors" placeholder="Все кураторы" />
      </div>
      <div class="mt-3 flex flex-wrap items-center gap-4">
        <label class="flex items-center gap-2 text-sm text-ink-700">
          <input type="checkbox" v-model="filters.ready" true-value="true" false-value="" /> Только готовые к инициации
        </label>
        <button class="text-sm text-saffron-600 hover:underline" @click="reset">Сбросить</button>
      </div>
    </div>

    <!-- Grouping -->
    <div class="card mb-5 p-6">
      <div class="mb-4 flex flex-wrap items-center gap-3">
        <span class="text-sm font-medium text-ink-700">Группировка:</span>
        <div class="w-64"><AppSelect v-model="groupBy" :options="groupOptions" /></div>
        <span class="ml-auto text-sm text-ink-700/60">Найдено: <b class="text-ink-900">{{ total }}</b></span>
      </div>

      <div v-if="loading" class="space-y-3">
        <div v-for="i in 5" :key="i" class="flex items-center gap-3">
          <AppSkeleton w="w-32" h="h-4" /><AppSkeleton w="w-full" h="h-3" />
        </div>
      </div>
      <ul v-else class="space-y-2.5">
        <li v-for="g in groups" :key="g.key" class="flex items-center gap-3">
          <span class="w-40 shrink-0 truncate text-sm text-ink-700">{{ g.key }}</span>
          <div class="h-5 flex-1 overflow-hidden rounded bg-parchment-100">
            <div class="h-full rounded bg-saffron-500/70" :style="{ width: (g.count / maxCount() * 100) + '%' }"></div>
          </div>
          <span class="w-10 shrink-0 text-right text-sm font-medium text-ink-900">{{ g.count }}</span>
        </li>
        <li v-if="!groups.length" class="text-sm text-ink-700/50">Нет данных</li>
      </ul>
    </div>

    <!-- Filtered table -->
    <div class="card overflow-hidden">
      <div class="border-b border-parchment-200 px-4 py-3 text-sm font-medium text-ink-700">Выборка ({{ total }})</div>
      <div class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead class="bg-parchment-50 text-left text-xs uppercase tracking-wide text-ink-700/60">
            <tr><th class="px-4 py-2">Имя</th><th class="px-4 py-2">Статус</th><th class="px-4 py-2">Область</th><th class="px-4 py-2">Город</th></tr>
          </thead>
          <tbody class="divide-y divide-parchment-100">
            <template v-if="loading">
              <tr v-for="i in 6" :key="i">
                <td class="px-4 py-3"><AppSkeleton w="w-40" /></td>
                <td class="px-4 py-3"><AppSkeleton w="w-20" /></td>
                <td class="px-4 py-3"><AppSkeleton w="w-24" /></td>
                <td class="px-4 py-3"><AppSkeleton w="w-24" /></td>
              </tr>
            </template>
            <template v-else>
              <tr v-for="d in rows" :key="d.id" class="hover:bg-parchment-50">
                <td class="px-4 py-2.5">
                  <RouterLink :to="{ name: 'disciple', params: { id: d.id } }" class="font-medium text-ink-900 hover:text-saffron-700">
                    {{ d.spiritual_name || d.material_name }}
                  </RouterLink>
                </td>
                <td class="px-4 py-2.5"><span class="badge" :class="STATUS_BADGE[d.initiation_status]">{{ STATUS_LABELS[d.initiation_status] }}</span></td>
                <td class="px-4 py-2.5 text-ink-700">{{ d.city || '—' }}</td>
                <td class="px-4 py-2.5 text-ink-700">{{ d.region || '—' }}</td>
              </tr>
              <tr v-if="!rows.length"><td colspan="4" class="px-4 py-8 text-center text-ink-700/50">Ничего не найдено</td></tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
