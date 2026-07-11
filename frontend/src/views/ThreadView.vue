<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppSkeleton from '../components/AppSkeleton.vue'
import { renderMarkdown } from '../lib/markdown'

const route = useRoute()
const auth = useAuthStore()
const id = computed(() => route.params.id)

const thread = ref(null)
const loading = ref(true)
const body = ref('')
const sending = ref(false)
const uploading = ref(false)
const scroller = ref(null)
const textarea = ref(null)
const fileInput = ref(null)

// realtime
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
const backTo = computed(() => (thread.value?.kind === 'report' ? { name: 'service-reports' } : { name: 'questions' }))
const canLike = computed(() => auth.isGuru || auth.user?.role === 'curator')

function fmtTime(iso) {
  return new Date(iso).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
}

async function scrollDown() {
  await nextTick()
  if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
}

async function load() {
  const { data } = await client.get(`/threads/${id.value}`)
  thread.value = data
  await scrollDown()
}

function connectWs() {
  const token = auth.token
  if (!token) return
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/api/ws/threads/${id.value}?token=${encodeURIComponent(token)}`)
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

async function send() {
  const text = body.value.trim()
  if (!text) return
  sending.value = true
  try {
    if (ws && ws.readyState === 1) {
      ws.send(JSON.stringify({ type: 'message', body: text }))
      body.value = '' // message will arrive back via broadcast
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

// --- markdown toolbar ---
function wrap(before, after = before, placeholder = '') {
  const el = textarea.value
  const start = el?.selectionStart ?? body.value.length
  const end = el?.selectionEnd ?? body.value.length
  const sel = body.value.slice(start, end) || placeholder
  body.value = body.value.slice(0, start) + before + sel + after + body.value.slice(end)
  nextTick(() => {
    el.focus()
    el.selectionStart = start + before.length
    el.selectionEnd = start + before.length + sel.length
  })
}
function insert(text) {
  const el = textarea.value
  const pos = el?.selectionStart ?? body.value.length
  body.value = body.value.slice(0, pos) + text + body.value.slice(pos)
  nextTick(() => { el.focus(); el.selectionStart = el.selectionEnd = pos + text.length })
}

async function onFiles(e) {
  const files = Array.from(e.target.files || [])
  if (!files.length) return
  uploading.value = true
  try {
    const fd = new FormData()
    files.forEach((f) => fd.append('files', f))
    const { data } = await client.post('/uploads', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    data.urls.forEach((u) => insert(`\n![](${u})\n`))
  } finally {
    uploading.value = false
    if (fileInput.value) fileInput.value.value = ''
  }
}

onMounted(async () => {
  try { await load(); connectWs() } finally { loading.value = false }
})
onBeforeUnmount(() => { if (ws) ws.close(); clearTimeout(typingTimer) })
</script>

<template>
  <div class="mx-auto flex h-[calc(100dvh-6rem)] max-w-6xl flex-col lg:h-[calc(100dvh-8rem)]">
    <div v-if="loading" class="space-y-4">
      <AppSkeleton w="w-40" h="h-9" />
      <div class="card space-y-4 p-6"><AppSkeleton v-for="i in 4" :key="i" h="h-10" /></div>
    </div>

    <template v-else-if="thread">
      <!-- header (always visible) -->
      <div class="mb-3 flex shrink-0 flex-wrap items-center gap-2">
        <h1 class="font-display text-2xl font-semibold text-ink-900">
          {{ thread.kind === 'report' ? 'Отчёт' : 'Вопрос' }} · {{ thread.disciple_name }}
        </h1>
        <span v-if="thread.period" class="badge bg-saffron-500/15 text-saffron-700">{{ periodLabel }}</span>
      </div>

      <!-- messages (fills remaining height) -->
      <div ref="scroller" class="card flex-1 space-y-3 overflow-y-auto p-5">
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

      <!-- composer -->
      <div class="mt-3 shrink-0">
        <div class="mb-1 h-5 text-sm text-saffron-700/80">
          <span v-if="typingName">{{ typingName }} печатает…</span>
        </div>
        <div class="mb-1.5 flex flex-wrap items-center gap-1">
          <button type="button" class="composer-btn font-bold" title="Жирный (Ctrl+B)" @click="wrap('**', '**', 'текст')">B</button>
          <button type="button" class="composer-btn italic" title="Курсив" @click="wrap('*', '*', 'текст')">I</button>
          <button type="button" class="composer-btn line-through" title="Зачёркнутый" @click="wrap('~~', '~~', 'текст')">S</button>
          <button type="button" class="composer-btn font-mono" title="Код" @click="wrap('`', '`', 'код')">&lt;/&gt;</button>
          <button type="button" class="composer-btn" title="Список" @click="insert('\n- ')">• Список</button>
          <button type="button" class="composer-btn" title="Ссылка" @click="wrap('[', '](https://)', 'текст')">🔗</button>
          <button type="button" class="composer-btn" title="Картинка" :disabled="uploading" @click="fileInput.click()">
            {{ uploading ? '…' : '🖼 Картинка' }}
          </button>
          <input ref="fileInput" type="file" accept="image/*" multiple class="hidden" @change="onFiles" />
        </div>
        <div class="flex items-end gap-2">
          <textarea ref="textarea" v-model="body" rows="3" class="input flex-1 resize-y"
                    placeholder="Написать сообщение… (поддерживается **markdown**)"
                    @input="notifyTyping" @keydown.enter.exact.prevent="send"></textarea>
          <button class="btn-primary" :disabled="sending || !body.trim()" @click="send">{{ sending ? '…' : 'Отправить' }}</button>
        </div>
        <p class="mt-1 text-xs text-ink-700/40">Enter — отправить · Shift+Enter — новая строка</p>
      </div>
    </template>
  </div>
</template>

<style scoped>
.composer-btn {
  @apply rounded-md border border-parchment-300 bg-white px-2 py-1 text-sm text-ink-700 transition-colors hover:bg-parchment-100 disabled:opacity-50;
}
.markdown-body :deep(a) { text-decoration: underline; }
.markdown-body :deep(ul) { margin: 0.25rem 0; }
</style>
