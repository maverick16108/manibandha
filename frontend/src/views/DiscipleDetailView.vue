<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import { STATUS_LABELS, STATUS_ORDER, STATUS_BADGE, MARITAL_LABELS, formatDate } from '../lib/format'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const id = computed(() => route.params.id)

const d = ref(null)
const loading = ref(true)
const newItem = reactive({ title: '', target: 'harinama' })

async function load() {
  const { data } = await client.get(`/disciples/${id.value}`)
  d.value = data
}

async function addItem() {
  if (!newItem.title.trim()) return
  await client.post(`/disciples/${id.value}/checklist`, { title: newItem.title, target: newItem.target })
  newItem.title = ''
  await load()
}

async function toggleItem(item) {
  await client.patch(`/disciples/${id.value}/checklist/${item.id}`, { is_done: !item.is_done })
  await load()
}

async function removeItem(item) {
  await client.delete(`/disciples/${id.value}/checklist/${item.id}`)
  await load()
}

async function remove() {
  if (!confirm('Удалить ученика безвозвратно?')) return
  await client.delete(`/disciples/${id.value}`)
  router.push({ name: 'disciples' })
}

onMounted(async () => {
  try { await load() } finally { loading.value = false }
})
</script>

<template>
  <div v-if="loading" class="text-ink-700/60">Загрузка…</div>
  <div v-else-if="d" class="mx-auto max-w-4xl">
    <RouterLink :to="{ name: 'disciples' }" class="mb-4 inline-block text-sm text-saffron-600 hover:underline">← К списку</RouterLink>

    <!-- Header -->
    <div class="card mb-6 p-6">
      <div class="flex flex-wrap items-start gap-6">
        <img v-if="d.photo_url" :src="d.photo_url" class="photo-bw h-28 w-28 rounded-xl object-cover" />
        <div v-else class="flex h-28 w-28 items-center justify-center rounded-xl bg-parchment-200 text-4xl text-ink-700">
          {{ (d.spiritual_name || d.material_name || '?')[0] }}
        </div>
        <div class="flex-1">
          <h1 class="font-display text-3xl font-semibold text-ink-900">{{ d.spiritual_name || d.material_name }}</h1>
          <p v-if="d.spiritual_name" class="text-ink-700/70">{{ d.material_name }}</p>
          <div class="mt-3 flex flex-wrap items-center gap-2">
            <span class="badge" :class="STATUS_BADGE[d.initiation_status]">{{ STATUS_LABELS[d.initiation_status] }}</span>
            <span v-if="d.ready_for_initiation" class="badge bg-saffron-500/15 text-saffron-700">Готов к инициации</span>
          </div>
        </div>
        <div v-if="auth.canEdit" class="flex gap-2">
          <RouterLink :to="{ name: 'disciple-edit', params: { id: d.id } }" class="btn-outline">Редактировать</RouterLink>
          <button v-if="auth.isStaff" class="btn border border-red-200 text-red-600 hover:bg-red-50" @click="remove">Удалить</button>
        </div>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-2">
      <!-- Info -->
      <div class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Данные</h3>
        <dl class="space-y-2 text-sm">
          <div class="flex justify-between"><dt class="text-ink-700/60">Телефон</dt><dd class="text-ink-800">{{ d.phone || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Email</dt><dd class="text-ink-800">{{ d.email || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Мессенджер</dt><dd class="text-ink-800">{{ d.messenger || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Страна / город</dt><dd class="text-ink-800">{{ d.country || '—' }}<span v-if="d.city">, {{ d.city }}</span></dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Храм</dt><dd class="text-ink-800">{{ d.temple?.name || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Семейное положение</dt><dd class="text-ink-800">{{ MARITAL_LABELS[d.marital_status] || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Дата рождения</dt><dd class="text-ink-800">{{ formatDate(d.date_of_birth) }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Наставник</dt><dd class="text-ink-800">{{ d.mentor?.full_name || '—' }}</dd></div>
        </dl>
      </div>

      <!-- Initiation + service -->
      <div class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Инициация и служение</h3>
        <dl class="space-y-2 text-sm">
          <div class="flex justify-between"><dt class="text-ink-700/60">Харинама</dt><dd class="text-ink-800">{{ formatDate(d.harinama_date) }}<span v-if="d.harinama_name"> · {{ d.harinama_name }}</span></dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Вторая инициация</dt><dd class="text-ink-800">{{ formatDate(d.brahman_date) }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Кто рекомендовал</dt><dd class="text-ink-800">{{ d.recommended_by || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Дата заявки</dt><dd class="text-ink-800">{{ formatDate(d.application_date) }}</dd></div>
        </dl>
        <div v-if="d.seva" class="mt-4"><div class="label">Севы</div><p class="text-sm text-ink-700">{{ d.seva }}</p></div>
        <div v-if="d.current_activity" class="mt-3"><div class="label">Деятельность</div><p class="text-sm text-ink-700">{{ d.current_activity }}</p></div>
        <div v-if="d.notes" class="mt-3"><div class="label">Примечания</div><p class="text-sm text-ink-700">{{ d.notes }}</p></div>
      </div>
    </div>

    <!-- Pipeline checklist -->
    <div class="card mt-6 p-6">
      <h3 class="mb-4 font-display text-xl text-ink-900">Путь аспиранта — чек-лист требований</h3>
      <ul class="space-y-2">
        <li v-for="item in d.checklist" :key="item.id" class="flex items-center gap-3 rounded-lg border border-parchment-200 px-3 py-2">
          <input type="checkbox" :checked="item.is_done" :disabled="!auth.canEdit" @change="toggleItem(item)" />
          <span class="flex-1 text-sm" :class="item.is_done ? 'text-ink-700/50 line-through' : 'text-ink-800'">{{ item.title }}</span>
          <span class="badge bg-parchment-200 text-ink-700">{{ STATUS_LABELS[item.target] }}</span>
          <button v-if="auth.isStaff" class="text-ink-700/40 hover:text-red-600" @click="removeItem(item)">✕</button>
        </li>
        <li v-if="!d.checklist.length" class="text-sm text-ink-700/50">Пунктов пока нет</li>
      </ul>

      <form v-if="auth.canEdit" class="mt-4 flex flex-wrap gap-2" @submit.prevent="addItem">
        <input v-model="newItem.title" class="input flex-1" placeholder="Новое требование (напр. «Стаж 1 год», «Обеты»)" />
        <select v-model="newItem.target" class="input w-40">
          <option v-for="s in STATUS_ORDER" :key="s" :value="s">{{ STATUS_LABELS[s] }}</option>
        </select>
        <button class="btn-primary">Добавить</button>
      </form>
    </div>
  </div>
</template>
