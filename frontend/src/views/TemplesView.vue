<script setup>
import { ref, reactive, onMounted, onBeforeUnmount, nextTick } from 'vue'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppSkeleton from '../components/AppSkeleton.vue'

const auth = useAuthStore()
const temples = ref([])
const loading = ref(true)
const editing = ref(null)
const form = reactive({ name: '', city: '', country: '', president_name: '', notes: '' })
const showForm = ref(false)
const nameInput = ref(null)
function focusName() { nextTick(() => nameInput.value?.focus()) }
function onKey(e) { if (e.key === 'Escape' && showForm.value) showForm.value = false }
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))

async function load() {
  loading.value = true
  try {
    const { data } = await client.get('/temples')
    temples.value = data
  } finally {
    loading.value = false
  }
}

function startNew() {
  editing.value = null
  Object.keys(form).forEach((k) => (form[k] = ''))
  showForm.value = true
  focusName()
}

function startEdit(t) {
  editing.value = t.id
  Object.keys(form).forEach((k) => (form[k] = t[k] ?? ''))
  showForm.value = true
  focusName()
}

async function save() {
  const payload = { ...form }
  if (editing.value) await client.patch(`/temples/${editing.value}`, payload)
  else await client.post('/temples', payload)
  showForm.value = false
  await load()
}

async function remove(t) {
  if (!confirm(`Удалить храм «${t.name}»?`)) return
  await client.delete(`/temples/${t.id}`)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <div class="mb-6 flex items-center justify-between">
      <h1 class="font-display text-3xl font-semibold text-ink-900">Храмы и общины</h1>
      <button v-if="auth.isStaff" class="btn-primary" @click="startNew">+ Добавить</button>
    </div>

    <div class="card divide-y divide-parchment-100">
      <template v-if="loading">
        <div v-for="i in 4" :key="'s' + i" class="flex items-center justify-between p-4">
          <div class="space-y-2"><AppSkeleton w="w-40" /><AppSkeleton w="w-56" h="h-3" /></div>
        </div>
      </template>
      <div v-for="t in temples" :key="t.id" v-show="!loading" class="flex items-center justify-between p-4">
        <div>
          <div class="font-medium text-ink-900">{{ t.name }}</div>
          <div class="text-sm text-ink-700/60">
            {{ [t.city, t.country].filter(Boolean).join(', ') || '—' }}
            <span v-if="t.president_name"> · Президент: {{ t.president_name }}</span>
          </div>
        </div>
        <div v-if="auth.isStaff" class="flex gap-2">
          <button class="btn-ghost" @click="startEdit(t)">Изменить</button>
          <button class="text-ink-700/40 hover:text-red-600" @click="remove(t)">✕</button>
        </div>
      </div>
      <div v-if="!temples.length" class="p-8 text-center text-ink-700/50">Храмов пока нет</div>
    </div>

    <!-- Modal-ish form -->
    <div v-if="showForm" class="fixed inset-0 z-40 flex items-center justify-center bg-ink-900/40 p-4" @click.self="showForm = false">
      <div class="card w-full max-w-lg p-6">
        <h3 class="mb-4 font-display text-2xl text-ink-900">{{ editing ? 'Изменить храм' : 'Новый храм' }}</h3>
        <form class="space-y-3" @submit.prevent="save">
          <div><label class="label">Название *</label><input ref="nameInput" v-model="form.name" class="input" required /></div>
          <div class="grid grid-cols-2 gap-3">
            <div><label class="label">Город</label><input v-model="form.city" class="input" /></div>
            <div><label class="label">Страна</label><input v-model="form.country" class="input" /></div>
          </div>
          <div><label class="label">Президент храма</label><input v-model="form.president_name" class="input" /></div>
          <div><label class="label">Примечания</label><textarea v-model="form.notes" rows="2" class="input"></textarea></div>
          <div class="flex gap-2 pt-2">
            <button class="btn-primary">Сохранить</button>
            <button type="button" class="btn-ghost" @click="showForm = false">Отмена</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
