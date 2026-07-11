<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppSelect from '../components/AppSelect.vue'
import AppIcon from '../components/AppIcon.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const kind = computed(() => route.meta.kind) // 'question' | 'report'
const isReport = computed(() => kind.value === 'report')

const disciples = ref([])
const error = ref('')
const saving = ref(false)
const now = new Date()
const MONTHS = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']
const form = reactive({ disciple_id: '', body: '', month: now.getMonth() + 1, year: now.getFullYear() })

const discipleOptions = computed(() => disciples.value.map((d) => ({ value: d.id, label: d.spiritual_name || d.material_name })))
const monthOptions = MONTHS.map((m, i) => ({ value: i + 1, label: m }))
const yearOptions = [now.getFullYear() - 1, now.getFullYear(), now.getFullYear() + 1].map((y) => ({ value: y, label: String(y) }))
const backTo = computed(() => (isReport.value ? { name: 'service-reports' } : { name: 'questions' }))

async function submit() {
  error.value = ''
  saving.value = true
  const payload = { kind: kind.value, body: form.body }
  if (!auth.user?.disciple_id) payload.disciple_id = form.disciple_id || null
  if (isReport.value) payload.period = `${form.year}-${String(form.month).padStart(2, '0')}`
  try {
    const { data } = await client.post('/threads', payload)
    router.push({ name: 'thread', params: { id: data.id } })
  } catch (e) {
    error.value = e.response?.data?.detail || 'Не удалось отправить'
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  form.disciple_id = auth.user?.disciple_id || ''
  if (!auth.user?.disciple_id) {
    try {
      const { data } = await client.get('/disciples', { params: { limit: 500 } })
      disciples.value = data.items
    } catch { /* students can't list */ }
  }
})
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <RouterLink :to="backTo" class="btn-outline mb-4">
      <AppIcon name="chevron" :size="16" class="rotate-90" /> Назад
    </RouterLink>
    <h1 class="mb-6 font-display text-3xl font-semibold text-ink-900">{{ isReport ? 'Новый отчёт о служении' : 'Новый вопрос гуру' }}</h1>

    <form class="card space-y-4 p-6" @submit.prevent="submit">
      <div v-if="!auth.user?.disciple_id">
        <label class="label">Ученик *</label>
        <AppSelect v-model="form.disciple_id" :options="discipleOptions" placeholder="Выберите ученика" />
      </div>
      <div v-if="isReport" class="grid grid-cols-2 gap-3 sm:max-w-md">
        <div><label class="label">Месяц</label><AppSelect v-model="form.month" :options="monthOptions" /></div>
        <div><label class="label">Год</label><AppSelect v-model="form.year" :options="yearOptions" /></div>
      </div>
      <div>
        <label class="label">{{ isReport ? 'Как прошло служение в этом месяце' : 'Ваш вопрос' }}</label>
        <textarea v-model="form.body" rows="8" class="input resize-y min-h-[10rem]" required autofocus></textarea>
      </div>
      <p v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>
      <div class="flex gap-2">
        <button class="btn-primary" :disabled="saving">{{ saving ? 'Отправка…' : 'Отправить' }}</button>
        <RouterLink :to="backTo" class="btn-ghost">Отмена</RouterLink>
      </div>
    </form>
  </div>
</template>
