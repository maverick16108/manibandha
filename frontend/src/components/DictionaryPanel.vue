<script setup>
import { ref, reactive, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import { confirmDialog } from '../composables/confirm'
import AppSkeleton from './AppSkeleton.vue'

const nameInput = ref(null)
function focusName() {
  nextTick(() => nameInput.value?.focus())
}
function onKey(e) {
  if (e.key === 'Escape' && showForm.value) showForm.value = false
}
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))

const props = defineProps({
  endpoint: { type: String, required: true }, // e.g. '/cities'
  withCountry: { type: Boolean, default: false },
  emptyText: { type: String, default: 'Пока пусто' },
})

const auth = useAuthStore()
const items = ref([])
const loading = ref(true)
const showForm = ref(false)
const editing = ref(null)
const error = ref('')
const form = reactive({ name: '', country: '' })

async function load() {
  loading.value = true
  try {
    const { data } = await client.get(props.endpoint)
    items.value = data
  } finally {
    loading.value = false
  }
}
watch(() => props.endpoint, load)

function startNew() {
  editing.value = null
  Object.assign(form, { name: '', country: '' })
  error.value = ''
  showForm.value = true
  focusName()
}
function startEdit(it) {
  editing.value = it.id
  Object.assign(form, { name: it.name, country: it.country ?? '' })
  error.value = ''
  showForm.value = true
  focusName()
}
async function save() {
  error.value = ''
  const payload = props.withCountry ? { name: form.name, country: form.country } : { name: form.name }
  try {
    if (editing.value) await client.patch(`${props.endpoint}/${editing.value}`, payload)
    else await client.post(props.endpoint, payload)
    showForm.value = false
    await load()
  } catch (e) {
    error.value = e.response?.data?.detail || 'Ошибка сохранения'
  }
}
async function remove(it) {
  if (!(await confirmDialog({ message: `Удалить «${it.name}»?` }))) return
  await client.delete(`${props.endpoint}/${it.id}`)
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <div class="mb-4 flex items-center justify-between">
      <span class="text-sm text-ink-700/60">Всего: {{ items.length }}</span>
      <button v-if="auth.isStaff" class="btn-primary" @click="startNew">+ Добавить</button>
    </div>

    <div class="card">
      <div v-if="loading" class="grid grid-cols-2 gap-px p-4 sm:grid-cols-3">
        <AppSkeleton v-for="i in 9" :key="i" w="w-24" />
      </div>
      <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <div v-for="it in items" :key="it.id"
             class="group flex items-center justify-between border-b border-parchment-100 px-4 py-3">
          <div>
            <div class="font-medium text-ink-900">{{ it.name }}</div>
            <div v-if="withCountry && it.country" class="text-xs text-ink-700/50">{{ it.country }}</div>
          </div>
          <div v-if="auth.isStaff" class="flex gap-2 opacity-0 transition-opacity group-hover:opacity-100">
            <button class="text-xs text-saffron-600 hover:underline" @click="startEdit(it)">✎</button>
            <button class="text-ink-700/40 hover:text-red-600" @click="remove(it)">✕</button>
          </div>
        </div>
        <div v-if="!items.length" class="col-span-full p-8 text-center text-ink-700/50">{{ emptyText }}</div>
      </div>
    </div>

    <div v-if="showForm" class="fixed inset-0 z-40 flex items-center justify-center bg-ink-900/40 p-4" @click.self="showForm = false">
      <div class="card w-full max-w-md p-6">
        <h3 class="mb-4 font-display text-2xl text-ink-900">{{ editing ? 'Изменить' : 'Добавить' }}</h3>
        <form class="space-y-3" @submit.prevent="save">
          <div><label class="label">Название *</label><input ref="nameInput" v-model="form.name" class="input" required /></div>
          <div v-if="withCountry"><label class="label">Страна</label><input v-model="form.country" class="input" placeholder="Россия" /></div>
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
