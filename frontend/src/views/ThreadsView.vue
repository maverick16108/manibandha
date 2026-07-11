<script setup>
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppSelect from '../components/AppSelect.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { formatDate } from '../lib/format'

const route = useRoute()
const auth = useAuthStore()
const kind = computed(() => route.meta.kind) // 'question' | 'report'
const isReport = computed(() => kind.value === 'report')

const threads = ref([])
const loading = ref(true)
const disciples = ref([])
const showForm = ref(false)
const error = ref('')
const now = new Date()
const MONTHS = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']
const form = reactive({ disciple_id: '', body: '', month: now.getMonth() + 1, year: now.getFullYear() })

const discipleOptions = computed(() => disciples.value.map((d) => ({ value: d.id, label: d.spiritual_name || d.material_name })))
const monthOptions = MONTHS.map((m, i) => ({ value: i + 1, label: m }))
const yearOptions = [now.getFullYear() - 1, now.getFullYear(), now.getFullYear() + 1].map((y) => ({ value: y, label: String(y) }))

async function load() {
  loading.value = true
  try {
    const { data } = await client.get('/threads', { params: { kind: kind.value } })
    threads.value = data
  } finally {
    loading.value = false
  }
}
watch(kind, load)

function periodLabel(p) {
  if (!p) return ''
  const [y, m] = p.split('-')
  return `${MONTHS[+m - 1]} ${y}`
}

function openForm() {
  Object.assign(form, { disciple_id: auth.user?.disciple_id || '', body: '', month: now.getMonth() + 1, year: now.getFullYear() })
  error.value = ''
  showForm.value = true
}
async function submit() {
  error.value = ''
  const payload = { kind: kind.value, body: form.body }
  if (!auth.user?.disciple_id) payload.disciple_id = form.disciple_id || null
  if (isReport.value) payload.period = `${form.year}-${String(form.month).padStart(2, '0')}`
  try {
    await client.post('/threads', payload)
    showForm.value = false
    await load()
  } catch (e) {
    error.value = e.response?.data?.detail || 'Не удалось отправить'
  }
}

onMounted(async () => {
  // guru/staff choose a disciple; a student writes about themselves
  if (!auth.user?.disciple_id) {
    try {
      const { data } = await client.get('/disciples', { params: { limit: 500 } })
      disciples.value = data.items
    } catch { /* students can't list — ignore */ }
  }
  await load()
})
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h1 class="font-display text-3xl font-semibold text-ink-900">{{ isReport ? 'Отчёты о служении' : 'Вопросы гуру' }}</h1>
        <p class="text-ink-700/60">{{ isReport ? 'Ежемесячные отчёты учеников · доступ: ученик, наставник, гуру' : 'Личные вопросы · видит только гуру и сам ученик' }}</p>
      </div>
      <button class="btn-primary" @click="openForm">{{ isReport ? '+ Новый отчёт' : '+ Новый вопрос' }}</button>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 4" :key="i" class="card space-y-2 p-4"><AppSkeleton w="w-48" /><AppSkeleton w="w-full" h="h-3" /></div>
    </div>

    <div v-else class="space-y-3">
      <RouterLink v-for="t in threads" :key="t.id" :to="{ name: 'thread', params: { id: t.id } }"
                  class="card block p-4 transition hover:border-saffron-400/50 hover:shadow">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <span class="font-medium text-ink-900">{{ t.disciple_name }}</span>
              <span v-if="t.period" class="badge bg-saffron-500/15 text-saffron-700">{{ periodLabel(t.period) }}</span>
            </div>
            <div v-if="t.subject" class="text-sm text-ink-800">{{ t.subject }}</div>
            <div class="mt-1 truncate text-sm text-ink-700/60">{{ t.last_preview }}</div>
          </div>
          <div class="shrink-0 text-right text-xs text-ink-700/50">
            <div>{{ formatDate(t.updated_at) }}</div>
            <div class="mt-1">💬 {{ t.messages_count }}</div>
          </div>
        </div>
      </RouterLink>
      <div v-if="!threads.length" class="card p-8 text-center text-ink-700/50">
        {{ isReport ? 'Отчётов пока нет' : 'Вопросов пока нет' }}
      </div>
    </div>

    <!-- create -->
    <div v-if="showForm" class="fixed inset-0 z-40 flex items-center justify-center bg-ink-900/40 p-4" @click.self="showForm = false">
      <div class="card w-full max-w-lg p-6">
        <h3 class="mb-4 font-display text-2xl text-ink-900">{{ isReport ? 'Новый отчёт' : 'Новый вопрос' }}</h3>
        <form class="space-y-3" @submit.prevent="submit">
          <div v-if="!auth.user?.disciple_id">
            <label class="label">Ученик *</label>
            <AppSelect v-model="form.disciple_id" :options="discipleOptions" placeholder="Выберите ученика" />
          </div>
          <div v-if="isReport" class="grid grid-cols-2 gap-3">
            <div><label class="label">Месяц</label><AppSelect v-model="form.month" :options="monthOptions" /></div>
            <div><label class="label">Год</label><AppSelect v-model="form.year" :options="yearOptions" /></div>
          </div>
          <div>
            <label class="label">{{ isReport ? 'Как прошло служение в этом месяце' : 'Ваш вопрос' }}</label>
            <textarea v-model="form.body" rows="5" class="input resize-y min-h-[7rem]" required></textarea>
          </div>
          <p v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>
          <div class="flex gap-2 pt-1">
            <button class="btn-primary">Отправить</button>
            <button type="button" class="btn-ghost" @click="showForm = false">Отмена</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
