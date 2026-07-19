<script setup>
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import client from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import AudioBar from '../components/AudioBar.vue'
import { renderMarkdown } from '../lib/markdown'
import { thumbUrl } from '../lib/format'
import { player, playAudio, seek } from '../composables/audioPlayer'
import { openLightbox, closeLightbox, setLightboxActions } from '../composables/lightbox'
import { showToast } from '../composables/toast'
import { confirmDialog } from '../composables/confirm'
import { usePageTitle } from '../composables/pageTitle'
import {
  chatState, initChat, openChat, closeChat, sendMessage, sendMessageTo, sendTyping,
  editMessage, deleteMessage, retryFailed, loadOlder, loadContacts, startDirect, startGroup,
  reactMessage, REACTION_EMOJIS, updateChat, pinChat, leaveChat, forwardMessages, loadAroundSeq, markActiveRead, imageAspect, imageColor, imageMicro, expandWindow, reorderPins,
  pinMessageInChat, unpinMessageInChat, localCacheStats, wipeLocalChatCache, onCallSignal, sendCallSignal, chatScrollMem, chatNav,
} from '../chat/store'

usePageTitle('Чат')

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const MAX_LEN = 1000
const body = ref('')
const replyTo = ref(null)
const editingMedia = ref('') // при правке фото: сохраняем медиа-часть (![](url)), редактируем только подпись
const playingId = ref(null)  // id видео-сообщения, которое сейчас проигрывается инлайн
// видео в чате: обратный таймер + полноэкранный просмотр по клику
const videoState = reactive({})
function onVideoTime(e, m) {
  const v = e.target
  const fin = Number.isFinite(v.duration) && v.duration > 0
  // webm из MediaRecorder часто отдаёт duration=Infinity — тогда показываем прошедшее время
  const rem = fin ? Math.max(0, v.duration - (v.currentTime || 0)) : (v.currentTime || 0)
  videoState[m.id] = { remain: fmtSec(rem), progress: fin ? (v.currentTime || 0) / v.duration : 0 }
}
function openVideoFull(e, m) {
  const v = e.currentTarget.closest('.video-box')?.querySelector('video')
  if (!v) return
  v.muted = false; v.controls = true
  const req = v.requestFullscreen || v.webkitRequestFullscreen || v.webkitEnterFullscreen
  if (req) { try { req.call(v) } catch { /* ignore */ } }
  const onFs = () => {
    if (!document.fullscreenElement && !document.webkitFullscreenElement) {
      v.muted = true; v.controls = false
      document.removeEventListener('fullscreenchange', onFs); document.removeEventListener('webkitfullscreenchange', onFs)
    }
  }
  document.addEventListener('fullscreenchange', onFs); document.addEventListener('webkitfullscreenchange', onFs)
}
const editingMsg = ref(null)
const scroller = ref(null)
const listWrap = ref(null)
const stickBottom = ref(true)          // держимся ли у нижнего края (иначе не дёргаем при подгрузке)
const chatOpening = ref(false)         // прячем ленту на время открытия/позиционирования (без мелькания)
const floatDate = reactive({ label: '', show: false }) // единственная плавающая дата (без встроенных плашек)
let floatRaf = 0
// коалесцируем в один кадр — иначе чтение layout на каждом scroll-событии даёт лаги на медиа
function updateFloatingDate() {
  if (floatRaf) return
  floatRaf = requestAnimationFrame(() => {
    floatRaf = 0
    const el = scroller.value; if (!el) { floatDate.show = false; return }
    const top = el.getBoundingClientRect().top
    const seps = el.querySelectorAll('[data-daysep]')
    let label = ''; let inlineAtTop = false
    for (const s of seps) {
      const st = s.getBoundingClientRect().top
      if (st <= top + 20) label = s.getAttribute('data-daysep')
      if (st > top - 6 && st < top + 46) inlineAtTop = true // встроенная плашка у самого верха — плавающую прячем
    }
    if (!label && seps.length) label = seps[0].getAttribute('data-daysep')
    floatDate.label = label
    floatDate.show = !!label && !inlineAtTop // не дублируем встроенную плашку
  })
}
let listObs = null
let pendingAnchor = null // {id, offset} — якорное сообщение, которое держим при догрузке контента
// топовое видимое сообщение + его смещение от верха вьюпорта (для устойчивого восстановления позиции)
function computeAnchor() {
  const el = scroller.value; if (!el) return null
  const top = el.getBoundingClientRect().top
  const nodes = el.querySelectorAll('[id^="msg-"]')
  for (const n of nodes) { const r = n.getBoundingClientRect(); if (r.bottom > top + 4) return { id: n.id.slice(4), offset: r.top - top } }
  return null
}
function restoreAnchor(a) {
  const el = scroller.value; if (!el || !a) return false
  const n = document.getElementById('msg-' + a.id); if (!n) return false
  const top = el.getBoundingClientRect().top
  const delta = (n.getBoundingClientRect().top - top) - a.offset
  if (Math.abs(delta) > 0.5) el.scrollTop += delta
  return true
}
// При догрузке контента (превью ссылок, картинки, файлы) высота меняется. Если стоим у низа —
// прижимаемся к низу; если восстанавливаем позицию — держим ЯКОРНОЕ сообщение на месте (не пиксель),
// иначе рост контента ВЫШЕ позиции «съезжает» ленту.
watch(listWrap, (el) => {
  if (listObs) { listObs.disconnect(); listObs = null }
  if (el && typeof ResizeObserver !== 'undefined') {
    listObs = new ResizeObserver(() => {
      if (stickBottom.value) { const s = scroller.value; if (s) s.scrollTop = s.scrollHeight }
      else if (pendingAnchor) restoreAnchor(pendingAnchor)
    })
    listObs.observe(el)
  }
})
const inputEl = ref(null)
const fileInput = ref(null)
const showNew = ref(false)
const showEmoji = ref(false)
const uploading = ref(false)

const EMOJI_PALETTE = ['😀','😁','😂','🤣','😊','😍','😘','😎','🤔','🙏','👍','👎','👌','🙌','👏','🔥','❤️','🧡','💛','💚','💙','💜','🎉','✨','⭐','🌟','💯','✅','❌','⚡','🌸','🌞','🍀','🕉️','📿','🙇','😢','😅','😉','🤗']

// эмодзи в сообщениях рисуем крупнее (как в мессенджерах)
const EMOJI_RE = /(\p{Extended_Pictographic}(?:️|‍\p{Extended_Pictographic})*)/gu
function esc(s) { return String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c])) }
function renderChatBody(b) {
  // вложения-файлы @[file](url|имя) → карточка со скачиванием (до markdown)
  const files = []
  const marked = (b || '').replace(/@\[file\]\(([^|)]+)\|([^)]*)\)/g, (_m, url, name) => {
    files.push({ url, name: decodeURIComponent(name || 'файл') })
    return `%%FILE${files.length - 1}%%`
  })
  let html = renderMarkdown(marked)
  html = html.replace(/%%FILE(\d+)%%/g, (_m, i) => {
    const f = files[+i]
    return `<a class="chat-file" href="${esc(f.url)}" download="${esc(f.name)}" target="_blank" rel="noopener"><span class="chat-file__ic">📎</span><span class="chat-file__name">${esc(f.name)}</span></a>`
  })
  return html.replace(EMOJI_RE, '<span class="chat-emoji">$1</span>')
}

// ── активный чат по маршруту ────────────────────────────────────────────
const activeId = computed(() => (route.params.id ? Number(route.params.id) : null))
const activeChat = computed(() => chatState.chats.find((c) => c.id === activeId.value) || null)

// Память позиций прокрутки чатов — в памяти модуля store (переживает уход на другую
// страницу и возврат, но обнуляется при полной перезагрузке страницы).
function rememberScroll(id) { if (!id || !scroller.value) return; chatScrollMem[id] = { top: scroller.value.scrollTop, atBottom: stickBottom.value, anchor: computeAnchor() } }
// Позиционируем ленту после открытия чата (общее для смены чата и монтирования роута).
function positionAfterOpen(id, saved) {
  const restore = !!saved && !saved.atBottom
  // по ЯКОРНОМУ сообщению, а не по пикселю — при догрузке превью/картинок ResizeObserver держит его на месте
  pendingAnchor = restore ? (saved.anchor || null) : null
  const setPos = () => {
    const el = scroller.value; if (!el) return
    if (restore) { if (!(pendingAnchor && restoreAnchor(pendingAnchor))) el.scrollTop = saved.top }
    else el.scrollTop = el.scrollHeight
  }
  setPos(); updateFloatingDate()
  ;[80, 220, 450, 800].forEach((d) => setTimeout(() => {
    if (activeId.value !== id) return
    if (!openSettled.value) { setPos(); if (!restore) stickBottom.value = true }
    updateFloatingDate()
  }, d))
  setTimeout(() => { openSettled.value = true }, 500)
  setTimeout(() => { if (activeId.value === id) pendingAnchor = null }, 1500)
}
watch(activeId, async (id, oldId) => {
  if (oldId && !editingMsg.value) saveDraft(oldId, body.value) // сохранить черновик прежнего чата
  if (oldId) rememberScroll(oldId)
  pendingAnchor = null
  replyTo.value = null; editingMsg.value = null; closeCtx()
  body.value = id ? loadDraft(id) : ''
  openSettled.value = false
  if (id) {
    chatNav.lastId = id // запоминаем последний открытый чат
    const saved = chatScrollMem[id]
    stickBottom.value = !(saved && !saved.atBottom)
    nextTick(() => inputEl.value?.focus()) // фокус сразу, не ждём загрузки истории
    await openChat(id) // обновляет сообщения (единый рендер) и резолвится синхронно после
    await nextTick() // позицию ставим в микротаске ДО первой отрисовки — нет пустого кадра/перемотки
    positionAfterOpen(id, saved)
    ensureLinkPreviews() // превью ссылок — в ФОНЕ (не await!): якорь+ResizeObserver держат позицию, пока карточки грузятся
  } else closeChat()
  nextTick(autoGrow)
}, { immediate: false })

// автооткрытие самого верхнего чата, когда стоим на пустом экране (десктоп)
function maybeAutoOpen() {
  if (chatState.ready && !activeId.value && chatState.chats.length && window.innerWidth >= 640) {
    // вернуться в ПОСЛЕДНИЙ открытый чат (если он ещё существует), иначе — верхний
    const last = chatNav.lastId && chatState.chats.find((c) => c.id === chatNav.lastId) ? chatNav.lastId : chatState.chats[0].id
    router.replace({ name: 'chat', params: { id: last } })
  }
}
watch(() => [chatState.ready, chatState.chats.length, activeId.value], maybeAutoOpen)

// ── черновики: запоминаем ввод по чату (переживает уход со страницы) ──────
function draftKey(id) { return `chatDraft:${id}` }
function loadDraft(id) { try { return localStorage.getItem(draftKey(id)) || '' } catch { return '' } }
function saveDraft(id, text) { try { if ((text || '').trim()) localStorage.setItem(draftKey(id), text); else localStorage.removeItem(draftKey(id)) } catch { /* ignore */ } }
let draftTimer = null
function saveDraftDebounced(id, text) { if (draftTimer) clearTimeout(draftTimer); draftTimer = setTimeout(() => saveDraft(id, text), 300) }

const openSettled = ref(false) // после открытия чата — можно авто-читать живые входящие
watch(() => chatState.messages.length, (n, old) => {
  nextTick(scrollToBottom)
  maybeAutoReadLive(n, old)
})
// живое входящее при активном просмотре (вкладка ВИДНА и мы у нижнего края) → сразу читаем:
// без разделителя «Непрочитанные» и без роста бейджа. Ориентир — видимость, а не фокус
// (при тесте в двух окнах неактивное окно всё равно «в чате»).
function maybeAutoReadLive(n, old) {
  if (!openSettled.value || !activeId.value || !stickBottom.value) return
  if (document.visibilityState === 'hidden') return
  if (n <= (old ?? chatState.messages.length - 1)) return
  const last = chatState.messages[chatState.messages.length - 1]
  if (!last || last.author_id === chatState.meId) return
  if (last.seq && last.seq > (chatState.unreadBeforeSeq || 0)) chatState.unreadBeforeSeq = last.seq
  markActiveRead()
}
// возврат на вкладку/в окно: если чат открыт и мы у нижнего края — дочитываем видимое
// (убираем «Непрочитанные» и бейдж, шлём read → у отправителя появятся 2 галочки)
function onChatVisible() {
  if (document.visibilityState === 'hidden' || !activeId.value || !stickBottom.value || !openSettled.value) return
  let maxSeq = 0
  for (const m of chatState.messages) if (m.seq && m.seq > maxSeq) maxSeq = m.seq
  if (maxSeq > (chatState.unreadBeforeSeq || 0)) chatState.unreadBeforeSeq = maxSeq
  markActiveRead()
}
function scrollToBottom() { nextTick(() => { const el = scroller.value; if (el) el.scrollTop = el.scrollHeight }) }

// ── список чатов ─────────────────────────────────────────────────────────
const search = ref('')
const filteredChats = computed(() => {
  const q = search.value.trim().toLowerCase()
  return q ? chatState.chats.filter((c) => (c.title || '').toLowerCase().includes(q)) : chatState.chats
})
function selectChat(c) { if (Number(c.id) === activeId.value) return; router.push({ name: 'chat', params: { id: c.id } }) }

// глобальный поиск по сообщениям всех чатов (в поле списка чатов)
const globalResults = ref([])
let globalSearchTimer = null
watch(search, (v) => {
  clearTimeout(globalSearchTimer)
  const q = (v || '').trim()
  if (q.length < 2) { globalResults.value = []; return }
  globalSearchTimer = setTimeout(async () => {
    try { const { data } = await client.get('/chats/search', { params: { q } }); if (search.value.trim() === q) globalResults.value = data }
    catch { globalResults.value = [] }
  }, 300)
})
function chatTitleById(id) { const c = chatState.chats.find((x) => x.id === id); return c ? c.title : 'Чат' }
function openSearchResult(r) {
  const target = r.id, chatId = r.chat_id
  search.value = ''; globalResults.value = []
  router.push({ name: 'chat', params: { id: chatId } })
  setTimeout(() => jumpToId(target), 500) // после открытия чата
}

// перетаскивание закреплённых чатов (native drag). @mousedown.prevent ломает старт drag,
// поэтому preventDefault ставим только для незакреплённых (чтобы не терять фокус композера)
const dragChatId = ref(null)
const dragOverChatId = ref(null)
function chatMouseDown(e, c) { if (!c.pinned) e.preventDefault() }
function pinDragStart(e, c) {
  if (!c.pinned) { e.preventDefault(); return }
  dragChatId.value = c.id
  try { e.dataTransfer.effectAllowed = 'move'; e.dataTransfer.setData('text/plain', String(c.id)) } catch { /* ignore */ }
}
function pinDragOver(e, c) {
  if (dragChatId.value && c.pinned && c.id !== dragChatId.value) { e.preventDefault(); dragOverChatId.value = c.id }
}
function pinDragLeave(c) { if (dragOverChatId.value === c.id) dragOverChatId.value = null }
function pinDrop(e, c) {
  if (!dragChatId.value || !c.pinned || c.id === dragChatId.value) return
  e.preventDefault()
  const ids = chatState.chats.filter((x) => x.pinned).map((x) => x.id)
  const from = ids.indexOf(dragChatId.value), to = ids.indexOf(c.id)
  if (from >= 0 && to >= 0) { ids.splice(to, 0, ids.splice(from, 1)[0]); reorderPins(ids) }
  dragChatId.value = null; dragOverChatId.value = null
}
function pinDragEnd() { dragChatId.value = null; dragOverChatId.value = null }
function backToList() { router.push({ name: 'chat-home' }) }

// ── список: умное время / превью с автором / галки статуса ──────────────
function fmtListTime(ts) {
  if (!ts) return ''
  const d = new Date(ts); const now = new Date()
  if (d.toDateString() === now.toDateString()) return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  const a = new Date(d.getFullYear(), d.getMonth(), d.getDate())
  const b = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const diff = Math.round((b - a) / 86400000)
  if (diff >= 1 && diff < 7) return ['Вс', 'Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб'][d.getDay()]
  return `${String(d.getDate()).padStart(2, '0')}.${String(d.getMonth() + 1).padStart(2, '0')}.${d.getFullYear()}`
}
function lastPreview(c) {
  const last = c.last
  if (!last) return 'Нет сообщений'
  if (last.deleted) return 'сообщение удалено'
  const text = snippet(last.body)
  if (last.author_id === chatState.meId) return `Вы: ${text}`
  if (c.type === 'group') {
    const who = (last.author_name || c.members.find((m) => m.user_id === last.author_id)?.full_name || '').split(' ')[0]
    return who ? `${who}: ${text}` : text
  }
  return text
}
function lastStatus(c) {
  const last = c.last
  if (!last || last.author_id !== chatState.meId) return null
  if (last.status === 'pending' || last.seq == null) return 'pending'
  if (last.status === 'failed') return 'failed'
  const others = (c.members || []).filter((m) => m.user_id !== chatState.meId)
  const read = others.length && others.every((m) => (m.last_read_seq || 0) >= last.seq)
  return read ? 'read' : 'sent'
}
// разделитель «Непрочитанные» перед первым непрочитанным (чужим) сообщением
const firstUnreadKey = computed(() => {
  const base = chatState.unreadBeforeSeq || 0
  const m = chatState.messages.find((x) => x.seq != null && x.seq > base && x.author_id !== chatState.meId)
  return m ? m.client_uuid : null
})

// ── контекстное меню списка чатов (закрепить / покинуть / удалить) ──────
const listCtx = reactive({ open: false, x: 0, y: 0, c: null })
function closeListCtx() { listCtx.open = false; listCtx.c = null }
function onListContext(e, c) { e.preventDefault(); listCtx.x = e.clientX; listCtx.y = e.clientY; listCtx.c = c; listCtx.open = true }
const listCtxStyle = computed(() => ({
  left: Math.min(listCtx.x, (typeof window !== 'undefined' ? window.innerWidth : 9999) - 230) + 'px',
  top: Math.min(listCtx.y, (typeof window !== 'undefined' ? window.innerHeight : 9999) - 150) + 'px',
}))
async function listPin() { const c = listCtx.c; closeListCtx(); if (c) await pinChat(c.id, !c.pinned) }
async function listLeave() {
  const c = listCtx.c; closeListCtx()
  if (!c) return
  const isG = c.type === 'group'
  const okYes = await confirmDialog({ message: isG ? 'Покинуть группу?' : 'Удалить чат?', confirmText: isG ? 'Покинуть' : 'Удалить', cancelText: 'Отмена', danger: true })
  if (!okYes) return
  const wasActive = activeId.value === c.id
  await leaveChat(c.id)
  if (wasActive) router.replace({ name: 'chat-home' })
}

// ── реакции ──────────────────────────────────────────────────────────────
// превью-аватар: если .thumb.webp не сгенерён (старая загрузка) — падаем на оригинал
function imgFull(e, full) { const el = e.target; if (el.dataset.f || !full) return; el.dataset.f = '1'; el.src = full }
function parseReactions(m) { try { return JSON.parse(m.reactions || '[]') } catch { return [] } }
async function onChip(m, emoji) { if (m.id) await reactMessage(m.id, emoji) }

// кто поставил реакцию (ПКМ по чипу) — как на форуме
const whoMenu = reactive({ open: false, x: 0, y: 0, list: [] })
const whoStyle = computed(() => {
  const w = 240, h = 300
  const x = Math.min(whoMenu.x, window.innerWidth - w - 8)
  const y = Math.min(whoMenu.y, window.innerHeight - h - 8)
  return { left: Math.max(8, x) + 'px', top: Math.max(8, y) + 'px' }
})
function openWho(e, r) { whoMenu.open = true; whoMenu.list = r.who || []; whoMenu.x = e.clientX; whoMenu.y = e.clientY }
function closeWho() { whoMenu.open = false; whoMenu.list = [] }

// ── контекстное меню (ПКМ) ─────────────────────────────────────────────
const ctx = reactive({ open: false, x: 0, y: 0, m: null, selText: '' })
function closeCtx() { ctx.open = false; ctx.m = null }
const ctxStyle = computed(() => {
  const vw = typeof window !== 'undefined' ? window.innerWidth : 9999
  const vh = typeof window !== 'undefined' ? window.innerHeight : 9999
  return {
    left: Math.max(8, Math.min(ctx.x, vw - 220)) + 'px',
    top: Math.max(8, Math.min(ctx.y, vh - 340)) + 'px', // меню высокое (6 пунктов + реакции) — не даём уйти за низ
  }
})
function onContext(e, m) {
  if (m.deleted) return
  e.preventDefault()
  ctx.x = e.clientX; ctx.y = e.clientY; ctx.m = m
  ctx.selText = (window.getSelection?.().toString() || '').trim()
  ctx.open = true
}
const EDIT_WINDOW = 24 * 3600_000
function canEdit(m) { return isMine(m) && m.id && !m.deleted && !isVoice(m) && (Date.now() - new Date(m.created_at).getTime()) <= EDIT_WINDOW }
// в группе удалять можно только своё (для всех); в личном — любое (чужое скрывается у себя)
function canDelete(m) { return m.id && !m.deleted && (isMine(m) || activeChat.value?.type === 'direct') }
// понятная подпись действия: своё в группе — «у всех», чужое — «скрыть у меня»
function delLabel(m) {
  if (!m) return 'Удалить'
  if (!isMine(m)) return 'Скрыть у меня'
  return activeChat.value?.type === 'group' ? 'Удалить у всех' : 'Удалить'
}
function isVoice(m) { return /@\[audio\]\(/.test(m.body || '') }

function ctxReply() { startReply(ctx.m, ctx.selText); closeCtx() }
async function ctxReact(emoji) { const m = ctx.m; closeCtx(); if (m?.id) await reactMessage(m.id, emoji) }
function ctxCopy() {
  const m = ctx.m
  const text = ctx.selText || cleanBody(contentBody(m))
  closeCtx()
  if (!text) return
  navigator.clipboard?.writeText(text).catch(() => {})
}
function ctxEdit() { startEdit(ctx.m); closeCtx() }
function ctxDelete() { const m = ctx.m; closeCtx(); askDelete(m) }
// диалог удаления
const deleteTarget = ref(null)
const deleteForAll = ref(true)
const peerName = computed(() => {
  const peer = (chatState.members || []).find((x) => x.user_id !== chatState.meId)
  return peer?.full_name || 'собеседника'
})
function askDelete(m) { if (!m?.id) return; deleteTarget.value = m; deleteForAll.value = isMine(m) }
async function confirmDelete() {
  const m = deleteTarget.value; deleteTarget.value = null
  if (!m?.id) return
  const isDir = activeChat.value?.type === 'direct'
  const everyone = !isDir ? true : (isMine(m) ? deleteForAll.value : false)
  // лёгкий CSS-эффект удаления: «взрывается и исчезает» (быстрое расширение + растворение)
  const el = document.getElementById(`msg-${m.id}`)
  if (el) { el.classList.add('msg-boom'); await new Promise((r) => setTimeout(r, 300)) }
  await deleteMessage(m.id, everyone)
}
function cleanBody(b) {
  return (b || '').replace(/@\[audio\]\([^)]*\)/g, '🎤 Голосовое сообщение').replace(/@\[videonote\]\([^)]*\)/g, '📹 Видеосообщение').replace(/!\[[^\]]*\]\([^)]*\)/g, '').trim()
}

// ── выделение нескольких сообщений (переслать / удалить) ───────────────────
const selectMode = ref(false)
const selected = reactive(new Set())
function enterSelect(m) { selectMode.value = true; if (m?.id) selected.add(m.id) }
function toggleSelect(m) { if (!m?.id) return; selected.has(m.id) ? selected.delete(m.id) : selected.add(m.id) }
function exitSelect() { selectMode.value = false; selected.clear() }
// выделение перетаскиванием: зажать ЛКМ на сообщении и вести — выделяется диапазон
let dragSel = null, dragMoved = false, suppressRowClick = false
function selDragStart(e, m, i) {
  if (!selectMode.value || e.button !== 0) return
  dragSel = { anchor: i, mode: selected.has(m.id) ? 'remove' : 'add', base: new Set(selected) }
  dragMoved = false
  window.addEventListener('mouseup', selDragEnd)
}
function selDragEnter(i) {
  if (!dragSel || i === dragSel.anchor) return
  dragMoved = true
  applyDragRange(i)
}
function applyDragRange(i) {
  const a = Math.min(dragSel.anchor, i), b = Math.max(dragSel.anchor, i)
  selected.clear()
  for (const id of dragSel.base) selected.add(id)
  for (let k = a; k <= b; k++) {
    const mm = chatState.messages[k]
    if (!mm?.id) continue
    if (dragSel.mode === 'add') selected.add(mm.id); else selected.delete(mm.id)
  }
}
function selDragEnd() {
  window.removeEventListener('mouseup', selDragEnd)
  if (dragMoved) suppressRowClick = true // клик после drag не должен переключать якорь
  dragSel = null
}
function onRowClick(e, m) {
  if (!selectMode.value) return
  e.preventDefault(); e.stopPropagation()
  if (suppressRowClick) { suppressRowClick = false; return }
  toggleSelect(m)
}
const selectedMsgs = computed(() => chatState.messages.filter((m) => selected.has(m.id)))

// ── пересылка ──
const forwardOpen = ref(false)
const forwardBodies = ref([])
const forwardSearch = ref('')
const forwardSearchInput = ref(null)
const forwardList = computed(() => {
  const q = forwardSearch.value.trim().toLowerCase()
  return (chatState.chats || []).filter((c) => !q || (c.title || '').toLowerCase().includes(q))
})
function openForward(bodies) {
  forwardBodies.value = (bodies || []).filter((b) => b && b.trim())
  if (!forwardBodies.value.length) return
  forwardSearch.value = ''; forwardOpen.value = true
  nextTick(() => forwardSearchInput.value?.focus())
}
async function doForward(chatId) {
  const bodies = forwardBodies.value
  forwardOpen.value = false
  await forwardMessages(chatId, bodies)
  exitSelect()
  if (chatId === activeId.value) scrollToBottom()
  else router.replace({ name: 'chat', params: { id: chatId } }) // открыть чат, куда переслали
}
function ctxForward() { const m = ctx.m; closeCtx(); if (m) openForward([fwdWrap(m)]) }
function ctxPin() {
  const m = ctx.m; closeCtx()
  if (!m?.id || !activeChat.value) return
  if (activeChat.value.pinned_message_id === m.id) unpinMessageInChat(activeId.value)
  else pinMessageInChat(activeId.value, m.id)
}
// закреплённое сообщение (шапка-плашка)
const pinnedMsg = computed(() => {
  const pid = activeChat.value?.pinned_message_id
  if (!pid) return null
  return chatState.messages.find((m) => m.id === pid) || { id: pid }
})
const pinnedText = computed(() => { const p = pinnedMsg.value; return p?.body ? snippet(p.body) : 'Сообщение' })
const pinnedPhoto = computed(() => { const p = pinnedMsg.value; return p?.body ? firstPhotoUrl(p.body) : null })
function ctxSelect() { const m = ctx.m; closeCtx(); enterSelect(m) }
function forwardSelected() { openForward(selectedMsgs.value.map(fwdWrap)) }

// ── удаление нескольких ──
const deleteManyOpen = ref(false)
const deleteManyForAll = ref(true)
function askDeleteSelected() { if (!selected.size) return; deleteManyForAll.value = true; deleteManyOpen.value = true }
async function confirmDeleteSelected() {
  const msgs = selectedMsgs.value.slice()
  deleteManyOpen.value = false
  exitSelect() // снять выделение и убрать из UI сразу
  const isDir = activeChat.value?.type === 'direct'
  // локально исчезают сразу; для большой пачки покажем снек с прогрессом
  if (msgs.length >= 8) showToast(`Удаление ${msgs.length} сообщ.…`)
  await Promise.all(msgs.map((m) => deleteMessage(m.id, !isDir ? isMine(m) : (isMine(m) ? deleteManyForAll.value : false))))
}
// копировать можно только когда есть текст (голосовое/фото — нечего копировать)
function canCopy(m) {
  if (!m) return false
  const t = (m.body || '').replace(/@\[audio\]\([^)]*\)/g, '').replace(/@\[videonote\]\([^)]*\)/g, '').replace(/!\[[^\]]*\]\([^)]*\)/g, '').trim()
  return !!t
}

function startReply(m, selText) {
  editingMsg.value = null
  const sel = (selText || '').trim()
  replyTo.value = { id: m.id, author_name: nameOf(m), body: sel ? sel.slice(0, 200) : snippet(contentBody(m)), quote: sel ? sel.slice(0, 300) : quoteText(contentBody(m)), photo: photoUrls(m)[0] || null }
  nextTick(() => inputEl.value?.focus())
}
// миниатюра фото у отвечаемого сообщения (по reply_to_id из локальной ленты)
function replyThumb(m) {
  if (!m || !m.reply_to_id) return null
  const src = (chatState.messages || []).find((x) => x.id === m.reply_to_id)
  const u = src ? photoUrls(src)[0] : null
  return u ? thumbUrl(u) : null
}
// Текст цитаты для пересылаемого reply_quote: чистим медиа-разметку, но СОХРАНЯЕМ переносы строк.
function quoteText(b) {
  return (b || '')
    .replace(/@\[audio\]\([^)]*\)/g, '🎤 Голосовое')
    .replace(/@\[videonote\]\([^)]*\)/g, '📹 Видеосообщение')
    .replace(/@\[video\]\([^)]*\)/g, 'Видео')
    .replace(/@\[file\]\([^|)]*\|([^)]*)\)/g, (_m, name) => { try { return '📎 ' + decodeURIComponent(name) } catch { return '📎 файл' } })
    .replace(/!\[[^\]]*\]\([^)]*\)/g, 'Фото')
    .trim().slice(0, 300)
}
function startEdit(m) {
  replyTo.value = null; editingMsg.value = m
  if (isPhoto(m)) {
    // фото: в поле только подпись; медиа (и маркер пересылки) сохраняем отдельно
    const fwd = (m.body || '').match(FWD_RE)?.[0] || ''
    const imgs = []; contentBody(m).replace(/!\[[^\]]*\]\([^)]+\)/g, (x) => { imgs.push(x); return '' })
    editingMedia.value = fwd + imgs.join('\n')
    body.value = captionText(m)
  } else {
    editingMedia.value = ''; body.value = m.body
  }
  nextTick(() => { autoGrow(); inputEl.value?.focus() })
}
function cancelEdit() { editingMsg.value = null; editingMedia.value = ''; body.value = activeId.value ? loadDraft(activeId.value) : '' }
function snippet(b) {
  return (b || '')
    .replace(FWD_RE, '')
    .replace(/@\[audio\]\([^)]*\)/g, '🎤 Голосовое')
    .replace(/@\[videonote\]\([^)]*\)/g, '📹 Видеосообщение')
    .replace(/@\[video\]\([^)]*\)/g, 'Видео')
    .replace(/@\[file\]\([^|)]*\|([^)]*)\)/g, (_m, name) => { try { return '📎 ' + decodeURIComponent(name) } catch { return '📎 файл' } })
    .replace(/!\[[^\]]*\]\([^)]*\)/g, 'Фото')
    .replace(/\s+/g, ' ').trim().slice(0, 80)
}
// видео: маркер @[video](url|poster|bytes)
const VIDEO_RE = /@\[video\]\(([^|)\s]+)\|([^|)]*)\|?([^|)]*)\|?([^)]*)\)/
const VIDEO_AUTO_MAX = 10 * 1024 * 1024 // ≤10МБ авто-проигрываем, крупнее — постер с кнопкой
// уже просмотренные (скачанные) видео авто-проигрываются всегда, независимо от размера — храним в localStorage
const VIDEO_LOADED_KEY = 'chatVideoLoaded'
function loadVideoLoaded() { try { const o = {}; for (const id of JSON.parse(localStorage.getItem(VIDEO_LOADED_KEY) || '[]')) o[id] = true; return o } catch { return {} } }
const videoLoaded = reactive(loadVideoLoaded())
function markVideoLoaded(id) {
  videoLoaded[id] = true
  try { const ids = Object.keys(videoLoaded).map(Number); localStorage.setItem(VIDEO_LOADED_KEY, JSON.stringify(ids.slice(-300))) } catch { /* ignore */ }
}
function videoAuto(m) {
  const v = videoOf(m)
  if (!v) return false
  return !!videoLoaded[m.id] || (v.size > 0 && v.size <= VIDEO_AUTO_MAX)
}
// первый URL медиа-миниатюры (фото или постер видео) — для превью списка/ответа/правки
function firstPhotoUrl(b) {
  const s = (b || '').replace(FWD_RE, '')
  const vn = s.match(VIDEONOTE_RE); if (vn) return vn[2] || null
  const vm = s.match(VIDEO_RE); if (vm) return vm[2] || null
  if (/@\[audio\]|@\[file\]/.test(s)) return null
  const m = s.match(/!\[[^\]]*\]\(([^)]+)\)/)
  return m ? m[1] : null
}
function isVideoMsg(m) { return VIDEO_RE.test(m?.body || '') }
function videoOf(m) { const mm = contentBody(m).match(VIDEO_RE); if (!mm) return null; const d = (mm[4] || '').match(/(\d+)x(\d+)/); return { url: mm[1], poster: mm[2] || '', size: Number(mm[3]) || 0, w: d ? +d[1] : 0, h: d ? +d[2] : 0 } }
function lastPhoto(c) { return firstPhotoUrl(c?.last?.body) }

// ── композер: авто-рост, лимит, отправка ───────────────────────────────
function autoGrow() {
  const el = inputEl.value
  if (!el) return
  el.style.height = 'auto'
  const border = el.offsetHeight - el.clientHeight // рамка (box-sizing: border-box) — иначе поле «худеет» на 2px
  el.style.height = Math.min(el.scrollHeight + border, 160) + 'px'
}
function onInput() {
  if (body.value.length > MAX_LEN) body.value = body.value.slice(0, MAX_LEN)
  autoGrow()
}
// ── кастомное контекстное меню поля ввода (вместо браузерного) ─────────────
const inputCtx = reactive({ open: false, x: 0, y: 0, hasSel: false })
function onInputContext(e) {
  e.preventDefault()
  const el = inputEl.value
  inputCtx.hasSel = !!el && el.selectionStart !== el.selectionEnd
  inputCtx.x = Math.min(e.clientX, window.innerWidth - 210); inputCtx.y = Math.min(e.clientY, window.innerHeight - 220); inputCtx.open = true
}
async function inputAction(name) {
  inputCtx.open = false
  const el = inputEl.value; if (!el) return
  el.focus()
  const start = el.selectionStart, end = el.selectionEnd
  const sel = body.value.slice(start, end)
  try {
    if (name === 'copy' && sel) { await navigator.clipboard?.writeText(sel) }
    else if (name === 'cut' && sel) {
      await navigator.clipboard?.writeText(sel)
      body.value = body.value.slice(0, start) + body.value.slice(end)
      nextTick(() => { el.selectionStart = el.selectionEnd = start; autoGrow() })
    } else if (name === 'paste') {
      const t = await navigator.clipboard?.readText()
      if (t != null) { body.value = body.value.slice(0, start) + t + body.value.slice(end); nextTick(() => { const p = start + t.length; el.selectionStart = el.selectionEnd = p; autoGrow() }) }
    } else if (name === 'selectall') { el.select() }
  } catch { /* буфер недоступен */ }
}
watch(body, () => { nextTick(autoGrow); if (activeId.value && !editingMsg.value) saveDraftDebounced(activeId.value, body.value) })

let lastTyping = 0
function onKeydown(e) {
  // не давать браузеру сбрасывать текст поля на Escape (нативный revert);
  // Escape в поле: снимаем ответ/редактирование, но НЕ стираем набранный текст
  // (stopPropagation — чтобы не сработал нативный «сброс» поля и глобальный обработчик)
  if (e.key === 'Escape') {
    e.preventDefault(); e.stopPropagation()
    if (editingMsg.value) cancelEdit()
    else if (replyTo.value) replyTo.value = null
    else inputEl.value?.blur()
    return
  }
  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); return }
  const now = Date.now()
  if (now - lastTyping > 2000) { lastTyping = now; sendTyping() }
}

async function send() {
  const text = body.value.trim()
  if (editingMsg.value) {
    const m = editingMsg.value
    // фото: пересобираем body из медиа + новой подписи (подпись можно и убрать)
    const isMedia = !!editingMedia.value
    if (!text && !isMedia) return
    const newBody = isMedia ? (editingMedia.value + (text ? '\n' + text : '')) : text
    editingMsg.value = null; editingMedia.value = ''; body.value = ''
    await editMessage(m.id, newBody)
    return
  }
  if (!text) return
  const reply = replyTo.value?.id || null
  const quote = replyTo.value?.quote || null
  body.value = ''; replyTo.value = null
  nextTick(autoGrow)
  await sendMessage(text, reply, quote)
  scrollToBottom()
}

function insertEmoji(e) {
  const el = inputEl.value
  const pos = el?.selectionStart ?? body.value.length
  body.value = (body.value.slice(0, pos) + e + body.value.slice(pos)).slice(0, MAX_LEN)
  nextTick(() => { autoGrow(); el?.focus(); const p = pos + e.length; el?.setSelectionRange(p, p) })
}

// ── вложения ──────────────────────────────────────────────────────────────
// прямая отправка файлов (без диалога): mode 'file' | 'picture'
async function uploadAndSend(files, mode = 'file') {
  const list = Array.from(files || [])
  if (!list.length || !activeId.value) return
  uploading.value = true
  try {
    for (const f of list) {
      const fd = new FormData(); fd.append('files', f)
      let data
      try { ({ data } = await client.post('/uploads', fd, { headers: { 'Content-Type': 'multipart/form-data' } })) }
      catch (e) { showToast(e.response?.data?.detail || 'Не удалось загрузить файл'); continue }
      const url = data.urls?.[0]
      if (!url) continue
      if (mode === 'picture' && (f.type || '').startsWith('image/')) await sendMessage(`![](${url})`)
      else {
        const name = (f.name || 'файл').replace(/[|)(]/g, '_')
        await sendMessage(`@[file](${url}|${encodeURIComponent(name)})`)
      }
    }
    scrollToBottom()
  } finally { uploading.value = false }
}

// диалог отправки вложений (картинки + файлы): превью, подпись, «сжать»
const composeItems = ref([])   // [{ file, url, isImage, size }]
const composeCaption = ref('')
const composeCompress = ref(true)
const composeInput = ref(null)
const composeCaptionInput = ref(null)
const showCompose = computed(() => composeItems.value.length > 0)
const composeImages = computed(() => composeItems.value.filter((it) => it.isImage))
const composeMedia = computed(() => composeItems.value.filter((it) => it.isImage || it.isVideo))
const composeFiles = computed(() => composeItems.value.filter((it) => !it.isImage && !it.isVideo))
function plural(n) { const a = n % 10, b = n % 100; if (a === 1 && b !== 11) return 'файл'; if (a >= 2 && a <= 4 && (b < 10 || b >= 20)) return 'файла'; return 'файлов' }
const composeTitle = computed(() => {
  const img = composeImages.value.length
  const vid = composeMedia.value.length - img
  if (!composeMedia.value.length) return 'Отправить как файл'
  if (!composeFiles.value.length) {
    if (img && vid) return 'Отправить медиа'
    return vid ? 'Отправить видео' : 'Отправить изображение'
  }
  return `Выбрано ${composeItems.value.length} ${plural(composeItems.value.length)}`
})
function fmtSize(bytes) { if (!bytes) return '0 Б'; if (bytes < 1024) return `${bytes} Б`; if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} КБ`; if (bytes < 1073741824) return `${(bytes / 1048576).toFixed(1)} МБ`; return `${(bytes / 1073741824).toFixed(2)} ГБ` }
function composeAutoGrow() { const el = composeCaptionInput.value; if (!el) return; el.style.height = 'auto'; const b = el.offsetHeight - el.clientHeight; el.style.height = Math.min(el.scrollHeight + b, 160) + 'px' }
watch(showCompose, (v) => { if (v) nextTick(() => { composeCaptionInput.value?.focus(); composeAutoGrow() }) })
function addComposeItems(files, compress) {
  if (compress !== undefined) composeCompress.value = compress
  for (const f of Array.from(files)) {
    const isImage = (f.type || '').startsWith('image/')
    const isVideo = (f.type || '').startsWith('video/')
    const it = reactive({ file: f, url: (isImage || isVideo) ? URL.createObjectURL(f) : null, isImage, isVideo, size: f.size, poster: null, posterBlob: null })
    composeItems.value.push(it)
    if (isVideo) capturePoster(f).then((r) => { if (r) { it.posterBlob = r.blob; it.poster = r.url; it.dimsW = r.w; it.dimsH = r.h } })
  }
}
// постер (кадр) видео на клиенте — ffmpeg на сервере не нужен
function capturePoster(file) {
  return new Promise((resolve) => {
    try {
      const v = document.createElement('video')
      v.preload = 'metadata'; v.muted = true; v.playsInline = true
      const src = URL.createObjectURL(file); v.src = src
      let settled = false; let vw = 0; let vh = 0
      const done = (blob) => { if (settled) return; settled = true; clearTimeout(guard); URL.revokeObjectURL(src); resolve(blob ? { blob, url: URL.createObjectURL(blob), w: vw, h: vh } : null) }
      // некоторые кодеки не дают ни seeked, ни error — не ждём вечно
      const guard = setTimeout(() => done(null), 3000)
      v.onloadeddata = () => { try { v.currentTime = Math.min(0.1, (v.duration || 1) / 2) } catch { done(null) } }
      v.onseeked = () => {
        try {
          vw = v.videoWidth || 320; vh = v.videoHeight || 240
          const c = document.createElement('canvas'); c.width = vw; c.height = vh
          c.getContext('2d').drawImage(v, 0, 0, c.width, c.height)
          c.toBlob((b) => done(b), 'image/jpeg', 0.82)
        } catch { done(null) }
      }
      v.onerror = () => done(null)
    } catch { resolve(null) }
  })
}
function revokeItem(it) { if (it.url) URL.revokeObjectURL(it.url); if (it.poster) URL.revokeObjectURL(it.poster) }
function removeComposeItem(it) { const i = composeItems.value.indexOf(it); if (i < 0) return; revokeItem(it); composeItems.value.splice(i, 1) }
function cancelCompose() { composeItems.value.forEach(revokeItem); composeItems.value = []; composeCaption.value = ''; composeCompress.value = true }
function onComposeAdd(ev) { addComposeItems(ev.target.files || []); if (composeInput.value) composeInput.value.value = '' }
// оптимистичная отправка вложений: диалог закрывается мгновенно, фото сразу видно в чате
// с лоадером, загрузка идёт в фоне, затем сообщение уходит на сервер.
const pendingUploads = reactive([])
let uploadSeq = 0
function removePending(pu) {
  const i = pendingUploads.indexOf(pu); if (i >= 0) pendingUploads.splice(i, 1)
  pu.previews.forEach((p) => p.url && URL.revokeObjectURL(p.url))
}
async function uploadOne(file) {
  const fd = new FormData(); fd.append('files', file)
  const { data } = await client.post('/uploads', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
  const url = data.urls?.[0]; if (!url) throw new Error('no url')
  return url
}
async function runUpload(pu) {
  pu.failed = false
  const { cap, imgs, vids, files, chatId } = pu
  try {
    let capUsed = false
    // альбом изображений — одним сообщением
    if (imgs.length) {
      const urls = await Promise.all(imgs.map((it) => uploadOne(it.file)))
      let body = urls.map((u) => `![](${u})`).join('')
      if (cap && !vids.length && !files.length) { body += `\n${cap}`; capUsed = true }
      await sendMessageTo(chatId, body)
    }
    // видео — каждое отдельным сообщением (видео + постер)
    for (const it of vids) {
      const vurl = await uploadOne(it.file)
      let purl = ''
      if (it.posterBlob) { try { purl = await uploadOne(new File([it.posterBlob], 'poster.jpg', { type: 'image/jpeg' })) } catch { purl = '' } }
      const wh = (it.dimsW && it.dimsH) ? `|${it.dimsW}x${it.dimsH}` : ''
      let s = `@[video](${vurl}|${purl}|${it.file.size || 0}${wh})`
      if (cap && !capUsed && !files.length) { s += `\n${cap}`; capUsed = true }
      await sendMessageTo(chatId, s)
    }
    // прочие файлы
    for (const it of files) {
      const url = await uploadOne(it.file)
      const name = (it.file.name || 'файл').replace(/[|)(]/g, '_')
      let s = `@[file](${url}|${encodeURIComponent(name)})`
      if (cap && !capUsed) { s += `\n${cap}`; capUsed = true }
      await sendMessageTo(chatId, s)
      if (it.url) URL.revokeObjectURL(it.url)
    }
    removePending(pu)
    scrollToBottom()
  } catch { pu.failed = true }
}
function retryPending(pu) { runUpload(pu) }
async function sendCompose() {
  const items = [...composeItems.value]; const cap = composeCaption.value.trim(); const compress = composeCompress.value
  if (!items.length) return
  const chatId = activeId.value
  const vids = items.filter((it) => it.isVideo)
  const imgs = compress ? items.filter((it) => it.isImage) : []       // «сжать» → альбом ![]
  const files = items.filter((it) => !it.isVideo && !imgs.includes(it)) // остальное — файлами
  composeItems.value = []; composeCaption.value = ''; composeCompress.value = true
  // постер видео мог не успеть сняться к моменту нажатия «Отправить» — дожимаем (с таймаутом),
  // иначе в превью попадал бы blob самого видео и <img> ломался в «точку».
  await Promise.all(vids.map(async (it) => { if (!it.posterBlob) { const r = await capturePoster(it.file); if (r) { it.posterBlob = r.blob; it.poster = r.url; it.dimsW = r.w; it.dimsH = r.h } } }))
  const previews = [
    ...imgs.map((it) => ({ url: it.url })),
    ...vids.map((it) => ({ url: it.poster || null, isVideo: true })),
  ]
  const pu = reactive({ id: `up-${uploadSeq++}`, chatId, cap, imgs, vids, files, failed: false, previews })
  if (previews.length) { pendingUploads.push(pu); nextTick(scrollToBottom) }
  runUpload(pu)
}

async function onPickFile(ev) {
  const files = Array.from(ev.target.files || [])
  if (fileInput.value) fileInput.value.value = ''
  if (files.length) addComposeItems(files)   // всё → диалог
}
// вставка из буфера (Ctrl+V) — картинку в диалог. Ловим глобально: где бы ни был фокус,
// вставленная картинка открывает диалог отправки. Текст в чужие поля не трогаем.
async function onPaste(e) {
  if (!activeId.value) return
  const imgs = Array.from(e.clipboardData?.items || [])
    .filter((i) => i.type.startsWith('image/')).map((i) => i.getAsFile()).filter(Boolean)
  if (imgs.length) { e.preventDefault(); addComposeItems(imgs) }
}

// пересылка: маркер «@[fwd](имя|аватар)» в начале тела — кто исходный автор
const FWD_RE = /^@\[fwd\]\(([^)]*)\)\n?/
function fwdParts(m) {
  const mm = (m?.body || '').match(FWD_RE); if (!mm) return null
  const [n, a] = mm[1].split('|')
  let name = n, avatar = a || ''
  try { name = decodeURIComponent(n) } catch { /* as is */ }
  try { avatar = a ? decodeURIComponent(a) : '' } catch { avatar = '' }
  return { name, avatar }
}
function fwdName(m) { return fwdParts(m)?.name || '' }
function fwdAvatar(m) { return fwdParts(m)?.avatar || '' }
function contentBody(m) { return (m?.body || '').replace(FWD_RE, '') }
function fwdWrap(m) {
  const b = m.body || ''
  if (FWD_RE.test(b)) return b            // уже переслано — сохраняем исходного автора
  return `@[fwd](${encodeURIComponent(nameOf(m))}|${encodeURIComponent(avatarOf(m) || '')})\n${b}`
}

// содержимое сообщения: картинки / подпись / вложения
function photoUrls(m) {
  const b = contentBody(m)
  if (m.deleted || /@\[audio\]|@\[file\]/.test(b)) return []
  const urls = []; b.replace(/!\[[^\]]*\]\(([^)]+)\)/g, (_x, u) => { urls.push(u); return '' }); return urls
}
function captionText(m) { return contentBody(m).replace(/!\[[^\]]*\]\([^)]*\)/g, '').replace(VIDEO_RE, '').replace(VIDEONOTE_RE, '').trim() }
function isPhoto(m) { return photoUrls(m).length > 0 }
// все фото чата по порядку — для навигации в лайтбоксе (←/→, свайп)
// все медиа чата (фото и видео) по порядку — для навигации в лайтбоксе
const allChatMedia = computed(() => {
  const out = []
  for (const m of chatState.messages) {
    if (m.deleted) continue
    if (isVideoMsg(m)) { const v = videoOf(m); if (v) out.push({ url: v.url, mid: m.id, video: true, poster: v.poster }) }
    else for (const u of photoUrls(m)) out.push({ url: u, mid: m.id })
  }
  return out
})
function mediaIndex(m, k) {
  let idx = 0
  for (const x of chatState.messages) {
    if (x.deleted) continue
    const n = isVideoMsg(x) ? (videoOf(x) ? 1 : 0) : photoUrls(x).length
    if (x.id === m.id) return idx + (k || 0)
    idx += n
  }
  return idx
}
function openPhoto(m, k) { const i = mediaIndex(m, k); openLightbox(allChatMedia.value[i]?.url, allChatMedia.value, i) }
function openVideoLightbox(m) { const i = mediaIndex(m, 0); openLightbox(allChatMedia.value[i]?.url, allChatMedia.value, i) }
function openVideoNote(m) { const v = videoNoteOf(m); if (v) openLightbox(v.url, [{ url: v.url, mid: m.id, video: true, poster: v.poster }], 0) }

// ── память устройства / кэш ────────────────────────────────────────────────
const showStorage = ref(false)
const storageBusy = ref(false)
const storageWiping = ref(false)
const storageInfo = reactive({ usage: 0, quota: 0, cacheBytes: 0, messages: 0, chats: 0 })
const storagePct = computed(() => storageInfo.quota ? Math.min(100, Math.round(storageInfo.usage / storageInfo.quota * 100)) : 0)
async function computeStorage() {
  storageBusy.value = true
  try {
    let usage = 0, quota = 0
    try { const e = await navigator.storage?.estimate?.(); usage = e?.usage || 0; quota = e?.quota || 0 } catch { /* ignore */ }
    let cacheBytes = 0
    try { for (const k of Object.keys(localStorage)) if (/^chat/i.test(k)) cacheBytes += ((localStorage.getItem(k) || '').length + k.length) * 2 } catch { /* ignore */ }
    const st = await localCacheStats()
    storageInfo.usage = usage; storageInfo.quota = quota; storageInfo.cacheBytes = cacheBytes
    storageInfo.messages = st.messages; storageInfo.chats = st.chats
  } finally { storageBusy.value = false }
}
function openStorage() { showStorage.value = true; computeStorage() }
function clearPreviewCache() {
  try { for (const k of ['chatLinkPreviews', 'chatImgDims', 'chatVideoLoaded', 'chatInfoCache']) localStorage.removeItem(k) } catch { /* ignore */ }
  for (const k of Object.keys(linkPreviews)) delete linkPreviews[k]
  computeStorage()
}
async function wipeAllCache() {
  if (!confirm('Очистить весь локальный кэш чатов? Сообщения и медиа заново подгрузятся с сервера. Страница перезагрузится.')) return
  storageWiping.value = true
  await wipeLocalChatCache()
  location.reload()
}
// сетка-альбом под количество фото (как в мессенджерах)
function albumCols(n) { return n <= 1 ? '' : (n <= 4 ? 'grid-cols-2' : 'grid-cols-3') }
function albumItemClass(n, k) { return (n === 3 && k === 0) ? 'col-span-2' : '' } // 3 фото: первое во всю ширину
// пропорции по факту загрузки (реактивно) — бокс корректируется СРАЗУ при загрузке миниатюры,
// а не через несколько секунд на следующей перерисовке
const imgAspects = reactive({})
// картинка загрузилась/ошибка → проявляем и убираем спиннер подложки
function markImgLoaded(e) { const el = e.target; el.style.opacity = 1; el.closest('.ph-box')?.classList.add('ph-done') }
function onImgLoad(e, u) {
  const el = e.target
  markImgLoaded(e) // плавное появление поверх подложки + скрыть спиннер
  if (u && el.naturalWidth && el.naturalHeight && !imgAspects[u]) imgAspects[u] = el.naturalWidth / el.naturalHeight
}
// ЯВНЫЕ ширина+высота бокса из соотношения сторон — бокс НЕ зависит от загрузки медиа
// (иначе пузырь схлопывается по intrinsic-размеру пустого <video>/<img> и потом «прыгает»).
function boxWH(aspect, maxW, maxH) {
  let w = maxW, h = Math.round(w / aspect)
  if (h > maxH) { h = maxH; w = Math.round(h * aspect) }
  return { width: w + 'px', height: h + 'px' }
}
// резерв места под одиночное фото + подложка «в цвет» (мгновенно, до загрузки).
function photoBoxStyle(u) {
  const aspect = imgAspects[u] || imageAspect(u) || 1.4
  return { ...boxWH(aspect, wide.value ? 512 : 340, 620), background: imageColor(u) || 'rgba(190,170,145,.35)' }
}
// стиль размытой подложки-микропревью (blur-up, как в Telegram); null → нет микро
function microBg(u) { const m = imageMicro(u); return m ? { backgroundImage: `url(${m})` } : null }
// резерв места под видео. Приоритет — размеры из маркера (@[video](...|ШxВ)), иначе — по постеру.
function videoBoxStyle(v) {
  const u = v?.poster || ''
  const aspect = (v && v.w && v.h) ? (v.w / v.h) : (imgAspects[u] || imageAspect(u) || 0.7)
  return { ...boxWH(aspect, wide.value ? 546 : 340, 600), background: imageColor(u) || '#1a1614' } // ~на 30% крупнее; подложка в цвет
}

// ── превью ссылок (OG-карточки) ───────────────────────────────────────────
// url → объект превью | false (нет/в процессе). Персистим в localStorage, чтобы карточка
// показывалась мгновенно (без дёрганья ленты), и префетчим заранее по списку чатов.
const LP_KEY = 'chatLinkPreviews'
function loadLinkPreviews() { try { return JSON.parse(localStorage.getItem(LP_KEY) || '{}') || {} } catch { return {} } }
const linkPreviews = reactive(loadLinkPreviews())
let lpSaveTimer = null
function saveLinkPreviews() {
  if (lpSaveTimer) return
  lpSaveTimer = setTimeout(() => {
    lpSaveTimer = null
    try {
      const out = {}
      for (const [k, v] of Object.entries(linkPreviews)) if (v && typeof v === 'object') out[k] = v
      const keys = Object.keys(out)
      if (keys.length > 300) for (const k of keys.slice(0, keys.length - 300)) delete out[k]
      localStorage.setItem(LP_KEY, JSON.stringify(out))
    } catch { /* ignore */ }
  }, 500)
}
const URL_IN_TEXT = /https?:\/\/[^\s<]+/i
function urlInBody(b) {
  if (!b || /!\[|@\[audio\]|@\[file\]/.test(b)) return null
  const mm = b.match(URL_IN_TEXT)
  return mm ? mm[0].replace(/[)\].,!?:;»"']+$/, '') : null
}
function firstLink(m) {
  if (!m || m.deleted || isVoice(m) || isPhoto(m) || /@\[file\]/.test(m.body || '')) return null
  return urlInBody(contentBody(m))
}
function linkCard(m) {
  const u = firstLink(m)
  const p = u ? linkPreviews[u] : null
  return (p && (p.title || p.image || p.description)) ? p : null
}
// подтягиваем превью ссылок текущего окна ДО позиционирования (кэшированные — мгновенно),
// чтобы карточка не появлялась позже и не растила сообщение (нет «прыжка» при открытии)
async function ensureLinkPreviews() {
  const links = []
  for (const m of chatState.messages) { const u = firstLink(m); if (u && !(u in linkPreviews)) links.push(u) }
  if (!links.length) return
  await Promise.race([Promise.all(links.slice(0, 20).map(fetchPreview)), new Promise((r) => setTimeout(r, 450))])
}
async function fetchPreview(u) {
  if (u in linkPreviews) return
  linkPreviews[u] = false // «в процессе» — не дёргаем повторно
  try {
    const { data } = await client.get('/link-preview', { params: { url: u } })
    const ok = data && (data.title || data.image || data.description)
    linkPreviews[u] = ok ? data : false
    if (ok) saveLinkPreviews()
  } catch { linkPreviews[u] = false }
}
watch(() => chatState.messages, (msgs) => {
  for (const m of msgs || []) { const u = firstLink(m); if (u && !(u in linkPreviews)) fetchPreview(u) }
}, { immediate: true })
// префетч заранее: превью последних сообщений всех чатов — к моменту открытия уже готово
watch(() => chatState.chats, (chats) => {
  for (const c of chats || []) { const u = urlInBody(c.last?.body || ''); if (u && !(u in linkPreviews)) fetchPreview(u) }
}, { immediate: true })
// drag&drop — если все картинки: две зоны (файлом/картинкой); иначе одна зона «файлом»
const dragOver = ref(false)
const hoverZone = ref(null)
const dragAllImages = ref(false)
function onDragOver(e) {
  if (!activeId.value || !e.dataTransfer?.types?.includes('Files')) return
  e.preventDefault(); dragOver.value = true
  const items = Array.from(e.dataTransfer.items || []).filter((it) => it.kind === 'file')
  dragAllImages.value = items.length > 0 && items.every((it) => (it.type || '').startsWith('image/'))
}
function onDragLeave(e) { if (!e.relatedTarget || !e.currentTarget.contains(e.relatedTarget)) { dragOver.value = false; hoverZone.value = null } }
function onZoneDrop(e, mode) {
  e.preventDefault(); dragOver.value = false; hoverZone.value = null
  const files = Array.from(e.dataTransfer?.files || [])
  if (files.length) addComposeItems(files, mode === 'picture')  // 'picture' → сжать, 'file' → без сжатия
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
// голосовое: оптимистично показываем с индикатором загрузки на сервер (кольцо + × отмена),
// т.к. длинное голосовое может грузиться долго
const pendingVoice = reactive([])
async function onRecStop() {
  const mime = mediaRecorder?.mimeType || 'audio/webm'
  const secs = recSeconds.value
  cleanupRec()
  if (recCanceled || !recChunks.length) { recChunks = []; return }
  const blob = new Blob(recChunks, { type: mime }); recChunks = []
  const ext = mime.includes('mp4') ? 'm4a' : mime.includes('ogg') ? 'ogg' : 'webm'
  const pv = reactive({ id: `pv-${uploadSeq++}`, chatId: activeId.value, seconds: secs, sent: 0, total: blob.size || 1, failed: false, blob, ext, ctrl: null })
  pendingVoice.push(pv); nextTick(scrollToBottom)
  uploadVoice(pv)
}
async function uploadVoice(pv) {
  pv.failed = false; pv.sent = 0
  pv.ctrl = new AbortController()
  try {
    const fd = new FormData(); fd.append('files', new File([pv.blob], `voice.${pv.ext}`, { type: pv.blob.type }))
    const { data } = await client.post('/uploads', fd, {
      headers: { 'Content-Type': 'multipart/form-data' }, signal: pv.ctrl.signal,
      onUploadProgress: (e) => { pv.sent = e.loaded; if (e.total) pv.total = e.total },
    })
    const url = data.urls?.[0]
    if (url) await sendMessageTo(pv.chatId, `@[audio](${url})`)
    removeVoice(pv); scrollToBottom()
  } catch {
    if (pv.ctrl?.signal.aborted) removeVoice(pv) // отменено пользователем
    else pv.failed = true
  }
}
function removeVoice(pv) { const i = pendingVoice.indexOf(pv); if (i >= 0) pendingVoice.splice(i, 1) }
function cancelVoice(pv) { try { pv.ctrl?.abort() } catch { /* ignore */ } removeVoice(pv) }
function retryVoice(pv) { uploadVoice(pv) }
function fmtKB(b) { return `${(b / 1024).toFixed(b < 1024 * 1024 ? 1 : 0)} KB` }
function stopRec() { if (mediaRecorder && mediaRecorder.state !== 'inactive') mediaRecorder.stop() }
function cancelRec() { recCanceled = true; stopRec() }

// ── единая механика записи по удержанию (голос ↔ кружок), как в Telegram ────
const recMode = ref('voice') // 'voice' | 'video'
const holdRec = reactive({ active: false, locked: false, seconds: 0, willCancel: false, upProgress: 0 })
let holdArmTimer = null; let holdDispTimer = null; let holdStartX = 0; let holdStartY = 0; let holdStartTs = 0; let holdArmed = false; let holdMoved = false
function fmtRecMs(s) { const mm = Math.floor(s / 60); const ss = Math.floor(s % 60); const t = Math.floor((s * 10) % 10); return `${mm}:${String(ss).padStart(2, '0')},${t}` }
function recPointerDown(e) {
  if (uploading.value || holdRec.active || holdRec.locked) return
  if (e.pointerType === 'mouse' && e.button !== 0) return
  holdStartX = e.clientX; holdStartY = e.clientY; holdMoved = false; holdArmed = true
  window.addEventListener('pointermove', recPointerMove)
  window.addEventListener('pointerup', recPointerUp)
  clearTimeout(holdArmTimer)
  holdArmTimer = setTimeout(() => { if (holdArmed) beginHoldRec() }, 220) // отличаем короткий тап от удержания
}
async function beginHoldRec() {
  holdRec.active = true; holdRec.locked = false; holdRec.willCancel = false; holdRec.seconds = 0; holdRec.upProgress = 0
  holdStartTs = Date.now()
  clearInterval(holdDispTimer)
  holdDispTimer = setInterval(() => { holdRec.seconds = (Date.now() - holdStartTs) / 1000 }, 100)
  if (recMode.value === 'video') await startVideoNote(); else await startRec()
}
function recPointerMove(e) {
  const dx = e.clientX - holdStartX, dy = e.clientY - holdStartY
  if (!holdRec.active) { if (Math.abs(dx) > 6 || Math.abs(dy) > 6) holdMoved = true; return }
  if (holdRec.locked) return
  // приближение к замку (0..1) — для подсветки; закрепляем только у самого замка
  holdRec.upProgress = Math.max(0, Math.min(1, -dy / 110))
  if (dy < -110) { holdRec.locked = true; holdRec.willCancel = false; holdRec.upProgress = 1; return }
  holdRec.willCancel = dx < -60 && dy > -30 // уводим вбок → отмена (но не при движении вверх)
}
function recPointerUp() {
  clearTimeout(holdArmTimer); holdArmed = false
  window.removeEventListener('pointermove', recPointerMove)
  window.removeEventListener('pointerup', recPointerUp)
  if (!holdRec.active) { // короткий тап без удержания → переключить режим голос/кружок
    if (!holdMoved) recMode.value = recMode.value === 'voice' ? 'video' : 'voice'
    return
  }
  if (holdRec.locked) return // закреплено — ждём явной отправки/отмены кнопками
  finishHoldRec(!holdRec.willCancel)
}
function finishHoldRec(send) {
  clearInterval(holdDispTimer)
  const wasVideo = recMode.value === 'video'
  holdRec.active = false; holdRec.locked = false
  if (send) { if (wasVideo) stopVideoNote(); else stopRec() }
  else { if (wasVideo) cancelVideoNote(); else cancelRec() }
}
function lockedSend() { finishHoldRec(true) }
function lockedCancel() { finishHoldRec(false) }

// ── кружки (видео-записи с камеры) ─────────────────────────────────────────
const vnRecording = ref(false)
const vnReady = ref(false)
const vnSeconds = ref(0)
const vnFrac = ref(0) // дробный прогресс записи 0..1 (для плавного кольца)
const vnPreview = ref(null)
let vnRecorder = null; let vnChunks = []; let vnStream = null; let vnTimer = null; let vnStart = 0; let vnCanceled = false
function pickVideoMime() {
  for (const c of ['video/webm;codecs=vp9,opus', 'video/webm;codecs=vp8,opus', 'video/webm', 'video/mp4']) {
    if (window.MediaRecorder && MediaRecorder.isTypeSupported(c)) return c
  }
  return ''
}
async function startVideoNote() {
  if (vnRecording.value || recording.value) return
  if (!navigator.mediaDevices?.getUserMedia || !window.MediaRecorder) { alert('Запись не поддерживается'); return }
  try { vnStream = await navigator.mediaDevices.getUserMedia({ video: { width: 480, height: 480, facingMode: 'user' }, audio: true }) }
  catch { alert('Нет доступа к камере'); return }
  vnChunks = []; vnCanceled = false; vnReady.value = false
  vnRecording.value = true; vnSeconds.value = 0
  await nextTick()
  if (vnPreview.value) { vnPreview.value.srcObject = vnStream; vnPreview.value.muted = true; try { await vnPreview.value.play() } catch { /* ignore */ } }
  // прогрев камеры: первые кадры тёмные (авто-экспозиция/баланс белого ещё не выставились).
  // стартуем запись только после стабилизации, иначе на зацикленном кружке мигают чёрные кадры.
  await new Promise((r) => setTimeout(r, 800))
  if (!vnRecording.value || vnCanceled) { cleanupVN(); return }
  const mime = pickVideoMime()
  vnRecorder = new MediaRecorder(vnStream, mime ? { mimeType: mime } : undefined)
  vnRecorder.ondataavailable = (e) => { if (e.data && e.data.size) vnChunks.push(e.data) }
  vnRecorder.onstop = onVideoNoteStop
  vnRecorder.start()
  vnReady.value = true; vnStart = Date.now(); vnFrac.value = 0
  clearInterval(vnTimer)
  vnTimer = setInterval(() => {
    const el = Date.now() - vnStart
    vnSeconds.value = Math.floor(el / 1000); vnFrac.value = Math.min(1, el / 60000)
    if (el >= 60000) stopVideoNote()
  }, 100)
}
function cleanupVN() {
  clearInterval(vnTimer); vnTimer = null; vnRecording.value = false; vnReady.value = false
  if (vnStream) { vnStream.getTracks().forEach((t) => t.stop()); vnStream = null }
}
async function onVideoNoteStop() {
  const mime = vnRecorder?.mimeType || 'video/webm'
  // постер снимаем с последнего кадра превью ДО остановки стрима
  let posterBlob = null
  try {
    const v = vnPreview.value
    if (v && v.videoWidth) { const c = document.createElement('canvas'); c.width = v.videoWidth; c.height = v.videoHeight; c.getContext('2d').drawImage(v, 0, 0); posterBlob = await new Promise((r) => c.toBlob(r, 'image/jpeg', 0.82)) }
  } catch { /* ignore */ }
  cleanupVN()
  if (vnCanceled || !vnChunks.length) { vnChunks = []; return }
  const blob = new Blob(vnChunks, { type: mime }); vnChunks = []
  // мгновенно показываем кружок с лоадером (оптимистично), загрузка идёт в фоне
  const pn = reactive({ id: `vn-${uploadSeq++}`, chatId: activeId.value, poster: posterBlob ? URL.createObjectURL(posterBlob) : null, blob, posterBlob, mime, failed: false })
  pendingNotes.push(pn); nextTick(scrollToBottom)
  runVideoNoteUpload(pn)
}
const pendingNotes = reactive([])
function removeNote(pn) { const i = pendingNotes.indexOf(pn); if (i >= 0) pendingNotes.splice(i, 1); if (pn.poster) URL.revokeObjectURL(pn.poster) }
function retryNote(pn) { runVideoNoteUpload(pn) }
async function runVideoNoteUpload(pn) {
  pn.failed = false
  try {
    const ext = pn.mime.includes('mp4') ? 'mp4' : 'webm'
    const vurl = await uploadOne(new File([pn.blob], `videonote.${ext}`, { type: pn.mime }))
    let purl = ''
    if (pn.posterBlob) { try { purl = await uploadOne(new File([pn.posterBlob], 'poster.jpg', { type: 'image/jpeg' })) } catch { purl = '' } }
    await sendMessageTo(pn.chatId, `@[videonote](${vurl}|${purl})`)
    removeNote(pn); scrollToBottom()
  } catch { pn.failed = true }
}
function stopVideoNote() {
  if (vnRecorder && vnRecorder.state !== 'inactive') { vnRecorder.stop(); return }
  cancelVideoNote() // ещё идёт прогрев камеры — записи нет, просто отменяем
}
function cancelVideoNote() { vnCanceled = true; if (vnRecorder && vnRecorder.state !== 'inactive') vnRecorder.stop(); else cleanupVN() }
// маркер кружка @[videonote](url|poster)
const VIDEONOTE_RE = /@\[videonote\]\(([^|)\s]+)\|([^)]*)\)/
function isVideoNote(m) { return VIDEONOTE_RE.test(m?.body || '') }
function videoNoteOf(m) { const mm = contentBody(m).match(VIDEONOTE_RE); return mm ? { url: mm[1], poster: mm[2] || '' } : null }
// воспроизведение кружка со звуком по клику (как в телеге): круг прокручивается со звуком,
// вокруг — прогресс; по завершении/повторном клике возвращаемся к беззвучному циклу.
const vnEls = {}
const vnSound = reactive({})
function setVnEl(id, el) { if (el) { vnEls[id] = el } else { delete vnEls[id]; vnSound[id] = false } } // размонтировался (смена чата) — сбрасываем звук/кольцо
// webm из MediaRecorder часто отдаёт duration=Infinity — «пинаем» перемоткой, чтобы узнать длину
function fixVnDuration(e) {
  const v = e.target
  if (v && v.duration === Infinity) {
    const onSeek = () => { v.removeEventListener('seeked', onSeek); try { v.currentTime = 0 } catch { /* ignore */ } }
    v.addEventListener('seeked', onSeek)
    try { v.currentTime = 1e101 } catch { /* ignore */ }
  }
}
function muteVn(v, id) { if (v) { v.muted = true; v.loop = true } vnSound[id] = false }
// плавное кольцо прогресса: обновляем каждый кадр, пока хоть один кружок играет со звуком
let vnRaf = 0
function vnTick() {
  let any = false
  for (const id in vnSound) {
    if (!vnSound[id]) continue
    const v = vnEls[id]; if (!v) continue
    any = true
    const fin = Number.isFinite(v.duration) && v.duration > 0
    videoState[id] = { remain: fmtSec(fin ? Math.max(0, v.duration - v.currentTime) : v.currentTime), progress: fin ? v.currentTime / v.duration : 0 }
  }
  vnRaf = any ? requestAnimationFrame(vnTick) : 0
}
function toggleVnSound(m) {
  const v = vnEls[m.id]; if (!v) return
  if (vnSound[m.id]) { muteVn(v, m.id); return }
  for (const id in vnSound) if (vnSound[id]) muteVn(vnEls[id], id) // глушим другие кружки
  v.muted = false; v.loop = false; try { v.currentTime = 0 } catch { /* ignore */ }
  vnSound[m.id] = true; v.play().catch(() => {})
  if (!vnRaf) vnRaf = requestAnimationFrame(vnTick)
}
function onVnEnded(m) {
  const v = vnEls[m.id]; if (!v) return
  v.muted = true; v.loop = true; vnSound[m.id] = false
  try { v.currentTime = 0 } catch { /* ignore */ } ; v.play().catch(() => {})
}

// ── участники / статусы ───────────────────────────────────────────────────
const isMine = (m) => m.author_id === chatState.meId
// «прочитано» = хотя бы ОДИН другой участник дочитал до этого seq (и для direct, и для группы)
const othersMaxReadSeq = computed(() => {
  let max = 0
  for (const mem of chatState.members || []) {
    if (mem.user_id === chatState.meId) continue
    if ((mem.last_read_seq || 0) > max) max = mem.last_read_seq || 0
  }
  return max
})
function statusOf(m) {
  if (!isMine(m)) return null
  if (m.status === 'pending') return 'pending'
  if (m.status === 'failed') return 'failed'
  if (m.seq && othersMaxReadSeq.value >= m.seq) return 'read'
  return 'sent'
}
const typingLabel = computed(() => { const t = chatState.typing[activeId.value]; return t ? `${t.name} печатает…` : '' })
// сообщения одного автора группируются, пока между ними < 15 минут
const GROUP_GAP = 15 * 60 * 1000
function sameGroup(a, b) {
  if (!a || !b || a.author_id !== b.author_id) return false
  const ta = a.created_at ? new Date(a.created_at).getTime() : 0
  const tb = b.created_at ? new Date(b.created_at).getTime() : 0
  return Math.abs(tb - ta) <= GROUP_GAP
}
function showAuthor(m, i) {
  if (isMine(m) || !isGroup.value) return false
  return !sameGroup(chatState.messages[i - 1], m) // первый в группе
}
// широкая область переписки → все сообщения слева; узкая → свои справа, чужие слева
const convEl = ref(null)
const wide = ref(false)
let resizeObs = null

// ширина панели списка чатов — с возможностью раздвигать (перетаскиванием)
const listWidth = ref(Number(localStorage.getItem('chatListWidth')) || 320)
const isDesktop = ref(typeof window !== 'undefined' && window.innerWidth >= 640)
function onWinResize() { isDesktop.value = window.innerWidth >= 640 }
function startResize(e) {
  const startX = e.clientX
  const startW = listWidth.value
  const move = (ev) => { listWidth.value = Math.max(240, Math.min(600, startW + (ev.clientX - startX))) }
  const up = () => {
    document.removeEventListener('mousemove', move); document.removeEventListener('mouseup', up)
    document.body.style.userSelect = ''
    try { localStorage.setItem('chatListWidth', String(listWidth.value)) } catch { /* ignore */ }
  }
  document.addEventListener('mousemove', move); document.addEventListener('mouseup', up)
  document.body.style.userSelect = 'none'
  e.preventDefault()
}
const isGroup = computed(() => activeChat.value?.type === 'group')
const memberById = computed(() => { const map = {}; for (const x of chatState.members || []) map[x.user_id] = x; return map })
function avatarOf(m) { return memberById.value[m.author_id]?.avatar_url || null }
function nameOf(m) { return m.author_name || memberById.value[m.author_id]?.full_name || '' }
const myAvatar = computed(() => memberById.value[chatState.meId]?.avatar_url || null)
const myName = computed(() => memberById.value[chatState.meId]?.full_name || '')
// имя автора цитируемого сообщения (если оно ещё в загруженной ленте)
function replyAuthorName(m) {
  if (!m.reply_to_id) return ''
  const q = chatState.messages.find((x) => x.id === m.reply_to_id)
  return q ? nameOf(q) : ''
}
function isRunEnd(m, i) { return !sameGroup(m, chatState.messages[i + 1]) } // последний в группе — к нему аватар
function rowJustify(m) { return (isMine(m) && !wide.value) ? 'justify-end' : 'justify-start' }
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
// когда открывается верхняя панель плеера, она отъедает высоту сверху — не даём
// проигрываемому голосовому «уехать» под композер: доводим его до видимой области.
watch(() => player.visible, (v) => {
  if (!v) return
  nextTick(() => {
    let btn = null
    try { btn = document.querySelector(`.voice-msg[data-audio="${player.src}"]`) } catch { /* невалидный селектор */ }
    const row = btn?.closest('[id^="msg-"]')
    if (row) setTimeout(() => row.scrollIntoView({ block: 'nearest', behavior: 'smooth' }), 280)
    else if (stickBottom.value) scrollToBottom()
  })
})

let scrollSaveTimer = null
async function onScroll() {
  if (ctx.open) closeCtx()
  const el = scroller.value
  if (!el) return
  updateFloatingDate()
  stickBottom.value = (el.scrollHeight - el.scrollTop - el.clientHeight) < 60
  // всегда держим память позиции свежей (переживёт уход с роута любым способом)
  if (openSettled.value && activeId.value) { clearTimeout(scrollSaveTimer); scrollSaveTimer = setTimeout(() => rememberScroll(activeId.value), 250) }
  if (el.scrollTop < 40 && !loadingOlder) {
    loadingOlder = true
    const prevH = el.scrollHeight
    try { await loadOlder() } finally { loadingOlder = false }
    // сохраняем позицию, когда контент вырос (сервер ИЛИ расширение окна) — без рывка
    nextTick(() => { const nh = el.scrollHeight; if (nh > prevH) el.scrollTop += nh - prevH })
  }
}
let loadingOlder = false

// ── создание чата ──────────────────────────────────────────────────────
const newTab = ref('direct')
const groupTitle = ref('')
const groupMembers = ref([])
const contactSearch = ref('')
const newSearchInput = ref(null)
const filteredContacts = computed(() => {
  const q = contactSearch.value.trim().toLowerCase()
  return q ? chatState.contacts.filter((u) => (u.full_name || '').toLowerCase().includes(q)) : chatState.contacts
})
async function openNew() {
  showNew.value = true; newTab.value = 'direct'; groupTitle.value = ''; groupMembers.value = []; contactSearch.value = ''
  nextTick(() => newSearchInput.value?.focus())
  await loadContacts()
}
function closeNew() { showNew.value = false }
async function pickDirect(u) { const id = await startDirect(u.id); closeNew(); router.push({ name: 'chat', params: { id } }) }
function toggleMember(u) { const i = groupMembers.value.indexOf(u.id); if (i >= 0) groupMembers.value.splice(i, 1); else groupMembers.value.push(u.id) }
async function createGroup() {
  const title = groupTitle.value.trim()
  if (!title || !groupMembers.value.length) return
  const id = await startGroup(title, [...groupMembers.value]); closeNew(); router.push({ name: 'chat', params: { id } })
}

// ── настройки группы (название + фото) ─────────────────────────────────
// панель информации о собеседнике (личный чат)
const showInfo = ref(false)
const infoData = ref(null)
let infoCache = {}
try { infoCache = JSON.parse(localStorage.getItem('chatInfoCache') || '{}') || {} } catch { infoCache = {} }
// статус собеседника в личном чате (в сети / был(а) …)
const peerStatus = ref(null) // { online, last_seen, id }
let peerStatusTimer = null
async function refreshPeerStatus() {
  if (!activeChat.value || activeChat.value.type !== 'direct' || !activeId.value) { peerStatus.value = null; return }
  const id = activeId.value
  try {
    const { data } = await client.get(`/chats/${id}/info`)
    if (activeId.value !== id) return
    if (data?.peer) { peerStatus.value = { online: data.peer.online, last_seen: data.peer.last_seen, id: data.peer.id }; infoCache[id] = data }
  } catch { /* оставляем как есть */ }
}
const peerStatusText = computed(() => {
  const p = peerStatus.value; if (!p) return ''
  if (p.online) return 'в сети'
  if (!p.last_seen) return 'не в сети'
  const d = new Date(p.last_seen); if (isNaN(d.getTime())) return 'не в сети'
  const now = new Date(); const diff = (now - d) / 1000
  if (diff < 90) return 'был(а) недавно'
  if (diff < 3600) return `был(а) ${Math.floor(diff / 60)} мин назад`
  if (d.toDateString() === now.toDateString()) return `был(а) в ${d.toLocaleTimeString('ru', { hour: '2-digit', minute: '2-digit' })}`
  const y = new Date(now); y.setDate(now.getDate() - 1)
  if (d.toDateString() === y.toDateString()) return `был(а) вчера в ${d.toLocaleTimeString('ru', { hour: '2-digit', minute: '2-digit' })}`
  return `был(а) ${d.toLocaleDateString('ru', { day: 'numeric', month: 'short' })}`
})
watch(activeId, () => { clearInterval(peerStatusTimer); peerStatus.value = null; refreshPeerStatus(); peerStatusTimer = setInterval(refreshPeerStatus, 30000) }, { immediate: true })

// ── звонок (WebRTC 1:1 поверх чат-сокета) ──────────────────────────────────
const RTC_FALLBACK = { iceServers: [{ urls: ['stun:stun.l.google.com:19302', 'stun:stun1.l.google.com:19302'] }] }
async function getRtcConfig() {
  try { const { data } = await client.get('/turn-credentials'); if (data?.iceServers?.length) return { iceServers: data.iceServers } } catch { /* fallback */ }
  return RTC_FALLBACK
}
const call = reactive({ open: false, name: '', avatar: '', peerId: null, status: 'idle', localVideo: false, remoteVideo: false, video: false, fullscreen: false })
function toggleCallFullscreen() { call.fullscreen = !call.fullscreen }
const incoming = reactive({ open: false, name: '', avatar: '', video: false, from: null, offer: null })
const callRemoteVideo = ref(null); const callLocalVideo = ref(null)
let pc = null; let localStream = null; let remoteStream = null; let remoteAudioEl = null; let pendingIce = []
let makingOffer = false; let politePeer = false
// вкладки одного браузера общаются напрямую — надёжно гасим входящий на других вкладках
let callBC = null
try { callBC = new BroadcastChannel('mani-call'); callBC.onmessage = (e) => { if (e.data === 'handled') dropIncoming() } } catch { /* нет поддержки */ }
function dropIncoming() { if (incoming.open) { incoming.open = false; incoming.offer = null; stopRingtone() } }
const callStatusText = computed(() => call.status === 'connected' ? 'Соединено' : (call.status === 'calling' ? 'Вызов…' : 'Готов к звонку'))
function attachRemoteVideo() { if (callRemoteVideo.value && remoteStream) { callRemoteVideo.value.srcObject = remoteStream; callRemoteVideo.value.muted = true; callRemoteVideo.value.play?.().catch(() => {}) } }
// удалённое видео привязываем НАДЁЖНО: элемент появляется только при connected+remoteVideo,
// а ontrack мог сработать раньше — перепривязываем при появлении элемента
watch([() => call.status, () => call.remoteVideo], () => nextTick(attachRemoteVideo))
async function ensureLocalStream(wantVideo) {
  if (!localStream) localStream = await navigator.mediaDevices.getUserMedia({ audio: true, video: wantVideo })
  else if (wantVideo && !localStream.getVideoTracks().length) { const vs = await navigator.mediaDevices.getUserMedia({ video: true }); localStream.addTrack(vs.getVideoTracks()[0]) }
  attachLocalVideo()
  return localStream
}

async function startCall(withVideo) {
  if (!activeChat.value || activeChat.value.type !== 'direct') return
  if (!peerStatus.value?.id) await refreshPeerStatus()
  call.peerId = peerStatus.value?.id || null
  if (!call.peerId) { showToast('Не удалось определить собеседника'); return }
  call.name = activeChat.value.title || ''; call.avatar = activeChat.value.avatar_url || ''
  call.localVideo = !!withVideo; call.video = !!withVideo; call.remoteVideo = false
  call.status = 'idle-outgoing'; call.open = true
}
function setupPc(peerId, cfg, polite) {
  politePeer = !!polite; makingOffer = false
  pc = new RTCPeerConnection(cfg || RTC_FALLBACK)
  pc.onicecandidate = (e) => { if (e.candidate) sendCallSignal({ to: peerId, subtype: 'ice', candidate: e.candidate }) }
  pc.ontrack = (e) => {
    remoteStream = e.streams[0]
    if (!remoteAudioEl) { remoteAudioEl = document.createElement('audio'); remoteAudioEl.autoplay = true; document.body.appendChild(remoteAudioEl) }
    remoteAudioEl.srcObject = remoteStream
    call.remoteVideo = remoteStream.getVideoTracks().length > 0
    nextTick(attachRemoteVideo)
    // собеседник мог вкл/выкл видео — следим за составом треков
    remoteStream.onaddtrack = remoteStream.onremovetrack = () => { call.remoteVideo = remoteStream.getVideoTracks().length > 0; nextTick(attachRemoteVideo) }
  }
  pc.onconnectionstatechange = () => {
    if (!pc) return
    if (pc.connectionState === 'connected') call.status = 'connected'
    if (['failed', 'closed'].includes(pc.connectionState)) endCall()
  }
}
function attachLocalVideo() { nextTick(() => { if (callLocalVideo.value && localStream) callLocalVideo.value.srcObject = localStream }) }
async function placeCall() {
  call.status = 'calling'
  try { await ensureLocalStream(call.localVideo) }
  catch { showToast('Нет доступа к микрофону/камере'); endCall(); return }
  const rtcCfg = await getRtcConfig()
  setupPc(call.peerId, rtcCfg, false) // звонящий — «невежливый» (при коллизии не уступает)
  localStream.getTracks().forEach((t) => pc.addTrack(t, localStream))
  const offer = await pc.createOffer()
  await pc.setLocalDescription(offer)
  sendCallSignal({ to: call.peerId, subtype: 'offer', sdp: offer, video: call.localVideo, name: myName.value })
}
async function acceptIncoming() {
  call.peerId = incoming.from; call.name = incoming.name; call.avatar = incoming.avatar
  call.video = incoming.video; call.localVideo = incoming.video; call.remoteVideo = false
  const offer = incoming.offer; incoming.open = false
  sendCallSignal({ to: chatState.meId, subtype: 'handled' }) // погасить звонок на своих других вкладках
  callBC?.postMessage('handled')
  stopRingtone()
  call.open = true; call.status = 'calling'
  try { await ensureLocalStream(call.localVideo) }
  catch { showToast('Нет доступа к микрофону/камере'); endCall(); return }
  const rtcCfg = await getRtcConfig()
  setupPc(call.peerId, rtcCfg, true) // принимающий — «вежливый» (уступает при коллизии renego)
  localStream.getTracks().forEach((t) => pc.addTrack(t, localStream))
  await pc.setRemoteDescription(new RTCSessionDescription(offer))
  for (const c of pendingIce) { try { await pc.addIceCandidate(c) } catch { /* ignore */ } }
  pendingIce = []
  const answer = await pc.createAnswer()
  await pc.setLocalDescription(answer)
  sendCallSignal({ to: call.peerId, subtype: 'answer', sdp: answer })
  call.status = 'connected'
}
function rejectIncoming() {
  if (incoming.from) sendCallSignal({ to: incoming.from, subtype: 'reject' })
  sendCallSignal({ to: chatState.meId, subtype: 'handled' }) // погасить звонок на своих других вкладках
  callBC?.postMessage('handled')
  incoming.open = false; incoming.offer = null; stopRingtone()
}
function endCall() {
  const to = call.peerId || incoming.from
  if (to && (call.open || incoming.open)) sendCallSignal({ to, subtype: 'end' })
  cleanupCall()
}
function cleanupCall() {
  try { pc && pc.close() } catch { /* ignore */ }
  pc = null; pendingIce = []; remoteStream = null; makingOffer = false; politePeer = false
  if (localStream) { localStream.getTracks().forEach((t) => t.stop()); localStream = null }
  if (remoteAudioEl) { try { remoteAudioEl.srcObject = null; remoteAudioEl.remove() } catch { /* ignore */ } remoteAudioEl = null }
  call.open = false; call.status = 'idle'; call.remoteVideo = false; call.localVideo = false; call.peerId = null; call.fullscreen = false
  incoming.open = false; incoming.offer = null
  stopRingtone() // на всякий случай — гарантированно глушим гудки при любом завершении
}
async function renegotiate() {
  if (!pc) return
  try { makingOffer = true; const offer = await pc.createOffer(); await pc.setLocalDescription(offer); sendCallSignal({ to: call.peerId, subtype: 'offer', sdp: offer, renego: true }) }
  catch { /* ignore */ } finally { makingOffer = false }
}
async function toggleCallVideo() {
  const want = !call.localVideo
  try { await ensureLocalStream(want) } catch { showToast('Нет доступа к камере'); return }
  call.localVideo = want
  const vt = localStream.getVideoTracks()[0]
  if (vt) vt.enabled = want
  if (pc && call.status === 'connected') {
    const sender = pc.getSenders().find((s) => s.track && s.track.kind === 'video')
    if (want && !sender && vt) { pc.addTrack(vt, localStream); await renegotiate() } // добавили видео-дорожку → пере-согласование
    else if (sender && sender.track) sender.track.enabled = want
  }
}
async function handleCallSignal(evt) {
  const sub = evt.subtype
  if (sub === 'offer') {
    if (evt.renego && pc) { // повторное согласование (вкл/выкл видео в процессе звонка)
      const collision = makingOffer || pc.signalingState !== 'stable'
      if (!politePeer && collision) return // «невежливый» игнорирует встречный offer
      try {
        if (collision) await Promise.all([pc.setLocalDescription({ type: 'rollback' }).catch(() => {}), pc.setRemoteDescription(new RTCSessionDescription(evt.sdp))])
        else await pc.setRemoteDescription(new RTCSessionDescription(evt.sdp))
        const answer = await pc.createAnswer(); await pc.setLocalDescription(answer)
        sendCallSignal({ to: evt.from, subtype: 'answer', sdp: answer })
      } catch { /* ignore */ }
      return
    }
    if (call.status === 'connected' || call.status === 'calling' || incoming.open) { sendCallSignal({ to: evt.from, subtype: 'busy' }); return }
    incoming.open = true; incoming.from = evt.from; incoming.name = evt.name || evt.from_name || 'Вызов'
    incoming.avatar = evt.from_avatar || ''; incoming.video = !!evt.video; incoming.offer = evt.sdp
  } else if (sub === 'answer') {
    if (pc && pc.signalingState === 'have-local-offer') { await pc.setRemoteDescription(new RTCSessionDescription(evt.sdp)); if (call.status !== 'connected') call.status = 'connected' }
    for (const c of pendingIce) { try { await pc.addIceCandidate(c) } catch { /* ignore */ } }
    pendingIce = []
  } else if (sub === 'ice') {
    const cand = new RTCIceCandidate(evt.candidate)
    if (pc && pc.remoteDescription) { try { await pc.addIceCandidate(cand) } catch { /* ignore */ } } else pendingIce.push(cand)
  } else if (sub === 'handled') {
    // звонок принят/отклонён на другой вкладке этого же пользователя — гасим входящий здесь
    if (incoming.open) { incoming.open = false; incoming.offer = null; stopRingtone() }
  } else if (sub === 'end' || sub === 'reject' || sub === 'busy') {
    if (sub === 'busy') showToast('Абонент занят')
    else if (sub === 'reject') showToast('Звонок отклонён')
    cleanupCall()
  }
}
onCallSignal(handleCallSignal)
// ── рингтон (генерируем через Web Audio, без файлов) ───────────────────────
let ringCtx = null; let ringTimer = null
function startRingtone(isIncoming) {
  stopRingtone()
  try {
    ringCtx = new (window.AudioContext || window.webkitAudioContext)()
    if (ringCtx.state === 'suspended') ringCtx.resume().catch(() => {})
    const tone = (offset, freq, dur, vol) => {
      if (!ringCtx) return
      const o = ringCtx.createOscillator(), g = ringCtx.createGain()
      o.type = 'sine'; o.frequency.value = freq
      const t = ringCtx.currentTime + offset
      g.gain.setValueAtTime(0.0001, t)
      g.gain.exponentialRampToValueAtTime(vol, t + 0.03)
      g.gain.setValueAtTime(vol, t + dur - 0.05)
      g.gain.exponentialRampToValueAtTime(0.0001, t + dur)
      o.connect(g); g.connect(ringCtx.destination); o.start(t); o.stop(t + dur + 0.05)
    }
    // «ring-ring» — два коротких гудка, затем пауза; повторяем
    const beep = () => {
      if (isIncoming) { tone(0, 540, 0.4, 0.22); tone(0.55, 680, 0.4, 0.22) }
      else { tone(0, 440, 0.45, 0.2); tone(0.6, 480, 0.45, 0.2) } // исходящий гудок вызова
    }
    beep(); ringTimer = setInterval(beep, isIncoming ? 2400 : 3000)
  } catch { /* аудио недоступно */ }
}
function stopRingtone() { if (ringTimer) { clearInterval(ringTimer); ringTimer = null } if (ringCtx) { try { ringCtx.close() } catch { /* ignore */ } ringCtx = null } }
watch(() => call.status, (s) => { if (s === 'calling') startRingtone(false); else stopRingtone() })
watch(() => incoming.open, (v) => { v ? startRingtone(true) : stopRingtone() })
async function openInfo() {
  const id = activeId.value
  showInfo.value = true
  infoData.value = infoCache[id] || null // мгновенно из кэша, если открывали раньше
  try {
    const { data } = await client.get(`/chats/${id}/info`)
    infoData.value = data; infoCache[id] = data
    try { localStorage.setItem('chatInfoCache', JSON.stringify(infoCache)) } catch { /* ignore */ }
  } catch { /* оставляем кэш */ }
}
function closeInfo() { showInfo.value = false; infoData.value = null }
// инфо о конкретном пользователе (клик по имени автора в группе)
async function openUserInfo(uid) {
  if (!uid || uid === chatState.meId) return
  showInfo.value = true
  const key = 'u' + uid
  infoData.value = infoCache[key] || null
  try {
    const { data } = await client.get(`/users/${uid}/card`)
    infoData.value = { type: 'direct', peer: data }; infoCache[key] = infoData.value
    try { localStorage.setItem('chatInfoCache', JSON.stringify(infoCache)) } catch { /* ignore */ }
  } catch { /* оставляем кэш */ }
}
function maritalLabel(v) {
  const m = { single: 'Не в браке', married: 'В браке', widowed: 'Вдова / вдовец', divorced: 'В разводе', unmarried: 'Не в браке' }
  return m[v] || v
}
const infoAvatar = computed(() => { const p = infoData.value?.peer; return p ? (p.avatar || null) : null })
const cityLine = computed(() => { const p = infoData.value?.peer; return p ? [p.city, p.region].filter(Boolean).join(', ') : '' })
function pluralWord(n, forms) { const a = n % 10, b = n % 100; if (a === 1 && b !== 11) return forms[0]; if (a >= 2 && a <= 4 && (b < 10 || b >= 20)) return forms[1]; return forms[2] }
const infoCountRows = computed(() => {
  const c = infoData.value?.counts; if (!c) return []
  const rows = []
  const add = (n, icon, forms, type) => { if (n > 0) rows.push({ n, icon, label: pluralWord(n, forms), type }) }
  add(c.photos, 'image', ['фотография', 'фотографии', 'фотографий'], 'photos')
  add(c.videos, 'video', ['видео', 'видео', 'видео'], 'videos')
  add(c.files, 'paperclip', ['файл', 'файла', 'файлов'], 'files')
  add(c.links, 'link', ['ссылка', 'ссылки', 'ссылок'], 'links')
  add(c.voice, 'mic', ['голосовое', 'голосовых', 'голосовых'], 'voice')
  add(c.common_groups, 'users', ['общая группа', 'общие группы', 'общих групп'], 'groups')
  return rows
})
// ── просмотр всех медиа по типу (как в Telegram) ───────────────────────────
const mediaBrowser = reactive({ open: false, type: null, title: '', items: [], loading: false, q: '' })
const MEDIA_TITLES = { photos: 'Фотографии', videos: 'Видео', files: 'Файлы', voice: 'Голосовые сообщения', links: 'Общие ссылки' }
const MONTHS_RU = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']
async function openMediaBrowser(type) {
  if (!type || !activeId.value) return
  const isGroups = type === 'groups'
  Object.assign(mediaBrowser, { open: true, type, title: isGroups ? 'Общие группы' : (MEDIA_TITLES[type] || 'Медиа'), items: [], q: '', loading: true })
  try {
    const url = isGroups ? `/chats/${activeId.value}/common-groups` : `/chats/${activeId.value}/media`
    const { data } = await client.get(url, isGroups ? {} : { params: { type } })
    if (mediaBrowser.type === type) mediaBrowser.items = Array.isArray(data) ? data : []
  } catch { mediaBrowser.items = [] } finally { mediaBrowser.loading = false }
}
function openGroupFromBrowser(g) { closeMediaBrowser(); closeInfo(); router.push({ name: 'chat', params: { id: String(g.id) } }) }
function closeMediaBrowser() { mediaBrowser.open = false; mediaBrowser.items = []; mediaBrowser.q = '' }
function fileOf(m) { const mm = (m.body || '').match(/@\[file\]\(([^|)]+)\|([^)]*)\)/); if (!mm) return null; let name = mm[2]; try { name = decodeURIComponent(mm[2]) } catch { /* as is */ } return { url: mm[1], name } }
function audioOf(m) { const mm = (m.body || '').match(/@\[audio\]\(([^)]+)\)/); return mm ? mm[1] : null }
function monthLabel(ds) { const d = ds ? new Date(ds) : null; if (!d || isNaN(d.getTime())) return ''; const now = new Date(); return MONTHS_RU[d.getMonth()] + (d.getFullYear() !== now.getFullYear() ? ' ' + d.getFullYear() : '') }
function fileExt(name) { const m = (name || '').match(/\.([a-z0-9]{1,5})$/i); return m ? m[1].toLowerCase() : 'file' }
const fileExtColor = (e) => ({ pdf: 'bg-red-500', zip: 'bg-amber-500', rar: 'bg-amber-500', doc: 'bg-blue-500', docx: 'bg-blue-500', xls: 'bg-green-600', xlsx: 'bg-green-600' }[e] || 'bg-sage-500')
const mediaExpanded = computed(() => {
  const out = []
  for (const m of mediaBrowser.items) {
    const ca = m.created_at
    if (mediaBrowser.type === 'photos') { for (const u of photoUrls(m)) out.push({ kind: 'photo', url: u, mid: m.id, created_at: ca }) }
    else if (mediaBrowser.type === 'videos') { const v = videoOf(m); if (v) out.push({ kind: 'video', url: v.url, poster: v.poster, mid: m.id, created_at: ca }) }
    else if (mediaBrowser.type === 'files') { const f = fileOf(m); if (f) out.push({ kind: 'file', url: f.url, name: f.name, ext: fileExt(f.name), mid: m.id, created_at: ca }) }
    else if (mediaBrowser.type === 'voice') { const a = audioOf(m); if (a) out.push({ kind: 'voice', url: a, mid: m.id, created_at: ca, author: m.author_name }) }
    else if (mediaBrowser.type === 'links') { const u = urlInBody(m.body); if (u) out.push({ kind: 'link', url: u, mid: m.id, created_at: ca, preview: linkPreviews[u] || null, text: cleanBody(m.body) }) }
  }
  return out
})
const mediaGroups = computed(() => {
  const q = mediaBrowser.q.trim().toLowerCase()
  let items = mediaExpanded.value
  if (q && ['files', 'links', 'voice'].includes(mediaBrowser.type)) {
    items = items.filter((it) => `${it.name || ''} ${it.url || ''} ${it.author || ''} ${it.text || ''}`.toLowerCase().includes(q))
  }
  const groups = []; let cur = null
  for (const it of items) { const label = monthLabel(it.created_at); if (!cur || cur.label !== label) { cur = { label, items: [] }; groups.push(cur) } cur.items.push(it) }
  return groups
})
function openBrowserMedia(it) {
  const list = mediaExpanded.value.filter((x) => x.kind === 'photo' || x.kind === 'video').map((x) => ({ url: x.url, mid: x.mid, video: x.kind === 'video', poster: x.poster }))
  const idx = list.findIndex((x) => x.url === it.url && x.mid === it.mid)
  openLightbox(it.url, list, Math.max(0, idx))
}
function playBrowserVoice(it) { playAudio(it.url, `${it.author || 'Голосовое'} · ${monthLabel(it.created_at)}`) }
function openBrowserFile(it) { const a = document.createElement('a'); a.href = it.url; a.target = '_blank'; a.rel = 'noopener'; a.download = it.name || ''; document.body.appendChild(a); a.click(); a.remove() }
// ── разделители дат в ленте (как в Telegram) ───────────────────────────────
const MONTHS_RU_GEN = ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря']
function sameDayTs(a, b) { if (!a || !b) return false; const x = new Date(a), y = new Date(b); return x.getFullYear() === y.getFullYear() && x.getMonth() === y.getMonth() && x.getDate() === y.getDate() }
function showDaySep(m, i) { if (i === 0) return true; return !sameDayTs(chatState.messages[i - 1]?.created_at, m.created_at) }
function dayLabel(ds) {
  const d = ds ? new Date(ds) : null; if (!d || isNaN(d.getTime())) return ''
  const now = new Date(); const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const diff = Math.round((today - new Date(d.getFullYear(), d.getMonth(), d.getDate())) / 86400000)
  if (diff === 0) return 'Сегодня'
  if (diff === 1) return 'Вчера'
  return `${d.getDate()} ${MONTHS_RU_GEN[d.getMonth()]}` + (d.getFullYear() !== now.getFullYear() ? ' ' + d.getFullYear() : '')
}
function shareContact() {
  const p = infoData.value?.peer; if (!p) return
  const parts = [`👤 ${p.name || 'Контакт'}`]
  if (p.phone) parts.push(`📞 ${p.phone}`)
  closeInfo(); openForward([parts.join('\n')])
}

const showGroupEdit = ref(false)
const gTitle = ref('')
const gPhoto = ref('')
const gUploading = ref(false)
const groupPhotoInput = ref(null)
const gTitleInput = ref(null)
function openGroupEdit() {
  if (!isGroup.value) return
  gTitle.value = activeChat.value.title || ''
  gPhoto.value = activeChat.value.avatar_url || ''
  showGroupEdit.value = true
  nextTick(() => gTitleInput.value?.focus())
}
async function onGroupPhoto(ev) {
  const f = (ev.target.files || [])[0]
  if (groupPhotoInput.value) groupPhotoInput.value.value = ''
  if (!f || !f.type.startsWith('image/')) return
  gUploading.value = true
  try {
    const fd = new FormData(); fd.append('files', f)
    const { data } = await client.post('/uploads', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    gPhoto.value = data.urls?.[0] || gPhoto.value
  } catch { showToast('Не удалось загрузить') } finally { gUploading.value = false }
}
async function saveGroup() {
  const title = gTitle.value.trim()
  if (!title) return
  await updateChat(activeId.value, { title, photo_url: gPhoto.value || null })
  showGroupEdit.value = false
}

// ── поиск по сообщениям внутри чата (Ctrl+F) ──────────────────────────────
// scope: 'this' — по открытому чату, 'all' — по всем моим чатам (комбобокс)
const searchChat = reactive({ open: false, q: '', results: [], loading: false, scope: 'this', comboOpen: false, sel: -1 })
const searchChatInput = ref(null)
let searchChatTimer = null
function openChatSearch() {
  if (!activeId.value) return
  searchChat.open = true
  nextTick(() => { searchChatInput.value?.focus(); searchChatInput.value?.select?.() })
}
function closeChatSearch() { Object.assign(searchChat, { open: false, q: '', results: [], loading: false, comboOpen: false, sel: -1 }) }
function setSearchScope(s) { searchChat.scope = s; searchChat.comboOpen = false; searchChat.sel = -1 }
function runChatSearch() {
  clearTimeout(searchChatTimer)
  searchChat.sel = -1
  const term = (searchChat.q || '').trim()
  if (term.length < 2) { searchChat.results = []; searchChat.loading = false; return }
  searchChat.loading = true
  const scope = searchChat.scope; const cid = activeId.value
  searchChatTimer = setTimeout(async () => {
    try {
      const url = scope === 'all' ? '/chats/search' : `/chats/${cid}/search`
      const { data } = await client.get(url, { params: { q: term } })
      if (searchChat.open && searchChat.scope === scope) searchChat.results = Array.isArray(data) ? data : []
    } catch { searchChat.results = [] } finally { searchChat.loading = false }
  }, 300)
}
watch(() => [searchChat.q, searchChat.scope], runChatSearch)
// «этот чат» привязан к открытому чату — при ручной смене чата закрываем; «мои чаты» оставляем
watch(activeId, () => { if (searchChat.open && searchChat.scope === 'this') closeChatSearch() })
// клик по результату: переходим к сообщению, поиск НЕ закрываем
async function jumpToMessage(m) {
  // «Мои чаты»: переходим в нужный чат и ДОЖИДАЕМСЯ его открытия (фикс. таймаут ненадёжен —
  // иногда чат ещё не успевал открыться и переход к сообщению не срабатывал)
  if (searchChat.scope === 'all' && m.chat_id && m.chat_id !== activeId.value) {
    router.push({ name: 'chat', params: { id: String(m.chat_id) } })
    for (let i = 0; i < 60 && activeId.value !== m.chat_id; i++) await new Promise((r) => setTimeout(r, 50))
    await nextTick()
  }
  // грузим окрестности сообщения с сервера, если его нет в текущем окне, затем прыгаем
  if (m.seq && !chatState.messages.some((x) => x.id === m.id)) {
    try { await loadAroundSeq(m.seq) } catch { /* ignore */ }
  }
  await flashScrollTo(m.id)
}
function searchNav(dir) {
  const n = searchChat.results.length; if (!n) return
  searchChat.sel = (searchChat.sel + dir + n) % n
  nextTick(() => document.getElementById('sres-' + searchChat.sel)?.scrollIntoView({ block: 'nearest' }))
}
function searchEnter() { const m = searchChat.results[searchChat.sel] || searchChat.results[0]; if (m) jumpToMessage(m) }
// переход к процитированному сообщению по клику на блок цитаты
async function jumpToId(id) {
  if (!id) return
  let last = -1
  while (!chatState.messages.some((x) => x.id === id)) {
    const n = await expandWindow()   // расширяем окно, пока не покажется цель или не кончится история
    if (n === last) break
    last = n
  }
  await flashScrollTo(id)
}
function jumpToReply(m) { return jumpToId(m?.reply_to_id) }
async function flashScrollTo(id) {
  await nextTick()
  const el = document.getElementById(`msg-${id}`)
  if (el) {
    el.scrollIntoView({ block: 'center', behavior: 'smooth' })
    // держим подсветку дольше: при поиске сначала открывается чат и мотает к сообщению,
    // короткий флеш успевал погаснуть до того, как пользователь увидит сообщение
    el.classList.add('msg-flash'); setTimeout(() => el.classList.remove('msg-flash'), 3000)
  }
}

function onGlobalKey(e) {
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey && e.code === 'KeyF') {
    if (activeId.value) { e.preventDefault(); openChatSearch() }
    return
  }
  if (e.key !== 'Escape') return
  if (call.fullscreen) { call.fullscreen = false; return } // выход из полноэкранного звонка
  if (showInfo.value) { closeInfo(); return }
  if (searchChat.open) { closeChatSearch(); return }
  if (recording.value) cancelRec()
  else if (forwardOpen.value) forwardOpen.value = false
  else if (deleteManyOpen.value) deleteManyOpen.value = false
  else if (selectMode.value) exitSelect()
  else if (deleteTarget.value) deleteTarget.value = null
  else if (showCompose.value) cancelCompose()
  else if (showGroupEdit.value) showGroupEdit.value = false
  else if (showNew.value) closeNew()
  else if (showEmoji.value) showEmoji.value = false
  else if (listCtx.open) closeListCtx()
  else if (ctx.open) closeCtx()
  else if (editingMsg.value) cancelEdit()
  else if (replyTo.value) replyTo.value = null
}

// печать в любом месте страницы → в поле ввода сообщения (как в мессенджерах)
function onDocType(e) {
  if (!activeId.value || recording.value || ctx.open || showNew.value || showCompose.value) return
  if (e.ctrlKey || e.metaKey || e.altKey) return
  const t = e.target
  const tag = (t.tagName || '').toLowerCase()
  if (tag === 'input' || tag === 'textarea' || tag === 'select' || t.isContentEditable) return
  const focusEnd = () => nextTick(() => { const el = inputEl.value; if (el) { el.focus(); el.selectionStart = el.selectionEnd = el.value.length; autoGrow() } })
  if (e.key.length === 1) {
    if (body.value.length >= MAX_LEN) return
    e.preventDefault(); body.value = (body.value + e.key).slice(0, MAX_LEN); focusEnd()
  } else if (e.key === 'Backspace' && body.value) {
    e.preventDefault(); body.value = body.value.slice(0, -1); focusEnd()
  }
}

onMounted(async () => {
  document.addEventListener('keydown', onGlobalKey)
  document.addEventListener('keydown', onDocType)
  document.addEventListener('paste', onPaste)  // вставка картинки — где бы ни был фокус
  // действия контекстного меню лайтбокса (ПКМ по фото)
  setLightboxActions({
    goto: (mid) => { closeLightbox(); if (mid) flashScrollTo(mid) },
    forward: (mid) => { const m = chatState.messages.find((x) => x.id === mid); closeLightbox(); if (m) openForward([fwdWrap(m)]) },
    delete: (mid) => { const m = chatState.messages.find((x) => x.id === mid); closeLightbox(); if (m) askDelete(m) },
  })
  document.addEventListener('visibilitychange', onChatVisible)
  window.addEventListener('focus', onChatVisible)
  if (convEl.value) {
    wide.value = convEl.value.clientWidth > 900 // синхронно ДО первой отрисовки — иначе свои медиа встают справа (narrow) и прыгают влево, когда обсервер выставит wide
    if (typeof ResizeObserver !== 'undefined') {
      resizeObs = new ResizeObserver((entries) => { for (const e of entries) wide.value = e.contentRect.width > 900 })
      resizeObs.observe(convEl.value)
    }
  }
  if (!auth.isPending && auth.user) {
    await initChat({ meId: auth.user.id, getToken: () => auth.token })
    if (activeId.value) {
      const id = activeId.value; const saved = chatScrollMem[id]
      stickBottom.value = !(saved && !saved.atBottom)
      await openChat(id); await nextTick(); positionAfterOpen(id, saved); ensureLinkPreviews() // вернуться на прежнее место; превью — в фоне
    } else maybeAutoOpen()
  }
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onGlobalKey)
  document.removeEventListener('keydown', onDocType)
  document.removeEventListener('paste', onPaste)
  document.removeEventListener('visibilitychange', onChatVisible)
  window.removeEventListener('focus', onChatVisible)
  resizeObs?.disconnect()
  listObs?.disconnect()
  cancelCompose()
  if (draftTimer) clearTimeout(draftTimer)
  if (activeId.value && !editingMsg.value) saveDraft(activeId.value, body.value) // сохранить черновик при уходе
  if (activeId.value) rememberScroll(activeId.value) // сохранить позицию прокрутки при уходе с роута
  if (recording.value) { recCanceled = true; stopRec() }
  cleanupRec(); closeChat()
})
</script>

<template>
  <div class="-m-4 flex h-screen overflow-hidden bg-white sm:-m-6 lg:-m-8">
    <!-- Список чатов -->
    <aside class="flex w-full shrink-0 flex-col border-r border-parchment-200" :class="activeId ? 'hidden sm:flex' : 'flex'"
           :style="isDesktop ? { width: listWidth + 'px' } : null">
      <!-- Панель поиска по открытому чату (Ctrl+F) -->
      <template v-if="searchChat.open">
        <div class="border-b border-parchment-200 p-3">
          <div class="relative">
            <AppIcon name="search" :size="16" class="absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-700/40" />
            <input ref="searchChatInput" v-model="searchChat.q" class="input h-9 w-full pl-8 pr-8 text-sm" placeholder="Поиск"
                   @keydown.down.prevent="searchNav(1)" @keydown.up.prevent="searchNav(-1)" @keydown.enter.prevent="searchEnter" @keydown.esc.prevent.stop="closeChatSearch" />
            <button v-if="searchChat.q" @click="searchChat.q = ''" title="Очистить"
                    class="absolute right-2 top-1/2 -translate-y-1/2 text-ink-700/40 hover:text-ink-700"><AppIcon name="close" :size="15" /></button>
          </div>
        </div>
        <!-- строка «Поиск в чате:» + комбобокс выбора области + крестик закрытия -->
        <div class="border-b border-parchment-200 px-3 pb-2 pt-2">
          <div class="mb-1 text-xs font-semibold text-ink-700/40">Поиск в чате:</div>
          <div class="relative flex items-center gap-2">
            <button class="flex flex-1 items-center gap-2 rounded-lg px-1 py-1 text-left hover:bg-parchment-50" @click="searchChat.comboOpen = !searchChat.comboOpen">
              <template v-if="searchChat.scope === 'this'">
                <img v-if="activeChat?.avatar_url" :src="thumbUrl(activeChat.avatar_url)" class="h-6 w-6 shrink-0 rounded-full object-cover" />
                <span v-else class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-[10px] font-semibold text-white">{{ initials(activeChat?.title) }}</span>
                <span class="truncate text-sm font-semibold text-ink-900">Этот чат</span>
              </template>
              <template v-else>
                <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-sage-500 text-white"><AppIcon name="chat" :size="14" /></span>
                <span class="truncate text-sm font-semibold text-ink-900">Мои чаты</span>
              </template>
              <AppIcon name="chevron" :size="14" class="shrink-0 text-ink-700/40 transition" :class="searchChat.comboOpen && 'rotate-180'" />
            </button>
            <button class="rounded-lg p-1.5 text-ink-700/60 hover:bg-parchment-100" title="Закрыть поиск" @click="closeChatSearch"><AppIcon name="close" :size="18" /></button>
            <!-- выпадающий список области -->
            <template v-if="searchChat.comboOpen">
              <div class="fixed inset-0 z-20" @click="searchChat.comboOpen = false"></div>
              <div class="absolute left-0 top-full z-30 mt-1 w-56 overflow-hidden rounded-xl bg-white py-1 shadow-lg ring-1 ring-parchment-200">
                <button class="flex w-full items-center gap-2.5 px-3 py-2 text-left hover:bg-parchment-50" @click="setSearchScope('this')">
                  <img v-if="activeChat?.avatar_url" :src="thumbUrl(activeChat.avatar_url)" class="h-6 w-6 shrink-0 rounded-full object-cover" />
                  <span v-else class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-[10px] font-semibold text-white">{{ initials(activeChat?.title) }}</span>
                  <span class="flex-1 truncate text-sm font-medium text-ink-900">Этот чат</span>
                  <AppIcon v-if="searchChat.scope === 'this'" name="check" :size="16" class="text-saffron-600" />
                </button>
                <button class="flex w-full items-center gap-2.5 px-3 py-2 text-left hover:bg-parchment-50" @click="setSearchScope('all')">
                  <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-sage-500 text-white"><AppIcon name="chat" :size="14" /></span>
                  <span class="flex-1 truncate text-sm font-medium text-ink-900">Мои чаты</span>
                  <AppIcon v-if="searchChat.scope === 'all'" name="check" :size="16" class="text-saffron-600" />
                </button>
              </div>
            </template>
          </div>
        </div>
        <div class="flex-1 overflow-y-auto">
          <p v-if="searchChat.q.trim().length < 2" class="p-8 text-center text-sm text-ink-700/40">Введите минимум 2 символа</p>
          <p v-else-if="searchChat.loading && !searchChat.results.length" class="p-4 text-sm text-ink-700/50">Идёт поиск…</p>
          <p v-else-if="!searchChat.results.length" class="p-8 text-center text-sm text-ink-700/50">Ничего не найдено</p>
          <button v-for="(m, mi) in searchChat.results" :id="'sres-' + mi" :key="m.id" @click="jumpToMessage(m)"
                  class="flex w-full flex-col gap-0.5 border-b border-parchment-100 px-3 py-2.5 text-left hover:bg-parchment-50"
                  :class="mi === searchChat.sel && 'bg-saffron-500/10'">
            <span class="flex items-center justify-between gap-2">
              <span class="truncate text-sm font-medium text-ink-900">{{ searchChat.scope === 'all' ? chatTitleById(m.chat_id) : (m.author_name || 'Без имени') }}</span>
              <span class="shrink-0 text-[11px] text-ink-700/40">{{ fmtListTime(m.created_at) }}</span>
            </span>
            <span v-if="searchChat.scope === 'all' && m.author_name" class="text-xs text-ink-700/50">{{ m.author_name }}</span>
            <span class="line-clamp-2 text-sm text-ink-700/70">{{ snippet(m.body) }}</span>
          </button>
        </div>
      </template>
      <template v-else>
      <div class="flex items-center gap-2 border-b border-parchment-200 p-3">
        <div class="relative flex-1">
          <AppIcon name="search" :size="16" class="absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-700/40" />
          <input v-model="search" class="input h-9 w-full pl-8 pr-8 text-sm" placeholder="Поиск"
                 @keydown.esc.prevent.stop="search = ''" @keydown.down.stop.prevent @keydown.up.stop.prevent />
          <button v-if="search" @click="search = ''" title="Очистить"
                  class="absolute right-2.5 top-1/2 -translate-y-1/2 text-ink-700/40 hover:text-ink-700"><AppIcon name="close" :size="15" /></button>
        </div>
        <button class="h-9 shrink-0 rounded-lg px-2 text-ink-700/60 hover:bg-parchment-100 hover:text-saffron-600" title="Память устройства" @click="openStorage"><AppIcon name="settings" :size="20" /></button>
        <button class="btn-primary h-9 shrink-0 px-3" title="Новый чат" @click="openNew"><AppIcon name="plus" :size="18" /></button>
      </div>
      <div class="flex-1 overflow-y-auto">
        <p v-if="!chatState.ready" class="p-4 text-sm text-ink-700/50">Загрузка…</p>
        <p v-else-if="!filteredChats.length" class="p-4 text-sm text-ink-700/50">Чатов пока нет. Нажмите «плюс», чтобы начать.</p>
        <button v-for="c in filteredChats" :key="c.id"
                class="flex w-full items-center gap-3 border-b border-parchment-100 px-3 py-2.5 text-left hover:bg-parchment-50"
                :class="[c.id === activeId && 'bg-saffron-500/20 shadow-[inset_3px_0_0_0_theme(colors.saffron.500)]', dragChatId === c.id && 'opacity-40', dragOverChatId === c.id && 'ring-2 ring-inset ring-saffron-400']"
                :draggable="c.pinned && !search" @mousedown="chatMouseDown($event, c)" @click="selectChat(c)" @contextmenu="onListContext($event, c)"
                @dragstart="pinDragStart($event, c)" @dragover="pinDragOver($event, c)" @dragleave="pinDragLeave(c)" @drop="pinDrop($event, c)" @dragend="pinDragEnd">
          <img v-if="c.avatar_url" :src="thumbUrl(c.avatar_url)" @error="imgFull($event, c.avatar_url)" class="photo-bw h-11 w-11 shrink-0 rounded-full object-cover" />
          <span v-else class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full text-base font-semibold text-white"
                :class="c.type === 'group' ? 'bg-gradient-to-br from-sage-400 to-sage-600' : 'bg-gradient-to-br from-saffron-400 to-saffron-600'">{{ initials(c.title) }}</span>
          <span class="min-w-0 flex-1">
            <span class="flex items-center justify-between gap-2">
              <span class="flex min-w-0 items-center gap-1">
                <AppIcon v-if="c.pinned" name="pin-chat" :size="13" class="shrink-0 text-ink-700/40" />
                <span class="truncate font-medium text-ink-900">{{ c.title }}</span>
              </span>
              <span class="flex shrink-0 items-center gap-1 text-[11px] text-ink-700/40">
                <template v-if="lastStatus(c)">
                  <AppIcon v-if="lastStatus(c) === 'pending'" name="clock" :size="12" />
                  <AppIcon v-else-if="lastStatus(c) === 'failed'" name="close" :size="12" class="text-red-500" />
                  <span v-else class="text-sky-500"><AppIcon :name="lastStatus(c) === 'read' ? 'check-double' : 'check'" :size="lastStatus(c) === 'read' ? 17 : 15" class="inline" /></span>
                </template>
                {{ fmtListTime(c.last?.created_at) }}
              </span>
            </span>
            <span class="flex items-center justify-between gap-2">
              <span class="flex min-w-0 items-center gap-1 text-sm text-ink-700/60">
                <img v-if="lastPhoto(c)" :src="thumbUrl(lastPhoto(c))" class="h-4 w-4 shrink-0 rounded-sm object-cover" />
                <span class="truncate">{{ lastPreview(c) }}</span>
              </span>
              <span v-if="c.unread" class="ml-1 inline-flex h-5 min-w-[1.25rem] shrink-0 items-center justify-center rounded-full bg-saffron-500 px-1.5 text-xs font-semibold text-white">{{ c.unread }}</span>
            </span>
          </span>
        </button>

        <!-- глобальный поиск: найденные сообщения во всех чатах -->
        <template v-if="search.trim().length >= 2 && globalResults.length">
          <div class="border-y border-parchment-200 bg-parchment-50 px-3 py-1.5 text-xs font-semibold uppercase tracking-wide text-ink-700/50">Сообщения</div>
          <button v-for="r in globalResults" :key="'g' + r.id" @click="openSearchResult(r)"
                  class="flex w-full flex-col gap-0.5 border-b border-parchment-100 px-3 py-2.5 text-left hover:bg-parchment-50">
            <span class="flex items-center justify-between gap-2">
              <span class="truncate text-sm font-medium text-ink-900">{{ chatTitleById(r.chat_id) }}</span>
              <span class="shrink-0 text-[11px] text-ink-700/40">{{ fmtListTime(r.created_at) }}</span>
            </span>
            <span class="line-clamp-2 text-sm text-ink-700/70"><span v-if="r.author_name" class="text-ink-700/50">{{ r.author_name }}: </span>{{ snippet(r.body) }}</span>
          </button>
        </template>
      </div>
      </template>
    </aside>

    <!-- разделитель для изменения ширины списка -->
    <div class="hidden w-1.5 shrink-0 cursor-col-resize transition-colors hover:bg-saffron-300/50 sm:block" @mousedown="startResize"></div>

    <!-- Разговор -->
    <section ref="convEl" class="relative flex min-w-0 flex-1 flex-col" :class="activeId ? 'flex' : 'hidden sm:flex'"
             @dragover="onDragOver" @dragleave="onDragLeave">
      <template v-if="activeChat">
        <!-- Панель выделения нескольких сообщений -->
        <header v-if="selectMode" class="flex items-center gap-2 border-b border-parchment-200 px-4 py-2.5">
          <button class="rounded-lg p-1.5 text-ink-700/60 hover:bg-parchment-100" title="Отмена" @click="exitSelect"><AppIcon name="close" :size="18" /></button>
          <div class="flex-1 truncate font-medium text-ink-900">Выбрано: {{ selected.size }}</div>
          <button class="flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium text-saffron-700 hover:bg-parchment-100 disabled:opacity-40" :disabled="!selected.size" @click="forwardSelected">
            <AppIcon name="reply" :size="22" class="-scale-x-100" /> Переслать <span v-if="selected.size" class="tabular-nums">{{ selected.size }}</span>
          </button>
          <button class="flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50 disabled:opacity-40" :disabled="!selected.size" @click="askDeleteSelected">
            <AppIcon name="trash" :size="22" /> Удалить <span v-if="selected.size" class="tabular-nums">{{ selected.size }}</span>
          </button>
        </header>
        <header v-else class="flex items-center gap-3 border-b border-parchment-200 px-4 py-2.5">
          <button class="rounded-lg p-1.5 text-ink-700/60 hover:bg-parchment-100 sm:hidden" @click="backToList"><AppIcon name="chevron" :size="18" class="rotate-90" /></button>
          <div class="flex min-w-0 flex-1 cursor-pointer items-center gap-3" @click="isGroup ? openGroupEdit() : openInfo()">
            <img v-if="activeChat.avatar_url" :src="thumbUrl(activeChat.avatar_url)" @error="imgFull($event, activeChat.avatar_url)" class="photo-bw h-9 w-9 shrink-0 rounded-full object-cover" />
            <span v-else class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-sm font-semibold text-white"
                  :class="activeChat.type === 'group' ? 'bg-gradient-to-br from-sage-400 to-sage-600' : 'bg-gradient-to-br from-saffron-400 to-saffron-600'">{{ initials(activeChat.title) }}</span>
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium text-ink-900">{{ activeChat.title }}</div>
              <div class="truncate text-xs">
                <span v-if="typingLabel" class="text-saffron-600">{{ typingLabel }}</span>
                <span v-else-if="activeChat.type === 'group'" class="text-ink-700/50">{{ activeChat.members.length }} участников</span>
                <span v-else :class="peerStatus?.online ? 'text-saffron-600' : 'text-ink-700/50'">{{ peerStatusText || (chatState.connection === 'online' ? 'в сети' : 'не в сети') }}</span>
              </div>
            </div>
          </div>
          <!-- действия: поиск / звонок / информация -->
          <div class="flex shrink-0 items-center gap-1">
            <button class="rounded-full p-2 text-ink-700/55 transition hover:bg-parchment-100 hover:text-saffron-600" title="Поиск в чате" @click.stop="openChatSearch">
              <AppIcon name="search" :size="26" />
            </button>
            <button v-if="activeChat.type === 'direct'" class="rounded-full p-2 text-ink-700/55 transition hover:bg-parchment-100 hover:text-saffron-600" title="Позвонить" @click.stop="startCall(false)">
              <AppIcon name="phone" :size="26" />
            </button>
            <button class="rounded-full p-2 text-ink-700/55 transition hover:bg-parchment-100 hover:text-saffron-600" title="Информация" @click.stop="isGroup ? openGroupEdit() : openInfo()">
              <AppIcon name="info" :size="26" />
            </button>
          </div>
        </header>

        <!-- Панель информации о собеседнике (личный чат) -->
        <transition name="info-slide">
          <div v-if="showInfo" class="absolute inset-0 z-30 flex">
            <div class="absolute inset-0 bg-ink-900/30" @click="closeInfo"></div>
            <div class="relative ml-auto flex h-full w-full flex-col bg-white shadow-2xl sm:max-w-sm">
              <header class="flex items-center gap-3 border-b border-parchment-200 px-4 py-3">
                <button class="rounded-lg p-1.5 text-ink-700/60 hover:bg-parchment-100" title="Закрыть" @click="closeInfo"><AppIcon name="close" :size="18" /></button>
                <div class="font-medium text-ink-900">Информация</div>
              </header>
              <div class="flex-1 overflow-y-auto">
                <div class="flex flex-col items-center gap-3 p-6">
                  <img v-if="infoAvatar" :src="infoAvatar" class="h-28 w-28 cursor-zoom-in rounded-full object-cover ring-1 ring-parchment-200" @click="openLightbox(infoAvatar)" />
                  <span v-else class="flex h-28 w-28 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-3xl font-semibold text-white">{{ initials(activeChat.title) }}</span>
                  <div class="text-center">
                    <div class="text-lg font-semibold text-ink-900">{{ infoData?.peer?.name || activeChat.title }}</div>
                    <div class="text-sm" :class="infoData?.peer?.online ? 'text-saffron-600' : 'text-ink-700/50'">{{ infoData?.peer?.online ? 'в сети' : 'не в сети' }}</div>
                  </div>
                </div>
                <div class="divide-y divide-parchment-100 border-t border-parchment-200">
                  <div v-if="infoData?.peer?.phone" class="px-6 py-3"><div class="text-[15px] text-ink-900">{{ infoData.peer.phone }}</div><div class="text-xs text-ink-700/50">Телефон</div></div>
                  <div v-if="infoData?.peer?.spiritual_name" class="px-6 py-3"><div class="text-[15px] text-ink-900">{{ infoData.peer.spiritual_name }}</div><div class="text-xs text-ink-700/50">Духовное имя</div></div>
                  <div v-if="cityLine" class="px-6 py-3"><div class="text-[15px] text-ink-900">{{ cityLine }}</div><div class="text-xs text-ink-700/50">Город</div></div>
                </div>
                <!-- счётчики медиа (как в Telegram) -->
                <div v-if="infoCountRows.length" class="divide-y divide-parchment-100 border-t border-parchment-200">
                  <button v-for="row in infoCountRows" :key="row.icon" class="flex w-full items-center gap-3 px-6 py-3 text-left transition hover:bg-parchment-50"
                          @click="openMediaBrowser(row.type)">
                    <AppIcon :name="row.icon" :size="20" class="shrink-0 text-ink-700/50" />
                    <span class="text-[15px] text-ink-900"><b class="tabular-nums">{{ row.n }}</b> {{ row.label }}</span>
                  </button>
                </div>
                <!-- поделиться контактом -->
                <div class="border-t border-parchment-200 p-2">
                  <button class="flex w-full items-center gap-3 rounded-lg px-4 py-2.5 text-left text-[15px] text-saffron-700 hover:bg-parchment-100" @click="shareContact">
                    <AppIcon name="reply" :size="19" class="-scale-x-100" /> Поделиться контактом
                  </button>
                </div>
              </div>
            </div>
          </div>
        </transition>

        <!-- Полноэкранный просмотр медиа по типу (фото/видео/файлы/аудио/ссылки) -->
        <transition name="info-slide">
          <div v-if="mediaBrowser.open" class="absolute inset-0 z-[35] flex">
            <div class="absolute inset-0 bg-ink-900/30" @click="closeMediaBrowser"></div>
            <div class="relative ml-auto flex h-full w-full flex-col bg-white shadow-2xl sm:max-w-md">
              <header class="flex items-center gap-2 border-b border-parchment-200 px-3 py-3">
                <button class="rounded-lg p-1.5 text-ink-700/60 hover:bg-parchment-100" title="Назад" @click="closeMediaBrowser"><AppIcon name="chevron" :size="18" class="rotate-90" /></button>
                <div class="flex-1 font-medium text-ink-900">{{ mediaBrowser.title }}</div>
                <button class="rounded-lg p-1.5 text-ink-700/60 hover:bg-parchment-100" title="Закрыть" @click="closeMediaBrowser(); closeInfo()"><AppIcon name="close" :size="18" /></button>
              </header>
              <div v-if="['files', 'links', 'voice'].includes(mediaBrowser.type)" class="border-b border-parchment-200 p-3">
                <div class="relative">
                  <AppIcon name="search" :size="16" class="absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-700/40" />
                  <input v-model="mediaBrowser.q" class="input h-9 w-full pl-8 pr-3 text-sm" placeholder="Поиск" />
                </div>
              </div>
              <div class="flex-1 overflow-y-auto">
                <p v-if="mediaBrowser.loading" class="p-6 text-center text-sm text-ink-700/50">Загрузка…</p>
                <!-- общие группы — простой список -->
                <template v-else-if="mediaBrowser.type === 'groups'">
                  <p v-if="!mediaBrowser.items.length" class="p-8 text-center text-sm text-ink-700/50">Общих групп нет</p>
                  <button v-for="g in mediaBrowser.items" :key="g.id" class="flex w-full items-center gap-3 px-4 py-2.5 text-left hover:bg-parchment-50" @click="openGroupFromBrowser(g)">
                    <img v-if="g.avatar" :src="thumbUrl(g.avatar)" class="h-12 w-12 shrink-0 rounded-full object-cover" />
                    <span v-else class="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-sage-400 to-sage-600 text-base font-semibold text-white">{{ initials(g.title) }}</span>
                    <span class="truncate text-[15px] text-ink-900">{{ g.title }}</span>
                  </button>
                </template>
                <p v-else-if="!mediaGroups.length" class="p-8 text-center text-sm text-ink-700/50">Ничего не найдено</p>
                <template v-for="g in mediaGroups" :key="g.label">
                  <div class="px-4 pb-1 pt-4 text-sm font-semibold text-ink-900">{{ g.label }}</div>
                  <!-- фото/видео — сетка -->
                  <div v-if="['photos', 'videos'].includes(mediaBrowser.type)" class="grid grid-cols-3 gap-0.5 px-0.5 sm:grid-cols-4">
                    <button v-for="(it, k) in g.items" :key="k" class="relative aspect-square overflow-hidden bg-ink-900/5" @click="openBrowserMedia(it)">
                      <img :src="thumbUrl(it.kind === 'video' ? (it.poster || it.url) : it.url)" loading="lazy" class="h-full w-full object-cover" />
                      <span v-if="it.kind === 'video'" class="absolute inset-0 flex items-center justify-center"><AppIcon name="play" :size="24" class="text-white drop-shadow" /></span>
                    </button>
                  </div>
                  <!-- файлы -->
                  <div v-else-if="mediaBrowser.type === 'files'">
                    <button v-for="(it, k) in g.items" :key="k" class="flex w-full items-center gap-3 px-4 py-2.5 text-left hover:bg-parchment-50" @click="openBrowserFile(it)">
                      <span class="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg text-[11px] font-bold uppercase text-white" :class="fileExtColor(it.ext)">{{ it.ext }}</span>
                      <span class="min-w-0 flex-1"><span class="block truncate text-[15px] text-ink-900">{{ it.name }}</span><span class="block text-xs text-ink-700/50">{{ new Date(it.created_at).toLocaleDateString('ru') }}</span></span>
                    </button>
                  </div>
                  <!-- голосовые -->
                  <div v-else-if="mediaBrowser.type === 'voice'">
                    <button v-for="(it, k) in g.items" :key="k" class="flex w-full items-center gap-3 px-4 py-2.5 text-left hover:bg-parchment-50" @click="playBrowserVoice(it)">
                      <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-saffron-500 text-white"><AppIcon name="play" :size="20" /></span>
                      <span class="min-w-0 flex-1"><span class="block truncate text-[15px] text-ink-900">{{ it.author || 'Голосовое' }}</span><span class="block text-xs text-ink-700/50">{{ new Date(it.created_at).toLocaleString('ru', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }) }}</span></span>
                    </button>
                  </div>
                  <!-- ссылки -->
                  <div v-else-if="mediaBrowser.type === 'links'">
                    <a v-for="(it, k) in g.items" :key="k" :href="it.url" target="_blank" rel="noopener" class="flex gap-3 px-4 py-3 hover:bg-parchment-50">
                      <img v-if="it.preview && it.preview.image" :src="it.preview.image" class="h-12 w-12 shrink-0 rounded-lg object-cover" />
                      <span v-else class="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-sage-500 text-lg font-bold text-white">{{ (it.preview && it.preview.title ? it.preview.title : it.url).replace(/^https?:\/\/(www\.)?/, '').charAt(0).toUpperCase() }}</span>
                      <span class="min-w-0 flex-1">
                        <span v-if="it.preview && it.preview.title" class="block truncate text-[15px] font-medium text-ink-900">{{ it.preview.title }}</span>
                        <span class="block truncate text-sm text-saffron-700">{{ it.url }}</span>
                      </span>
                    </a>
                  </div>
                </template>
                <div class="h-4"></div>
              </div>
            </div>
          </div>
        </transition>

        <!-- Плашка закреплённого сообщения -->
        <div v-if="activeChat && pinnedMsg" class="flex cursor-pointer items-center gap-2 border-b border-parchment-200 bg-white/70 px-4 py-2 hover:bg-parchment-50" @click="jumpToId(pinnedMsg.id)">
          <div class="w-0.5 shrink-0 self-stretch rounded bg-saffron-400"></div>
          <img v-if="pinnedPhoto" :src="thumbUrl(pinnedPhoto)" class="h-8 w-8 shrink-0 rounded object-cover" />
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-1 text-xs font-semibold text-saffron-700"><AppIcon name="pin-chat" :size="12" /> Закреплённое сообщение</div>
            <div class="truncate text-sm text-ink-700/70">{{ pinnedText }}</div>
          </div>
          <button class="rounded-lg p-1.5 text-ink-700/50 hover:bg-parchment-100" title="Открепить" @click.stop="unpinMessageInChat(activeId)"><AppIcon name="close" :size="16" /></button>
        </div>

        <!-- плеер оверлеем поверх верха ленты — не сдвигает чат вниз при появлении -->
        <div class="relative flex min-h-0 flex-1 flex-col">
        <div class="pointer-events-none absolute inset-x-0 top-0 z-20 [&>*]:pointer-events-auto"><AudioBar /></div>

        <div ref="scroller" class="chat-bg flex flex-1 flex-col overflow-y-auto p-4"
             @scroll="onScroll" @click="onScrollerClick" @mousedown="onScrollerDown" @touchstart="onScrollerDown" @contextmenu.prevent>
          <div ref="listWrap" class="mt-auto space-y-1">
          <template v-for="(m, i) in chatState.messages" :key="m.client_uuid">
          <!-- встроенная плашка даты между днями (остаётся в ленте); плавающая сверху её дублирует
               только когда встроенная ушла вверх за экран (см. updateFloatingDate) -->
          <div v-if="showDaySep(m, i)" :data-daysep="dayLabel(m.created_at)" class="my-2 flex justify-center">
            <span class="rounded-full bg-ink-900/55 px-3 py-1 text-xs font-semibold text-white shadow-sm">{{ dayLabel(m.created_at) }}</span>
          </div>
          <div v-if="m.client_uuid === firstUnreadKey" class="my-3 flex items-center gap-2 px-2">
            <span class="h-px flex-1 bg-saffron-400/60"></span>
            <span class="rounded-full bg-saffron-500 px-3 py-0.5 text-xs font-semibold text-white shadow-sm">Непрочитанные</span>
            <span class="h-px flex-1 bg-saffron-400/60"></span>
          </div>
          <div :id="`msg-${m.id}`"
               class="group relative flex items-end gap-2 rounded-xl px-1 transition-colors"
               :class="[rowJustify(m), selectMode && 'cursor-pointer select-none pr-10', selectMode && selected.has(m.id) && 'bg-saffron-500/10']"
               @click.capture="onRowClick($event, m)"
               @mousedown="selDragStart($event, m, i)" @mouseenter="selDragEnter(i)">
            <!-- чекбокс выбора: у ПРАВОГО края (сообщения остаются на своих сторонах) -->
            <div v-if="selectMode" class="absolute right-1.5 top-1/2 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-full border-2 transition" :class="selected.has(m.id) ? 'border-saffron-500 bg-saffron-500 text-white' : 'border-ink-700/25 bg-white/50'">
              <AppIcon v-if="selected.has(m.id)" name="check" :size="14" />
            </div>
            <!-- аватар (в группах, слева от сообщения — и у чужих, и у своих) -->
            <template v-if="isGroup && !isMine(m)">
              <img v-if="avatarOf(m) && isRunEnd(m, i)" :src="thumbUrl(avatarOf(m))" @error="imgFull($event, avatarOf(m))" class="photo-bw sticky bottom-1.5 h-10 w-10 shrink-0 rounded-full object-cover" />
              <span v-else-if="isRunEnd(m, i)" class="sticky bottom-1.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-sage-400 to-sage-600 text-sm font-semibold text-white">{{ initials(nameOf(m)) }}</span>
              <span v-else class="h-10 w-10 shrink-0"></span>
            </template>
            <template v-else-if="isGroup && isMine(m)">
              <img v-if="myAvatar && isRunEnd(m, i)" :src="thumbUrl(myAvatar)" @error="imgFull($event, myAvatar)" class="photo-bw h-10 w-10 shrink-0 rounded-full object-cover" />
              <span v-else-if="isRunEnd(m, i)" class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-sm font-semibold text-white">{{ initials(myName) }}</span>
              <span v-else class="h-10 w-10 shrink-0"></span>
            </template>
            <!-- ФОТО-сообщение: без «полей» пузыря (как в телеге) -->
            <!-- кружок (видео-запись) — круглый плеер: авто muted+loop, клик → звук + кольцо прогресса -->
            <div v-if="isVideoNote(m)" class="flex flex-col gap-1" @contextmenu="onContext($event, m)">
              <div class="relative h-[19.5rem] w-[19.5rem]">
                <div class="h-full w-full overflow-hidden rounded-full bg-ink-900/[0.06] shadow-sm">
                  <video :ref="(el) => setVnEl(m.id, el)" :src="videoNoteOf(m).url" :poster="thumbUrl(videoNoteOf(m).poster || '')"
                         autoplay muted loop playsinline disablepictureinpicture controlslist="nodownload noremoteplayback nofullscreen"
                         class="pointer-events-none h-full w-full -scale-x-100 object-cover"
                         @loadedmetadata="fixVnDuration" @timeupdate="onVideoTime($event, m)" @ended="onVnEnded(m)"></video>
                </div>
                <!-- кольцо прогресса при проигрывании со звуком -->
                <svg v-if="vnSound[m.id]" class="pointer-events-none absolute inset-0 h-full w-full -rotate-90" viewBox="0 0 100 100">
                  <circle cx="50" cy="50" r="48.5" fill="none" stroke="rgba(255,255,255,0.3)" stroke-width="2" />
                  <circle cx="50" cy="50" r="48.5" fill="none" stroke="#fff" stroke-width="2.5" stroke-linecap="round"
                          :stroke-dasharray="304.7" :stroke-dashoffset="304.7 * (1 - (videoState[m.id]?.progress || 0))" />
                </svg>
                <div class="absolute inset-0 cursor-pointer rounded-full" @click.stop="toggleVnSound(m)" title="Включить звук"></div>
                <!-- отсчёт + звук: тёмная плашка у ЛЕВОГО края квадрата кружка -->
                <span class="pointer-events-none absolute bottom-2 left-0 flex items-center gap-1.5 rounded-full bg-black/55 px-3 py-1 text-[15px] text-white">
                  <AppIcon :name="vnSound[m.id] ? 'volume' : 'volume-x'" :size="19" /><span class="tabular-nums">{{ videoState[m.id]?.remain || '' }}</span>
                </span>
                <!-- время + статус: тёмная плашка у ПРАВОГО края квадрата кружка -->
                <span class="pointer-events-none absolute bottom-2 right-0 flex items-center gap-1 rounded-full bg-black/55 px-2.5 py-1 text-[13px] text-white">
                  <span class="tabular-nums">{{ fmtTime(m.created_at) }}</span>
                  <template v-if="statusOf(m)"><AppIcon v-if="statusOf(m) === 'pending'" name="clock" :size="15" /><AppIcon v-else-if="statusOf(m) === 'read'" name="check-double" :size="16" /><AppIcon v-else-if="statusOf(m) === 'sent'" name="check" :size="15" /></template>
                </span>
              </div>
              <div v-if="parseReactions(m).length" class="flex flex-wrap gap-1 px-1" :class="isMine(m) ? 'justify-end' : ''">
                <button v-for="r in parseReactions(m)" :key="r.emoji" @click.stop="onChip(m, r.emoji)" @contextmenu.prevent.stop="openWho($event, r)"
                        class="flex items-center gap-1 rounded-full px-2 py-0.5 leading-none ring-1 transition"
                        :class="m.my_reaction === r.emoji ? 'bg-saffron-500/25 text-saffron-800 ring-saffron-400' : 'bg-saffron-500/10 text-ink-700 ring-transparent hover:bg-saffron-500/20'"><span class="text-base leading-none">{{ r.emoji }}</span><span v-if="r.count > 1" class="text-sm font-semibold tabular-nums">{{ r.count }}</span></button>
              </div>
            </div>
            <!-- видео-сообщение -->
            <div v-else-if="isVideoMsg(m)" class="relative overflow-hidden rounded-2xl shadow-sm"
                 :class="[wide ? 'max-w-[560px]' : 'max-w-[85%]', isMine(m) ? 'bg-saffron-500 text-white' : 'bg-white text-ink-900 ring-1 ring-parchment-200']"
                 @contextmenu="onContext($event, m)">
              <div v-if="showAuthor(m, i)" class="cursor-pointer px-3 pt-2 text-sm font-semibold text-sage-600" @click.stop="openUserInfo(m.author_id)">{{ nameOf(m) }}</div>
              <div v-if="fwdName(m)" class="flex items-center gap-1.5 px-3 pt-2 text-sm font-semibold" :class="isMine(m) ? 'text-white/90' : 'text-saffron-700'">
                <AppIcon name="reply" :size="12" class="-scale-x-100" /> <span>Переслано от</span>
                <img v-if="fwdAvatar(m)" :src="thumbUrl(fwdAvatar(m))" class="h-4 w-4 shrink-0 rounded-full object-cover" />
                <span v-else class="flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-saffron-500 text-[8px] font-semibold text-white">{{ initials(fwdName(m)) }}</span>
                <span class="truncate">{{ fwdName(m) }}</span>
              </div>
              <!-- ≤10МБ или уже запущено: авто muted+loop + таймер; крупнее: постер с кнопкой (тот же размер).
                   Место резервируется по постеру (object-cover) — при открытии чата не «прыгает». -->
              <div class="video-box ph-box relative flex justify-center overflow-hidden" :style="videoBoxStyle(videoOf(m))">
                <span class="ph-spin pointer-events-none absolute inset-0 flex items-center justify-center"><span class="h-8 w-8 animate-spin rounded-full border-2 border-white/45 border-t-white/90"></span></span>
                <template v-if="videoAuto(m)">
                  <video :src="videoOf(m).url" :poster="thumbUrl(videoOf(m).poster || '')" autoplay muted loop playsinline
                         disablepictureinpicture controlslist="nodownload noremoteplayback nofullscreen"
                         style="opacity:0;transition:opacity .35s ease" class="pointer-events-none relative block h-full w-full object-cover" @loadeddata="markImgLoaded($event)" @timeupdate="onVideoTime($event, m)"></video>
                  <div class="absolute inset-0 cursor-pointer" @click.stop="openVideoLightbox(m)"></div>
                  <span class="pointer-events-none absolute left-2 top-2 flex items-center gap-1 rounded-full bg-black/55 px-2.5 py-1 text-sm text-white">
                    <span class="tabular-nums">{{ videoState[m.id]?.remain || '' }}</span>
                    <AppIcon name="volume-x" :size="15" />
                  </span>
                </template>
                <template v-else>
                  <img :src="thumbUrl(videoOf(m)?.poster || '')" @error="imgFull($event, videoOf(m)?.poster); markImgLoaded($event)" @load="onImgLoad($event, videoOf(m)?.poster)"
                       style="opacity:0;transition:opacity .35s ease" class="relative block h-full w-full object-cover" />
                  <button class="absolute inset-0 flex items-center justify-center" @click.stop="markVideoLoaded(m.id); openVideoLightbox(m)" title="Смотреть видео">
                    <span class="flex h-14 w-14 items-center justify-center rounded-full bg-black/50 text-white ring-2 ring-white/40"><AppIcon name="play" :size="26" /></span>
                  </button>
                </template>
              </div>
              <div v-if="captionText(m)" class="markdown-body break-words px-3.5 pt-1.5 text-[15px]" :class="isMine(m) && 'markdown-on-accent'" v-html="renderChatBody(captionText(m))"></div>
              <div class="flex items-end justify-between gap-2 px-2.5 pb-1.5 pt-1">
                <div class="flex flex-wrap gap-1">
                  <button v-for="r in parseReactions(m)" :key="r.emoji" @click.stop="onChip(m, r.emoji)" @contextmenu.prevent.stop="openWho($event, r)" title="ПКМ — кто поставил"
                          class="flex items-center gap-1 rounded-full px-2 py-0.5 leading-none ring-1 transition"
                          :class="m.my_reaction === r.emoji ? 'bg-saffron-500/25 text-saffron-800 ring-saffron-400' : 'bg-saffron-500/10 text-ink-700 ring-transparent hover:bg-saffron-500/20'"><span class="text-lg leading-none">{{ r.emoji }}</span><span v-if="r.count < 4 && r.who && r.who.length" class="-my-0.5 -mr-2 flex items-center"><template v-for="(w, wi) in r.who" :key="wi"><img v-if="w.avatar" :src="thumbUrl(w.avatar)" class="block h-[22px] w-[22px] rounded-full object-cover" :class="wi > 0 && '-ml-2'" /><span v-else class="flex h-[22px] w-[22px] items-center justify-center rounded-full bg-sage-500 text-[9px] font-semibold text-white" :class="wi > 0 && '-ml-2'">{{ initials(w.name) }}</span></template></span><span v-else-if="r.count > 1" class="text-sm font-semibold tabular-nums">{{ r.count }}</span></button>
                </div>
                <div class="flex shrink-0 items-center gap-1 pb-0.5 text-[11px]" :class="isMine(m) ? 'text-white/70' : 'text-ink-700/40'">
                  <span>{{ fmtTime(m.created_at) }}</span>
                  <template v-if="statusOf(m)"><AppIcon v-if="statusOf(m) === 'pending'" name="clock" :size="15" /><AppIcon v-else-if="statusOf(m) === 'read'" name="check-double" :size="16" /><AppIcon v-else-if="statusOf(m) === 'sent'" name="check" :size="15" /></template>
                </div>
              </div>
            </div>

            <div v-else-if="isPhoto(m)" class="relative overflow-hidden rounded-2xl shadow-sm"
                 :class="[wide ? 'max-w-[420px]' : 'max-w-[80%]', (captionText(m) || fwdName(m) || showAuthor(m, i)) && (isMine(m) ? 'bg-saffron-500 text-white' : 'bg-white text-ink-900 ring-1 ring-parchment-200')]"
                 @contextmenu="onContext($event, m)">
              <div v-if="showAuthor(m, i)" class="cursor-pointer px-3 pt-2 text-sm font-semibold text-sage-600" @click.stop="openUserInfo(m.author_id)">{{ nameOf(m) }}</div>
              <div v-if="fwdName(m)" class="flex items-center gap-1.5 px-3 pt-2 text-sm font-semibold" :class="isMine(m) ? 'text-white/90' : 'text-saffron-700'">
                <AppIcon name="reply" :size="12" class="-scale-x-100" /> <span>Переслано от</span>
                <img v-if="fwdAvatar(m)" :src="thumbUrl(fwdAvatar(m))" class="h-4 w-4 shrink-0 rounded-full object-cover" />
                <span v-else class="flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-saffron-500 text-[8px] font-semibold text-white">{{ initials(fwdName(m)) }}</span>
                <span class="truncate">{{ fwdName(m) }}</span>
              </div>
              <div v-if="m.reply_preview" @click.stop="jumpToReply(m)" class="mx-3 mt-2 flex cursor-pointer items-center gap-2 rounded-r-md border-l-2 border-saffron-400 bg-black/5 py-1 pl-2 pr-2 text-xs transition hover:bg-black/10">
                <img v-if="replyThumb(m)" :src="replyThumb(m)" class="h-8 w-8 shrink-0 rounded object-cover" />
                <div class="min-w-0 flex-1">
                  <div v-if="replyAuthorName(m)" class="font-semibold text-saffron-700">{{ replyAuthorName(m) }}</div>
                  <div class="whitespace-pre-wrap break-words text-ink-700/70">{{ m.reply_preview }}</div>
                </div>
              </div>
              <div v-if="photoUrls(m).length === 1" class="ph-box relative w-full overflow-hidden" :style="photoBoxStyle(photoUrls(m)[0])">
                <div v-if="microBg(photoUrls(m)[0])" class="ph-blur" :style="microBg(photoUrls(m)[0])"></div>
                <span class="ph-spin pointer-events-none absolute inset-0 flex items-center justify-center"><span class="h-7 w-7 animate-spin rounded-full border-2 border-white/45 border-t-white/90"></span></span>
                <img :src="photoUrls(m)[0]" @error="imgFull($event, photoUrls(m)[0]); markImgLoaded($event)" @load="onImgLoad($event, photoUrls(m)[0])"
                     class="relative block h-full w-full cursor-zoom-in object-cover" @click.stop="openPhoto(m, 0)" />
              </div>
              <div v-else class="grid gap-0.5" :class="albumCols(photoUrls(m).length)" :style="{ width: (wide ? 512 : 340) + 'px' }">
                <div v-for="(u, k) in photoUrls(m).slice(0, 10)" :key="k" class="ph-box relative aspect-square overflow-hidden" :class="albumItemClass(photoUrls(m).length, k)" :style="{ background: imageColor(u) || 'rgba(190,170,145,.35)' }">
                  <div v-if="microBg(u)" class="ph-blur" :style="microBg(u)"></div>
                  <span class="ph-spin pointer-events-none absolute inset-0 flex items-center justify-center"><span class="h-6 w-6 animate-spin rounded-full border-2 border-white/45 border-t-white/90"></span></span>
                  <img :src="thumbUrl(u)" @error="imgFull($event, u); markImgLoaded($event)" @load="markImgLoaded($event)"
                       class="relative h-full w-full cursor-zoom-in object-cover" @click.stop="openPhoto(m, k)" />
                </div>
              </div>
              <div v-if="captionText(m)" class="markdown-body break-words px-3.5 pt-1.5 text-[15px]" :class="isMine(m) && 'markdown-on-accent'" v-html="renderChatBody(captionText(m))"></div>
              <!-- реакции + время в одной строке (с подписью) -->
              <div v-if="captionText(m)" class="flex items-end justify-between gap-2 px-2.5 pb-1.5 pt-1">
                <div class="flex flex-wrap gap-1">
                  <button v-for="r in parseReactions(m)" :key="r.emoji" @click.stop="onChip(m, r.emoji)" @contextmenu.prevent.stop="openWho($event, r)" title="ПКМ — кто поставил"
                          class="flex items-center gap-1 rounded-full px-2 py-0.5 leading-none ring-1 transition"
                          :class="m.my_reaction === r.emoji ? 'bg-saffron-500/25 text-saffron-800 ring-saffron-400' : 'bg-saffron-500/10 text-ink-700 ring-transparent hover:bg-saffron-500/20'"><span class="text-lg leading-none">{{ r.emoji }}</span><span v-if="r.count < 4 && r.who && r.who.length" class="-my-0.5 -mr-2 flex items-center"><template v-for="(w, wi) in r.who" :key="wi"><img v-if="w.avatar" :src="thumbUrl(w.avatar)" class="block h-[22px] w-[22px] rounded-full object-cover" :class="wi > 0 && '-ml-2'" /><span v-else class="flex h-[22px] w-[22px] items-center justify-center rounded-full bg-sage-500 text-[9px] font-semibold text-white" :class="wi > 0 && '-ml-2'">{{ initials(w.name) }}</span></template></span><span v-else-if="r.count > 1" class="text-sm font-semibold tabular-nums">{{ r.count }}</span></button>
                </div>
                <div class="flex shrink-0 items-center gap-1 pb-0.5 text-[11px]" :class="isMine(m) ? 'text-white/70' : 'text-ink-700/40'">
                  <span>{{ fmtTime(m.created_at) }}</span>
                  <template v-if="statusOf(m)"><AppIcon v-if="statusOf(m) === 'pending'" name="clock" :size="15" /><AppIcon v-else-if="statusOf(m) === 'read'" name="check-double" :size="16" /><AppIcon v-else-if="statusOf(m) === 'sent'" name="check" :size="15" /></template>
                </div>
              </div>
              <!-- без подписи: реакции слева + время справа, одной линией оверлеем на фото -->
              <div v-else class="pointer-events-none absolute inset-x-1.5 bottom-1.5 flex items-center justify-between gap-2">
                <div class="pointer-events-auto flex flex-wrap gap-1">
                  <button v-for="r in parseReactions(m)" :key="r.emoji" @click.stop="onChip(m, r.emoji)" @contextmenu.prevent.stop="openWho($event, r)" title="ПКМ — кто поставил"
                          class="inline-flex items-center gap-1 rounded-full bg-black/45 px-1.5 py-0.5 text-white ring-1 ring-white/20"
                          :class="m.my_reaction === r.emoji && 'ring-2 ring-white/70'"><span class="text-base leading-none">{{ r.emoji }}</span><span v-if="r.count < 4 && r.who && r.who.length" class="-my-0.5 -mr-2 flex items-center"><template v-for="(w, wi) in r.who" :key="wi"><img v-if="w.avatar" :src="thumbUrl(w.avatar)" class="block h-[22px] w-[22px] rounded-full object-cover ring-1 ring-black/20" :class="wi > 0 && '-ml-2'" /><span v-else class="flex h-[22px] w-[22px] items-center justify-center rounded-full bg-sage-500 text-[9px] font-semibold text-white ring-1 ring-black/20" :class="wi > 0 && '-ml-2'">{{ initials(w.name) }}</span></template></span><span v-else-if="r.count > 1" class="text-xs font-semibold tabular-nums">{{ r.count }}</span></button>
                </div>
                <div class="pointer-events-auto ml-auto flex shrink-0 items-center gap-1 rounded-full bg-black/45 px-1.5 py-0.5 text-[11px] text-white">
                  <span>{{ fmtTime(m.created_at) }}</span>
                  <template v-if="statusOf(m)"><AppIcon v-if="statusOf(m) === 'pending'" name="clock" :size="15" /><AppIcon v-else-if="statusOf(m) === 'read'" name="check-double" :size="16" /><AppIcon v-else-if="statusOf(m) === 'sent'" name="check" :size="15" /></template>
                </div>
              </div>
            </div>

            <!-- обычное сообщение -->
            <div v-else class="relative rounded-2xl px-3.5 py-2 text-[15px] shadow-sm"
                 :class="[isMine(m) ? 'bg-saffron-500 text-white' : 'bg-white text-ink-900 ring-1 ring-parchment-200', wide ? 'max-w-[600px]' : 'max-w-[78%]']"
                 :data-audio-label="`${nameOf(m) || 'Голосовое'} · ${fmtTime(m.created_at)}`"
                 @contextmenu="onContext($event, m)">
              <div v-if="showAuthor(m, i)" class="mb-0.5 cursor-pointer text-sm font-semibold text-sage-600" @click.stop="openUserInfo(m.author_id)">{{ nameOf(m) }}</div>
              <div v-if="fwdName(m)" class="mb-1 flex items-center gap-1.5 text-sm font-semibold" :class="isMine(m) ? 'text-white/90' : 'text-saffron-700'">
                <AppIcon name="reply" :size="12" class="-scale-x-100" /> <span>Переслано от</span>
                <img v-if="fwdAvatar(m)" :src="thumbUrl(fwdAvatar(m))" class="h-4 w-4 shrink-0 rounded-full object-cover" />
                <span v-else class="flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-[8px] font-semibold text-white" :class="isMine(m) ? 'bg-white/30' : 'bg-saffron-500'">{{ initials(fwdName(m)) }}</span>
                <span class="truncate">{{ fwdName(m) }}</span>
              </div>
              <div v-if="m.reply_preview" @click.stop="jumpToReply(m)" class="mb-1 flex cursor-pointer items-center gap-2 rounded-r-md border-l-2 py-1 pl-2 pr-2 text-xs transition" :class="isMine(m) ? 'border-white/70 bg-white/10 hover:bg-white/20' : 'border-saffron-400 bg-saffron-500/5 hover:bg-saffron-500/10'">
                <img v-if="replyThumb(m)" :src="replyThumb(m)" class="h-8 w-8 shrink-0 rounded object-cover" />
                <div class="min-w-0 flex-1">
                  <div v-if="replyAuthorName(m)" class="font-semibold" :class="isMine(m) ? 'text-white' : 'text-saffron-700'">{{ replyAuthorName(m) }}</div>
                  <div class="whitespace-pre-wrap break-words opacity-80">{{ m.reply_preview }}</div>
                </div>
              </div>
              <div v-if="m.deleted" class="italic opacity-60">сообщение удалено</div>
              <div v-else class="markdown-body break-words" :class="isMine(m) && 'markdown-on-accent'" v-html="renderChatBody(contentBody(m))"></div>

              <!-- OG-превью ссылки -->
              <a v-if="linkCard(m)" :href="firstLink(m)" target="_blank" rel="noopener noreferrer"
                 class="mt-1.5 block overflow-hidden rounded-lg border-l-[3px] no-underline"
                 :class="isMine(m) ? 'border-white/70 bg-white/10' : 'border-saffron-400 bg-saffron-500/5'">
                <div class="px-2.5 pb-1.5 pt-1">
                  <div v-if="linkCard(m).site_name" class="text-[11px] font-semibold uppercase tracking-wide" :class="isMine(m) ? 'text-white/70' : 'text-saffron-700'">{{ linkCard(m).site_name }}</div>
                  <div v-if="linkCard(m).title" class="line-clamp-2 text-sm font-semibold leading-snug" :class="isMine(m) ? 'text-white' : 'text-ink-900'">{{ linkCard(m).title }}</div>
                  <div v-if="linkCard(m).description" class="mt-0.5 line-clamp-2 text-xs leading-snug" :class="isMine(m) ? 'text-white/80' : 'text-ink-700/70'">{{ linkCard(m).description }}</div>
                </div>
                <!-- 16:9-бокс: место занято заранее (без «прыжка»), YouTube-полосы 4:3 тоже кадрируются -->
                <div v-if="linkCard(m).image" class="aspect-video overflow-hidden">
                  <img :src="linkCard(m).image" class="block h-full w-full object-cover"
                       @error="$event.target.parentElement.style.display='none'" />
                </div>
              </a>

              <div class="mt-1 flex items-end justify-between gap-2">
                <div v-if="parseReactions(m).length" class="flex flex-wrap gap-1">
                  <button v-for="r in parseReactions(m)" :key="r.emoji" @click.stop="onChip(m, r.emoji)" @contextmenu.prevent.stop="openWho($event, r)"
                          title="ПКМ — кто поставил"
                          class="flex items-center gap-1 rounded-full px-2 py-0.5 leading-none ring-1 transition"
                          :class="isMine(m)
                            ? (m.my_reaction === r.emoji ? 'bg-white/25 ring-white/60' : 'bg-white/15 ring-white/20 hover:bg-white/25')
                            : (m.my_reaction === r.emoji ? 'bg-saffron-500/25 text-saffron-800 ring-saffron-400' : 'bg-saffron-500/10 text-ink-700 ring-transparent hover:bg-saffron-500/20')">
                    <span class="text-lg leading-none">{{ r.emoji }}</span><span v-if="r.count < 4 && r.who && r.who.length" class="-my-0.5 -mr-2 flex items-center"><template v-for="(w, wi) in r.who" :key="wi"><img v-if="w.avatar" :src="thumbUrl(w.avatar)" class="block h-[22px] w-[22px] rounded-full object-cover" :class="wi > 0 && '-ml-2'" /><span v-else class="flex h-[22px] w-[22px] items-center justify-center rounded-full bg-sage-500 text-[9px] font-semibold text-white" :class="wi > 0 && '-ml-2'">{{ initials(w.name) }}</span></template></span><span v-else-if="r.count > 1" class="text-sm font-semibold tabular-nums">{{ r.count }}</span>
                  </button>
                </div>
                <span v-else></span>
                <div class="flex shrink-0 items-center gap-1 text-[11px]" :class="isMine(m) ? 'text-white/70' : 'text-ink-700/40'">
                  <span v-if="m.edit_count">изм. · </span>
                  <span>{{ fmtTime(m.created_at) }}</span>
                  <template v-if="statusOf(m)">
                    <AppIcon v-if="statusOf(m) === 'pending'" name="clock" :size="15" />
                    <button v-else-if="statusOf(m) === 'failed'" class="text-red-200" title="Не отправлено — повторить" @click.stop="retryFailed"><AppIcon name="close" :size="15" /></button>
                    <AppIcon v-else-if="statusOf(m) === 'read'" name="check-double" :size="16" />
                    <AppIcon v-else-if="statusOf(m) === 'sent'" name="check" :size="15" />
                  </template>
                </div>
              </div>
            </div>
            <div v-if="selectMode" class="flex shrink-0 items-center self-center pl-1">
              <span class="flex h-6 w-6 items-center justify-center rounded-full border-2 transition" :class="selected.has(m.id) ? 'border-saffron-500 bg-saffron-500 text-white' : 'border-parchment-400 bg-white/80'">
                <AppIcon v-if="selected.has(m.id)" name="check" :size="14" />
              </span>
            </div>
          </div>
          </template>

          <!-- оптимистичные загрузки фото (мгновенно, с лоадером; уходят на сервер в фоне) -->
          <div v-for="pu in pendingUploads.filter((p) => p.chatId === activeId && p.previews.length)" :key="pu.id" class="flex justify-end px-1">
            <div class="relative max-w-[78%] overflow-hidden rounded-2xl bg-saffron-500 shadow-sm">
              <div class="grid gap-0.5" :class="albumCols(pu.previews.length)">
                <div v-for="(p, k) in pu.previews" :key="k" class="relative" :class="albumItemClass(pu.previews.length, k)">
                  <img v-if="p.url" :src="p.url" class="w-full object-cover" :class="pu.previews.length === 1 ? 'max-h-[400px]' : 'aspect-square'" />
                  <div v-else class="flex w-full items-center justify-center bg-ink-900/80" :class="pu.previews.length === 1 ? 'h-64' : 'aspect-square'"><AppIcon name="play" :size="30" class="text-white/70" /></div>
                  <div class="absolute inset-0 flex items-center justify-center bg-black/30">
                    <span v-if="!pu.failed" class="h-7 w-7 animate-spin rounded-full border-2 border-white/40 border-t-white"></span>
                    <button v-else class="flex items-center gap-1 rounded-full bg-black/55 px-3 py-1.5 text-xs font-medium text-white" @click="retryPending(pu)"><AppIcon name="reply" :size="14" class="-scale-x-100" /> Повторить</button>
                  </div>
                </div>
              </div>
              <div v-if="pu.cap" class="px-3.5 py-1.5 text-[15px] text-white">{{ pu.cap }}</div>
            </div>
          </div>

          <!-- оптимистичное голосовое: индикатор загрузки на сервер (кольцо + × отмена) -->
          <div v-for="pv in pendingVoice.filter((p) => p.chatId === activeId)" :key="pv.id" class="flex justify-end px-1">
            <div class="flex max-w-[78%] items-center gap-3 rounded-2xl bg-saffron-500 px-3 py-2.5 text-white shadow-sm">
              <button class="relative flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-white/25 transition hover:bg-white/35" :title="pv.failed ? 'Повторить' : 'Отменить'" @click="pv.failed ? retryVoice(pv) : cancelVoice(pv)">
                <svg v-if="!pv.failed" class="absolute inset-0 h-full w-full -rotate-90" viewBox="0 0 44 44">
                  <circle cx="22" cy="22" r="20" fill="none" stroke="rgba(255,255,255,0.35)" stroke-width="2.5" />
                  <circle cx="22" cy="22" r="20" fill="none" stroke="#fff" stroke-width="2.5" stroke-linecap="round" :stroke-dasharray="125.6" :stroke-dashoffset="125.6 * (1 - pv.sent / pv.total)" style="transition: stroke-dashoffset .2s linear" />
                </svg>
                <AppIcon :name="pv.failed ? 'reply' : 'close'" :size="18" :class="pv.failed && '-scale-x-100'" />
              </button>
              <div class="min-w-0">
                <div class="text-sm tabular-nums">{{ fmtRec(pv.seconds) }}</div>
                <div class="text-xs text-white/75">{{ pv.failed ? 'Ошибка загрузки' : `${fmtKB(pv.sent)} / ${fmtKB(pv.total)}` }}</div>
              </div>
              <AppIcon name="clock" :size="14" class="ml-1 shrink-0 self-end text-white/70" />
            </div>
          </div>

          <!-- оптимистичный кружок (появляется мгновенно с лоадером) -->
          <div v-for="pn in pendingNotes.filter((p) => p.chatId === activeId)" :key="pn.id" class="flex justify-end px-1">
            <div class="relative h-[19.5rem] w-[19.5rem] overflow-hidden rounded-full shadow-sm">
              <img v-if="pn.poster" :src="pn.poster" class="h-full w-full -scale-x-100 object-cover" />
              <div class="absolute inset-0 flex items-center justify-center bg-black/30">
                <span v-if="!pn.failed" class="h-9 w-9 animate-spin rounded-full border-2 border-white/40 border-t-white"></span>
                <button v-else class="flex items-center gap-1 rounded-full bg-black/55 px-3 py-1.5 text-xs font-medium text-white" @click="retryNote(pn)"><AppIcon name="reply" :size="14" class="-scale-x-100" /> Повторить</button>
              </div>
            </div>
          </div>
          </div>
        </div>

        <!-- единая плавающая дата при скролле (не накладывается, в отличие от sticky-плашек) -->
        <div class="pointer-events-none absolute inset-x-0 top-2 z-[6] flex justify-center transition-opacity duration-300" :class="floatDate.show ? 'opacity-100' : 'opacity-0'">
          <transition name="datefade" mode="out-in">
            <span :key="floatDate.label" class="rounded-full bg-ink-900/55 px-3 py-1 text-xs font-semibold text-white shadow-sm backdrop-blur-sm">{{ floatDate.label }}</span>
          </transition>
        </div>
        </div>

        <!-- кнопка «вниз» (видна, когда прокручено вверх) -->
        <transition name="fade">
          <button v-if="activeChat && !stickBottom && !holdRec.active" @click="stickBottom = true; scrollToBottom()" title="Вниз"
                  class="absolute bottom-24 right-5 z-20 flex h-11 w-11 items-center justify-center rounded-full bg-white text-ink-700 shadow-lg ring-1 ring-parchment-200 transition hover:bg-parchment-50">
            <AppIcon name="chevron" :size="22" />
          </button>
        </transition>

        <!-- Композер -->
        <div class="border-t border-parchment-200 p-3">
          <div v-if="replyTo" class="mb-2 flex items-center gap-2 rounded-lg bg-parchment-100 px-3 py-1.5 text-sm">
            <AppIcon name="reply" :size="19" class="shrink-0 text-saffron-600" />
            <img v-if="replyTo.photo" :src="thumbUrl(replyTo.photo)" class="h-8 w-8 shrink-0 rounded object-cover" />
            <span class="min-w-0 flex-1 truncate text-ink-700/70"><b class="text-ink-800">{{ replyTo.author_name }}</b>: {{ replyTo.body }}</span>
            <button class="text-ink-700/50 hover:text-ink-900" @click="replyTo = null"><AppIcon name="close" :size="15" /></button>
          </div>
          <div v-else-if="editingMsg" class="mb-2 flex items-center gap-2 rounded-lg border-l-2 border-saffron-400 bg-parchment-100 px-3 py-1.5 text-sm">
            <AppIcon name="edit" :size="19" class="shrink-0 text-saffron-600" />
            <img v-if="firstPhotoUrl(editingMsg.body)" :src="thumbUrl(firstPhotoUrl(editingMsg.body))" class="h-8 w-8 shrink-0 rounded object-cover" />
            <span class="min-w-0 flex-1 truncate text-ink-700/70"><b class="text-saffron-700">Редактирование</b> · {{ snippet(editingMsg.body) }}</span>
            <button class="text-ink-700/50 hover:text-ink-900" @click="cancelEdit"><AppIcon name="close" :size="15" /></button>
          </div>

          <!-- ЗАПИСЬ по удержанию (голос ↔ кружок): та же высота строки, кнопка справа -->
          <div v-if="holdRec.active" class="relative flex min-h-[2.75rem] items-center gap-3">
            <span class="ml-1 h-3 w-3 shrink-0 animate-pulse rounded-full bg-red-500"></span>
            <span class="shrink-0 tabular-nums text-sm text-ink-800">{{ fmtRecMs(holdRec.seconds) }}</span>
            <span class="flex-1 truncate text-center text-sm" :class="holdRec.willCancel ? 'font-medium text-red-600' : 'text-ink-700/45'">
              {{ holdRec.willCancel ? 'Отпустите для отмены' : (holdRec.locked ? (recMode === 'video' ? 'Кружок закреплён' : 'Голосовое закреплено') : 'Для отмены отпустите курсор вне поля') }}
            </span>
            <template v-if="holdRec.locked">
              <button class="rounded-lg px-3 py-1.5 text-sm text-ink-700/60 hover:bg-parchment-100" @click="lockedCancel">Отмена</button>
              <button class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-saffron-500 text-white hover:bg-saffron-600" title="Отправить" @click="lockedSend"><AppIcon name="send" :size="20" /></button>
            </template>
            <template v-else>
              <!-- подсказка «тяни вверх, чтобы закрепить» — над кнопкой; подсвечивается при приближении -->
              <div class="pointer-events-none absolute right-1 flex flex-col items-center gap-1.5 rounded-full px-2.5 py-3 shadow-lg transition-colors"
                   :class="holdRec.upProgress > 0.6 ? 'bg-saffron-500 text-white' : 'bg-ink-900/70 text-white/90'"
                   :style="{ bottom: (56 + holdRec.upProgress * 26) + 'px', transform: `scale(${1 + holdRec.upProgress * 0.18})` }">
                <AppIcon name="lock" :size="18" />
                <AppIcon name="chevron" :size="15" :class="holdRec.upProgress > 0.05 ? '' : 'animate-bounce'" class="rotate-180" />
              </div>
              <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-red-500 text-white shadow-lg ring-4 ring-red-500/25">
                <AppIcon :name="recMode === 'video' ? 'video' : 'mic'" :size="22" />
              </span>
            </template>
          </div>

          <div v-else class="relative flex items-end gap-2">
            <button class="mb-0.5 shrink-0 rounded-full p-2 text-ink-700/60 hover:bg-parchment-100 hover:text-saffron-600" title="Прикрепить" :disabled="uploading" @click="fileInput.click()">
              <AppIcon name="paperclip" :size="24" />
            </button>
            <input ref="fileInput" type="file" multiple class="hidden" @change="onPickFile" />

            <textarea ref="inputEl" v-model="body" rows="1" :maxlength="MAX_LEN"
                      class="chat-input min-h-[2.75rem] flex-1 resize-none rounded-2xl border border-parchment-300 bg-parchment-50 px-4 py-2.5 text-base leading-6 focus:border-saffron-400 focus:outline-none focus:ring-1 focus:ring-saffron-400"
                      placeholder="Сообщение…" @input="onInput" @keydown="onKeydown" @contextmenu="onInputContext"></textarea>

            <div class="relative mb-0.5 shrink-0">
              <button class="rounded-full p-2 text-ink-700/60 hover:bg-parchment-100 hover:text-saffron-600" title="Эмодзи" @click="showEmoji = !showEmoji">
                <AppIcon name="react" :size="28" />
              </button>
              <template v-if="showEmoji">
                <div class="fixed inset-0 z-10" @click="showEmoji = false"></div>
                <div class="absolute bottom-full right-0 z-20 mb-2 grid w-80 grid-cols-7 gap-1 rounded-xl bg-white p-2 shadow-lg ring-1 ring-parchment-200">
                  <button v-for="e in EMOJI_PALETTE" :key="e" class="rounded p-1 text-2xl leading-none hover:bg-parchment-100" @click="insertEmoji(e)">{{ e }}</button>
                </div>
              </template>
            </div>

            <button v-if="body.trim()" class="mb-0.5 shrink-0 rounded-full bg-saffron-500 p-2 text-white hover:bg-saffron-600" title="Отправить" @click="send">
              <AppIcon name="send" :size="20" />
            </button>
            <button v-else class="mb-0.5 shrink-0 touch-none select-none rounded-full p-2 text-ink-700/60 transition hover:bg-parchment-100 hover:text-saffron-600"
                    :title="recMode === 'video' ? 'Кружок — удерживайте; коротко — голосовое' : 'Голосовое — удерживайте; коротко — кружок'"
                    :disabled="uploading" @pointerdown="recPointerDown" @contextmenu.prevent>
              <AppIcon :name="recMode === 'video' ? 'video' : 'mic'" :size="24" />
            </button>
          </div>

          <!-- запись кружка: круглое превью с камеры (без фона), таймер; управление — жестами -->
          <div v-if="vnRecording" class="pointer-events-none absolute inset-0 z-40 flex flex-col items-center justify-center gap-5">
            <div class="relative h-96 w-96">
              <div class="h-full w-full overflow-hidden rounded-full shadow-2xl">
                <video ref="vnPreview" muted playsinline disablepictureinpicture controlslist="nodownload noremoteplayback nofullscreen" class="pointer-events-none h-full w-full -scale-x-100 object-cover"></video>
              </div>
              <!-- кольцо прогресса записи (до 1 минуты) -->
              <svg class="pointer-events-none absolute inset-0 h-full w-full -rotate-90" viewBox="0 0 100 100">
                <circle cx="50" cy="50" r="48.5" fill="none" stroke="rgba(0,0,0,0.12)" stroke-width="2.5" />
                <circle cx="50" cy="50" r="48.5" fill="none" :stroke="vnReady ? '#e0902a' : 'rgba(224,144,42,0.4)'" stroke-width="3" stroke-linecap="round"
                        :stroke-dasharray="304.7" :stroke-dashoffset="304.7 * (1 - vnFrac)" style="transition: stroke-dashoffset .12s linear" />
              </svg>
              <span class="absolute left-1/2 top-3 flex -translate-x-1/2 items-center gap-1.5 rounded-full bg-black/55 px-3 py-1 text-sm text-white">
                <span class="h-2 w-2 rounded-full bg-red-500" :class="vnReady && 'animate-pulse'"></span>
                <span class="tabular-nums">{{ vnReady ? fmtRec(vnSeconds) : 'подготовка…' }}</span>
              </span>
            </div>
            <!-- запасные кнопки (если запись не через удержание) -->
            <div v-if="!holdRec.active" class="pointer-events-auto flex items-center gap-4">
              <button class="flex h-12 w-12 items-center justify-center rounded-full bg-white text-ink-700 shadow ring-1 ring-parchment-200 hover:bg-parchment-50" title="Отмена" @click="cancelVideoNote">
                <AppIcon name="close" :size="24" />
              </button>
              <button class="flex h-14 w-14 items-center justify-center rounded-full bg-saffron-500 text-white shadow-lg transition hover:bg-saffron-600 disabled:opacity-40" title="Отправить кружок" :disabled="!vnReady" @click="stopVideoNote">
                <AppIcon name="send" :size="24" />
              </button>
            </div>
          </div>
        </div>
      </template>

      <div v-else class="flex flex-1 flex-col items-center justify-center text-center text-ink-700/40">
        <AppIcon name="chat" :size="48" />
        <p class="mt-3 text-sm">Выберите чат или начните новый</p>
      </div>

      <!-- Перетаскивание: если все картинки — две зоны; иначе одна зона «файлы» -->
      <div v-if="dragOver && activeChat" class="absolute inset-0 z-30 flex flex-col gap-3 bg-parchment-100/70 p-4 backdrop-blur-sm">
        <template v-if="dragAllImages">
          <div class="flex flex-1 flex-col items-center justify-center rounded-3xl border-2 border-dashed transition-colors"
               :class="hoverZone === 'file' ? 'border-saffron-500 bg-saffron-500/20' : 'border-saffron-300 bg-white/70'"
               @dragover.prevent="hoverZone = 'file'" @dragleave="hoverZone = null" @drop="onZoneDrop($event, 'file')">
            <AppIcon name="paperclip" :size="34" class="text-saffron-600" />
            <div class="mt-3 font-display text-2xl font-semibold text-saffron-700">Перетащите сюда</div>
            <div class="mt-1 text-sm text-ink-700/60">чтобы отправить файлом — без сжатия</div>
          </div>
          <div class="flex flex-1 flex-col items-center justify-center rounded-3xl border-2 border-dashed transition-colors"
               :class="hoverZone === 'picture' ? 'border-sage-500 bg-sage-500/20' : 'border-sage-400 bg-white/70'"
               @dragover.prevent="hoverZone = 'picture'" @dragleave="hoverZone = null" @drop="onZoneDrop($event, 'picture')">
            <AppIcon name="image" :size="34" class="text-sage-600" />
            <div class="mt-3 font-display text-2xl font-semibold text-sage-600">Перетащите сюда</div>
            <div class="mt-1 text-sm text-ink-700/60">чтобы отправить картинкой — быстро</div>
          </div>
        </template>
        <div v-else class="flex flex-1 flex-col items-center justify-center rounded-3xl border-2 border-dashed border-saffron-400 bg-white/70"
             @dragover.prevent @drop="onZoneDrop($event, 'file')">
          <AppIcon name="paperclip" :size="34" class="text-saffron-600" />
          <div class="mt-3 font-display text-2xl font-semibold text-saffron-700">Перетащите сюда файлы</div>
          <div class="mt-1 text-sm text-ink-700/60">чтобы отправить без сжатия</div>
        </div>
      </div>
    </section>

    <!-- Контекстное меню (ПКМ) -->
    <template v-if="ctx.open">
      <div class="fixed inset-0 z-40" @click="closeCtx" @contextmenu.prevent="closeCtx"></div>
      <div class="fixed z-50 w-60 overflow-hidden rounded-xl bg-white py-1 shadow-xl ring-1 ring-parchment-200" :style="ctxStyle">
        <div class="flex justify-around px-2 py-2">
          <button v-for="e in REACTION_EMOJIS" :key="e" class="rounded-full p-1 text-2xl leading-none transition hover:scale-125" @click="ctxReact(e)">{{ e }}</button>
        </div>
        <div class="my-1 border-t border-parchment-100"></div>
        <button class="flex w-full items-center gap-3 px-3.5 py-2.5 text-left text-[15px] text-ink-700 hover:bg-parchment-100" @click="ctxReply"><AppIcon name="reply" :size="19" /> Ответить</button>
        <button v-if="canCopy(ctx.m) || ctx.selText" class="flex w-full items-center gap-3 px-3.5 py-2.5 text-left text-[15px] text-ink-700 hover:bg-parchment-100" @click="ctxCopy"><AppIcon name="copy" :size="19" /> Копировать</button>
        <button v-if="canEdit(ctx.m)" class="flex w-full items-center gap-3 px-3.5 py-2.5 text-left text-[15px] text-ink-700 hover:bg-parchment-100" @click="ctxEdit"><AppIcon name="edit" :size="19" /> Изменить</button>
        <button v-if="ctx.m && !ctx.m.deleted" class="flex w-full items-center gap-3 px-3.5 py-2.5 text-left text-[15px] text-ink-700 hover:bg-parchment-100" @click="ctxForward"><AppIcon name="reply" :size="19" class="-scale-x-100" /> Переслать</button>
        <button v-if="ctx.m && ctx.m.id && !ctx.m.deleted" class="flex w-full items-center gap-3 px-3.5 py-2.5 text-left text-[15px] text-ink-700 hover:bg-parchment-100" @click="ctxPin"><AppIcon name="pin-chat" :size="19" /> {{ activeChat?.pinned_message_id === ctx.m.id ? 'Открепить' : 'Закрепить' }}</button>
        <button v-if="canDelete(ctx.m)" class="flex w-full items-center gap-3 border-t border-parchment-100 px-3.5 py-2.5 text-left text-[15px] text-red-600 hover:bg-red-50" @click="ctxDelete"><AppIcon name="trash" :size="19" /> {{ delLabel(ctx.m) }}</button>
        <button v-if="ctx.m && !ctx.m.deleted" class="flex w-full items-center gap-3 border-t border-parchment-100 px-3.5 py-2.5 text-left text-[15px] text-ink-700 hover:bg-parchment-100" @click="ctxSelect"><AppIcon name="check" :size="19" /> Выделить</button>
      </div>
    </template>

    <!-- Кто поставил реакцию (ПКМ по чипу) -->
    <template v-if="whoMenu.open">
      <div class="fixed inset-0 z-40" @click="closeWho" @contextmenu.prevent="closeWho"></div>
      <div class="fixed z-50 max-h-[280px] w-60 overflow-y-auto rounded-xl border border-parchment-200 bg-white p-1.5 shadow-xl" :style="whoStyle">
        <div v-for="(u, k) in whoMenu.list" :key="k" class="flex items-center gap-2 rounded-lg px-2 py-1.5">
          <img v-if="u.avatar" :src="thumbUrl(u.avatar)" @error="imgFull($event, u.avatar)" class="photo-bw h-7 w-7 shrink-0 rounded-full object-cover" />
          <span v-else class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-sage-400 to-sage-600 text-xs font-semibold text-white">{{ initials(u.name) }}</span>
          <span class="truncate text-sm text-ink-800">{{ u.name || '—' }}</span>
        </div>
        <div v-if="!whoMenu.list.length" class="px-2 py-1.5 text-sm text-ink-700/50">Пока никого</div>
      </div>
    </template>

    <!-- Диалог удаления сообщения -->
    <div v-if="deleteTarget" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-4" @click.self="deleteTarget = null">
      <div class="w-full max-w-sm overflow-hidden rounded-xl bg-white shadow-xl">
        <div class="p-5">
          <h3 class="font-medium text-ink-900">{{ isMine(deleteTarget) ? 'Удалить это сообщение?' : 'Скрыть это сообщение?' }}</h3>
          <label v-if="activeChat?.type === 'direct' && isMine(deleteTarget)" class="mt-4 flex items-center gap-2.5 text-sm text-ink-800">
            <input type="checkbox" v-model="deleteForAll" class="h-4 w-4" /> Также удалить для {{ peerName }}
          </label>
          <p v-else-if="activeChat?.type === 'group' && isMine(deleteTarget)" class="mt-3 text-sm text-ink-700/70">Сообщение будет удалено для всех в этом чате.</p>
          <p v-else class="mt-3 text-sm text-ink-700/70">Сообщение будет скрыто только у вас — у остальных оно останется.</p>
        </div>
        <div class="flex justify-end gap-2 border-t border-parchment-200 p-3">
          <button class="btn-ghost" @click="deleteTarget = null">Отмена</button>
          <button class="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700" @click="confirmDelete">{{ delLabel(deleteTarget) }}</button>
        </div>
      </div>
    </div>

    <!-- Диалог удаления нескольких сообщений -->
    <div v-if="deleteManyOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-4" @click.self="deleteManyOpen = false">
      <div class="w-full max-w-sm overflow-hidden rounded-xl bg-white shadow-xl">
        <div class="p-5">
          <h3 class="font-medium text-ink-900">Удалить {{ selected.size }} сообщ.?</h3>
          <label v-if="activeChat?.type === 'direct'" class="mt-4 flex items-center gap-2.5 text-sm text-ink-800">
            <input type="checkbox" v-model="deleteManyForAll" class="h-4 w-4" /> Также удалить у {{ peerName }}
          </label>
          <p v-else class="mt-3 text-sm text-ink-700/70">Ваши сообщения удалятся для всех, чужие — скроются у вас.</p>
        </div>
        <div class="flex justify-end gap-2 border-t border-parchment-200 p-3">
          <button class="btn-ghost" @click="deleteManyOpen = false">Отмена</button>
          <button class="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700" @click="confirmDeleteSelected">Удалить</button>
        </div>
      </div>
    </div>

    <!-- Выбор чата для пересылки -->
    <div v-if="forwardOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-4" @click.self="forwardOpen = false">
      <div class="flex max-h-[70vh] w-full max-w-sm flex-col overflow-hidden rounded-xl bg-white shadow-xl">
        <div class="flex items-center gap-2 border-b border-parchment-200 p-3">
          <div class="font-medium text-ink-900">Переслать в…</div>
          <span class="text-sm text-ink-700/50">{{ forwardBodies.length }} сообщ.</span>
          <button class="ml-auto rounded-lg p-1 text-ink-700/50 hover:bg-parchment-100" @click="forwardOpen = false"><AppIcon name="close" :size="18" /></button>
        </div>
        <div class="p-3">
          <input ref="forwardSearchInput" v-model="forwardSearch" class="input" placeholder="Поиск чата…" @keydown.esc.prevent.stop="forwardOpen = false" />
        </div>
        <div class="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
          <button v-for="c in forwardList" :key="c.id" class="flex w-full items-center gap-3 rounded-lg px-2 py-2 text-left hover:bg-parchment-100" @click="doForward(c.id)">
            <img v-if="c.avatar_url" :src="thumbUrl(c.avatar_url)" @error="imgFull($event, c.avatar_url)" class="photo-bw h-10 w-10 shrink-0 rounded-full object-cover" />
            <span v-else class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-sm font-semibold text-white" :class="c.type === 'group' ? 'bg-gradient-to-br from-sage-400 to-sage-600' : 'bg-gradient-to-br from-saffron-400 to-saffron-600'">{{ initials(c.title) }}</span>
            <span class="min-w-0 flex-1 truncate font-medium text-ink-900">{{ c.title }}</span>
          </button>
          <div v-if="!forwardList.length" class="px-2 py-4 text-center text-sm text-ink-700/50">Ничего не найдено</div>
        </div>
      </div>
    </div>

    <!-- Кастомное контекстное меню поля ввода -->
    <template v-if="inputCtx.open">
      <div class="fixed inset-0 z-[85]" @click="inputCtx.open = false" @contextmenu.prevent="inputCtx.open = false"></div>
      <div class="fixed z-[86] w-52 overflow-hidden rounded-xl bg-white py-1 shadow-2xl ring-1 ring-parchment-200" :style="{ left: inputCtx.x + 'px', top: inputCtx.y + 'px' }" @click.stop>
        <button class="flex w-full items-center gap-3 px-4 py-2 text-left text-[15px] text-ink-900 hover:bg-parchment-100 disabled:text-ink-700/30 disabled:hover:bg-transparent" :disabled="!inputCtx.hasSel" @click="inputAction('cut')"><AppIcon name="trash" :size="17" class="opacity-0" /> Вырезать</button>
        <button class="flex w-full items-center gap-3 px-4 py-2 text-left text-[15px] text-ink-900 hover:bg-parchment-100 disabled:text-ink-700/30 disabled:hover:bg-transparent" :disabled="!inputCtx.hasSel" @click="inputAction('copy')"><AppIcon name="copy" :size="17" /> Копировать</button>
        <button class="flex w-full items-center gap-3 px-4 py-2 text-left text-[15px] text-ink-900 hover:bg-parchment-100" @click="inputAction('paste')"><AppIcon name="reply" :size="17" class="opacity-0" /> Вставить</button>
        <button class="flex w-full items-center gap-3 border-t border-parchment-100 px-4 py-2 text-left text-[15px] text-ink-900 hover:bg-parchment-100" @click="inputAction('selectall')"><AppIcon name="check" :size="17" class="opacity-0" /> Выделить всё</button>
      </div>
    </template>

    <!-- Окно звонка (попап / на весь экран) -->
    <div v-if="call.open" class="fixed inset-0 z-[80] flex items-center justify-center bg-ink-900/60" :class="call.fullscreen ? 'p-0' : 'p-4'">
      <div class="relative flex flex-col items-center overflow-hidden bg-ink-900 text-white shadow-2xl"
           :class="call.fullscreen ? 'h-full w-full rounded-none' : 'w-full max-w-2xl rounded-2xl'">
        <!-- видео собеседника на весь экран (когда соединено и есть видео) -->
        <template v-if="call.status === 'connected' && call.remoteVideo">
          <video ref="callRemoteVideo" autoplay playsinline class="w-full bg-black object-cover" :class="call.fullscreen ? 'h-full' : 'aspect-video max-h-[70vh]'"></video>
          <!-- имя + статус поверх видео -->
          <div class="pointer-events-none absolute left-0 right-0 top-0 bg-gradient-to-b from-black/50 to-transparent p-4 text-center">
            <div class="text-lg font-semibold">{{ call.name }}</div>
            <div class="text-xs text-white/70">{{ callStatusText }}</div>
          </div>
        </template>
        <!-- аватар (аудио-звонок или ещё нет видео) -->
        <div v-else class="flex flex-col items-center px-8 py-10" :class="call.fullscreen && 'flex-1 justify-center'">
          <div class="text-sm text-white/45">{{ callStatusText }}</div>
          <img v-if="call.avatar" :src="thumbUrl(call.avatar)" class="mt-6 rounded-full object-cover shadow-xl" :class="call.fullscreen ? 'h-56 w-56' : 'h-40 w-40'" />
          <span v-else class="mt-6 flex items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 font-semibold shadow-xl" :class="call.fullscreen ? 'h-56 w-56 text-7xl' : 'h-40 w-40 text-5xl'">{{ initials(call.name) }}</span>
          <div class="mt-5 text-2xl font-semibold">{{ call.name }}</div>
          <div v-if="call.status !== 'connected'" class="mt-2 text-center text-sm text-white/50">Если Вы хотите начать видеозвонок,<br>нажмите на значок камеры.</div>
        </div>
        <!-- своё видео превью -->
        <video v-show="call.localVideo" ref="callLocalVideo" autoplay playsinline muted class="absolute -scale-x-100 rounded-lg object-cover shadow-lg ring-2 ring-white/20"
               :class="call.fullscreen ? 'bottom-28 right-6 h-40 w-32' : 'right-4 top-4 h-28 w-24'"></video>
        <!-- развернуть/свернуть -->
        <button v-if="call.status === 'connected'" class="absolute right-3 top-3 z-10 rounded-full bg-black/40 p-2 text-white transition hover:bg-black/60" :title="call.fullscreen ? 'Свернуть' : 'На весь экран'" @click="toggleCallFullscreen">
          <AppIcon :name="call.fullscreen ? 'minimize' : 'maximize'" :size="20" />
        </button>
        <!-- панель кнопок -->
        <div class="flex items-end justify-center gap-12" :class="[call.status === 'connected' && call.remoteVideo ? 'absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/60 to-transparent pb-8 pt-10' : 'pb-10']">
          <button class="flex flex-col items-center gap-2.5" @click="toggleCallVideo">
            <span class="flex h-[4.5rem] w-[4.5rem] items-center justify-center rounded-full text-white shadow-lg transition active:scale-95" :class="call.localVideo ? 'bg-sky-600' : 'bg-white/15 hover:bg-white/25'"><AppIcon name="video" :size="30" /></span>
            <span class="text-sm text-white/70">{{ call.localVideo ? 'Выкл. видео' : 'Вкл. видео' }}</span>
          </button>
          <button class="flex flex-col items-center gap-2.5" @click="endCall">
            <span class="flex h-[4.5rem] w-[4.5rem] items-center justify-center rounded-full bg-red-500 text-white shadow-lg transition hover:bg-red-600 active:scale-95"><AppIcon name="phone" :size="30" class="rotate-[135deg]" /></span>
            <span class="text-sm text-white/70">{{ call.status === 'connected' ? 'Завершить' : 'Отменить' }}</span>
          </button>
          <button v-if="call.status === 'idle-outgoing'" class="flex flex-col items-center gap-2.5" @click="placeCall">
            <span class="flex h-[4.5rem] w-[4.5rem] items-center justify-center rounded-full bg-sky-500 text-white shadow-lg transition hover:bg-sky-600 active:scale-95"><AppIcon name="phone" :size="30" /></span>
            <span class="text-sm text-white/70">Позвонить</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Входящий звонок (попап) -->
    <div v-if="incoming.open" class="fixed inset-0 z-[81] flex items-center justify-center bg-ink-900/60 p-4">
      <div class="flex w-full max-w-sm flex-col items-center rounded-2xl bg-ink-900 p-8 text-white shadow-2xl">
        <div class="text-sm text-white/45">Входящий {{ incoming.video ? 'видеозвонок' : 'звонок' }}</div>
        <img v-if="incoming.avatar" :src="thumbUrl(incoming.avatar)" class="mt-6 h-32 w-32 rounded-full object-cover shadow-xl" />
        <span v-else class="mt-6 flex h-32 w-32 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-4xl font-semibold shadow-xl">{{ initials(incoming.name) }}</span>
        <div class="mt-4 text-xl font-semibold">{{ incoming.name }}</div>
        <div class="mt-8 flex items-end gap-12">
          <button class="flex flex-col items-center gap-2" @click="rejectIncoming">
            <span class="flex h-14 w-14 items-center justify-center rounded-full bg-red-500 text-white transition hover:bg-red-600 animate-pulse"><AppIcon name="phone" :size="24" class="rotate-[135deg]" /></span>
            <span class="text-xs text-white/60">Отклонить</span>
          </button>
          <button class="flex flex-col items-center gap-2" @click="acceptIncoming">
            <span class="flex h-14 w-14 items-center justify-center rounded-full bg-green-500 text-white transition hover:bg-green-600"><AppIcon name="phone" :size="24" /></span>
            <span class="text-xs text-white/60">Принять</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Диалог отправки вложений (картинки + файлы) -->
    <!-- Память устройства / кэш -->
    <div v-if="showStorage" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-4" @click.self="showStorage = false">
      <div class="flex max-h-[85vh] w-full max-w-md flex-col overflow-hidden rounded-xl bg-white shadow-xl">
        <div class="flex items-center justify-between border-b border-parchment-200 px-4 py-3">
          <h3 class="font-medium text-ink-900">Память устройства</h3>
          <button class="text-ink-700/40 hover:text-ink-900" @click="showStorage = false"><AppIcon name="close" :size="18" /></button>
        </div>
        <div class="flex-1 space-y-4 overflow-y-auto p-4">
          <!-- всего занято -->
          <div class="rounded-xl bg-parchment-50 p-4 ring-1 ring-parchment-200">
            <div class="flex items-baseline justify-between">
              <span class="text-sm text-ink-700/60">Занято приложением</span>
              <span class="text-lg font-semibold text-ink-900">{{ storageBusy ? '…' : fmtSize(storageInfo.usage) }}</span>
            </div>
            <div class="mt-2 h-2 w-full overflow-hidden rounded-full bg-parchment-200">
              <div class="h-full rounded-full bg-saffron-500 transition-all" :style="{ width: storagePct + '%' }"></div>
            </div>
            <div class="mt-1.5 text-xs text-ink-700/40">
              <template v-if="storageInfo.quota">из ~{{ fmtSize(storageInfo.quota) }} доступно на устройстве</template>
              <template v-else>оценка недоступна в этом браузере</template>
            </div>
          </div>

          <!-- что в кэше -->
          <div class="divide-y divide-parchment-100 overflow-hidden rounded-xl ring-1 ring-parchment-200">
            <div class="flex items-center justify-between gap-3 px-4 py-3">
              <div class="min-w-0">
                <div class="text-[15px] text-ink-900">База чатов</div>
                <div class="text-xs text-ink-700/50">{{ storageInfo.messages }} сообщений · {{ storageInfo.chats }} чатов</div>
              </div>
              <span class="shrink-0 text-xs text-ink-700/40">локально</span>
            </div>
            <div class="flex items-center justify-between gap-3 px-4 py-3">
              <div class="min-w-0">
                <div class="text-[15px] text-ink-900">Превью и метаданные</div>
                <div class="text-xs text-ink-700/50">ссылки, размеры фото, состояния · {{ fmtSize(storageInfo.cacheBytes) }}</div>
              </div>
              <button class="shrink-0 rounded-lg px-3 py-1.5 text-sm font-medium text-saffron-700 hover:bg-saffron-500/10" @click="clearPreviewCache">Очистить</button>
            </div>
          </div>

          <p class="text-xs leading-relaxed text-ink-700/50">
            Медиа-файлы (фото и видео) хранятся в кэше браузера и очищаются вместе с ним.
            Полная очистка удалит локальную копию переписки — она заново загрузится с сервера, ничего не потеряется.
          </p>

          <button class="btn-outline w-full text-red-600 ring-red-200 hover:bg-red-50" :disabled="storageWiping" @click="wipeAllCache">
            <AppIcon name="trash" :size="16" /> {{ storageWiping ? 'Очистка…' : 'Очистить весь кэш чатов' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="showCompose" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-4" @click.self="cancelCompose">
      <div class="flex max-h-[85vh] w-full max-w-md flex-col overflow-hidden rounded-xl bg-white shadow-xl">
        <div class="flex items-center justify-between border-b border-parchment-200 px-4 py-3">
          <h3 class="font-medium text-ink-900">{{ composeTitle }}</h3>
          <button class="text-ink-700/40 hover:text-ink-900" @click="cancelCompose"><AppIcon name="close" :size="18" /></button>
        </div>
        <div class="flex-1 space-y-3 overflow-y-auto p-4">
          <!-- файлы -->
          <div v-for="(it, k) in composeFiles" :key="'f' + k" class="flex items-center gap-3 rounded-lg bg-parchment-50 px-3 py-2 ring-1 ring-parchment-200">
            <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-saffron-500/15 text-saffron-700"><AppIcon name="paperclip" :size="18" /></span>
            <span class="min-w-0 flex-1">
              <span class="block truncate text-sm text-ink-900">{{ it.file.name }}</span>
              <span class="block text-xs text-ink-700/50">{{ fmtSize(it.size) }}</span>
            </span>
            <button class="text-ink-700/40 hover:text-red-600" title="Убрать" @click="removeComposeItem(it)"><AppIcon name="trash" :size="16" /></button>
          </div>
          <!-- картинки и видео -->
          <div v-if="composeMedia.length" class="grid grid-cols-3 gap-2">
            <div v-for="(it, k) in composeMedia" :key="'m' + k" class="group relative aspect-square overflow-hidden rounded-lg bg-ink-900/5 ring-1 ring-parchment-200">
              <img v-if="it.isVideo ? it.poster : it.url" :src="it.isVideo ? it.poster : it.url" class="h-full w-full object-cover" />
              <div v-if="it.isVideo" class="pointer-events-none absolute inset-0 flex items-center justify-center">
                <span class="flex h-9 w-9 items-center justify-center rounded-full bg-black/50 text-white"><AppIcon name="play" :size="18" /></span>
              </div>
              <button class="absolute right-1 top-1 hidden rounded-full bg-ink-900/60 p-1 text-white group-hover:block" title="Убрать" @click="removeComposeItem(it)"><AppIcon name="trash" :size="14" /></button>
            </div>
            <button class="flex aspect-square items-center justify-center rounded-lg border-2 border-dashed border-parchment-300 text-ink-700/50 hover:border-saffron-400 hover:text-saffron-600" title="Добавить" @click="composeInput.click()">
              <AppIcon name="plus" :size="24" />
            </button>
          </div>
          <button v-else class="btn-outline w-full text-sm" @click="composeInput.click()"><AppIcon name="plus" :size="16" /> Добавить</button>
          <input ref="composeInput" type="file" multiple class="hidden" @change="onComposeAdd" />
          <label v-if="composeImages.length" class="flex items-center gap-2.5 text-sm text-ink-800">
            <input type="checkbox" v-model="composeCompress" class="h-4 w-4" /> Сжать изображение
          </label>
          <div>
            <label class="label">Подпись</label>
            <textarea ref="composeCaptionInput" v-model="composeCaption" rows="1" :maxlength="MAX_LEN"
                      class="input max-h-40 resize-none overflow-y-auto" placeholder="Добавьте подпись…"
                      @input="composeAutoGrow" @keydown.enter.exact.prevent="sendCompose"></textarea>
          </div>
        </div>
        <div class="flex items-center justify-end gap-2 border-t border-parchment-200 p-3">
          <button class="btn-ghost" @click="cancelCompose">Отмена</button>
          <button class="btn-primary" :disabled="!composeItems.length" @click="sendCompose">Отправить</button>
        </div>
      </div>
    </div>

    <!-- Контекстное меню списка чатов (ПКМ) -->
    <template v-if="listCtx.open">
      <div class="fixed inset-0 z-40" @click="closeListCtx" @contextmenu.prevent="closeListCtx"></div>
      <div class="fixed z-50 w-52 overflow-hidden rounded-xl bg-white py-1 shadow-xl ring-1 ring-parchment-200" :style="listCtxStyle">
        <button class="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm text-ink-700 hover:bg-parchment-100" @click="listPin">
          <AppIcon name="pin-chat" :size="15" /> {{ listCtx.c?.pinned ? 'Открепить' : 'Закрепить' }}
        </button>
        <button class="flex w-full items-center gap-2.5 border-t border-parchment-100 px-3 py-2 text-left text-sm text-red-600 hover:bg-red-50" @click="listLeave">
          <AppIcon name="logout" :size="15" /> {{ listCtx.c?.type === 'group' ? 'Покинуть группу' : 'Удалить чат' }}
        </button>
      </div>
    </template>

    <!-- Модалка нового чата -->
    <div v-if="showNew" class="fixed inset-0 z-40 flex items-center justify-center bg-ink-900/40 p-4" @click.self="closeNew">
      <div class="flex h-[80vh] w-full max-w-2xl flex-col overflow-hidden rounded-xl bg-white shadow-xl">
        <div class="flex border-b border-parchment-200">
          <button class="flex-1 px-4 py-3 text-sm font-medium" :class="newTab === 'direct' ? 'border-b-2 border-saffron-500 text-saffron-700' : 'text-ink-700/60'" @click="newTab = 'direct'">Личный чат</button>
          <button class="flex-1 px-4 py-3 text-sm font-medium" :class="newTab === 'group' ? 'border-b-2 border-saffron-500 text-saffron-700' : 'text-ink-700/60'" @click="newTab = 'group'">Группа</button>
          <button class="px-3 text-ink-700/40 hover:text-ink-900" @click="closeNew"><AppIcon name="close" :size="18" /></button>
        </div>
        <div v-if="newTab === 'group'" class="border-b border-parchment-200 p-3"><input v-model="groupTitle" class="input" placeholder="Название группы" /></div>
        <div class="border-b border-parchment-200 p-3">
          <div class="relative">
            <AppIcon name="search" :size="16" class="absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-700/40" />
            <input ref="newSearchInput" v-model="contactSearch" class="input h-9 w-full pl-8 text-sm" placeholder="Поиск участников" />
          </div>
        </div>
        <div class="flex-1 overflow-y-auto">
          <p v-if="!filteredContacts.length" class="p-4 text-sm text-ink-700/50">{{ chatState.contacts.length ? 'Никто не найден.' : 'Нет доступных контактов.' }}</p>
          <button v-for="u in filteredContacts" :key="u.id"
                  class="flex w-full items-center gap-3 border-b border-parchment-100 px-4 py-2.5 text-left hover:bg-parchment-50"
                  @click="newTab === 'direct' ? pickDirect(u) : toggleMember(u)">
            <img v-if="u.avatar_url" :src="thumbUrl(u.avatar_url)" @error="imgFull($event, u.avatar_url)" class="photo-bw h-9 w-9 shrink-0 rounded-full object-cover" />
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

    <!-- Модалка настроек группы (название + фото) -->
    <div v-if="showGroupEdit" class="fixed inset-0 z-40 flex items-center justify-center bg-ink-900/40 p-4" @click.self="showGroupEdit = false">
      <div class="w-full max-w-md overflow-hidden rounded-xl bg-white shadow-xl">
        <div class="flex items-center justify-between border-b border-parchment-200 px-4 py-3">
          <h3 class="font-medium text-ink-900">Настройки группы</h3>
          <button class="text-ink-700/40 hover:text-ink-900" @click="showGroupEdit = false"><AppIcon name="close" :size="18" /></button>
        </div>
        <div class="space-y-4 p-4">
          <div class="flex items-center gap-4">
            <img v-if="gPhoto" :src="gPhoto" class="photo-bw h-16 w-16 shrink-0 rounded-full object-cover" />
            <span v-else class="flex h-16 w-16 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-sage-400 to-sage-600 text-2xl font-semibold text-white">{{ initials(gTitle || activeChat?.title) }}</span>
            <div class="flex gap-2">
              <button class="btn-outline text-sm" :disabled="gUploading" @click="groupPhotoInput.click()">{{ gUploading ? '…' : 'Загрузить фото' }}</button>
              <button v-if="gPhoto" class="btn-ghost text-sm text-red-600" @click="gPhoto = ''">Убрать</button>
            </div>
            <input ref="groupPhotoInput" type="file" accept="image/*" class="hidden" @change="onGroupPhoto" />
          </div>
          <div>
            <label class="label">Название</label>
            <input ref="gTitleInput" v-model="gTitle" class="input" placeholder="Название группы" />
          </div>
        </div>
        <div class="flex justify-end gap-2 border-t border-parchment-200 p-3">
          <button class="btn-ghost" @click="showGroupEdit = false">Отмена</button>
          <button class="btn-primary" :disabled="!gTitle.trim()" @click="saveGroup">Сохранить</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* кроссфейд плавающей даты: старая исчезает, новая встаёт на её место (без наложения) */
.datefade-enter-active, .datefade-leave-active { transition: opacity .28s ease, transform .28s ease; }
.datefade-enter-from { opacity: 0; transform: translateY(-6px); }
.datefade-leave-to { opacity: 0; transform: translateY(6px); }
/* подложка медиа: спиннер поверх цветной подложки, пока не загрузилось; после загрузки — прячем */
.ph-done .ph-spin { display: none; }
/* размытая подложка-микропревью (blur-up): картинка «проявляется» из неё, как в Telegram */
.ph-blur { position: absolute; inset: 0; background-size: cover; background-position: center; filter: blur(14px); transform: scale(1.18); }

/* лёгкий эффект удаления: «взрывается и исчезает» (быстрое расширение + растворение) */
.msg-boom { pointer-events: none; will-change: transform, opacity, filter; animation: msgBoom .3s ease-out forwards; }
@keyframes msgBoom {
  0% { transform: scale(1); opacity: 1; filter: blur(0); }
  100% { transform: scale(1.45); opacity: 0; filter: blur(9px); }
}
/* подсветка сообщения при переходе из поиска */
.msg-flash { animation: msgFlash 1.6s ease; border-radius: 0.75rem; }
@keyframes msgFlash {
  0%, 40% { background-color: rgba(200, 116, 42, 0.22); }
  100% { background-color: transparent; }
}
.markdown-on-accent :deep(a) { color: #fff; text-decoration: underline; }
.markdown-on-accent :deep(blockquote) { border-color: rgba(255,255,255,.5); color: rgba(255,255,255,.85); }
/* эмодзи в сообщениях — крупнее текста */
.markdown-body :deep(.chat-emoji) { font-size: 1.5em; line-height: 1; vertical-align: -0.15em; }

/* карточка файла-вложения */
.markdown-body :deep(.chat-file) {
  display: inline-flex; align-items: center; gap: 0.5rem;
  padding: 0.5rem 0.75rem; margin: 0.15rem 0;
  border-radius: 0.75rem; background: rgba(0,0,0,0.05); text-decoration: none;
  max-width: 100%;
}
.markdown-on-accent :deep(.chat-file) { background: rgba(255,255,255,0.18); color: #fff; }
.markdown-body :deep(.chat-file__ic) { font-size: 1.2em; }
.markdown-body :deep(.chat-file__name) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; text-decoration: underline; }

/* тематический ведический фон переписки — ажурные мандалы */
.chat-bg {
  background-color: #f8e9d5;
  background-image: url('../assets/chat-veda-bg.webp');
  background-size: 460px auto;
  background-repeat: repeat;
}
</style>
