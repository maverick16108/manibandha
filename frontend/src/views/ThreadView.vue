<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppSkeleton from '../components/AppSkeleton.vue'
import AppIcon from '../components/AppIcon.vue'

const route = useRoute()
const auth = useAuthStore()
const id = computed(() => route.params.id)

const thread = ref(null)
const loading = ref(true)
const body = ref('')
const sending = ref(false)
const listEnd = ref(null)

const MONTHS = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']
const periodLabel = computed(() => {
  const p = thread.value?.period
  if (!p) return ''
  const [y, m] = p.split('-')
  return `${MONTHS[+m - 1]} ${y}`
})
const backTo = computed(() => (thread.value?.kind === 'report' ? { name: 'service-reports' } : { name: 'questions' }))
const canLike = computed(() => auth.isGuru || auth.user?.role === 'curator')

async function toggleLike(m) {
  if (!canLike.value) return
  const { data } = await client.post(`/threads/${id.value}/messages/${m.id}/like`)
  m.likes = data.likes
  m.liked = data.liked
}

function fmtTime(iso) {
  const d = new Date(iso)
  return d.toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
}

async function load() {
  const { data } = await client.get(`/threads/${id.value}`)
  thread.value = data
  await nextTick()
  listEnd.value?.scrollIntoView({ behavior: 'smooth' })
}

async function send() {
  if (!body.value.trim()) return
  sending.value = true
  try {
    await client.post(`/threads/${id.value}/messages`, { body: body.value.trim() })
    body.value = ''
    await load()
  } finally {
    sending.value = false
  }
}

onMounted(async () => {
  try { await load() } finally { loading.value = false }
})
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <div v-if="loading" class="space-y-4">
      <AppSkeleton w="w-40" h="h-9" />
      <div class="card space-y-4 p-6"><AppSkeleton v-for="i in 4" :key="i" h="h-10" /></div>
    </div>

    <div v-else-if="thread">
      <RouterLink :to="backTo" class="btn-outline mb-4">
        <AppIcon name="chevron" :size="16" class="rotate-90" /> Назад
      </RouterLink>

      <div class="card mb-4 p-5">
        <div class="flex flex-wrap items-center gap-2">
          <h1 class="font-display text-2xl font-semibold text-ink-900">
            {{ thread.kind === 'report' ? 'Отчёт' : 'Вопрос' }} · {{ thread.disciple_name }}
          </h1>
          <span v-if="thread.period" class="badge bg-saffron-500/15 text-saffron-700">{{ periodLabel }}</span>
        </div>
        <p v-if="thread.subject" class="mt-1 text-ink-700">{{ thread.subject }}</p>
      </div>

      <!-- messages -->
      <div class="card mb-4 space-y-3 p-5">
        <div v-for="m in thread.messages" :key="m.id"
             class="flex flex-col" :class="m.author_id === auth.user?.id ? 'items-end' : 'items-start'">
          <div class="max-w-[85%] rounded-2xl px-4 py-2.5"
               :class="m.author_id === auth.user?.id ? 'bg-saffron-500 text-white' : 'bg-parchment-100 text-ink-800'">
            <div class="mb-0.5 text-xs opacity-70">{{ m.author_name || 'Аноним' }} · {{ fmtTime(m.created_at) }}</div>
            <div class="whitespace-pre-wrap">{{ m.body }}</div>
          </div>
          <button v-if="thread.kind === 'report'"
                  class="mt-1 flex items-center gap-1 rounded-full px-2 py-0.5 text-sm transition-colors"
                  :class="[m.liked ? 'text-red-500' : 'text-ink-700/40', canLike ? 'cursor-pointer hover:bg-parchment-100' : 'cursor-default']"
                  :disabled="!canLike" @click="toggleLike(m)">
            <span>{{ m.liked ? '❤' : '♡' }}</span><span v-if="m.likes" class="text-xs">{{ m.likes }}</span>
          </button>
        </div>
        <div v-if="!thread.messages.length" class="text-center text-sm text-ink-700/50">Сообщений пока нет</div>
        <div ref="listEnd"></div>
      </div>

      <!-- composer -->
      <form class="flex items-end gap-2" @submit.prevent="send">
        <textarea v-model="body" rows="2" class="input flex-1 resize-y" placeholder="Написать сообщение…"
                  @keydown.enter.exact.prevent="send"></textarea>
        <button class="btn-primary" :disabled="sending || !body.trim()">{{ sending ? '…' : 'Отправить' }}</button>
      </form>
    </div>
  </div>
</template>
