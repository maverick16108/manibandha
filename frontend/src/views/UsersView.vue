<script setup>
import { ref, reactive, onMounted } from 'vue'
import client from '../api/client'
import { ROLE_LABELS } from '../lib/format'

const users = ref([])
const showForm = ref(false)
const editing = ref(null)
const error = ref('')
const form = reactive({ email: '', full_name: '', role: 'secretary', password: '', is_active: true })

async function load() {
  const { data } = await client.get('/users')
  users.value = data
}

function startNew() {
  editing.value = null
  Object.assign(form, { email: '', full_name: '', role: 'secretary', password: '', is_active: true })
  error.value = ''
  showForm.value = true
}

function startEdit(u) {
  editing.value = u.id
  Object.assign(form, { email: u.email, full_name: u.full_name, role: u.role, password: '', is_active: u.is_active })
  error.value = ''
  showForm.value = true
}

async function save() {
  error.value = ''
  try {
    if (editing.value) {
      const payload = { full_name: form.full_name, role: form.role, is_active: form.is_active }
      if (form.password) payload.password = form.password
      await client.patch(`/users/${editing.value}`, payload)
    } else {
      await client.post('/users', form)
    }
    showForm.value = false
    await load()
  } catch (e) {
    error.value = e.response?.data?.detail || 'Ошибка сохранения'
  }
}

async function remove(u) {
  if (!confirm(`Удалить пользователя ${u.full_name}?`)) return
  await client.delete(`/users/${u.id}`)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <div class="mb-6 flex items-center justify-between">
      <h1 class="font-display text-3xl font-semibold text-ink-900">Пользователи</h1>
      <button class="btn-primary" @click="startNew">+ Добавить</button>
    </div>

    <div class="card divide-y divide-parchment-100">
      <div v-for="u in users" :key="u.id" class="flex items-center justify-between p-4">
        <div>
          <div class="font-medium text-ink-900">{{ u.full_name }} <span v-if="!u.is_active" class="badge bg-red-100 text-red-700">отключён</span></div>
          <div class="text-sm text-ink-700/60">{{ u.email }} · {{ ROLE_LABELS[u.role] }}</div>
        </div>
        <div class="flex gap-2">
          <button class="btn-ghost" @click="startEdit(u)">Изменить</button>
          <button class="text-ink-700/40 hover:text-red-600" @click="remove(u)">✕</button>
        </div>
      </div>
    </div>

    <div v-if="showForm" class="fixed inset-0 z-40 flex items-center justify-center bg-ink-900/40 p-4" @click.self="showForm = false">
      <div class="card w-full max-w-lg p-6">
        <h3 class="mb-4 font-display text-2xl text-ink-900">{{ editing ? 'Изменить пользователя' : 'Новый пользователь' }}</h3>
        <form class="space-y-3" @submit.prevent="save">
          <div><label class="label">Имя *</label><input v-model="form.full_name" class="input" required /></div>
          <div v-if="!editing"><label class="label">Email *</label><input v-model="form.email" type="email" class="input" required /></div>
          <div class="grid grid-cols-2 gap-3">
            <div><label class="label">Роль</label>
              <select v-model="form.role" class="input">
                <option v-for="(l, k) in ROLE_LABELS" :key="k" :value="k">{{ l }}</option>
              </select>
            </div>
            <div><label class="label">{{ editing ? 'Новый пароль' : 'Пароль *' }}</label>
              <input v-model="form.password" type="password" class="input" :required="!editing" :placeholder="editing ? 'оставьте пустым' : ''" />
            </div>
          </div>
          <label class="flex items-center gap-2 text-sm text-ink-700"><input type="checkbox" v-model="form.is_active" /> Активен</label>
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
