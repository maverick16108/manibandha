<script setup>
import { ref, reactive, onMounted } from 'vue'
import client from '../api/client'
import { STATUS_LABELS, STATUS_ORDER } from '../lib/format'

const summary = ref(null)
const temples = ref([])
const mentors = ref([])
const filters = reactive({ status: '', country: '', temple_id: '', mentor_id: '' })

function params() {
  const p = {}
  for (const [k, v] of Object.entries(filters)) if (v !== '') p[k] = v
  return p
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

onMounted(async () => {
  const [s, t, m] = await Promise.all([
    client.get('/reports/summary'),
    client.get('/temples'),
    client.get('/users/mentors'),
  ])
  summary.value = s.data
  temples.value = t.data
  mentors.value = m.data
})
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <h1 class="mb-6 font-display text-3xl font-semibold text-ink-900">Отчёты</h1>

    <div v-if="summary" class="mb-6 grid gap-4 sm:grid-cols-3">
      <div class="card p-5"><div class="text-sm text-ink-700/60">Всего</div><div class="font-display text-3xl text-ink-900">{{ summary.total }}</div></div>
      <div class="card p-5"><div class="text-sm text-ink-700/60">Готовы к инициации</div><div class="font-display text-3xl text-saffron-600">{{ summary.ready_for_initiation }}</div></div>
      <div class="card p-5"><div class="text-sm text-ink-700/60">Статусов</div><div class="font-display text-3xl text-ink-900">{{ summary.by_status.length }}</div></div>
    </div>

    <div class="card p-6">
      <h3 class="mb-4 font-display text-xl text-ink-900">Экспорт списка</h3>
      <div class="grid gap-3 sm:grid-cols-4">
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
      <div class="mt-4 flex gap-3">
        <button class="btn-primary" @click="exportFile('xlsx')">⬇ Скачать Excel</button>
        <button class="btn-outline" @click="exportFile('pdf')">⬇ Скачать PDF</button>
      </div>
      <p class="mt-3 text-sm text-ink-700/60">Экспорт учитывает выбранные фильтры. Список аспирантов, готовых к инициации, — на вкладке «Ученики» (фильтр «Только готовые»).</p>
    </div>
  </div>
</template>
