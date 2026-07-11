<script setup>
import { ref, reactive, onMounted } from 'vue'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppSkeleton from '../components/AppSkeleton.vue'

const auth = useAuthStore()
const cities = ref([])
const loading = ref(true)
const showForm = ref(false)
const editing = ref(null)
const error = ref('')
const form = reactive({ name: '', country: '' })

async function load() {
  loading.value = true
  try {
    const { data } = await client.get('/cities')
    cities.value = data
  } finally {
    loading.value = false
  }
}

function startNew() {
  editing.value = null
  Object.assign(form, { name: '', country: '' })
  error.value = ''
  showForm.value = true
}
function startEdit(c) {
  editing.value = c.id
  Object.assign(form, { name: c.name, country: c.country ?? '' })
  error.value = ''
  showForm.value = true
}
async function save() {
  error.value = ''
  try {
    if (editing.value) await client.patch(`/cities/${editing.value}`, form)
    else await client.post('/cities', form)
    showForm.value = false
    await load()
  } catch (e) {
    error.value = e.response?.data?.detail || 'Ошибка сохранения'
  }
}
async function remove(c) {
  if (!confirm(`Удалить город «${c.name}»?`)) return
  await client.delete(`/cities/${c.id}`)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h1 class="font-display text-3xl font-semibold text-ink-900">Города</h1>
        <p class="text-ink-700/60">Справочник для анкет и фильтров</p>
      </div>
      <button v-if="auth.isStaff" class="btn-primary" @click="startNew">+ Добавить</button>
    </div>

    <div class="card">
      <div v-if="loading" class="grid grid-cols-2 gap-px sm:grid-cols-3">
        <div v-for="i in 9" :key="i" class="p-4"><AppSkeleton w="w-24" /></div>
      </div>
      <div v-else class="grid grid-cols-2 divide-parchment-100 sm:grid-cols-3">
        <div v-for="c in cities" :key="c.id" class="group flex items-center justify-between border-b border-parchment-100 px-4 py-3">
          <div>
            <div class="font-medium text-ink-900">{{ c.name }}</div>
            <div v-if="c.country" class="text-xs text-ink-700/50">{{ c.country }}</div>
          </div>
          <div v-if="auth.isStaff" class="flex gap-1 opacity-0 transition-opacity group-hover:opacity-100">
            <button class="text-xs text-saffron-600 hover:underline" @click="startEdit(c)">✎</button>
            <button class="text-ink-700/40 hover:text-red-600" @click="remove(c)">✕</button>
          </div>
        </div>
        <div v-if="!cities.length" class="col-span-full p-8 text-center text-ink-700/50">Городов пока нет</div>
      </div>
    </div>

    <div v-if="showForm" class="fixed inset-0 z-40 flex items-center justify-center bg-ink-900/40 p-4" @click.self="showForm = false">
      <div class="card w-full max-w-md p-6">
        <h3 class="mb-4 font-display text-2xl text-ink-900">{{ editing ? 'Изменить город' : 'Новый город' }}</h3>
        <form class="space-y-3" @submit.prevent="save">
          <div><label class="label">Название *</label><input v-model="form.name" class="input" required /></div>
          <div><label class="label">Страна</label><input v-model="form.country" class="input" placeholder="Россия" /></div>
          <p v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>
          <div class="flex gap-2 pt-2">
            <button class="btn-primary">Сохранить</button>
            <button type="button" class="btn-ghost" @click="showForm = false">Отмена</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
