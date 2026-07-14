<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import MarkdownEditor from '../components/MarkdownEditor.vue'
import { renderMarkdown } from '../lib/markdown'
import { extractImageUrls, preloadImages } from '../lib/preload'
import { usePageTitle } from '../composables/pageTitle'
import { backTarget } from '../composables/backTarget'
import { confirmDialog } from '../composables/confirm'

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

async function load(silent = false) {
  try {
    const { data } = await client.get(`/forum/topics/${id.value}`)
    await preloadImages(data.posts.flatMap((p) => extractImageUrls(p.body)))
    topic.value = data
  } finally {
    if (!silent) loading.value = false
  }
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
  load()
  nowTimer = setInterval(() => { nowTs.value = Date.now() }, 20000)
  poll = setInterval(() => load(true), 20000)
})
onBeforeUnmount(() => { clearInterval(nowTimer); clearInterval(poll); backTarget.value = null })
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <div v-if="loading" class="space-y-4">
      <AppSkeleton w="w-64" h="h-8" />
      <div class="card space-y-3 p-5"><AppSkeleton /><AppSkeleton w="w-4/5" /></div>
    </div>

    <template v-else-if="topic">
      <h1 class="mb-1 font-display text-2xl font-semibold text-ink-900">{{ topic.title }}</h1>
      <p class="mb-5 text-sm text-ink-700/50">Тему создал {{ topic.author_name || 'Аноним' }}</p>

      <div class="space-y-3">
        <article v-for="p in posts" :key="p.id" data-post class="card p-4 sm:p-5">
          <div class="mb-2 flex items-center gap-3">
            <img v-if="p.author_avatar" :src="p.author_avatar" class="photo-bw h-9 w-9 shrink-0 rounded-full object-cover" />
            <span v-else class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-sm font-semibold text-white">{{ initials(p.author_name) }}</span>
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-medium text-ink-800">{{ p.author_name || 'Аноним' }}</div>
              <div class="text-xs text-ink-700/50">{{ fmt(p.created_at) }}<span v-if="p.edit_count"> · изменено{{ p.edit_count > 1 ? ` ×${p.edit_count}` : '' }}</span></div>
            </div>
            <div v-if="canEdit(p) || canDelete(p)" class="flex shrink-0 items-center gap-1">
              <button v-if="canEdit(p)" class="rounded-md px-2 py-1 text-xs text-ink-700/50 hover:bg-parchment-100 hover:text-ink-700" @click="startEdit(p)">Изменить</button>
              <button v-if="canDelete(p)" class="rounded-md px-2 py-1 text-xs text-red-500/70 hover:bg-red-50 hover:text-red-600" @click="removePost(p)">Удалить</button>
            </div>
          </div>
          <div class="markdown-body break-words text-ink-800" v-html="renderMarkdown(p.body)"></div>
        </article>
      </div>

      <!-- ввод сообщения (тот же интерфейс, что и в чате) -->
      <div v-if="auth.can('forum.post')" class="mt-5">
        <div v-if="editingPost" class="mb-1 flex items-center gap-2 rounded-lg border-l-2 border-saffron-400 bg-parchment-100 px-3 py-1.5">
          <AppIcon name="edit" :size="16" class="shrink-0 text-saffron-600" />
          <div class="min-w-0 flex-1 text-sm font-semibold text-saffron-700">Редактирование</div>
          <button class="shrink-0 rounded-full p-1 text-ink-700/50 hover:bg-parchment-200" @click="cancelEdit"><AppIcon name="close" :size="16" /></button>
        </div>
        <MarkdownEditor v-model="body" :rows="4" type-anywhere hide-hint :draft-scope="`forum-topic:${id}`" placeholder="Написать сообщение…" @submit="send" />
        <div class="mt-1 flex justify-end gap-2">
          <button v-if="editingPost" class="btn-ghost" @click="cancelEdit">Отмена</button>
          <button class="btn-primary" :disabled="sending || !body.trim()" @click="send">{{ editingPost ? (sending ? '…' : 'Сохранить') : (sending ? '…' : 'Отправить') }}</button>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.markdown-body :deep(a) { text-decoration: underline; }
.markdown-body :deep(ul) { margin: 0.25rem 0; padding-left: 1.25rem; list-style: disc; }
.markdown-body :deep(ol) { margin: 0.25rem 0; padding-left: 1.35rem; list-style: decimal; }
.markdown-body :deep(img) { max-height: 22rem; border-radius: 0.5rem; margin: 0.35rem 0; }
</style>
