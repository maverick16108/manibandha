<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import client from '../api/client'
import AppDatePicker from '../components/AppDatePicker.vue'
import MarkdownEditor from '../components/MarkdownEditor.vue'
import { usePageTitle } from '../composables/pageTitle'

const route = useRoute()
const router = useRouter()
const id = computed(() => route.params.id)
const isEdit = computed(() => !!id.value)

usePageTitle(() => (isEdit.value ? 'Изменить событие' : 'Новое событие'))

const form = reactive({ title: '', location: '', starts_on: '', ends_on: '', description: '' })
const error = ref('')
const saving = ref(false)

function clean(obj) {
  const out = {}
  for (const [k, v] of Object.entries(obj)) out[k] = v === '' ? null : v
  return out
}

async function save() {
  error.value = ''
  if (!form.title.trim() || !form.starts_on) { error.value = 'Заполните название и дату начала'; return }
  saving.value = true
  try {
    const payload = clean(form)
    if (isEdit.value) await client.patch(`/events/${id.value}`, payload)
    else await client.post('/events', payload)
    router.push({ name: 'calendar' })
  } catch (e) {
    error.value = e.response?.data?.detail || 'Ошибка сохранения'
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  if (isEdit.value) {
    const { data } = await client.get(`/events/${id.value}`)
    for (const k of Object.keys(form)) form[k] = data[k] ?? ''
  }
})
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <form class="card space-y-4 p-6" @submit.prevent="save">
      <div><label class="label">Название *</label><input v-model="form.title" class="input" required autofocus /></div>
      <div><label class="label">Место (где находится гуру)</label><input v-model="form.location" class="input" placeholder="Вриндаван, Индия" /></div>
      <div class="grid gap-3 sm:grid-cols-2">
        <div><label class="label">Дата начала *</label><AppDatePicker v-model="form.starts_on" /></div>
        <div><label class="label">Дата окончания</label><AppDatePicker v-model="form.ends_on" /></div>
      </div>
      <div><label class="label">Описание</label><MarkdownEditor v-model="form.description" :rows="8" placeholder="Описание события… (можно вставлять фото)" /></div>
      <p v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>
      <div class="flex gap-2">
        <button class="btn-primary" :disabled="saving">{{ saving ? 'Сохранение…' : 'Сохранить' }}</button>
        <RouterLink :to="{ name: 'calendar' }" class="btn-ghost">Отмена</RouterLink>
      </div>
    </form>
  </div>
</template>
