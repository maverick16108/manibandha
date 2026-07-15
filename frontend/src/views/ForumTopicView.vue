<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import MarkdownEditor from '../components/MarkdownEditor.vue'
import EventsFastScroll from '../components/EventsFastScroll.vue'
import { renderMarkdown } from '../lib/markdown'
import { extractImageUrls, preloadImages } from '../lib/preload'
import { usePageTitle } from '../composables/pageTitle'
import { backTarget } from '../composables/backTarget'
import { confirmDialog } from '../composables/confirm'
import { refreshNavCounts } from '../composables/navCounts'

const route = useRoute()
const auth = useAuthStore()
const id = computed(() => route.params.id)
backTarget.value = { name: 'forum' }

const topic = ref(null)
const loading = ref(true)
const body = ref('')
const sending = ref(false)
const editingPost = ref(null)
let prevBody = ''

const EDIT_WINDOW = 3600_000
const nowTs = ref(Date.now())
let nowTimer = null
const isMod = computed(() => auth.can('forum.moderate'))
function canEdit(p) {
  return p.author_id === auth.user?.id && (nowTs.value - new Date(p.created_at).getTime()) <= EDIT_WINDOW
}
function canDelete(p) { return isMod.value || canEdit(p) }
const posts = computed(() => topic.value?.posts || [])

usePageTitle(() => topic.value?.title || 'Тема')

function fmt(iso) {
  return new Date(iso).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
}
function initials(name) { return (name || '?').trim()[0]?.toUpperCase() || '?' }

// мини-профиль участника форума
const card = ref(null)
async function openCard(userId) {
  if (!userId) return
  card.value = { _loading: true }
  try { const { data } = await client.get(`/forum/users/${userId}`); card.value = data } catch { card.value = null }
}
function closeCard() { card.value = null }
function onEsc(e) {
  if (e.key !== 'Escape') return
  if (whoMenu.open) closeWho()
  else if (picker.open) closePicker()
  else if (card.value) closeCard()
}
function placeLine(c) { return [c.city, c.region, c.country].filter(Boolean).join(', ') }

// точки для быстрого скроллера по датам сообщений
const feedPoints = computed(() => posts.value.map((p) => ({
  id: `post-${p.id}`,
  label: new Date(p.created_at).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }),
})))

async function load(silent = false) {
  try {
    // фоновый опрос не накручивает счётчик просмотров
    const { data } = await client.get(`/forum/topics/${id.value}`, { params: silent ? { count: false } : {} })
    await preloadImages(data.posts.flatMap((p) => extractImageUrls(p.body)))
    topic.value = data
  } finally {
    if (!silent) loading.value = false
  }
}

// ── реакции-эмодзи (любой смайлик, как в вопросах) ──
const QUICK = ['❤️', '👍', '🙏', '🔥', '😂', '🎉']
const EMOJI_PALETTE = [
  '❤️', '🧡', '💛', '💚', '💙', '💜', '🤍', '🔥', '👍', '👎', '🙏', '👏', '🙌', '🤝', '💪', '✌️',
  '😀', '😁', '😂', '🤣', '😊', '😇', '🙂', '😍', '🥰', '😘', '😎', '🤩', '🥳', '😌', '🤔', '😴',
  '😢', '😭', '😱', '😳', '🤯', '😤', '😅', '😉', '😋', '🤗', '🤭', '🫡', '🫶', '💯', '✨', '⭐',
  '🎉', '🎊', '🌸', '🌺', '🌼', '🌈', '☀️', '🕉️', '🪔', '📿', '🌿', '🍀', '☘️', '🐄', '🦚', '🪷',
]
const picker = reactive({ open: false, postId: null, x: 0, y: 0 })
const pickerStyle = computed(() => {
  const w = 288, h = 300
  const x = Math.min(picker.x, window.innerWidth - w - 8)
  const y = Math.min(picker.y, window.innerHeight - h - 8)
  return { left: Math.max(8, x) + 'px', top: Math.max(8, y) + 'px' }
})
function openPicker(e, p) {
  picker.open = true; picker.postId = p.id
  picker.x = e.clientX; picker.y = e.clientY
}
function closePicker() { picker.open = false; picker.postId = null }
async function react(p, emoji) {
  closePicker()
  try {
    const { data } = await client.post(`/forum/posts/${p.id}/like`, { emoji })
    p.reactions = data.reactions; p.likes = data.likes; p.liked = data.liked; p.likers = data.likers
  } catch { /* игнор */ }
}
function reactPicked(emoji) {
  const p = posts.value.find((x) => x.id === picker.postId)
  if (p) react(p, emoji)
}

// ПКМ по реакции — список тех, кто её поставил (с аватарками, со скроллом)
const whoMenu = reactive({ open: false, x: 0, y: 0, list: [] })
const whoStyle = computed(() => {
  const w = 240, h = 280
  const x = Math.min(whoMenu.x, window.innerWidth - w - 8)
  const y = Math.min(whoMenu.y, window.innerHeight - h - 8)
  return { left: Math.max(8, x) + 'px', top: Math.max(8, y) + 'px' }
})
function openWho(e, r) {
  whoMenu.open = true; whoMenu.list = r.who || []
  whoMenu.x = e.clientX; whoMenu.y = e.clientY
}
function closeWho() { whoMenu.open = false; whoMenu.list = [] }

// ── цитирование ──
function quoteInto(author, text) {
  const q = `> **${author}**:\n` + String(text).split('\n').map((l) => `> ${l}`).join('\n') + '\n\n'
  body.value = (body.value ? body.value.replace(/\s*$/, '') + '\n\n' : '') + q
  nextTick(() => { const ta = document.querySelector('textarea'); if (ta) { ta.focus(); ta.scrollIntoView({ block: 'center' }) } })
}
function quotePost(p) {
  const text = (p.body || '').replace(/@\[audio\]\([^)]*\)/g, '🎤 голосовое').replace(/!\[[^\]]*\]\([^)]*\)/g, '🖼 фото').trim()
  quoteInto(p.author_name || 'Аноним', text)
}
const quoteBar = reactive({ show: false, x: 0, y: 0, text: '', author: '' })
function onDocMouseUp(e) {
  if (e.target.closest?.('.quote-float')) return
  const sel = window.getSelection()
  const text = sel ? sel.toString().trim() : ''
  const art = e.target.closest?.('[data-post]')
  if (!text || !art) { quoteBar.show = false; return }
  const rect = sel.getRangeAt(0).getBoundingClientRect()
  quoteBar.text = text
  quoteBar.author = art.dataset.postAuthor || 'Аноним'
  quoteBar.x = rect.left + rect.width / 2
  quoteBar.y = rect.top - 10
  quoteBar.show = true
}
function doQuoteSelection() {
  quoteInto(quoteBar.author, quoteBar.text)
  quoteBar.show = false
  window.getSelection()?.removeAllRanges()
}
function hideQuoteBar() { quoteBar.show = false }
// при прокрутке закрываем всплывающие меню (иначе «висят» на месте, а страница уезжает)
function onWindowScroll() {
  hideQuoteBar()
  if (picker.open) closePicker()
  if (whoMenu.open) closeWho()
}

async function send() {
  if (editingPost.value) { await saveEdit(); return }
  const text = body.value.trim()
  if (!text) return
  sending.value = true
  try {
    await client.post(`/forum/topics/${id.value}/posts`, { body: text })
    body.value = ''
    try { await client.delete(`/drafts/forum-topic:${id.value}`) } catch { /* игнор */ }
    await load(true)
    await nextTick()
    scrollToEnd()
  } finally {
    sending.value = false
  }
}
function scrollToEnd() {
  const els = document.querySelectorAll('[data-post]')
  els[els.length - 1]?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}

function startEdit(p) {
  prevBody = body.value
  editingPost.value = p
  body.value = p.body
}
function cancelEdit() { editingPost.value = null; body.value = prevBody; prevBody = '' }
async function saveEdit() {
  const p = editingPost.value
  const text = body.value.trim()
  if (!p || !text) return
  sending.value = true
  try {
    const { data } = await client.patch(`/forum/posts/${p.id}`, { body: text })
    p.body = data.body; p.edit_count = data.edit_count
    editingPost.value = null; body.value = prevBody; prevBody = ''
    await preloadImages(extractImageUrls(p.body))
  } catch (e) {
    alert(e.response?.data?.detail || 'Не удалось изменить')
  } finally {
    sending.value = false
  }
}
async function removePost(p) {
  const ok = await confirmDialog({ message: 'Удалить сообщение?', confirmText: 'Удалить', danger: true })
  if (!ok) return
  try {
    await client.delete(`/forum/posts/${p.id}`)
    topic.value.posts = topic.value.posts.filter((x) => x.id !== p.id)
    if (!topic.value.posts.length) { window.history.length > 1 ? history.back() : (location.href = '/app/forum') }
  } catch (e) {
    alert(e.response?.data?.detail || 'Не удалось удалить')
  }
}

let poll = null
onMounted(() => {
  load().then(refreshNavCounts) // тема прочитана — сразу гасим бейдж в меню
  nowTimer = setInterval(() => { nowTs.value = Date.now() }, 20000)
  poll = setInterval(() => load(true), 5000) // живой опрос — новые сообщения/лайки видны почти сразу
  document.addEventListener('mouseup', onDocMouseUp)
  document.addEventListener('keydown', onEsc)
  window.addEventListener('scroll', onWindowScroll, { passive: true })
})
onBeforeUnmount(() => {
  clearInterval(nowTimer); clearInterval(poll); backTarget.value = null
  document.removeEventListener('mouseup', onDocMouseUp)
  document.removeEventListener('keydown', onEsc)
  window.removeEventListener('scroll', onWindowScroll)
})
</script>

<template>
  <div class="mx-auto max-w-6xl lg:flex lg:items-start lg:gap-4">
    <div class="min-w-0 flex-1">
    <div v-if="loading" class="space-y-4">
      <AppSkeleton w="w-64" h="h-8" />
      <div class="card space-y-3 p-5"><AppSkeleton /><AppSkeleton w="w-4/5" /></div>
    </div>

    <template v-else-if="topic">
      <div v-if="topic.section_title" class="mb-1 flex items-center gap-1.5">
        <span class="h-2.5 w-2.5 rounded-sm" :style="{ background: topic.section_color }"></span>
        <span class="text-sm font-medium text-ink-700/70">{{ topic.section_title }}</span>
      </div>
      <h1 class="mb-1 font-display text-2xl font-semibold text-ink-900">{{ topic.title }}</h1>
      <p class="mb-4 text-sm text-ink-700/50">Тему создал {{ topic.author_name || 'Аноним' }}</p>
      <img v-if="topic.cover_url" :src="topic.cover_url" alt="" class="mb-5 max-h-72 w-full rounded-xl object-cover" />

      <div class="space-y-3">
        <article v-for="p in posts" :id="`post-${p.id}`" :key="p.id" data-post :data-post-author="p.author_name || 'Аноним'" class="card scroll-mt-20 p-4 sm:p-5" @contextmenu.prevent="openPicker($event, p)">
          <div class="mb-2 flex items-center gap-3">
            <button class="shrink-0" title="Профиль" @click="openCard(p.author_id)">
              <img v-if="p.author_avatar" :src="p.author_avatar" class="photo-bw h-9 w-9 rounded-full object-cover ring-2 ring-transparent transition hover:ring-saffron-400" />
              <span v-else class="flex h-9 w-9 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-sm font-semibold text-white ring-2 ring-transparent transition hover:ring-saffron-400">{{ initials(p.author_name) }}</span>
            </button>
            <div class="min-w-0 flex-1">
              <button class="block max-w-full truncate text-left text-sm font-medium text-ink-800 hover:text-saffron-700 hover:underline" @click="openCard(p.author_id)">{{ p.author_name || 'Аноним' }}</button>
              <div class="text-xs text-ink-700/50">{{ fmt(p.created_at) }}<span v-if="p.edit_count"> · изменено{{ p.edit_count > 1 ? ` ×${p.edit_count}` : '' }}</span></div>
            </div>
            <div v-if="canEdit(p) || canDelete(p)" class="flex shrink-0 items-center gap-1">
              <button v-if="canEdit(p)" class="rounded-lg p-1.5 text-ink-700/50 transition hover:bg-parchment-100 hover:text-ink-800" title="Изменить" @click="startEdit(p)"><AppIcon name="edit" :size="17" /></button>
              <button v-if="canDelete(p)" class="rounded-lg p-1.5 text-red-500/70 transition hover:bg-red-50 hover:text-red-600" title="Удалить" @click="removePost(p)"><AppIcon name="trash" :size="17" /></button>
            </div>
          </div>
          <div class="markdown-body break-words text-ink-800" v-html="renderMarkdown(p.body)"></div>

          <!-- реакции-эмодзи (любой смайлик), кто поставил, цитирование -->
          <div class="mt-2 flex flex-wrap items-center gap-1.5">
            <button v-for="r in p.reactions" :key="r.emoji"
                    class="flex items-center gap-1 rounded-full border px-2 py-0.5 transition"
                    :class="r.mine ? 'border-saffron-400 bg-saffron-50' : 'border-parchment-200 hover:bg-parchment-100'"
                    @click="react(p, r.emoji)"
                    @contextmenu.prevent.stop="openWho($event, r)">
              <span class="text-xl leading-none">{{ r.emoji }}</span>
              <span v-if="r.count > 1" class="text-sm font-semibold text-ink-700">{{ r.count }}</span>
            </button>
            <button class="flex h-8 w-8 items-center justify-center rounded-full text-lg leading-none text-ink-700/40 transition hover:bg-parchment-100 hover:text-saffron-600" title="Поставить реакцию" @click.stop="openPicker($event, p)">🙂</button>
            <button v-if="auth.can('forum.post')" class="ml-auto flex items-center gap-1 rounded-md px-2 py-0.5 text-xs text-ink-700/50 hover:bg-parchment-100 hover:text-saffron-700" @click="quotePost(p)">
              <AppIcon name="reply" :size="14" /> Цитировать
            </button>
          </div>
        </article>
      </div>

      <!-- выбор реакции: 6 быстрых + палитра любых смайликов -->
      <template v-if="picker.open">
        <div class="fixed inset-0 z-40" @click="closePicker" @contextmenu.prevent="closePicker"></div>
        <div class="fixed z-50 w-72 rounded-xl border border-parchment-200 bg-white p-2 shadow-xl" :style="pickerStyle">
          <div class="mb-1.5 flex justify-between gap-0.5 border-b border-parchment-100 pb-1.5">
            <button v-for="e in QUICK" :key="e" class="rounded-full px-0.5 text-2xl leading-none transition-transform hover:scale-125" @click="reactPicked(e)">{{ e }}</button>
          </div>
          <div class="grid max-h-44 grid-cols-8 gap-0.5 overflow-y-auto">
            <button v-for="e in EMOJI_PALETTE" :key="e" class="rounded p-1 text-xl leading-none transition hover:bg-parchment-100" @click="reactPicked(e)">{{ e }}</button>
          </div>
        </div>
      </template>

      <!-- список поставивших реакцию (ПКМ по чипу) -->
      <template v-if="whoMenu.open">
        <div class="fixed inset-0 z-40" @click="closeWho" @contextmenu.prevent="closeWho"></div>
        <div class="fixed z-50 max-h-64 w-60 overflow-y-auto rounded-xl border border-parchment-200 bg-white p-1.5 shadow-xl" :style="whoStyle">
          <div v-for="(w, wi) in whoMenu.list" :key="wi" class="flex items-center gap-2 rounded-lg px-2 py-1.5">
            <img v-if="w.avatar" :src="w.avatar" class="photo-bw h-7 w-7 shrink-0 rounded-full object-cover" />
            <span v-else class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-xs font-semibold text-white">{{ initials(w.name) }}</span>
            <span class="truncate text-sm text-ink-800">{{ w.name || 'Участник' }}</span>
          </div>
        </div>
      </template>

      <!-- плавающая кнопка «Цитировать выделенное» -->
      <button v-if="quoteBar.show && auth.can('forum.post')"
              class="quote-float fixed z-50 -translate-x-1/2 -translate-y-full rounded-lg bg-ink-900 px-3 py-1.5 text-sm font-medium text-white shadow-lg"
              :style="{ left: quoteBar.x + 'px', top: quoteBar.y + 'px' }"
              @mousedown.prevent @click="doQuoteSelection">
        Цитировать
      </button>

      <!-- ввод сообщения (тот же интерфейс, что и в чате) -->
      <div v-if="auth.can('forum.post')" class="mt-5">
        <div v-if="editingPost" class="mb-1 flex items-center gap-2 rounded-lg border-l-2 border-saffron-400 bg-parchment-100 px-3 py-1.5">
          <AppIcon name="edit" :size="16" class="shrink-0 text-saffron-600" />
          <div class="min-w-0 flex-1 text-sm font-semibold text-saffron-700">Редактирование</div>
          <button class="shrink-0 rounded-full p-1 text-ink-700/50 hover:bg-parchment-200" @click="cancelEdit"><AppIcon name="close" :size="16" /></button>
        </div>
        <MarkdownEditor v-model="body" :rows="4" grip="top" scroll-page type-anywhere hide-hint :draft-scope="editingPost ? '' : `forum-topic:${id}`" placeholder="Написать сообщение…" @submit="send" />
        <div class="mt-1 flex justify-end gap-2">
          <button v-if="editingPost" class="btn-ghost" @click="cancelEdit">Отмена</button>
          <button class="btn-primary" :disabled="sending || !body.trim()" @click="send">{{ editingPost ? (sending ? '…' : 'Сохранить') : (sending ? '…' : 'Отправить') }}</button>
        </div>
      </div>
    </template>
    </div>

    <!-- быстрый скроллер по датам сообщений -->
    <aside v-if="topic && feedPoints.length > 3" class="sticky top-20 hidden h-[calc(100vh-6rem)] w-8 shrink-0 lg:block">
      <EventsFastScroll :points="feedPoints" />
    </aside>

    <!-- мини-профиль участника -->
    <div v-if="card" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-4" @click.self="closeCard">
      <div class="card w-full max-w-sm p-6 text-center">
        <template v-if="card._loading">
          <div class="py-8 text-ink-700/50">Загрузка…</div>
        </template>
        <template v-else>
          <img v-if="card.photo" :src="card.photo" class="photo-bw mx-auto h-28 w-28 rounded-xl object-cover" />
          <div v-else class="mx-auto flex h-28 w-28 items-center justify-center rounded-xl bg-gradient-to-br from-saffron-400 to-saffron-600 text-4xl font-semibold text-white">{{ initials(card.name) }}</div>
          <h3 class="mt-4 font-display text-2xl font-semibold text-ink-900">{{ card.name || 'Участник' }}</h3>
          <p v-if="placeLine(card)" class="mt-1 flex items-center justify-center gap-1 text-sm text-ink-700/70"><AppIcon name="pin" :size="14" class="text-saffron-600" /> {{ placeLine(card) }}</p>
          <button class="btn-ghost mt-5" @click="closeCard">Закрыть</button>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.markdown-body :deep(a) { text-decoration: underline; }
.markdown-body :deep(ul) { margin: 0.25rem 0; padding-left: 1.25rem; list-style: disc; }
.markdown-body :deep(ol) { margin: 0.25rem 0; padding-left: 1.35rem; list-style: decimal; }
.markdown-body :deep(blockquote) { border-left: 3px solid rgba(200,116,42,0.5); background: rgba(200,116,42,0.06); padding: 0.4rem 0.75rem; margin: 0.4rem 0; border-radius: 0 0.4rem 0.4rem 0; color: rgba(60,50,45,0.85); }
.markdown-body :deep(img) { max-height: 22rem; border-radius: 0.5rem; margin: 0.35rem 0; }
</style>
