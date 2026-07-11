<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import client from '../api/client'
import AppSelect from '../components/AppSelect.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { confirmDialog } from '../composables/confirm'
import { ROLE_LABELS } from '../lib/format'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Пользователи')

const roleOptions = Object.entries(ROLE_LABELS).map(([value, label]) => ({ value, label }))
const loading = ref(true)
const nameInput = ref(null)
function focusName() { nextTick(() => nameInput.value?.focus()) }

const users = ref([])
const showForm = ref(false)
const editing = ref(null)
const error = ref('')
const form = reactive({ email: '', full_name: '', role: 'secretary', password: '', is_active: true, disciple_id: '' })
const disciples = ref([])
const discipleOptions = computed(() => [{ value: '', label: '— не привязан —' }, ...disciples.value.map((d) => ({ value: d.id, label: d.spiritual_name || d.material_name }))])

async function load() {
  loading.value = true
  try {
    const [u, d] = await Promise.all([client.get('/users'), client.get('/disciples', { params: { limit: 500 } })])
    users.value = u.data
    disciples.value = d.data.items
  } finally {
    loading.value = false
  }
}

function startNew() {
  editing.value = null
  Object.assign(form, { email: '', full_name: '', role: 'secretary', password: '', is_active: true, disciple_id: '' })
  error.value = ''
  showForm.value = true
  focusName()
}

function startEdit(u) {
  editing.value = u.id
  Object.assign(form, { email: u.email, full_name: u.full_name, role: u.role, password: '', is_active: u.is_active, disciple_id: u.disciple_id ?? '' })
  error.value = ''
  showForm.value = true
  focusName()
}

function onKey(e) { if (e.key === 'Escape' && showForm.value) showForm.value = false }
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))

async function save() {
  error.value = ''
  try {
    const discipleId = form.disciple_id || null
    if (editing.value) {
      const payload = { full_name: form.full_name, role: form.role, is_active: form.is_active, disciple_id: discipleId }
      if (form.password) payload.password = form.password
      await client.patch(`/users/${editing.value}`, payload)
    } else {
      await client.post('/users', { ...form, disciple_id: discipleId })
    }
    showForm.value = false
    await load()
  } catch (e) {
    error.value = e.response?.data?.detail || 'Ошибка сохранения'
  }
}

async function remove(u) {
  if (!(await confirmDialog({ message: `Удалить пользователя ${u.full_name}?` }))) return
  await client.delete(`/users/${u.id}`)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <div class="mb-6 flex items-center justify-between">
      <button class="btn-primary" @click="startNew">+ Добавить</button>
    </div>

    <div class="card divide-y divide-parchment-100">
      <template v-if="loading">
        <div v-for="i in 4" :key="'s' + i" class="flex items-center justify-between p-4">
          <div class="space-y-2"><AppSkeleton w="w-40" /><AppSkeleton w="w-56" h="h-3" /></div>
          <AppSkeleton w="w-20" h="h-8" />
        </div>
      </template>
      <div v-for="u in users" :key="u.id" v-show="!loading" class="flex items-center justify-between p-4">
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
          <div><label class="label">Имя *</label><input ref="nameInput" v-model="form.full_name" class="input" required /></div>
          <div v-if="!editing"><label class="label">Email *</label><input v-model="form.email" type="email" class="input" required /></div>
          <div class="grid grid-cols-2 gap-3">
            <div><label class="label">Роль</label>
              <AppSelect v-model="form.role" :options="roleOptions" />
            </div>
            <div><label class="label">{{ editing ? 'Новый пароль' : 'Пароль *' }}</label>
              <input v-model="form.password" type="password" class="input" :required="!editing" :placeholder="editing ? 'оставьте пустым' : ''" />
            </div>
          </div>
          <div v-if="form.role === 'student'">
            <label class="label">Анкета ученика</label>
            <AppSelect v-model="form.disciple_id" :options="discipleOptions" placeholder="— не привязан —" />
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
