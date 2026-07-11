<script setup>
import { ref, computed, onMounted } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import { confirmDialog } from '../composables/confirm'
import AppSkeleton from '../components/AppSkeleton.vue'
import { renderMarkdown } from '../lib/markdown'
import { formatDate } from '../lib/format'

const auth = useAuthStore()
const router = useRouter()
const events = ref([])
const loading = ref(true)

function todayStr() {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}
const today = todayStr()
function isNow(e) {
  return e.starts_on <= today && (e.ends_on || e.starts_on) >= today
}
function dateRange(e) {
  return e.ends_on && e.ends_on !== e.starts_on ? `${formatDate(e.starts_on)} — ${formatDate(e.ends_on)}` : formatDate(e.starts_on)
}
const current = computed(() => events.value.find(isNow))

async function load() {
  loading.value = true
  try {
    const { data } = await client.get('/events')
    events.value = data
  } finally {
    loading.value = false
  }
}
async function remove(e) {
  if (!(await confirmDialog({ message: `Удалить событие «${e.title}»?` }))) return
  await client.delete(`/events/${e.id}`)
  await load()
}
onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-5xl">
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h1 class="font-display text-3xl font-semibold text-ink-900">Календарь событий</h1>
        <p class="text-ink-700/60">Где находится гуру и что происходит</p>
      </div>
      <RouterLink v-if="auth.isStaff" :to="{ name: 'event-new' }" class="btn-primary">+ Событие</RouterLink>
    </div>

    <!-- current location banner -->
    <div v-if="current" class="card mb-6 border-saffron-400/50 bg-saffron-500/5 p-5">
      <div class="mb-1 text-sm uppercase tracking-wide text-saffron-700">Сейчас</div>
      <div class="font-display text-2xl text-ink-900">{{ current.title }}</div>
      <div v-if="current.location" class="text-ink-700">📍 {{ current.location }} · {{ dateRange(current) }}</div>
    </div>

    <div v-if="loading" class="space-y-4">
      <div v-for="i in 3" :key="i" class="card space-y-3 p-5"><AppSkeleton w="w-56" h="h-6" /><AppSkeleton /><AppSkeleton w="w-2/3" /></div>
    </div>

    <div v-else class="space-y-4">
      <div v-for="e in events" :key="e.id" class="card p-5" :class="isNow(e) && 'border-saffron-400/50'">
        <div class="flex items-start justify-between gap-3">
          <div>
            <div class="flex flex-wrap items-center gap-2">
              <h3 class="font-display text-xl font-semibold text-ink-900">{{ e.title }}</h3>
              <span v-if="isNow(e)" class="badge bg-saffron-500 text-white">Сейчас</span>
            </div>
            <div class="mt-0.5 text-sm text-ink-700/70">
              <span v-if="e.location">📍 {{ e.location }} · </span>{{ dateRange(e) }}
            </div>
          </div>
          <div v-if="auth.isStaff" class="flex shrink-0 gap-2">
            <RouterLink :to="{ name: 'event-edit', params: { id: e.id } }" class="btn-ghost">Изменить</RouterLink>
            <button class="text-ink-700/40 hover:text-red-600" @click="remove(e)">✕</button>
          </div>
        </div>
        <div v-if="e.description" class="markdown-body mt-3 text-ink-700" v-html="renderMarkdown(e.description)"></div>
      </div>
      <div v-if="!events.length" class="card p-8 text-center text-ink-700/50">Событий пока нет</div>
    </div>
  </div>
</template>

<style scoped>
.markdown-body :deep(a) { text-decoration: underline; color: #a85e1f; }
.markdown-body :deep(img) { max-height: 22rem; border-radius: 0.5rem; margin: 0.5rem 0; }
.markdown-body :deep(ul) { list-style: disc; margin-left: 1.25rem; }
</style>
