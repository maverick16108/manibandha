<script setup>
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppSkeleton from '../components/AppSkeleton.vue'
import AppIcon from '../components/AppIcon.vue'
import AudioBar from '../components/AudioBar.vue'
import MarkdownEditor from '../components/MarkdownEditor.vue'
import { renderMarkdown } from '../lib/markdown'
import { extractImageUrls, preloadImages } from '../lib/preload'
import { usePageTitle } from '../composables/pageTitle'
import { backTarget } from '../composables/backTarget'
import { confirmDialog } from '../composables/confirm'
import { player, playAudio, seek, closePlayer } from '../composables/audioPlayer'

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
  if (ctx.open) closeCtx() // меню у курсора устаревает при прокрутке
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
// редактирование/удаление своих сообщений — в течение часа
const EDIT_WINDOW = 3600_000
const nowTs = ref(Date.now())
let nowTimer = null
const editingId = ref(null)
const editText = ref('')
const savingEdit = ref(false)
const REACTIONS = ['❤️', '👍', '🙏', '🔥', '😂', '🎉']
// контекстное меню у курсора: mode 'edit' (своё) | 'react' (чужое)
const ctx = reactive({ open: false, x: 0, y: 0, mode: '', m: null })
function closeCtx() { ctx.open = false; ctx.m = null }
const ctxStyle = computed(() => ({
  left: Math.min(ctx.x, (typeof window !== 'undefined' ? window.innerWidth : 9999) - 190) + 'px',
  top: Math.min(ctx.y, (typeof window !== 'undefined' ? window.innerHeight : 9999) - 150) + 'px',
}))
function canModify(m) {
  return m.author_id === auth.user?.id && (nowTs.value - new Date(m.created_at).getTime()) <= EDIT_WINDOW
}

usePageTitle(() => {
  const t = thread.value
  if (!t) return ''
  const head = t.kind === 'report' ? 'Отчёт' : (t.subject || 'Вопрос')
  return `${head} · ${t.disciple_name}`
})
// «Назад» из ветки — в соответствующий список (а не в форму создания)
watch(thread, (t) => {
  if (!t) return
  backTarget.value = t.kind === 'report' ? { name: 'service-reports' }
    : t.kind === 'approval' ? { name: 'approvals' }
    : { name: 'questions' }
})

function fmtTime(iso) {
  return new Date(iso).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
}
function dayKey(iso) {
  const d = new Date(iso)
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`
}
function dayLabel(iso) {
  const d = new Date(iso)
  const now = new Date()
  const yesterday = new Date(now); yesterday.setDate(now.getDate() - 1)
  if (dayKey(iso) === dayKey(now)) return 'Сегодня'
  if (dayKey(iso) === dayKey(yesterday)) return 'Вчера'
  const opts = { day: 'numeric', month: 'long' }
  if (d.getFullYear() !== now.getFullYear()) opts.year = 'numeric'
  return d.toLocaleDateString('ru-RU', opts)
}
// разделитель дня: показывать перед первым сообщением дня
function daySep(i) {
  const msgs = thread.value?.messages || []
  if (i === 0) return msgs[0] ? dayLabel(msgs[0].created_at) : null
  return dayKey(msgs[i].created_at) !== dayKey(msgs[i - 1].created_at) ? dayLabel(msgs[i].created_at) : null
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
  await preloadImages(data.messages.flatMap((m) => extractImageUrls(m.body))) // фото вперёд — без скачков
  thread.value = data
  await scrollDown()
  nextTick(syncVoiceButtons)
}

function connectWs() {
  if (!auth.token) return
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/api/ws/threads/${id.value}?token=${encodeURIComponent(auth.token)}`)
  ws.onmessage = (ev) => {
    const data = JSON.parse(ev.data)
    if (data.type === 'message') {
      if (!thread.value.messages.some((m) => m.id === data.message.id)) {
        preloadImages(extractImageUrls(data.message.body)).then(() => {
          thread.value.messages.push(data.message)
          scrollDown()
        })
      }
    } else if (data.type === 'edit') {
      const m = thread.value?.messages.find((x) => x.id === data.message.id)
      if (m) {
        m.body = data.message.body
        m.edit_count = data.message.edit_count
        preloadImages(extractImageUrls(m.body))
      }
    } else if (data.type === 'delete') {
      if (thread.value) thread.value.messages = thread.value.messages.filter((x) => x.id !== data.message_id)
    } else if (data.type === 'react') {
      const m = thread.value?.messages.find((x) => x.id === data.message_id)
      if (m) {
        // «моя» реакция считается локально (в broadcast mine относится к автору действия)
        const myEmoji = (m.reactions || []).find((r) => r.mine)?.emoji
        m.reactions = data.reactions.map((r) => ({ ...r, mine: r.emoji === myEmoji }))
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
async function react(m, emoji) {
  try {
    const { data } = await client.post(`/threads/${id.value}/messages/${m.id}/react`, { emoji })
    m.reactions = data.reactions
  } catch { /* игнор */ }
}
function fmtSec(s) {
  if (!s || !isFinite(s)) return '0:00'
  return `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, '0')}`
}
// клик по кнопке голосового — играть/пауза; клик по треку текущего — перемотать
function onScrollerClick(e) {
  const btn = e.target.closest('.voice-msg')
  if (!btn) return
  e.preventDefault()
  const src = btn.dataset.audio
  const track = e.target.closest('.voice-msg__track')
  if (track && player.src === src && player.duration) {
    const rect = track.getBoundingClientRect()
    const frac = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
    seek(frac * player.duration)
    return
  }
  const labelEl = btn.closest('[data-audio-label]')
  playAudio(src, labelEl?.dataset.audioLabel || 'Голосовое сообщение')
}
// подсветка играющей кнопки, прогресс/ползунок/время внутри неё
function syncVoiceButtons() {
  document.querySelectorAll('.voice-msg').forEach((b) => {
    const cur = b.dataset.audio === player.src
    b.classList.toggle('is-playing', cur && player.playing)
    const pct = cur && player.duration ? (player.currentTime / player.duration) * 100 : 0
    const fill = b.querySelector('.voice-msg__fill'); if (fill) fill.style.width = pct + '%'
    const knob = b.querySelector('.voice-msg__knob'); if (knob) knob.style.left = pct + '%'
    const time = b.querySelector('.voice-msg__time'); if (time) time.textContent = cur ? fmtSec(player.currentTime) : '0:00'
  })
}
watch(() => [player.src, player.playing, player.currentTime, player.duration], () => nextTick(syncVoiceButtons))

function onContext(e, m) {
  const own = m.author_id === auth.user?.id
  if (own && !canModify(m)) return // свои просроченные — обычное меню браузера
  e.preventDefault()
  ctx.x = e.clientX
  ctx.y = e.clientY
  ctx.mode = own ? 'edit' : 'react' // своё — правка/удаление, чужое — реакции
  ctx.m = m
  ctx.open = true
}
function startEdit(m) { editingId.value = m.id; editText.value = m.body; closeCtx() }
function cancelEdit() { editingId.value = null; editText.value = '' }
async function saveEdit(m) {
  const text = editText.value.trim()
  if (!text) return
  savingEdit.value = true
  try {
    const { data } = await client.patch(`/threads/${id.value}/messages/${m.id}`, { body: text })
    m.body = data.body
    m.edit_count = data.edit_count
    editingId.value = null
    await preloadImages(extractImageUrls(m.body))
  } catch (e) {
    alert(e.response?.data?.detail || 'Не удалось изменить сообщение')
  } finally {
    savingEdit.value = false
  }
}
async function removeMessage(m) {
  const ok = await confirmDialog({ message: 'Удалить сообщение? Это действие необратимо.', confirmText: 'Удалить', danger: true })
  if (!ok) return
  try {
    await client.delete(`/threads/${id.value}/messages/${m.id}`)
    thread.value.messages = thread.value.messages.filter((x) => x.id !== m.id)
  } catch (e) {
    alert(e.response?.data?.detail || 'Не удалось удалить сообщение')
  }
}

onMounted(async () => {
  try { await load(); connectWs() } finally { loading.value = false }
  nowTimer = setInterval(() => { nowTs.value = Date.now() }, 20000) // прячет «Изменить/Удалить» после часа
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
onBeforeUnmount(() => { if (ws) ws.close(); clearTimeout(typingTimer); clearInterval(nowTimer); if (resizeObs) resizeObs.disconnect(); backTarget.value = null; closePlayer() })
</script>

<template>
  <div class="mx-auto flex h-[calc(100dvh-4rem)] max-w-6xl flex-col -mt-4 -mb-4 sm:-mt-6 sm:-mb-6 lg:-mt-8 lg:-mb-8">
    <div v-if="loading" class="space-y-4">
      <AppSkeleton w="w-40" h="h-9" />
      <div class="card space-y-4 p-6"><AppSkeleton v-for="i in 4" :key="i" h="h-10" /></div>
    </div>

    <template v-else-if="thread">
      <div v-if="thread.period" class="mb-2 flex shrink-0 flex-wrap items-center gap-2">
        <span class="badge bg-saffron-500/15 text-saffron-700">{{ periodLabel }}</span>
      </div>

      <!-- плеер голосовых — внутри чата, сверху -->
      <AudioBar />

      <div ref="scroller" class="flex-1 space-y-3 overflow-y-auto pt-3 pb-1 pl-1 pr-4" @scroll="onScroll" @click="onScrollerClick">
        <template v-for="(m, i) in thread.messages" :key="m.id">
          <div v-if="daySep(i)" class="flex justify-center py-1">
            <span class="rounded-full bg-white px-3 py-1 text-xs font-medium text-ink-700/60 ring-1 ring-parchment-200">{{ daySep(i) }}</span>
          </div>
          <div class="group relative flex flex-col" :class="m.author_id === auth.user?.id ? 'items-end' : 'items-start'">
            <div class="max-w-[85%] rounded-2xl px-4 py-2.5"
                 :class="[m.author_id === auth.user?.id ? 'bg-saffron-500 text-white' : 'bg-white text-ink-800 ring-1 ring-parchment-200', editingId === m.id && 'w-full']"
                 @contextmenu="onContext($event, m)">
              <div class="mb-0.5 flex flex-wrap items-center gap-x-1.5 text-xs opacity-70">
                <span>{{ m.author_name || 'Аноним' }} · {{ fmtTime(m.created_at) }}</span>
                <span v-if="m.edit_count" :title="`изменено раз: ${m.edit_count}`">· изменено{{ m.edit_count > 1 ? ` ×${m.edit_count}` : '' }}</span>
              </div>
              <div v-if="editingId === m.id">
                <textarea v-model="editText" rows="3"
                          class="w-full resize-y rounded-lg border border-parchment-300 bg-white p-2 text-ink-800 focus:border-saffron-400 focus:outline-none"></textarea>
                <div class="mt-1 flex justify-end gap-2">
                  <button class="rounded-md px-2 py-1 text-xs text-ink-700/70 hover:bg-black/5" @click="cancelEdit">Отмена</button>
                  <button class="rounded-md bg-white px-2 py-1 text-xs font-medium text-saffron-700 ring-1 ring-parchment-300 hover:bg-parchment-50 disabled:opacity-50"
                          :disabled="savingEdit || !editText.trim()" @click="saveEdit(m)">Сохранить</button>
                </div>
              </div>
              <div v-else class="markdown-body break-words" :data-audio-label="`${m.author_name || 'Голосовое'} · ${fmtTime(m.created_at)}`" v-html="renderMarkdown(m.body)"></div>

              <!-- реакции (в стиле Telegram) — пилюли внизу сообщения -->
              <div v-if="editingId !== m.id && m.reactions && m.reactions.length" class="mt-1.5 flex flex-wrap gap-1">
                <button v-for="r in m.reactions" :key="r.emoji"
                        class="flex items-center gap-1 rounded-full px-2.5 py-1 leading-none transition disabled:cursor-default"
                        :class="m.author_id === auth.user?.id
                          ? 'bg-white/20 hover:bg-white/25'
                          : (r.mine ? 'bg-saffron-500/25 text-saffron-800 ring-1 ring-saffron-400' : 'bg-saffron-500/10 text-ink-700 hover:bg-saffron-500/20')"
                        :disabled="m.author_id === auth.user?.id"
                        @click.stop="react(m, r.emoji)">
                  <span class="text-xl leading-none">{{ r.emoji }}</span><span v-if="r.count > 1" class="text-sm font-semibold">{{ r.count }}</span>
                </button>
              </div>
            </div>

          </div>
        </template>
        <div v-if="!thread.messages.length" class="text-center text-sm text-ink-700/50">Сообщений пока нет</div>
      </div>

      <!-- контекстное меню у курсора (правый клик по сообщению) -->
      <template v-if="ctx.open">
        <div class="fixed inset-0 z-40" @click="closeCtx" @contextmenu.prevent="closeCtx"></div>
        <!-- своё сообщение: правка/удаление -->
        <div v-if="ctx.mode === 'edit'" class="fixed z-50 w-44 overflow-hidden rounded-lg border border-parchment-200 bg-white py-1 shadow-xl" :style="ctxStyle">
          <button class="flex w-full items-center gap-2 px-3 py-2.5 text-left text-sm text-ink-700 hover:bg-parchment-100" @click="startEdit(ctx.m)">
            <AppIcon name="edit" :size="16" /> Изменить
          </button>
          <button class="flex w-full items-center gap-2 px-3 py-2.5 text-left text-sm text-red-600 hover:bg-red-50" @click="removeMessage(ctx.m); closeCtx()">
            <AppIcon name="trash" :size="16" /> Удалить
          </button>
        </div>
        <!-- чужое сообщение: выбор реакции -->
        <div v-else class="fixed z-50 flex gap-1 rounded-full border border-parchment-200 bg-white px-2 py-1.5 shadow-xl" :style="ctxStyle">
          <button v-for="e in REACTIONS" :key="e"
                  class="rounded-full px-1 py-0.5 text-2xl leading-none transition-transform hover:scale-125"
                  @click="react(ctx.m, e); closeCtx()">{{ e }}</button>
        </div>
      </template>

      <div class="mt-1 shrink-0 pb-4">
        <div class="h-5 text-sm text-saffron-700/80"><span v-if="typingName">{{ typingName }} печатает…</span></div>
        <MarkdownEditor v-model="body" :rows="3" submit-on-enter type-anywhere hide-hint :voice="auth.isGuru" :draft-scope="`thread:${id}`" placeholder="Написать сообщение…" @submit="send" />
        <div class="mt-1 flex justify-end">
          <button class="btn-primary" :disabled="sending || !body.trim()" @click="send">{{ sending ? '…' : 'Отправить' }}</button>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.markdown-body :deep(a) { text-decoration: underline; }
.markdown-body :deep(ul) { margin: 0.25rem 0; padding-left: 1.25rem; list-style: disc; }
.markdown-body :deep(ol) { margin: 0.25rem 0; padding-left: 1.35rem; list-style: decimal; }
.markdown-body :deep(img) { max-height: 18rem; border-radius: 0.5rem; }
</style>
