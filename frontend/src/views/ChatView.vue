<script setup>
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import client from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import AudioBar from '../components/AudioBar.vue'
import { renderMarkdown } from '../lib/markdown'
import { player, playAudio, seek } from '../composables/audioPlayer'
import { showToast } from '../composables/toast'
import { usePageTitle } from '../composables/pageTitle'
import {
  chatState, initChat, openChat, closeChat, sendMessage, sendTyping,
  editMessage, deleteMessage, retryFailed, loadOlder, loadContacts, startDirect, startGroup,
  reactMessage, REACTION_EMOJIS,
} from '../chat/store'

usePageTitle('Чат')

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const MAX_LEN = 1000
const body = ref('')
const replyTo = ref(null)
const editingMsg = ref(null)
const scroller = ref(null)
const inputEl = ref(null)
const fileInput = ref(null)
const showNew = ref(false)
const showEmoji = ref(false)
const uploading = ref(false)

const EMOJI_PALETTE = ['😀','😁','😂','🤣','😊','😍','😘','😎','🤔','🙏','👍','👎','👌','🙌','👏','🔥','❤️','🧡','💛','💚','💙','💜','🎉','✨','⭐','🌟','💯','✅','❌','⚡','🌸','🌞','🍀','🕉️','📿','🙇','😢','😅','😉','🤗']

// ── активный чат по маршруту ────────────────────────────────────────────
const activeId = computed(() => (route.params.id ? Number(route.params.id) : null))
const activeChat = computed(() => chatState.chats.find((c) => c.id === activeId.value) || null)

watch(activeId, async (id) => {
  replyTo.value = null; editingMsg.value = null; body.value = ''; closeCtx()
  if (id) { await openChat(id); scrollToBottom() }
  else closeChat()
}, { immediate: false })

watch(() => chatState.messages.length, () => nextTick(scrollToBottom))
function scrollToBottom() { nextTick(() => { const el = scroller.value; if (el) el.scrollTop = el.scrollHeight }) }

// ── список чатов ─────────────────────────────────────────────────────────
const search = ref('')
const filteredChats = computed(() => {
  const q = search.value.trim().toLowerCase()
  return q ? chatState.chats.filter((c) => (c.title || '').toLowerCase().includes(q)) : chatState.chats
})
function selectChat(c) { router.push({ name: 'chat', params: { id: c.id } }) }
function backToList() { router.push({ name: 'chat-home' }) }

// ── реакции ──────────────────────────────────────────────────────────────
function parseReactions(m) { try { return JSON.parse(m.reactions || '[]') } catch { return [] } }
async function onChip(m, emoji) { if (m.id) await reactMessage(m.id, emoji) }

// ── контекстное меню (ПКМ) ─────────────────────────────────────────────
const ctx = reactive({ open: false, x: 0, y: 0, m: null, selText: '' })
function closeCtx() { ctx.open = false; ctx.m = null }
const ctxStyle = computed(() => ({
  left: Math.min(ctx.x, (typeof window !== 'undefined' ? window.innerWidth : 9999) - 220) + 'px',
  top: Math.min(ctx.y, (typeof window !== 'undefined' ? window.innerHeight : 9999) - 220) + 'px',
}))
function onContext(e, m) {
  if (m.deleted) return
  e.preventDefault()
  ctx.x = e.clientX; ctx.y = e.clientY; ctx.m = m
  ctx.selText = (window.getSelection?.().toString() || '').trim()
  ctx.open = true
}
const EDIT_WINDOW = 24 * 3600_000
function canEdit(m) { return isMine(m) && m.id && !m.deleted && !isVoice(m) && (Date.now() - new Date(m.created_at).getTime()) <= EDIT_WINDOW }
function canDelete(m) { return isMine(m) && m.id && !m.deleted }
function isVoice(m) { return /@\[audio\]\(/.test(m.body || '') }

function ctxReply() { startReply(ctx.m); closeCtx() }
async function ctxReact(emoji) { const m = ctx.m; closeCtx(); if (m?.id) await reactMessage(m.id, emoji) }
function ctxCopy() {
  const m = ctx.m
  const text = ctx.selText || cleanBody(m.body)
  closeCtx()
  if (!text) return
  navigator.clipboard?.writeText(text).then(() => showToast('Скопировано')).catch(() => {})
}
function ctxEdit() { startEdit(ctx.m); closeCtx() }
async function ctxDelete() { const m = ctx.m; closeCtx(); if (m?.id) await deleteMessage(m.id) }
function cleanBody(b) {
  return (b || '').replace(/@\[audio\]\([^)]*\)/g, '🎤 Голосовое сообщение').replace(/!\[[^\]]*\]\([^)]*\)/g, '').trim()
}

function startReply(m) { editingMsg.value = null; replyTo.value = { id: m.id, author_name: m.author_name, body: snippet(m.body) } }
function startEdit(m) { replyTo.value = null; editingMsg.value = m; body.value = m.body; nextTick(() => { autoGrow(); inputEl.value?.focus() }) }
function cancelEdit() { editingMsg.value = null; body.value = '' }
function snippet(b) {
  return (b || '').replace(/@\[audio\]\([^)]*\)/g, '🎤 Голосовое').replace(/!\[[^\]]*\]\([^)]*\)/g, '🖼 Фото').replace(/\s+/g, ' ').trim().slice(0, 80)
}

// ── композер: авто-рост, лимит, отправка ───────────────────────────────
function autoGrow() {
  const el = inputEl.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 160) + 'px'
}
function onInput() {
  if (body.value.length > MAX_LEN) body.value = body.value.slice(0, MAX_LEN)
  autoGrow()
}
watch(body, () => nextTick(autoGrow))

let lastTyping = 0
function onKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); return }
  const now = Date.now()
  if (now - lastTyping > 2000) { lastTyping = now; sendTyping() }
}

async function send() {
  const text = body.value.trim()
  if (!text) return
  if (editingMsg.value) {
    const m = editingMsg.value
    editingMsg.value = null; body.value = ''
    await editMessage(m.id, text)
    return
  }
  const reply = replyTo.value?.id || null
  body.value = ''; replyTo.value = null
  nextTick(autoGrow)
  await sendMessage(text, reply)
  scrollToBottom()
}

function insertEmoji(e) {
  const el = inputEl.value
  const pos = el?.selectionStart ?? body.value.length
  body.value = (body.value.slice(0, pos) + e + body.value.slice(pos)).slice(0, MAX_LEN)
  nextTick(() => { autoGrow(); el?.focus(); const p = pos + e.length; el?.setSelectionRange(p, p) })
}

// ── вложения (скрепка) ───────────────────────────────────────────────────
async function onPickFile(ev) {
  const files = Array.from(ev.target.files || []).filter((f) => f.type.startsWith('image/'))
  if (fileInput.value) fileInput.value.value = ''
  if (!files.length || !activeId.value) return
  uploading.value = true
  try {
    const fd = new FormData()
    files.forEach((f) => fd.append('files', f))
    const { data } = await client.post('/uploads', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    for (const u of data.urls || []) { await sendMessage(`![](${u})`); }
    scrollToBottom()
  } catch { showToast('Не удалось загрузить файл') } finally { uploading.value = false }
}

// ── голосовые ────────────────────────────────────────────────────────────
const recording = ref(false)
const recSeconds = ref(0)
let mediaRecorder = null; let recChunks = []; let recStream = null; let recTimer = null; let recStart = 0; let recCanceled = false
function fmtRec(s) { return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}` }
function pickMime() {
  for (const c of ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4', 'audio/ogg']) {
    if (window.MediaRecorder && MediaRecorder.isTypeSupported(c)) return c
  }
  return ''
}
async function startRec() {
  if (recording.value) return
  if (!navigator.mediaDevices?.getUserMedia || !window.MediaRecorder) { alert('Запись не поддерживается'); return }
  try { recStream = await navigator.mediaDevices.getUserMedia({ audio: true }) } catch { alert('Нет доступа к микрофону'); return }
  recChunks = []; recCanceled = false
  const mime = pickMime()
  mediaRecorder = new MediaRecorder(recStream, mime ? { mimeType: mime } : undefined)
  mediaRecorder.ondataavailable = (e) => { if (e.data && e.data.size) recChunks.push(e.data) }
  mediaRecorder.onstop = onRecStop
  mediaRecorder.start()
  recording.value = true; recSeconds.value = 0; recStart = Date.now()
  clearInterval(recTimer)
  recTimer = setInterval(() => { recSeconds.value = Math.floor((Date.now() - recStart) / 1000); if (recSeconds.value >= 300) stopRec() }, 250)
}
function cleanupRec() {
  clearInterval(recTimer); recTimer = null; recording.value = false
  if (recStream) { recStream.getTracks().forEach((t) => t.stop()); recStream = null }
}
async function onRecStop() {
  const mime = mediaRecorder?.mimeType || 'audio/webm'
  cleanupRec()
  if (recCanceled || !recChunks.length) { recChunks = []; return }
  const blob = new Blob(recChunks, { type: mime }); recChunks = []
  uploading.value = true
  try {
    const ext = mime.includes('mp4') ? 'm4a' : mime.includes('ogg') ? 'ogg' : 'webm'
    const fd = new FormData(); fd.append('files', blob, `voice.${ext}`)
    const { data } = await client.post('/uploads', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    const url = data.urls?.[0]
    if (url) { await sendMessage(`@[audio](${url})`); scrollToBottom() }
  } finally { uploading.value = false }
}
function stopRec() { if (mediaRecorder && mediaRecorder.state !== 'inactive') mediaRecorder.stop() }
function cancelRec() { recCanceled = true; stopRec() }

// ── участники / статусы ───────────────────────────────────────────────────
const isMine = (m) => m.author_id === chatState.meId
const peerReadSeq = computed(() => {
  const ch = activeChat.value
  if (!ch || ch.type !== 'direct') return 0
  const peer = (chatState.members || []).find((x) => x.user_id !== chatState.meId)
  return peer ? peer.last_read_seq || 0 : 0
})
function statusOf(m) {
  if (!isMine(m)) return null
  if (m.status === 'pending') return 'pending'
  if (m.status === 'failed') return 'failed'
  if (activeChat.value?.type === 'direct' && m.seq && peerReadSeq.value >= m.seq) return 'read'
  return 'sent'
}
const typingLabel = computed(() => { const t = chatState.typing[activeId.value]; return t ? `${t.name} печатает…` : '' })
function showAuthor(m, i) {
  if (isMine(m) || activeChat.value?.type !== 'group') return false
  const prev = chatState.messages[i - 1]
  return !prev || prev.author_id !== m.author_id
}
function fmtTime(ts) { if (!ts) return ''; const d = new Date(ts); return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}` }
function initials(name) { return (name || '?').trim()[0]?.toUpperCase() || '?' }

// ── голосовые: проигрывание внутри пузыря ─────────────────────────────────
let voiceDragging = false
function onScrollerClick(e) {
  if (voiceDragging) { voiceDragging = false; return }
  const btn = e.target.closest('.voice-msg')
  if (!btn) return
  e.preventDefault()
  const src = btn.dataset.audio
  const wave = e.target.closest('.voice-msg__wave')
  if (wave && player.src === src && player.duration) { seek(waveFrac(e, wave) * player.duration); return }
  const labelEl = btn.closest('[data-audio-label]')
  playAudio(src, labelEl?.dataset.audioLabel || 'Голосовое сообщение')
}
function waveFrac(e, wave) { const r = wave.getBoundingClientRect(); const x = e.touches ? e.touches[0].clientX : e.clientX; return Math.max(0, Math.min(1, (x - r.left) / r.width)) }
function onScrollerDown(e) {
  const wave = e.target.closest('.voice-msg__wave')
  if (!wave) return
  const src = wave.closest('.voice-msg')?.dataset.audio
  if (!src || player.src !== src || !player.duration) return
  const move = (ev) => { voiceDragging = true; seek(waveFrac(ev, wave) * player.duration); if (ev.cancelable) ev.preventDefault() }
  const up = () => { window.removeEventListener('mousemove', move); window.removeEventListener('mouseup', up); window.removeEventListener('touchmove', move); window.removeEventListener('touchend', up) }
  window.addEventListener('mousemove', move); window.addEventListener('mouseup', up)
  window.addEventListener('touchmove', move, { passive: false }); window.addEventListener('touchend', up)
}
function fmtSec(s) { return (!s || !isFinite(s)) ? '0:00' : `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, '0')}` }
function syncVoiceButtons() {
  document.querySelectorAll('.voice-msg').forEach((b) => {
    const cur = b.dataset.audio === player.src
    b.classList.toggle('is-playing', cur && player.playing)
    const pct = cur && player.duration ? (player.currentTime / player.duration) * 100 : 0
    const played = b.querySelector('.vw-played'); if (played) played.style.clipPath = `inset(0 ${100 - pct}% 0 0)`
    const time = b.querySelector('.voice-msg__time')
    if (time) time.textContent = (cur && player.duration) ? '-' + fmtSec(player.duration - player.currentTime) : '0:00'
  })
}
watch(() => [player.src, player.playing, player.currentTime, player.duration], () => nextTick(syncVoiceButtons))

async function onScroll() {
  if (ctx.open) closeCtx()
  const el = scroller.value
  if (el && el.scrollTop < 40) { const prevH = el.scrollHeight; const n = await loadOlder(); if (n) nextTick(() => { el.scrollTop = el.scrollHeight - prevH }) }
}

// ── создание чата ──────────────────────────────────────────────────────
const newTab = ref('direct')
const groupTitle = ref('')
const groupMembers = ref([])
async function openNew() { showNew.value = true; newTab.value = 'direct'; groupTitle.value = ''; groupMembers.value = []; await loadContacts() }
function closeNew() { showNew.value = false }
async function pickDirect(u) { const id = await startDirect(u.id); closeNew(); router.push({ name: 'chat', params: { id } }) }
function toggleMember(u) { const i = groupMembers.value.indexOf(u.id); if (i >= 0) groupMembers.value.splice(i, 1); else groupMembers.value.push(u.id) }
async function createGroup() {
  const title = groupTitle.value.trim()
  if (!title || !groupMembers.value.length) return
  const id = await startGroup(title, [...groupMembers.value]); closeNew(); router.push({ name: 'chat', params: { id } })
}

function onGlobalKey(e) { if (e.key === 'Escape') { if (recording.value) cancelRec(); else if (ctx.open) closeCtx(); else if (editingMsg.value) cancelEdit(); else if (replyTo.value) replyTo.value = null; else showEmoji.value = false } }

onMounted(async () => {
  document.addEventListener('keydown', onGlobalKey)
  if (!auth.isPending && auth.user) {
    await initChat({ meId: auth.user.id, getToken: () => auth.token })
    if (activeId.value) { await openChat(activeId.value); scrollToBottom() }
  }
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onGlobalKey)
  if (recording.value) { recCanceled = true; stopRec() }
  cleanupRec(); closeChat()
})
</script>

<template>
  <div class="-m-4 flex h-[calc(100vh-4rem)] overflow-hidden bg-white sm:-m-6 lg:-m-8">
    <!-- Список чатов -->
    <aside class="flex w-full shrink-0 flex-col border-r border-parchment-200 sm:w-80" :class="activeId ? 'hidden sm:flex' : 'flex'">
      <div class="flex items-center gap-2 border-b border-parchment-200 p-3">
        <div class="relative flex-1">
          <AppIcon name="search" :size="16" class="absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-700/40" />
          <input v-model="search" class="input h-9 w-full pl-8 text-sm" placeholder="Поиск" />
        </div>
        <button class="btn-primary h-9 shrink-0 px-3" title="Новый чат" @click="openNew"><AppIcon name="edit" :size="16" /></button>
      </div>
      <div class="flex-1 overflow-y-auto">
        <p v-if="!chatState.ready" class="p-4 text-sm text-ink-700/50">Загрузка…</p>
        <p v-else-if="!filteredChats.length" class="p-4 text-sm text-ink-700/50">Чатов пока нет. Нажмите «карандаш», чтобы начать.</p>
        <button v-for="c in filteredChats" :key="c.id"
                class="flex w-full items-center gap-3 border-b border-parchment-100 px-3 py-2.5 text-left hover:bg-parchment-50"
                :class="c.id === activeId && 'bg-saffron-500/10'" @click="selectChat(c)">
          <img v-if="c.avatar_url" :src="c.avatar_url" class="photo-bw h-11 w-11 shrink-0 rounded-full object-cover" />
          <span v-else class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full text-base font-semibold text-white"
                :class="c.type === 'group' ? 'bg-gradient-to-br from-sage-400 to-sage-600' : 'bg-gradient-to-br from-saffron-400 to-saffron-600'">
            <AppIcon v-if="c.type === 'group'" name="users" :size="20" /><template v-else>{{ initials(c.title) }}</template>
          </span>
          <span class="min-w-0 flex-1">
            <span class="flex items-center justify-between gap-2">
              <span class="truncate font-medium text-ink-900">{{ c.title }}</span>
              <span class="shrink-0 text-[11px] text-ink-700/40">{{ fmtTime(c.last?.created_at) }}</span>
            </span>
            <span class="flex items-center justify-between gap-2">
              <span class="truncate text-sm text-ink-700/60">{{ c.last ? (c.last.deleted ? 'сообщение удалено' : snippet(c.last.body)) : 'Нет сообщений' }}</span>
              <span v-if="c.unread" class="ml-1 inline-flex h-5 min-w-[1.25rem] shrink-0 items-center justify-center rounded-full bg-saffron-500 px-1.5 text-xs font-semibold text-white">{{ c.unread }}</span>
            </span>
          </span>
        </button>
      </div>
    </aside>

    <!-- Разговор -->
    <section class="flex min-w-0 flex-1 flex-col" :class="activeId ? 'flex' : 'hidden sm:flex'">
      <template v-if="activeChat">
        <header class="flex items-center gap-3 border-b border-parchment-200 px-4 py-2.5">
          <button class="rounded-lg p-1.5 text-ink-700/60 hover:bg-parchment-100 sm:hidden" @click="backToList"><AppIcon name="chevron" :size="18" class="rotate-90" /></button>
          <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-sm font-semibold text-white"
                :class="activeChat.type === 'group' ? 'bg-gradient-to-br from-sage-400 to-sage-600' : 'bg-gradient-to-br from-saffron-400 to-saffron-600'">
            <AppIcon v-if="activeChat.type === 'group'" name="users" :size="16" /><template v-else>{{ initials(activeChat.title) }}</template>
          </span>
          <div class="min-w-0 flex-1">
            <div class="truncate font-medium text-ink-900">{{ activeChat.title }}</div>
            <div class="truncate text-xs text-ink-700/50">
              <span v-if="typingLabel" class="text-saffron-600">{{ typingLabel }}</span>
              <span v-else-if="activeChat.type === 'group'">{{ activeChat.members.length }} участников</span>
              <span v-else>{{ chatState.connection === 'online' ? 'в сети' : 'не в сети' }}</span>
            </div>
          </div>
        </header>

        <div ref="scroller" class="flex-1 space-y-1 overflow-y-auto bg-parchment-50/40 p-4"
             @scroll="onScroll" @click="onScrollerClick" @mousedown="onScrollerDown" @touchstart="onScrollerDown">
          <div v-for="(m, i) in chatState.messages" :key="m.client_uuid" :id="`msg-${m.id}`"
               class="group flex" :class="isMine(m) ? 'justify-end' : 'justify-start'">
            <div class="relative max-w-[78%] rounded-2xl px-3 py-2 text-sm shadow-sm"
                 :class="isMine(m) ? 'bg-saffron-500 text-white' : 'bg-white text-ink-900 ring-1 ring-parchment-200'"
                 :data-audio-label="`${m.author_name || 'Голосовое'} · ${fmtTime(m.created_at)}`"
                 @contextmenu="onContext($event, m)">
              <div v-if="showAuthor(m, i)" class="mb-0.5 text-xs font-semibold text-sage-600">{{ m.author_name }}</div>
              <div v-if="m.reply_preview" class="mb-1 border-l-2 pl-2 text-xs opacity-80" :class="isMine(m) ? 'border-white/60' : 'border-saffron-400'">{{ m.reply_preview }}</div>
              <div v-if="m.deleted" class="italic opacity-60">сообщение удалено</div>
              <div v-else class="markdown-body break-words" :class="isMine(m) && 'markdown-on-accent'" v-html="renderMarkdown(m.body)"></div>

              <div v-if="parseReactions(m).length" class="mt-1 flex flex-wrap gap-1">
                <button v-for="r in parseReactions(m)" :key="r.emoji" @click.stop="onChip(m, r.emoji)"
                        class="inline-flex items-center gap-0.5 rounded-full px-1.5 py-0.5 text-xs ring-1 transition"
                        :class="m.my_reaction === r.emoji ? (isMine(m) ? 'bg-white/25 ring-white/60' : 'bg-saffron-500/15 ring-saffron-400') : (isMine(m) ? 'bg-white/10 ring-white/25' : 'bg-parchment-100 ring-parchment-200')">
                  <span>{{ r.emoji }}</span><span class="tabular-nums">{{ r.count }}</span>
                </button>
              </div>

              <div class="mt-0.5 flex items-center justify-end gap-1 text-[10px]" :class="isMine(m) ? 'text-white/70' : 'text-ink-700/40'">
                <span v-if="m.edit_count">изм. · </span>
                <span>{{ fmtTime(m.created_at) }}</span>
                <template v-if="statusOf(m)">
                  <AppIcon v-if="statusOf(m) === 'pending'" name="clock" :size="12" />
                  <button v-else-if="statusOf(m) === 'failed'" class="text-red-200" title="Не отправлено — повторить" @click.stop="retryFailed"><AppIcon name="close" :size="12" /></button>
                  <AppIcon v-else-if="statusOf(m) === 'read'" name="check" :size="12" class="-mr-1" />
                  <AppIcon v-if="statusOf(m) === 'read' || statusOf(m) === 'sent'" name="check" :size="12" />
                </template>
              </div>
            </div>
          </div>
        </div>

        <AudioBar />

        <!-- Композер -->
        <div class="border-t border-parchment-200 p-3">
          <div v-if="replyTo" class="mb-2 flex items-center gap-2 rounded-lg bg-parchment-100 px-3 py-1.5 text-sm">
            <AppIcon name="reply" :size="14" class="text-saffron-600" />
            <span class="min-w-0 flex-1 truncate text-ink-700/70"><b class="text-ink-800">{{ replyTo.author_name }}</b>: {{ replyTo.body }}</span>
            <button class="text-ink-700/50 hover:text-ink-900" @click="replyTo = null"><AppIcon name="close" :size="15" /></button>
          </div>
          <div v-else-if="editingMsg" class="mb-2 flex items-center gap-2 rounded-lg bg-parchment-100 px-3 py-1.5 text-sm">
            <AppIcon name="edit" :size="14" class="text-saffron-600" />
            <span class="min-w-0 flex-1 truncate text-ink-700/70">Редактирование сообщения</span>
            <button class="text-ink-700/50 hover:text-ink-900" @click="cancelEdit"><AppIcon name="close" :size="15" /></button>
          </div>

          <div v-if="recording" class="flex items-center gap-3 rounded-2xl bg-red-500/10 px-4 py-3 ring-1 ring-red-300">
            <span class="h-2.5 w-2.5 animate-pulse rounded-full bg-red-500"></span>
            <span class="flex-1 text-sm text-red-700">Идёт запись… <span class="tabular-nums">{{ fmtRec(recSeconds) }}</span></span>
            <button class="btn-ghost text-sm text-ink-700/60" @click="cancelRec">Отмена</button>
            <button class="btn-primary h-9 px-4" @click="stopRec">Отправить</button>
          </div>

          <div v-else class="relative flex items-end gap-2">
            <button class="mb-0.5 shrink-0 rounded-full p-2 text-ink-700/60 hover:bg-parchment-100 hover:text-saffron-600" title="Прикрепить" :disabled="uploading" @click="fileInput.click()">
              <AppIcon name="paperclip" :size="20" />
            </button>
            <input ref="fileInput" type="file" accept="image/*" multiple class="hidden" @change="onPickFile" />

            <textarea ref="inputEl" v-model="body" rows="1" :maxlength="MAX_LEN"
                      class="min-h-[2.5rem] flex-1 resize-none rounded-2xl border border-parchment-300 bg-parchment-50 px-4 py-2.5 text-sm leading-5 focus:border-saffron-400 focus:outline-none focus:ring-1 focus:ring-saffron-400"
                      placeholder="Сообщение…" @input="onInput" @keydown="onKeydown"></textarea>

            <div class="relative mb-0.5 shrink-0">
              <button class="rounded-full p-2 text-ink-700/60 hover:bg-parchment-100 hover:text-saffron-600" title="Эмодзи" @click="showEmoji = !showEmoji">
                <AppIcon name="react" :size="20" />
              </button>
              <template v-if="showEmoji">
                <div class="fixed inset-0 z-10" @click="showEmoji = false"></div>
                <div class="absolute bottom-full right-0 z-20 mb-2 grid w-64 grid-cols-8 gap-1 rounded-xl bg-white p-2 shadow-lg ring-1 ring-parchment-200">
                  <button v-for="e in EMOJI_PALETTE" :key="e" class="rounded p-1 text-lg leading-none hover:bg-parchment-100" @click="insertEmoji(e)">{{ e }}</button>
                </div>
              </template>
            </div>

            <button v-if="body.trim()" class="mb-0.5 shrink-0 rounded-full bg-saffron-500 p-2 text-white hover:bg-saffron-600" title="Отправить" @click="send">
              <AppIcon name="forward" :size="20" />
            </button>
            <button v-else class="mb-0.5 shrink-0 rounded-full p-2 text-ink-700/60 hover:bg-parchment-100 hover:text-saffron-600" title="Голосовое" :disabled="uploading" @click="startRec">
              <AppIcon name="mic" :size="20" />
            </button>
          </div>
        </div>
      </template>

      <div v-else class="flex flex-1 flex-col items-center justify-center text-center text-ink-700/40">
        <AppIcon name="chat" :size="48" />
        <p class="mt-3 text-sm">Выберите чат или начните новый</p>
      </div>
    </section>

    <!-- Контекстное меню (ПКМ) -->
    <template v-if="ctx.open">
      <div class="fixed inset-0 z-40" @click="closeCtx" @contextmenu.prevent="closeCtx"></div>
      <div class="fixed z-50 w-52 overflow-hidden rounded-xl bg-white py-1 shadow-xl ring-1 ring-parchment-200" :style="ctxStyle">
        <div class="flex justify-around px-2 py-1.5">
          <button v-for="e in REACTION_EMOJIS" :key="e" class="rounded-full p-1 text-lg leading-none transition hover:scale-125" @click="ctxReact(e)">{{ e }}</button>
        </div>
        <div class="my-1 border-t border-parchment-100"></div>
        <button class="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm text-ink-700 hover:bg-parchment-100" @click="ctxReply"><AppIcon name="reply" :size="15" /> Ответить</button>
        <button class="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm text-ink-700 hover:bg-parchment-100" @click="ctxCopy"><AppIcon name="copy" :size="15" /> Копировать</button>
        <button v-if="canEdit(ctx.m)" class="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm text-ink-700 hover:bg-parchment-100" @click="ctxEdit"><AppIcon name="edit" :size="15" /> Изменить</button>
        <button v-if="canDelete(ctx.m)" class="flex w-full items-center gap-2.5 border-t border-parchment-100 px-3 py-2 text-left text-sm text-red-600 hover:bg-red-50" @click="ctxDelete"><AppIcon name="trash" :size="15" /> Удалить</button>
      </div>
    </template>

    <!-- Модалка нового чата -->
    <div v-if="showNew" class="fixed inset-0 z-40 flex items-center justify-center bg-ink-900/40 p-4" @click.self="closeNew">
      <div class="w-full max-w-md overflow-hidden rounded-xl bg-white shadow-xl">
        <div class="flex border-b border-parchment-200">
          <button class="flex-1 px-4 py-3 text-sm font-medium" :class="newTab === 'direct' ? 'border-b-2 border-saffron-500 text-saffron-700' : 'text-ink-700/60'" @click="newTab = 'direct'">Личный чат</button>
          <button class="flex-1 px-4 py-3 text-sm font-medium" :class="newTab === 'group' ? 'border-b-2 border-saffron-500 text-saffron-700' : 'text-ink-700/60'" @click="newTab = 'group'">Группа</button>
          <button class="px-3 text-ink-700/40 hover:text-ink-900" @click="closeNew"><AppIcon name="close" :size="18" /></button>
        </div>
        <div v-if="newTab === 'group'" class="border-b border-parchment-200 p-3"><input v-model="groupTitle" class="input" placeholder="Название группы" /></div>
        <div class="max-h-80 overflow-y-auto">
          <p v-if="!chatState.contacts.length" class="p-4 text-sm text-ink-700/50">Нет доступных контактов.</p>
          <button v-for="u in chatState.contacts" :key="u.id"
                  class="flex w-full items-center gap-3 border-b border-parchment-100 px-4 py-2.5 text-left hover:bg-parchment-50"
                  @click="newTab === 'direct' ? pickDirect(u) : toggleMember(u)">
            <img v-if="u.avatar_url" :src="u.avatar_url" class="photo-bw h-9 w-9 shrink-0 rounded-full object-cover" />
            <span v-else class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-sm font-semibold text-white">{{ initials(u.full_name) }}</span>
            <span class="min-w-0 flex-1 truncate text-ink-900">{{ u.full_name }}</span>
            <AppIcon v-if="newTab === 'group' && groupMembers.includes(u.id)" name="check" :size="18" class="text-saffron-600" />
          </button>
        </div>
        <div v-if="newTab === 'group'" class="flex items-center justify-between gap-3 border-t border-parchment-200 p-3">
          <span class="text-sm text-ink-700/60">Выбрано: {{ groupMembers.length }}</span>
          <button class="btn-primary" :disabled="!groupTitle.trim() || !groupMembers.length" @click="createGroup">Создать группу</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.markdown-on-accent :deep(a) { color: #fff; text-decoration: underline; }
.markdown-on-accent :deep(blockquote) { border-color: rgba(255,255,255,.5); color: rgba(255,255,255,.85); }
</style>
