<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppSkeleton from '../components/AppSkeleton.vue'
import MarkdownEditor from '../components/MarkdownEditor.vue'
import { renderMarkdown } from '../lib/markdown'
import { usePageTitle } from '../composables/pageTitle'

const route = useRoute()
const auth = useAuthStore()
const id = computed(() => route.params.id)

const thread = ref(null)
const loading = ref(true)
const body = ref('')
const sending = ref(false)
const scroller = ref(null)
const atBottom = ref(true)
let resizeObs = null
function onScroll() {
  const el = scroller.value
  if (el) atBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 48
}

let ws = null
const typingName = ref('')
let typingTimer = null
let lastTypingSent = 0

const MONTHS = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']
const periodLabel = computed(() => {
  const p = thread.value?.period
  if (!p) return ''
  const [y, m] = p.split('-')
  return `${MONTHS[+m - 1]} ${y}`
})
const canLike = computed(() => auth.isGuru || auth.user?.role === 'curator')

usePageTitle(() => {
  const t = thread.value
  if (!t) return ''
  const head = t.kind === 'report' ? 'Отчёт' : (t.subject || 'Вопрос')
  return `${head} · ${t.disciple_name}`
})

function fmtTime(iso) {
  return new Date(iso).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
}
async function scrollDown() {
  await nextTick()
  // rAF x2 — дождаться финальной раскладки (высота flex-1 контейнера), иначе не долистывает
  requestAnimationFrame(() => requestAnimationFrame(() => {
    const el = scroller.value
    if (el) el.scrollTop = el.scrollHeight
  }))
}
async function load() {
  const { data } = await client.get(`/threads/${id.value}`)
  thread.value = data
  await scrollDown()
}

function connectWs() {
  if (!auth.token) return
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/api/ws/threads/${id.value}?token=${encodeURIComponent(auth.token)}`)
  ws.onmessage = (ev) => {
    const data = JSON.parse(ev.data)
    if (data.type === 'message') {
      if (!thread.value.messages.some((m) => m.id === data.message.id)) {
        thread.value.messages.push(data.message)
        scrollDown()
      }
    } else if (data.type === 'typing' && data.user_id !== auth.user?.id) {
      typingName.value = data.name
      clearTimeout(typingTimer)
      typingTimer = setTimeout(() => (typingName.value = ''), 2500)
    }
  }
  ws.onclose = () => { ws = null }
}
function notifyTyping() {
  const now = Date.now()
  if (ws && ws.readyState === 1 && now - lastTypingSent > 1500) {
    lastTypingSent = now
    ws.send(JSON.stringify({ type: 'typing' }))
  }
}
watch(body, (v) => { if (v) notifyTyping() })

async function send() {
  const text = body.value.trim()
  if (!text) return
  sending.value = true
  try {
    if (ws && ws.readyState === 1) {
      ws.send(JSON.stringify({ type: 'message', body: text }))
      body.value = ''
    } else {
      await client.post(`/threads/${id.value}/messages`, { body: text })
      body.value = ''
      await load()
    }
  } finally {
    sending.value = false
  }
}
async function toggleLike(m) {
  if (!canLike.value) return
  const { data } = await client.post(`/threads/${id.value}/messages/${m.id}/like`)
  m.likes = data.likes
  m.liked = data.liked
}

onMounted(async () => {
  try { await load(); connectWs() } finally { loading.value = false }
  await nextTick()
  const el = scroller.value
  if (el) {
    // при изменении высоты скроллера (поле ввода расширилось) — держим низ, если были внизу
    resizeObs = new ResizeObserver(() => {
      if (atBottom.value && scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
    })
    resizeObs.observe(el)
  }
})
onBeforeUnmount(() => { if (ws) ws.close(); clearTimeout(typingTimer); if (resizeObs) resizeObs.disconnect() })
</script>

<template>
  <div class="mx-auto flex h-[calc(100dvh-6rem)] max-w-6xl flex-col -mb-4 sm:-mb-6 lg:-mb-8">
    <div v-if="loading" class="space-y-4">
      <AppSkeleton w="w-40" h="h-9" />
      <div class="card space-y-4 p-6"><AppSkeleton v-for="i in 4" :key="i" h="h-10" /></div>
    </div>

    <template v-else-if="thread">
      <div v-if="thread.period" class="mb-2 flex shrink-0 flex-wrap items-center gap-2">
        <span class="badge bg-saffron-500/15 text-saffron-700">{{ periodLabel }}</span>
      </div>

      <div ref="scroller" class="card flex-1 space-y-3 overflow-y-auto p-5" @scroll="onScroll">
        <div v-for="m in thread.messages" :key="m.id"
             class="flex flex-col" :class="m.author_id === auth.user?.id ? 'items-end' : 'items-start'">
          <div class="max-w-[85%] rounded-2xl px-4 py-2.5"
               :class="m.author_id === auth.user?.id ? 'bg-saffron-500 text-white' : 'bg-parchment-100 text-ink-800'">
            <div class="mb-0.5 text-xs opacity-70">{{ m.author_name || 'Аноним' }} · {{ fmtTime(m.created_at) }}</div>
            <div class="markdown-body break-words" v-html="renderMarkdown(m.body)"></div>
          </div>
          <button v-if="thread.kind === 'report'"
                  class="mt-1 flex items-center gap-1 rounded-full px-2 py-0.5 text-sm transition-colors"
                  :class="[m.liked ? 'text-red-500' : 'text-ink-700/40', canLike ? 'cursor-pointer hover:bg-parchment-100' : 'cursor-default']"
                  :disabled="!canLike" @click="toggleLike(m)">
            <span>{{ m.liked ? '❤' : '♡' }}</span><span v-if="m.likes" class="text-xs">{{ m.likes }}</span>
          </button>
        </div>
        <div v-if="!thread.messages.length" class="text-center text-sm text-ink-700/50">Сообщений пока нет</div>
      </div>

      <div class="mt-2 shrink-0">
        <div class="h-5 text-sm text-saffron-700/80"><span v-if="typingName">{{ typingName }} печатает…</span></div>
        <MarkdownEditor v-model="body" :rows="3" submit-on-enter type-anywhere :draft-scope="`thread:${id}`" placeholder="Написать сообщение…" @submit="send" />
        <div class="mt-2 flex justify-end">
          <button class="btn-primary" :disabled="sending || !body.trim()" @click="send">{{ sending ? '…' : 'Отправить' }}</button>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.markdown-body :deep(a) { text-decoration: underline; }
.markdown-body :deep(ul) { margin: 0.25rem 0; }
.markdown-body :deep(img) { max-height: 18rem; border-radius: 0.5rem; }
</style>
