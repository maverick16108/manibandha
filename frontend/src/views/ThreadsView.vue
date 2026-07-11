<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppSelect from '../components/AppSelect.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { formatDate } from '../lib/format'
import { usePageTitle } from '../composables/pageTitle'

const route = useRoute()
const auth = useAuthStore()
const kind = computed(() => route.meta.kind) // 'question' | 'report'
const isReport = computed(() => kind.value === 'report')

usePageTitle(() => (isReport.value ? 'Отчёты о служении' : 'Вопросы гуру'))

const threads = ref([])
const loading = ref(true)
const disciples = ref([])
const filterDisciple = ref('')
const MONTHS = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']

// guru/staff can filter by disciple; a student only ever sees their own
const showFilter = computed(() => !auth.user?.disciple_id)
const discipleOptions = computed(() => [{ value: '', label: 'Все ученики' }, ...disciples.value.map((d) => ({ value: d.id, label: d.spiritual_name || d.material_name }))])

async function load() {
  loading.value = true
  try {
    const params = { kind: kind.value }
    if (filterDisciple.value) params.disciple_id = filterDisciple.value
    const { data } = await client.get('/threads', { params })
    threads.value = data
  } finally {
    loading.value = false
  }
}
watch([kind, filterDisciple], load)

function periodLabel(p) {
  if (!p) return ''
  const [y, m] = p.split('-')
  return `${MONTHS[+m - 1]} ${y}`
}

onMounted(async () => {
  if (showFilter.value) {
    try {
      const { data } = await client.get('/disciples', { params: { limit: 500 } })
      disciples.value = data.items
    } catch { /* ignore */ }
  }
  await load()
})
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <div class="mb-6 flex items-center justify-between">
      <div>
        <p class="text-ink-700/60">{{ isReport ? 'Ежемесячные отчёты учеников · доступ: ученик, наставник, гуру' : 'Личные вопросы · видит только гуру и сам ученик' }}</p>
      </div>
      <RouterLink v-if="auth.user?.disciple_id" :to="{ name: isReport ? 'report-new' : 'question-new' }" class="btn-primary">
        {{ isReport ? '+ Новый отчёт' : '+ Новый вопрос' }}
      </RouterLink>
    </div>

    <div v-if="showFilter" class="card mb-4 p-3 sm:max-w-sm">
      <AppSelect v-model="filterDisciple" :options="discipleOptions" placeholder="Все ученики" />
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
              <span v-if="t.unread" class="h-2.5 w-2.5 shrink-0 rounded-full bg-saffron-500" title="Новое"></span>
              <span class="font-medium text-ink-900">{{ t.disciple_name }}</span>
              <span v-if="t.period" class="badge bg-saffron-500/15 text-saffron-700">{{ periodLabel(t.period) }}</span>
              <span v-if="t.unread" class="badge bg-saffron-500/15 text-saffron-700">Новое</span>
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
  </div>
</template>
