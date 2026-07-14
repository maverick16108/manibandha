<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { RouterLink } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Форум')
const auth = useAuthStore()
const topics = ref([])
const loading = ref(true)

function fmt(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  const now = new Date()
  const same = d.toDateString() === now.toDateString()
  return same
    ? d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: '2-digit' })
}

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const { data } = await client.get('/forum/topics')
    topics.value = data
  } finally {
    loading.value = false
  }
}
let poll = null
onMounted(() => { load(); poll = setInterval(() => load(true), 20000) })
onBeforeUnmount(() => clearInterval(poll))
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <div class="mb-6 flex items-center justify-between gap-3">
      <p class="text-ink-700/60">Общение учеников — задавайте вопросы, делитесь опытом</p>
      <RouterLink v-if="auth.can('forum.post')" :to="{ name: 'forum-new' }" class="btn-primary shrink-0">+ Тема</RouterLink>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 4" :key="i" class="card space-y-2 p-4"><AppSkeleton w="w-64" /><AppSkeleton w="w-40" h="h-3" /></div>
    </div>

    <div v-else-if="!topics.length" class="card p-10 text-center text-ink-700/50">
      Пока нет тем. <RouterLink v-if="auth.can('forum.post')" :to="{ name: 'forum-new' }" class="text-saffron-700 hover:underline">Создайте первую</RouterLink>
    </div>

    <div v-else class="space-y-3">
      <RouterLink v-for="t in topics" :key="t.id" :to="{ name: 'forum-topic', params: { id: t.id } }"
                  class="card block p-4 transition hover:border-saffron-400/50 hover:shadow">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <AppIcon v-if="t.pinned" name="pin" :size="15" class="shrink-0 text-saffron-600" />
              <h3 class="truncate font-display text-lg font-semibold text-ink-900">{{ t.title }}</h3>
            </div>
            <div class="mt-0.5 text-sm text-ink-700/60">Автор: {{ t.author_name || 'Аноним' }}</div>
            <div v-if="t.last_post_preview" class="mt-1 truncate text-sm text-ink-700/70">
              <span class="font-medium text-ink-700">{{ t.last_post_author }}:</span> {{ t.last_post_preview }}
            </div>
          </div>
          <div class="shrink-0 text-right text-xs text-ink-700/50">
            <div class="inline-flex items-center gap-1"><AppIcon name="forum" :size="14" /> {{ t.posts_count }}</div>
            <div class="mt-1">{{ fmt(t.last_post_at || t.updated_at) }}</div>
          </div>
        </div>
      </RouterLink>
    </div>
  </div>
</template>
