<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import { RouterLink } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import { STATUS_LABELS, STATUS_ORDER, STATUS_BADGE, formatDate } from '../lib/format'

const auth = useAuthStore()
const items = ref([])
const total = ref(0)
const loading = ref(true)
const temples = ref([])
const mentors = ref([])

const filters = reactive({ q: '', status: '', country: '', temple_id: '', mentor_id: '', ready: '' })

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
  const [t, m] = await Promise.all([client.get('/temples'), client.get('/users/mentors')])
  temples.value = t.data
  mentors.value = m.data
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
        <button class="btn-outline" @click="exportFile('xlsx')">⬇ Excel</button>
        <button class="btn-outline" @click="exportFile('pdf')">⬇ PDF</button>
        <RouterLink v-if="auth.isStaff" :to="{ name: 'disciple-new' }" class="btn-primary">+ Добавить</RouterLink>
      </div>
    </div>

    <!-- Filters -->
    <div class="card mb-5 p-4">
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-6">
        <input v-model="filters.q" class="input lg:col-span-2" placeholder="🔍 Поиск по имени…" />
        <select v-model="filters.status" class="input">
          <option value="">Все статусы</option>
          <option v-for="s in STATUS_ORDER" :key="s" :value="s">{{ STATUS_LABELS[s] }}</option>
        </select>
        <input v-model="filters.country" class="input" placeholder="Страна" />
        <select v-model="filters.temple_id" class="input">
          <option value="">Все храмы</option>
          <option v-for="t in temples" :key="t.id" :value="t.id">{{ t.name }}</option>
        </select>
        <select v-model="filters.mentor_id" class="input">
          <option value="">Все наставники</option>
          <option v-for="m in mentors" :key="m.id" :value="m.id">{{ m.full_name }}</option>
        </select>
      </div>
      <div class="mt-3 flex items-center gap-4">
        <label class="flex items-center gap-2 text-sm text-ink-700">
          <input type="checkbox" v-model="filters.ready" true-value="true" false-value="" />
          Только готовые к инициации
        </label>
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
              </td>
              <td class="px-4 py-3 text-ink-700">{{ d.country || '—' }}<span v-if="d.city">, {{ d.city }}</span></td>
              <td class="px-4 py-3 text-ink-700">{{ d.temple?.name || '—' }}</td>
              <td class="px-4 py-3 text-ink-700">{{ d.mentor?.full_name || '—' }}</td>
            </tr>
            <tr v-if="!loading && !items.length">
              <td colspan="5" class="px-4 py-10 text-center text-ink-700/50">Ученики не найдены</td>
            </tr>
            <tr v-if="loading">
              <td colspan="5" class="px-4 py-10 text-center text-ink-700/50">Загрузка…</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
