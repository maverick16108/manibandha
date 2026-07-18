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
import { openLightbox } from '../composables/lightbox'
import { showToast } from '../composables/toast'
import { confirmDialog } from '../composables/confirm'
import { usePageTitle } from '../composables/pageTitle'
import {
  chatState, initChat, openChat, closeChat, sendMessage, sendTyping,
  editMessage, deleteMessage, retryFailed, loadOlder, loadContacts, startDirect, startGroup,
  reactMessage, REACTION_EMOJIS, updateChat, pinChat, leaveChat, forwardMessages, loadAroundSeq, markActiveRead, imageAspect,
} from '../chat/store'

usePageTitle('Чат')

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const MAX_LEN = 1000
const body = ref('')
const replyTo = ref(null)
const editingMedia = ref('') // при правке фото: сохраняем медиа-часть (![](url)), редактируем только подпись
const editingMsg = ref(null)
const scroller = ref(null)
const listWrap = ref(null)
const stickBottom = ref(true)          // держимся ли у нижнего края (иначе не дёргаем при подгрузке)
let listObs = null
// Пока пользователь у нижнего края — прижимаем ленту к низу при любом росте высоты
// (догрузка картинок/файлов и т.п.), чтобы не было «прыжка» после открытия чата.
watch(listWrap, (el) => {
  if (listObs) { listObs.disconnect(); listObs = null }
  if (el && typeof ResizeObserver !== 'undefined') {
    listObs = new ResizeObserver(() => {
      if (!stickBottom.value) return
      const s = scroller.value; if (s) s.scrollTop = s.scrollHeight
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

watch(activeId, async (id, oldId) => {
  if (oldId && !editingMsg.value) saveDraft(oldId, body.value) // сохранить черновик прежнего чата
  replyTo.value = null; editingMsg.value = null; closeCtx()
  body.value = id ? loadDraft(id) : ''
  openSettled.value = false
  if (id) {
    stickBottom.value = true
    nextTick(() => inputEl.value?.focus()) // фокус сразу, не ждём загрузки истории
    await openChat(id); scrollToBottom()
    setTimeout(() => { openSettled.value = true }, 500) // после открытия — авто-читаем живые входящие
  } else closeChat()
  nextTick(autoGrow)
}, { immediate: false })

// автооткрытие самого верхнего чата, когда стоим на пустом экране (десктоп)
function maybeAutoOpen() {
  if (chatState.ready && !activeId.value && chatState.chats.length && window.innerWidth >= 640) {
    router.replace({ name: 'chat', params: { id: chatState.chats[0].id } })
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
function selectChat(c) { router.push({ name: 'chat', params: { id: c.id } }) }
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
  await deleteMessage(m.id, everyone)
}
function cleanBody(b) {
  return (b || '').replace(/@\[audio\]\([^)]*\)/g, '🎤 Голосовое сообщение').replace(/!\[[^\]]*\]\([^)]*\)/g, '').trim()
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
  // удаляем все одним пакетом (параллельно) — быстро и надёжно
  await Promise.all(msgs.map((m) => deleteMessage(m.id, !isDir ? isMine(m) : (isMine(m) ? deleteManyForAll.value : false))))
}
// копировать можно только когда есть текст (голосовое/фото — нечего копировать)
function canCopy(m) {
  if (!m) return false
  const t = (m.body || '').replace(/@\[audio\]\([^)]*\)/g, '').replace(/!\[[^\]]*\]\([^)]*\)/g, '').trim()
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
    .replace(/@\[file\]\([^|)]*\|([^)]*)\)/g, (_m, name) => { try { return '📎 ' + decodeURIComponent(name) } catch { return '📎 файл' } })
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '🖼 Фото')
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
    .replace(/@\[file\]\([^|)]*\|([^)]*)\)/g, (_m, name) => { try { return '📎 ' + decodeURIComponent(name) } catch { return '📎 файл' } })
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '🖼 Фото')
    .replace(/\s+/g, ' ').trim().slice(0, 80)
}

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
watch(body, () => { nextTick(autoGrow); if (activeId.value && !editingMsg.value) saveDraftDebounced(activeId.value, body.value) })

let lastTyping = 0
function onKeydown(e) {
  // не давать браузеру сбрасывать текст поля на Escape (нативный revert);
  // закрытие оверлеев/ответа/редактирования делает глобальный обработчик
  if (e.key === 'Escape') { e.preventDefault(); return }
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
const composeFiles = computed(() => composeItems.value.filter((it) => !it.isImage))
function plural(n) { const a = n % 10, b = n % 100; if (a === 1 && b !== 11) return 'файл'; if (a >= 2 && a <= 4 && (b < 10 || b >= 20)) return 'файла'; return 'файлов' }
const composeTitle = computed(() => {
  if (!composeImages.value.length) return 'Отправить как файл'
  if (!composeFiles.value.length) return 'Отправить изображение'
  return `Выбрано ${composeItems.value.length} ${plural(composeItems.value.length)}`
})
function fmtSize(bytes) { if (!bytes) return ''; if (bytes < 1024) return `${bytes} Б`; if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} КБ`; return `${(bytes / 1048576).toFixed(1)} МБ` }
function composeAutoGrow() { const el = composeCaptionInput.value; if (!el) return; el.style.height = 'auto'; const b = el.offsetHeight - el.clientHeight; el.style.height = Math.min(el.scrollHeight + b, 160) + 'px' }
watch(showCompose, (v) => { if (v) nextTick(() => { composeCaptionInput.value?.focus(); composeAutoGrow() }) })
function addComposeItems(files, compress) {
  if (compress !== undefined) composeCompress.value = compress
  for (const f of Array.from(files)) {
    const isImage = (f.type || '').startsWith('image/')
    composeItems.value.push({ file: f, url: isImage ? URL.createObjectURL(f) : null, isImage, size: f.size })
  }
}
function removeComposeItem(it) { const i = composeItems.value.indexOf(it); if (i < 0) return; if (it.url) URL.revokeObjectURL(it.url); composeItems.value.splice(i, 1) }
function cancelCompose() { composeItems.value.forEach((it) => it.url && URL.revokeObjectURL(it.url)); composeItems.value = []; composeCaption.value = ''; composeCompress.value = true }
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
  const { compress, cap, imgs, files } = pu
  try {
    if (imgs.length) {
      const urls = await Promise.all(imgs.map((it) => uploadOne(it.file)))
      let body = urls.map((u) => `![](${u})`).join('')
      if (cap && !files.length) body += `\n${cap}`
      await sendMessage(body)
      removePending(pu)
    }
    let first = !imgs.length
    for (const it of files) {
      const url = await uploadOne(it.file)
      const name = (it.file.name || 'файл').replace(/[|)(]/g, '_')
      let s = `@[file](${url}|${encodeURIComponent(name)})`
      if (first && cap) { s += `\n${cap}`; first = false }
      await sendMessage(s)
      if (it.url) URL.revokeObjectURL(it.url)
    }
    scrollToBottom()
  } catch { pu.failed = true }
}
function retryPending(pu) { runUpload(pu) }
async function sendCompose() {
  const items = [...composeItems.value]; const cap = composeCaption.value.trim(); const compress = composeCompress.value
  if (!items.length) return
  const chatId = activeId.value
  // с «сжать» — картинки идут альбомом (![]); без сжатия — всё как файлы
  const imgs = compress ? items.filter((it) => it.isImage) : []
  const files = compress ? items.filter((it) => !it.isImage) : items
  // закрываем диалог мгновенно, blob-превью переносим в pending (НЕ revoke)
  composeItems.value = []; composeCaption.value = ''; composeCompress.value = true
  const pu = reactive({ id: `up-${uploadSeq++}`, chatId, compress, cap, imgs, files, failed: false,
    previews: imgs.map((it) => ({ url: it.url })) })
  if (imgs.length) { pendingUploads.push(pu); nextTick(scrollToBottom) }
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

// пересылка: маркер «@[fwd](имя)» в начале тела — кто исходный автор
const FWD_RE = /^@\[fwd\]\(([^)]*)\)\n?/
function fwdName(m) { const mm = (m?.body || '').match(FWD_RE); if (!mm) return ''; try { return decodeURIComponent(mm[1]) } catch { return mm[1] } }
function contentBody(m) { return (m?.body || '').replace(FWD_RE, '') }
function fwdWrap(m) {
  const b = m.body || ''
  if (FWD_RE.test(b)) return b            // уже переслано — сохраняем исходного автора
  return `@[fwd](${encodeURIComponent(nameOf(m))})\n${b}`
}

// содержимое сообщения: картинки / подпись / вложения
function photoUrls(m) {
  const b = contentBody(m)
  if (m.deleted || /@\[audio\]|@\[file\]/.test(b)) return []
  const urls = []; b.replace(/!\[[^\]]*\]\(([^)]+)\)/g, (_x, u) => { urls.push(u); return '' }); return urls
}
function captionText(m) { return contentBody(m).replace(/!\[[^\]]*\]\([^)]*\)/g, '').trim() }
function isPhoto(m) { return photoUrls(m).length > 0 }
// все фото чата по порядку — для навигации в лайтбоксе (←/→, свайп)
const allChatPhotos = computed(() => {
  const out = []
  for (const m of chatState.messages) if (!m.deleted) for (const u of photoUrls(m)) out.push(u)
  return out
})
function openPhoto(u) { openLightbox(u, allChatPhotos.value) }
// сетка-альбом под количество фото (как в мессенджерах)
function albumCols(n) { return n <= 1 ? '' : (n <= 4 ? 'grid-cols-2' : 'grid-cols-3') }
function albumItemClass(n, k) { return (n === 3 && k === 0) ? 'col-span-2' : '' } // 3 фото: первое во всю ширину
// резерв места под одиночное фото по известным пропорциям — чтобы лента не «прыгала» при загрузке
function photoBoxStyle(u) {
  const r = imageAspect(u)
  return r ? { aspectRatio: String(r), maxHeight: '400px' } : { minHeight: '140px' }
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

async function onScroll() {
  if (ctx.open) closeCtx()
  const el = scroller.value
  if (!el) return
  stickBottom.value = (el.scrollHeight - el.scrollTop - el.clientHeight) < 60
  if (el.scrollTop < 40) { const prevH = el.scrollHeight; const n = await loadOlder(); if (n) nextTick(() => { el.scrollTop = el.scrollHeight - prevH }) }
}

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
function maritalLabel(v) {
  const m = { single: 'Не в браке', married: 'В браке', widowed: 'Вдова / вдовец', divorced: 'В разводе', unmarried: 'Не в браке' }
  return m[v] || v
}
const infoAvatar = computed(() => { const p = infoData.value?.peer; return p ? (p.avatar || null) : null })
const cityLine = computed(() => { const p = infoData.value?.peer; return p ? [p.city, p.region].filter(Boolean).join(', ') : '' })

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
const searchChat = reactive({ open: false, q: '', results: [], loading: false })
const searchChatInput = ref(null)
let searchChatTimer = null
function openChatSearch() {
  if (!activeId.value) return
  searchChat.open = true
  nextTick(() => { searchChatInput.value?.focus(); searchChatInput.value?.select?.() })
}
function closeChatSearch() { searchChat.open = false; searchChat.q = ''; searchChat.results = []; searchChat.loading = false }
watch(() => searchChat.q, (q) => {
  clearTimeout(searchChatTimer)
  const term = (q || '').trim()
  if (term.length < 2) { searchChat.results = []; searchChat.loading = false; return }
  searchChat.loading = true
  searchChatTimer = setTimeout(async () => {
    const cid = activeId.value
    try {
      const { data } = await client.get(`/chats/${cid}/search`, { params: { q: term } })
      if (activeId.value === cid && searchChat.open) searchChat.results = data
    } catch { searchChat.results = [] } finally { searchChat.loading = false }
  }, 300)
})
watch(activeId, () => { if (searchChat.open) closeChatSearch() })
async function jumpToMessage(m) {
  closeChatSearch()
  if (!chatState.messages.some((x) => x.id === m.id)) {
    try { await loadAroundSeq(m.seq) } catch { /* ignore */ }
  }
  await nextTick()
  const el = document.getElementById(`msg-${m.id}`)
  if (el) {
    el.scrollIntoView({ block: 'center', behavior: 'smooth' })
    el.classList.add('msg-flash'); setTimeout(() => el.classList.remove('msg-flash'), 1600)
  }
}

function onGlobalKey(e) {
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey && e.code === 'KeyF') {
    if (activeId.value) { e.preventDefault(); openChatSearch() }
    return
  }
  if (e.key !== 'Escape') return
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
  document.addEventListener('visibilitychange', onChatVisible)
  window.addEventListener('focus', onChatVisible)
  if (convEl.value && typeof ResizeObserver !== 'undefined') {
    resizeObs = new ResizeObserver((entries) => { for (const e of entries) wide.value = e.contentRect.width > 900 })
    resizeObs.observe(convEl.value)
  }
  if (!auth.isPending && auth.user) {
    await initChat({ meId: auth.user.id, getToken: () => auth.token })
    if (activeId.value) { stickBottom.value = true; await openChat(activeId.value); scrollToBottom(); setTimeout(() => { openSettled.value = true }, 500) }
    else maybeAutoOpen()
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
        <div class="flex items-center gap-2 border-b border-parchment-200 p-3">
          <div class="relative flex-1">
            <AppIcon name="search" :size="16" class="absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-700/40" />
            <input ref="searchChatInput" v-model="searchChat.q" class="input h-9 w-full pl-8 pr-8 text-sm" placeholder="Поиск в этом чате" />
            <button v-if="searchChat.q" @click="searchChat.q = ''" title="Очистить"
                    class="absolute right-2 top-1/2 -translate-y-1/2 text-ink-700/40 hover:text-ink-700"><AppIcon name="close" :size="15" /></button>
          </div>
          <button class="rounded-lg p-1.5 text-ink-700/60 hover:bg-parchment-100" title="Закрыть поиск" @click="closeChatSearch"><AppIcon name="close" :size="18" /></button>
        </div>
        <div class="border-b border-parchment-200 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-ink-700/50">Поиск в чате</div>
        <div class="flex-1 overflow-y-auto">
          <p v-if="searchChat.q.trim().length < 2" class="p-8 text-center text-sm text-ink-700/40">Введите минимум 2 символа</p>
          <p v-else-if="searchChat.loading && !searchChat.results.length" class="p-4 text-sm text-ink-700/50">Идёт поиск…</p>
          <p v-else-if="!searchChat.results.length" class="p-8 text-center text-sm text-ink-700/50">Ничего не найдено</p>
          <button v-for="m in searchChat.results" :key="m.id" @click="jumpToMessage(m)"
                  class="flex w-full flex-col gap-0.5 border-b border-parchment-100 px-3 py-2.5 text-left hover:bg-parchment-50">
            <span class="flex items-center justify-between gap-2">
              <span class="truncate text-sm font-medium text-ink-900">{{ m.author_name || 'Без имени' }}</span>
              <span class="shrink-0 text-[11px] text-ink-700/40">{{ fmtListTime(m.created_at) }}</span>
            </span>
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
        <button class="btn-primary h-9 shrink-0 px-3" title="Новый чат" @click="openNew"><AppIcon name="plus" :size="18" /></button>
      </div>
      <div class="flex-1 overflow-y-auto">
        <p v-if="!chatState.ready" class="p-4 text-sm text-ink-700/50">Загрузка…</p>
        <p v-else-if="!filteredChats.length" class="p-4 text-sm text-ink-700/50">Чатов пока нет. Нажмите «плюс», чтобы начать.</p>
        <button v-for="c in filteredChats" :key="c.id"
                class="flex w-full items-center gap-3 border-b border-parchment-100 px-3 py-2.5 text-left hover:bg-parchment-50"
                :class="c.id === activeId && 'bg-saffron-500/10'" @click="selectChat(c)" @contextmenu="onListContext($event, c)">
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
              <span class="truncate text-sm text-ink-700/60">{{ lastPreview(c) }}</span>
              <span v-if="c.unread" class="ml-1 inline-flex h-5 min-w-[1.25rem] shrink-0 items-center justify-center rounded-full bg-saffron-500 px-1.5 text-xs font-semibold text-white">{{ c.unread }}</span>
            </span>
          </span>
        </button>
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
              <div class="truncate text-xs text-ink-700/50">
                <span v-if="typingLabel" class="text-saffron-600">{{ typingLabel }}</span>
                <span v-else-if="activeChat.type === 'group'">{{ activeChat.members.length }} участников</span>
                <span v-else>{{ chatState.connection === 'online' ? 'в сети' : 'не в сети' }}</span>
              </div>
            </div>
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
              </div>
            </div>
          </div>
        </transition>

        <AudioBar />

        <div ref="scroller" class="chat-bg flex flex-1 flex-col overflow-y-auto p-4"
             @scroll="onScroll" @click="onScrollerClick" @mousedown="onScrollerDown" @touchstart="onScrollerDown">
          <div ref="listWrap" class="mt-auto space-y-1">
          <template v-for="(m, i) in chatState.messages" :key="m.client_uuid">
          <div v-if="m.client_uuid === firstUnreadKey" class="my-3 flex items-center gap-2 px-2">
            <span class="h-px flex-1 bg-saffron-400/60"></span>
            <span class="rounded-full bg-saffron-500 px-3 py-0.5 text-xs font-semibold text-white shadow-sm">Непрочитанные</span>
            <span class="h-px flex-1 bg-saffron-400/60"></span>
          </div>
          <div :id="`msg-${m.id}`"
               class="group flex items-end gap-2 rounded-xl px-1 transition-colors"
               :class="[selectMode ? 'cursor-pointer select-none justify-start' : rowJustify(m), selectMode && selected.has(m.id) && 'bg-saffron-500/10']"
               @click.capture="onRowClick($event, m)"
               @mousedown="selDragStart($event, m, i)" @mouseenter="selDragEnter(i)">
            <!-- аватар (в группах, слева от сообщения — и у чужих, и у своих) -->
            <template v-if="isGroup && !isMine(m)">
              <img v-if="avatarOf(m) && isRunEnd(m, i)" :src="thumbUrl(avatarOf(m))" @error="imgFull($event, avatarOf(m))" class="photo-bw h-10 w-10 shrink-0 rounded-full object-cover" />
              <span v-else-if="isRunEnd(m, i)" class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-sage-400 to-sage-600 text-sm font-semibold text-white">{{ initials(nameOf(m)) }}</span>
              <span v-else class="h-10 w-10 shrink-0"></span>
            </template>
            <template v-else-if="isGroup && isMine(m)">
              <img v-if="myAvatar && isRunEnd(m, i)" :src="thumbUrl(myAvatar)" @error="imgFull($event, myAvatar)" class="photo-bw h-10 w-10 shrink-0 rounded-full object-cover" />
              <span v-else-if="isRunEnd(m, i)" class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-sm font-semibold text-white">{{ initials(myName) }}</span>
              <span v-else class="h-10 w-10 shrink-0"></span>
            </template>
            <!-- ФОТО-сообщение: без «полей» пузыря (как в телеге) -->
            <div v-if="isPhoto(m)" class="relative overflow-hidden rounded-2xl shadow-sm"
                 :class="[wide ? 'max-w-[420px]' : 'max-w-[80%]', captionText(m) && (isMine(m) ? 'bg-saffron-500 text-white' : 'bg-white text-ink-900 ring-1 ring-parchment-200')]"
                 @contextmenu="onContext($event, m)">
              <div v-if="showAuthor(m, i)" class="px-3 pt-2 text-xs font-semibold text-sage-600">{{ nameOf(m) }}</div>
              <div v-if="fwdName(m)" class="flex items-center gap-1 px-3 pt-2 text-xs font-semibold" :class="captionText(m) && isMine(m) ? 'text-white/90' : 'text-saffron-700'">
                <AppIcon name="reply" :size="12" class="-scale-x-100" /> Переслано от {{ fwdName(m) }}
              </div>
              <div v-if="m.reply_preview" class="mx-3 mt-2 flex items-center gap-2 rounded-r-md border-l-2 border-saffron-400 bg-black/5 py-1 pl-2 pr-2 text-xs">
                <img v-if="replyThumb(m)" :src="replyThumb(m)" class="h-8 w-8 shrink-0 rounded object-cover" />
                <div class="min-w-0 flex-1">
                  <div v-if="replyAuthorName(m)" class="font-semibold text-saffron-700">{{ replyAuthorName(m) }}</div>
                  <div class="whitespace-pre-wrap break-words text-ink-700/70">{{ m.reply_preview }}</div>
                </div>
              </div>
              <div v-if="photoUrls(m).length === 1" class="w-full overflow-hidden" :style="photoBoxStyle(photoUrls(m)[0])">
                <img :src="thumbUrl(photoUrls(m)[0])" @error="imgFull($event, photoUrls(m)[0])"
                     class="block h-full max-h-[400px] w-full cursor-zoom-in object-cover" @click.stop="openPhoto(photoUrls(m)[0])" />
              </div>
              <div v-else class="grid gap-0.5" :class="albumCols(photoUrls(m).length)">
                <img v-for="(u, k) in photoUrls(m).slice(0, 10)" :key="k" :src="thumbUrl(u)" @error="imgFull($event, u)"
                     class="aspect-square w-full cursor-zoom-in object-cover" :class="albumItemClass(photoUrls(m).length, k)"
                     @click.stop="openPhoto(u)" />
              </div>
              <div v-if="captionText(m)" class="markdown-body break-words px-3.5 pt-1.5 text-[15px]" :class="isMine(m) && 'markdown-on-accent'" v-html="renderChatBody(captionText(m))"></div>
              <!-- реакции + время в одной строке (с подписью) -->
              <div v-if="captionText(m)" class="flex items-end justify-between gap-2 px-2.5 pb-1.5 pt-1">
                <div class="flex flex-wrap gap-1">
                  <button v-for="r in parseReactions(m)" :key="r.emoji" @click.stop="onChip(m, r.emoji)" @contextmenu.prevent.stop="openWho($event, r)" title="ПКМ — кто поставил"
                          class="flex items-center gap-1 rounded-full px-2.5 py-1 leading-none ring-1 transition"
                          :class="m.my_reaction === r.emoji ? 'bg-saffron-500/25 text-saffron-800 ring-saffron-400' : 'bg-saffron-500/10 text-ink-700 ring-transparent hover:bg-saffron-500/20'"><span class="text-xl leading-none">{{ r.emoji }}</span><span v-if="r.count > 1" class="text-sm font-semibold tabular-nums">{{ r.count }}</span></button>
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
                          :class="m.my_reaction === r.emoji && 'ring-2 ring-white/70'"><span class="text-lg leading-none">{{ r.emoji }}</span><span v-if="r.count > 1" class="text-xs font-semibold tabular-nums">{{ r.count }}</span></button>
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
              <div v-if="showAuthor(m, i)" class="mb-0.5 text-xs font-semibold text-sage-600">{{ nameOf(m) }}</div>
              <div v-if="fwdName(m)" class="mb-1 flex items-center gap-1 text-xs font-semibold" :class="isMine(m) ? 'text-white/90' : 'text-saffron-700'">
                <AppIcon name="reply" :size="12" class="-scale-x-100" /> Переслано от {{ fwdName(m) }}
              </div>
              <div v-if="m.reply_preview" class="mb-1 flex items-center gap-2 rounded-r-md border-l-2 py-1 pl-2 pr-2 text-xs" :class="isMine(m) ? 'border-white/70 bg-white/10' : 'border-saffron-400 bg-saffron-500/5'">
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
                <!-- YouTube-миниатюра hqdefault — 4:3 с чёрными полями; кадрируем до 16:9 -->
                <div v-if="linkCard(m).image" class="overflow-hidden" :class="linkCard(m).image.includes('ytimg') && 'aspect-video'">
                  <img :src="linkCard(m).image" class="block w-full object-cover"
                       :class="linkCard(m).image.includes('ytimg') ? 'h-full' : 'max-h-[360px]'"
                       @error="$event.target.parentElement.style.display='none'" />
                </div>
              </a>

              <div class="mt-1 flex items-end justify-between gap-2">
                <div v-if="parseReactions(m).length" class="flex flex-wrap gap-1">
                  <button v-for="r in parseReactions(m)" :key="r.emoji" @click.stop="onChip(m, r.emoji)" @contextmenu.prevent.stop="openWho($event, r)"
                          title="ПКМ — кто поставил"
                          class="flex items-center gap-1 rounded-full px-2.5 py-1 leading-none ring-1 transition"
                          :class="isMine(m)
                            ? (m.my_reaction === r.emoji ? 'bg-white/25 ring-white/60' : 'bg-white/15 ring-white/20 hover:bg-white/25')
                            : (m.my_reaction === r.emoji ? 'bg-saffron-500/25 text-saffron-800 ring-saffron-400' : 'bg-saffron-500/10 text-ink-700 ring-transparent hover:bg-saffron-500/20')">
                    <span class="text-xl leading-none">{{ r.emoji }}</span><span v-if="r.count > 1" class="text-sm font-semibold tabular-nums">{{ r.count }}</span>
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
                  <img :src="p.url" class="w-full object-cover" :class="pu.previews.length === 1 ? 'max-h-[400px]' : 'aspect-square'" />
                  <div class="absolute inset-0 flex items-center justify-center bg-black/30">
                    <span v-if="!pu.failed" class="h-7 w-7 animate-spin rounded-full border-2 border-white/40 border-t-white"></span>
                    <button v-else class="flex items-center gap-1 rounded-full bg-black/55 px-3 py-1.5 text-xs font-medium text-white" @click="retryPending(pu)"><AppIcon name="reply" :size="14" class="-scale-x-100" /> Повторить</button>
                  </div>
                </div>
              </div>
              <div v-if="pu.cap" class="px-3.5 py-1.5 text-[15px] text-white">{{ pu.cap }}</div>
            </div>
          </div>
          </div>
        </div>

        <!-- кнопка «вниз» (видна, когда прокручено вверх) -->
        <transition name="fade">
          <button v-if="activeChat && !stickBottom" @click="stickBottom = true; scrollToBottom()" title="Вниз"
                  class="absolute bottom-24 right-5 z-20 flex h-11 w-11 items-center justify-center rounded-full bg-white text-ink-700 shadow-lg ring-1 ring-parchment-200 transition hover:bg-parchment-50">
            <AppIcon name="chevron" :size="22" />
          </button>
        </transition>

        <!-- Композер -->
        <div class="border-t border-parchment-200 p-3">
          <div v-if="replyTo" class="mb-2 flex items-center gap-2 rounded-lg bg-parchment-100 px-3 py-1.5 text-sm">
            <AppIcon name="reply" :size="14" class="shrink-0 text-saffron-600" />
            <img v-if="replyTo.photo" :src="thumbUrl(replyTo.photo)" class="h-8 w-8 shrink-0 rounded object-cover" />
            <span class="min-w-0 flex-1 truncate text-ink-700/70"><b class="text-ink-800">{{ replyTo.author_name }}</b>: {{ replyTo.body }}</span>
            <button class="text-ink-700/50 hover:text-ink-900" @click="replyTo = null"><AppIcon name="close" :size="15" /></button>
          </div>
          <div v-else-if="editingMsg" class="mb-2 flex items-center gap-2 rounded-lg border-l-2 border-saffron-400 bg-parchment-100 px-3 py-1.5 text-sm">
            <AppIcon name="edit" :size="14" class="shrink-0 text-saffron-600" />
            <span class="min-w-0 flex-1 truncate text-ink-700/70"><b class="text-saffron-700">Редактирование</b> · {{ snippet(editingMsg.body) }}</span>
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
              <AppIcon name="paperclip" :size="24" />
            </button>
            <input ref="fileInput" type="file" multiple class="hidden" @change="onPickFile" />

            <textarea ref="inputEl" v-model="body" rows="1" :maxlength="MAX_LEN"
                      class="chat-input min-h-[2.75rem] flex-1 resize-none rounded-2xl border border-parchment-300 bg-parchment-50 px-4 py-2.5 text-base leading-6 focus:border-saffron-400 focus:outline-none focus:ring-1 focus:ring-saffron-400"
                      placeholder="Сообщение…" @input="onInput" @keydown="onKeydown"></textarea>

            <div class="relative mb-0.5 shrink-0">
              <button class="rounded-full p-2 text-ink-700/60 hover:bg-parchment-100 hover:text-saffron-600" title="Эмодзи" @click="showEmoji = !showEmoji">
                <AppIcon name="react" :size="24" />
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
            <button v-else class="mb-0.5 shrink-0 rounded-full p-2 text-ink-700/60 hover:bg-parchment-100 hover:text-saffron-600" title="Голосовое" :disabled="uploading" @click="startRec">
              <AppIcon name="mic" :size="24" />
            </button>
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
      <div class="fixed z-50 w-52 overflow-hidden rounded-xl bg-white py-1 shadow-xl ring-1 ring-parchment-200" :style="ctxStyle">
        <div class="flex justify-around px-2 py-1.5">
          <button v-for="e in REACTION_EMOJIS" :key="e" class="rounded-full p-1 text-lg leading-none transition hover:scale-125" @click="ctxReact(e)">{{ e }}</button>
        </div>
        <div class="my-1 border-t border-parchment-100"></div>
        <button class="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm text-ink-700 hover:bg-parchment-100" @click="ctxReply"><AppIcon name="reply" :size="15" /> Ответить</button>
        <button v-if="canCopy(ctx.m) || ctx.selText" class="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm text-ink-700 hover:bg-parchment-100" @click="ctxCopy"><AppIcon name="copy" :size="15" /> Копировать</button>
        <button v-if="canEdit(ctx.m)" class="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm text-ink-700 hover:bg-parchment-100" @click="ctxEdit"><AppIcon name="edit" :size="15" /> Изменить</button>
        <button v-if="ctx.m && !ctx.m.deleted" class="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm text-ink-700 hover:bg-parchment-100" @click="ctxForward"><AppIcon name="reply" :size="15" class="-scale-x-100" /> Переслать</button>
        <button v-if="canDelete(ctx.m)" class="flex w-full items-center gap-2.5 border-t border-parchment-100 px-3 py-2 text-left text-sm text-red-600 hover:bg-red-50" @click="ctxDelete"><AppIcon name="trash" :size="15" /> {{ delLabel(ctx.m) }}</button>
        <button v-if="ctx.m && !ctx.m.deleted" class="flex w-full items-center gap-2.5 border-t border-parchment-100 px-3 py-2 text-left text-sm text-ink-700 hover:bg-parchment-100" @click="ctxSelect"><AppIcon name="check" :size="15" /> Выделить</button>
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

    <!-- Диалог отправки вложений (картинки + файлы) -->
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
          <!-- картинки -->
          <div v-if="composeImages.length" class="grid grid-cols-3 gap-2">
            <div v-for="(it, k) in composeImages" :key="'i' + k" class="group relative aspect-square overflow-hidden rounded-lg ring-1 ring-parchment-200">
              <img :src="it.url" class="h-full w-full object-cover" />
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
