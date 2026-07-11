<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import client from '../api/client'
import { STATUS_LABELS, STATUS_ORDER, MARITAL_LABELS } from '../lib/format'

const route = useRoute()
const router = useRouter()
const id = computed(() => route.params.id)
const isEdit = computed(() => !!id.value)

const temples = ref([])
const mentors = ref([])
const error = ref('')
const saving = ref(false)

const form = reactive({
  material_name: '', spiritual_name: '', photo_url: '',
  phone: '', email: '', messenger: '',
  country: '', city: '', temple_id: '',
  marital_status: '', date_of_birth: '',
  initiation_status: 'aspirant', harinama_date: '', harinama_name: '', brahman_date: '',
  seva: '', current_activity: '',
  mentor_id: '', recommended_by: '', application_date: '', ready_for_initiation: false,
  notes: '',
})

function clean(obj) {
  const out = {}
  for (const [k, v] of Object.entries(obj)) out[k] = v === '' ? null : v
  return out
}

async function save() {
  error.value = ''
  saving.value = true
  try {
    const payload = clean(form)
    if (isEdit.value) {
      await client.patch(`/disciples/${id.value}`, payload)
      router.push({ name: 'disciple', params: { id: id.value } })
    } else {
      const { data } = await client.post('/disciples', payload)
      router.push({ name: 'disciple', params: { id: data.id } })
    }
  } catch (e) {
    error.value = e.response?.data?.detail || 'Ошибка сохранения'
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  const [t, m] = await Promise.all([client.get('/temples'), client.get('/users/mentors')])
  temples.value = t.data
  mentors.value = m.data
  if (isEdit.value) {
    const { data } = await client.get(`/disciples/${id.value}`)
    for (const k of Object.keys(form)) {
      if (k === 'temple_id' || k === 'mentor_id') form[k] = data[k] ?? ''
      else form[k] = data[k] ?? (typeof form[k] === 'boolean' ? false : '')
    }
  }
})
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <RouterLink :to="{ name: 'disciples' }" class="mb-4 inline-block text-sm text-saffron-600 hover:underline">← К списку</RouterLink>
    <h1 class="mb-6 font-display text-3xl font-semibold text-ink-900">
      {{ isEdit ? 'Редактировать ученика' : 'Новый ученик' }}
    </h1>

    <form class="space-y-6" @submit.prevent="save">
      <section class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Основное</h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <div><label class="label">Мирское имя *</label><input v-model="form.material_name" class="input" required /></div>
          <div><label class="label">Духовное имя</label><input v-model="form.spiritual_name" class="input" /></div>
          <div class="sm:col-span-2"><label class="label">Фото (URL)</label><input v-model="form.photo_url" class="input" placeholder="https://…" /></div>
        </div>
      </section>

      <section class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Контакты и место</h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <div><label class="label">Телефон</label><input v-model="form.phone" class="input" /></div>
          <div><label class="label">Email</label><input v-model="form.email" type="email" class="input" /></div>
          <div><label class="label">Мессенджер</label><input v-model="form.messenger" class="input" /></div>
          <div><label class="label">Семейное положение</label>
            <select v-model="form.marital_status" class="input">
              <option value="">—</option>
              <option v-for="(l, k) in MARITAL_LABELS" :key="k" :value="k">{{ l }}</option>
            </select>
          </div>
          <div><label class="label">Страна</label><input v-model="form.country" class="input" /></div>
          <div><label class="label">Город</label><input v-model="form.city" class="input" /></div>
          <div><label class="label">Храм / община</label>
            <select v-model="form.temple_id" class="input">
              <option value="">—</option>
              <option v-for="t in temples" :key="t.id" :value="t.id">{{ t.name }}</option>
            </select>
          </div>
          <div><label class="label">Дата рождения</label><input v-model="form.date_of_birth" type="date" class="input" /></div>
        </div>
      </section>

      <section class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Инициация</h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <div><label class="label">Статус</label>
            <select v-model="form.initiation_status" class="input">
              <option v-for="s in STATUS_ORDER" :key="s" :value="s">{{ STATUS_LABELS[s] }}</option>
            </select>
          </div>
          <div></div>
          <div><label class="label">Дата харинамы</label><input v-model="form.harinama_date" type="date" class="input" /></div>
          <div><label class="label">Духовное имя (харинама)</label><input v-model="form.harinama_name" class="input" /></div>
          <div><label class="label">Дата второй инициации (брахман)</label><input v-model="form.brahman_date" type="date" class="input" /></div>
        </div>
      </section>

      <section class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Путь аспиранта и служение</h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <div><label class="label">Наставник</label>
            <select v-model="form.mentor_id" class="input">
              <option value="">—</option>
              <option v-for="m in mentors" :key="m.id" :value="m.id">{{ m.full_name }}</option>
            </select>
          </div>
          <div><label class="label">Кто рекомендовал</label><input v-model="form.recommended_by" class="input" placeholder="Наставник / президент храма" /></div>
          <div><label class="label">Дата заявки</label><input v-model="form.application_date" type="date" class="input" /></div>
          <div class="flex items-end">
            <label class="flex items-center gap-2 text-sm text-ink-700">
              <input type="checkbox" v-model="form.ready_for_initiation" /> Готов(а) к инициации
            </label>
          </div>
          <div class="sm:col-span-2"><label class="label">Севы (служение)</label><textarea v-model="form.seva" rows="2" class="input"></textarea></div>
          <div class="sm:col-span-2"><label class="label">Текущая деятельность</label><textarea v-model="form.current_activity" rows="2" class="input"></textarea></div>
          <div class="sm:col-span-2"><label class="label">Примечания</label><textarea v-model="form.notes" rows="3" class="input"></textarea></div>
        </div>
      </section>

      <p v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>

      <div class="flex gap-3">
        <button type="submit" class="btn-primary" :disabled="saving">{{ saving ? 'Сохранение…' : 'Сохранить' }}</button>
        <RouterLink :to="{ name: 'disciples' }" class="btn-ghost">Отмена</RouterLink>
      </div>
    </form>
  </div>
</template>
