<script setup>
import { ref, reactive, onMounted, onBeforeUnmount, watch, computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppSelect from '../components/AppSelect.vue'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { STATUS_LABELS, STATUS_ORDER, STATUS_BADGE } from '../lib/format'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Ученики')

const auth = useAuthStore()
const route = useRoute()
const items = ref([])
const total = ref(0)
const loading = ref(true)
const mentors = ref([])
const regions = ref([])
const cities = ref([])

const filters = reactive({ q: '', status: '', region: '', city: '', mentor_id: '', ready: '', ready_pranama: '', event_month: '' })

const MONTHS = ['', 'янв', 'фев', 'мар', 'апр', 'май', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек']
function monthLabel(ym) {
  const [y, m] = (ym || '').split('-')
  return m ? `${MONTHS[+m]} ${y}` : ym
}
// какое событие произошло у ученика в выбранном месяце (для фильтра по месяцу)
function eventLabel(d) {
  const m = filters.event_month
  if (!m) return ''
  if (d.harinama_date && d.harinama_date.startsWith(m)) return 'Харинама'
  if (d.brahman_date && d.brahman_date.startsWith(m)) return 'Брахман'
  if (d.pranama_date && d.pranama_date.startsWith(m)) return 'Пранама-мантра'
  return ''
}

const statusOptions = [{ value: '', label: 'Все статусы' }, ...STATUS_ORDER.map((s) => ({ value: s, label: STATUS_LABELS[s] }))]
const mentorOptions = computed(() => [{ value: '', label: 'Все наставники' }, ...mentors.value.map((m) => ({ value: m.id, label: m.spiritual_name || m.material_name }))])
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

// Type anywhere on the page to search by name
const searchInput = ref(null)
function onDocKey(e) {
  if (e.ctrlKey || e.metaKey || e.altKey) return
  const t = e.target
  const tag = (t.tagName || '').toLowerCase()
  if (tag === 'input' || tag === 'textarea' || tag === 'select' || t.isContentEditable) return
  if (e.key.length === 1 && e.key !== ' ') {
    filters.q += e.key
    searchInput.value?.focus()
    e.preventDefault()
  } else if (e.key === 'Backspace' && filters.q) {
    filters.q = filters.q.slice(0, -1)
    e.preventDefault()
  }
}
onMounted(() => document.addEventListener('keydown', onDocKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onDocKey))

onMounted(async () => {
  // seed filters from URL query (deep links from the dashboard)
  for (const k of Object.keys(filters)) if (route.query[k] != null) filters[k] = String(route.query[k])
  const [m, r, c] = await Promise.all([
    client.get('/disciples', { params: { is_mentor: true, limit: 500 } }), client.get('/regions'), client.get('/cities'),
  ])
  mentors.value = m.data.items
  regions.value = r.data
  cities.value = c.data
  await load()
})
</script>

<template>
  <div>
    <div class="mb-6 flex flex-wrap items-center justify-between gap-3">
      <div>
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
          <input ref="searchInput" v-model="filters.q" class="input pl-9" placeholder="Поиск по имени…" />
        </div>
        <AppSelect v-model="filters.status" :options="statusOptions" placeholder="Все статусы" />
        <AppSelect v-model="filters.region" :options="regionOptions" placeholder="Все области" />
        <AppSelect v-model="filters.city" :options="cityOptions" placeholder="Все города" />
        <AppSelect v-model="filters.mentor_id" :options="mentorOptions" placeholder="Все наставники" />
        <div class="flex flex-col justify-center gap-1.5 px-1">
          <label class="flex items-center gap-2 text-sm text-ink-700">
            <input type="checkbox" v-model="filters.ready_pranama" true-value="true" false-value="" /> Готовые к пранаме
          </label>
          <label class="flex items-center gap-2 text-sm text-ink-700">
            <input type="checkbox" v-model="filters.ready" true-value="true" false-value="" /> Готовые к инициации
          </label>
        </div>
      </div>
      <div class="mt-3 flex flex-wrap items-center gap-3">
        <span v-if="filters.event_month" class="badge bg-saffron-500/15 text-saffron-700">
          События: {{ monthLabel(filters.event_month) }}
          <button class="ml-1 text-saffron-700/70 hover:text-saffron-700" @click="filters.event_month = ''">✕</button>
        </span>
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
              <th class="px-4 py-3">Область / Город</th>
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
                <td class="px-4 py-3">
                  <span class="badge" :class="STATUS_BADGE[d.initiation_status]">{{ STATUS_LABELS[d.initiation_status] }}</span>
                  <span v-if="eventLabel(d)" class="badge ml-1 bg-orange-100 text-orange-800">{{ eventLabel(d) }}</span>
                </td>
                <td class="px-4 py-3 text-ink-700">{{ d.region || d.country || '—' }}<span v-if="d.city">, {{ d.city }}</span></td>
                <td class="px-4 py-3 text-ink-700">{{ d.mentor?.name || '—' }}</td>
              </tr>
              <tr v-if="!items.length"><td colspan="4" class="px-4 py-10 text-center text-ink-700/50">Ученики не найдены</td></tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
