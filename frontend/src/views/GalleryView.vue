<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
defineOptions({ name: 'GalleryView' })
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { confirmDialog } from '../composables/confirm'
import { showToast } from '../composables/toast'
import { usePageTitle } from '../composables/pageTitle'
import { useAutoRefresh } from '../composables/useAutoRefresh'
import { cachedGet, invalidatePrefix, peekCache } from '../composables/apiCache'
import { thumbUrl, imgFull } from '../lib/format'
import { onEscape } from '../composables/useEscape'

usePageTitle('Галерея')
const auth = useAuthStore()
const canManage = computed(() => auth.can('gallery.manage'))

const loading = ref(true)
const albums = ref([])
const open = ref(null) // открытый альбом { id, title, description, is_home, photos, bw }
const uploading = ref(false)
const fileInput = ref(null)
const editAlbum = ref(null) // модалка создания/редактирования { id?, title, description }
const savingAlbum = ref(false)
const lb = ref(-1) // индекс фото в лайтбоксе

async function load(silent = false) {
  const cached = peekCache('/gallery/albums')
  if (cached) { albums.value = cached.albums || []; loading.value = false }
  else if (!silent) loading.value = true
  try {
    const data = await cachedGet('/gallery/albums', { force: silent })
    albums.value = data.albums || []
  } finally { loading.value = false }
}
onMounted(load)
useAutoRefresh(load)
onEscape(() => { if (lb.value >= 0) lb.value = -1; else if (editAlbum.value) editAlbum.value = null; else if (open.value) open.value = null })

async function openAlbum(a) {
  open.value = { ...a, photos: [] }
  try { const { data } = await client.get(`/gallery/albums/${a.id}`); open.value = data } catch { showToast('Не удалось открыть альбом') }
}
async function refreshOpen() { if (!open.value) return; const { data } = await client.get(`/gallery/albums/${open.value.id}`); open.value = data }

// ── создание / редактирование альбома ──
function newAlbum() { editAlbum.value = { title: '', description: '' } }
function editCurrent() { editAlbum.value = { id: open.value.id, title: open.value.title, description: open.value.description || '' } }
async function saveAlbum() {
  const t = (editAlbum.value.title || '').trim()
  if (!t) return
  savingAlbum.value = true
  try {
    if (editAlbum.value.id) {
      await client.patch(`/gallery/albums/${editAlbum.value.id}`, { title: t, description: editAlbum.value.description })
      if (open.value?.id === editAlbum.value.id) { open.value.title = t; open.value.description = editAlbum.value.description }
    } else {
      await client.post('/gallery/albums', { title: t, description: editAlbum.value.description })
    }
    editAlbum.value = null
    invalidatePrefix('/gallery'); await load(true)
  } catch (e) { showToast(e.response?.data?.detail || 'Не удалось сохранить') } finally { savingAlbum.value = false }
}
async function delAlbum() {
  if (!open.value || open.value.is_home) return
  if (!(await confirmDialog({ message: `Удалить альбом «${open.value.title}» со всеми фотографиями?`, confirmText: 'Удалить', danger: true }))) return
  try { await client.delete(`/gallery/albums/${open.value.id}`); open.value = null; invalidatePrefix('/gallery'); await load(true) }
  catch (e) { showToast(e.response?.data?.detail || 'Не удалось удалить') }
}

// ── фотографии ──
async function uploadFiles(fileList) {
  const files = [...(fileList || [])].filter((f) => f.type.startsWith('image/'))
  if (!files.length || !open.value) return
  uploading.value = true
  try {
    const fd = new FormData()
    files.forEach((f) => fd.append('files', f))
    const { data } = await client.post('/uploads', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    const urls = data.urls || []
    if (urls.length) {
      await client.post(`/gallery/albums/${open.value.id}/photos`, { urls })
      await refreshOpen(); invalidatePrefix('/gallery')
    }
  } catch { showToast('Не удалось загрузить фото') } finally { uploading.value = false }
}
function onFiles(ev) { const f = ev.target.files; if (fileInput.value) fileInput.value.value = ''; uploadFiles(f) }
// drag-and-drop загрузка в открытый альбом
const dragOver = ref(false)
function onDrop(ev) { dragOver.value = false; if (canManage.value) uploadFiles(ev.dataTransfer?.files) }
async function delPhoto(p) {
  if (!(await confirmDialog({ message: 'Удалить фотографию?', confirmText: 'Удалить', danger: true }))) return
  try { await client.delete(`/gallery/photos/${p.id}`); open.value.photos = open.value.photos.filter((x) => x.id !== p.id); invalidatePrefix('/gallery') }
  catch { showToast('Не удалось удалить') }
}
async function toggleBw() {
  if (!open.value) return
  open.value.bw = !open.value.bw
  try { await client.patch(`/gallery/albums/${open.value.id}`, { bw: open.value.bw }) } catch { showToast('Не удалось сохранить') }
}

const lbRot = ref(0)
function lbRotate() { lbRot.value = (lbRot.value + 90) % 360 }
function lbNext() { if (open.value?.photos?.length) lb.value = (lb.value + 1) % open.value.photos.length }
function lbPrev() { if (open.value?.photos?.length) lb.value = (lb.value - 1 + open.value.photos.length) % open.value.photos.length }
watch(lb, () => { lbRot.value = 0 }) // сброс поворота при смене фото
function onLbKey(e) {
  if (lb.value < 0) return
  if (e.key === 'ArrowLeft') lbPrev()
  else if (e.key === 'ArrowRight') lbNext()
  else if (e.key === 'r' || e.key === 'R' || e.key === 'к' || e.key === 'К') lbRotate()
}
onMounted(() => document.addEventListener('keydown', onLbKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onLbKey))
</script>

<template>
  <div>
    <!-- ── СПИСОК АЛЬБОМОВ ── -->
    <template v-if="!open">
      <div class="mx-auto mb-6 flex max-w-6xl flex-wrap items-center justify-between gap-3">
        <p class="text-ink-700/60">Альбомы с фотографиями</p>
        <button v-if="canManage" class="btn-primary" @click="newAlbum">+ Альбом</button>
      </div>

      <div v-if="loading" class="grid gap-5 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
        <div v-for="i in 6" :key="i" class="card overflow-hidden"><AppSkeleton h="h-44" /><div class="p-4"><AppSkeleton w="w-40" /></div></div>
      </div>

      <div v-else-if="!albums.length" class="card mx-auto max-w-6xl p-12 text-center text-ink-700/50">Альбомов пока нет</div>

      <div v-else class="grid gap-5 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
        <button v-for="a in albums" :key="a.id" class="card group overflow-hidden text-left transition hover:shadow-md" @click="openAlbum(a)">
          <div class="relative aspect-[4/3] w-full overflow-hidden bg-parchment-100">
            <img v-if="a.cover" :src="thumbUrl(a.cover)" @error="imgFull($event, a.cover)" class="h-full w-full object-cover transition duration-300 group-hover:scale-105" />
            <div v-else class="flex h-full w-full items-center justify-center text-ink-700/25"><AppIcon name="image" :size="40" /></div>
            <span v-if="a.is_home" class="absolute left-3 top-3 rounded-full bg-saffron-500 px-2.5 py-0.5 text-xs font-medium text-white shadow">Главная</span>
          </div>
          <div class="p-4">
            <div class="truncate font-medium text-ink-900">{{ a.title }}</div>
            <div class="mt-0.5 text-sm text-ink-700/50">{{ a.photo_count }} фото</div>
          </div>
        </button>
      </div>
    </template>

    <!-- ── АЛЬБОМ ── -->
    <template v-else>
      <div class="mx-auto mb-5 flex max-w-6xl flex-wrap items-center gap-3">
        <button class="btn-outline shrink-0" @click="open = null"><AppIcon name="chevron" :size="16" class="rotate-90" /> Альбомы</button>
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <h2 class="truncate font-display text-2xl font-semibold text-ink-900">{{ open.title }}</h2>
            <span v-if="open.is_home" class="rounded-full bg-saffron-500 px-2.5 py-0.5 text-xs font-medium text-white">Главная</span>
          </div>
          <p v-if="open.description" class="truncate text-sm text-ink-700/60">{{ open.description }}</p>
        </div>
        <div v-if="canManage" class="flex shrink-0 items-center gap-2">
          <button class="btn-outline text-sm" @click="fileInput.click()" :disabled="uploading"><AppIcon name="image" :size="15" /> {{ uploading ? 'Загрузка…' : 'Добавить фото' }}</button>
          <button v-if="!open.is_home" class="btn-ghost p-2" title="Переименовать" @click="editCurrent"><AppIcon name="edit" :size="18" /></button>
          <button v-if="!open.is_home" class="rounded-lg p-2 text-ink-700/40 transition hover:bg-red-50 hover:text-red-600" title="Удалить альбом" @click="delAlbum"><AppIcon name="trash" :size="18" /></button>
          <input ref="fileInput" type="file" accept="image/*" multiple class="hidden" @change="onFiles" />
        </div>
      </div>

      <!-- ч/б для главной -->
      <label v-if="open.is_home && canManage" class="mx-auto mb-4 flex max-w-6xl items-center gap-2 text-sm text-ink-800">
        <input type="checkbox" :checked="open.bw" @change="toggleBw" /> Показывать фото на главной чёрно-белыми
      </label>

      <!-- зона перетаскивания фото (drag-and-drop) -->
      <div class="relative rounded-2xl transition" :class="dragOver && 'ring-2 ring-saffron-400 ring-offset-2'"
           @dragenter.prevent="canManage && (dragOver = true)" @dragover.prevent @dragleave.prevent="dragOver = false" @drop.prevent="onDrop">
        <div v-if="dragOver" class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center rounded-2xl bg-saffron-500/10 text-saffron-700">
          <span class="rounded-full bg-white/90 px-4 py-2 text-sm font-medium shadow">Отпустите, чтобы добавить фото</span>
        </div>

        <div v-if="!open.photos.length" class="card p-12 text-center text-ink-700/50">
          {{ canManage ? 'Перетащите фотографии сюда или нажмите «Добавить фото»' : 'В альбоме пока нет фотографий' }}
        </div>
        <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          <div v-for="(p, i) in open.photos" :key="p.id" class="group relative aspect-square overflow-hidden rounded-xl bg-parchment-100">
            <img :src="thumbUrl(p.url)" @error="imgFull($event, p.url)" class="h-full w-full cursor-zoom-in object-cover transition duration-300 group-hover:scale-105" @click="lb = i" />
            <button v-if="canManage" class="absolute right-2 top-2 rounded-lg bg-black/45 p-1.5 text-white opacity-0 transition hover:bg-red-600 group-hover:opacity-100" title="Удалить" @click.stop="delPhoto(p)"><AppIcon name="trash" :size="16" /></button>
          </div>
        </div>
      </div>
    </template>

    <!-- создание/редактирование альбома -->
    <div v-if="editAlbum" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-4" @click.self="editAlbum = null">
      <div class="w-full max-w-md overflow-hidden rounded-2xl bg-white shadow-xl">
        <header class="flex items-center justify-between border-b border-parchment-200 px-5 py-3.5">
          <h3 class="font-medium text-ink-900">{{ editAlbum.id ? 'Изменить альбом' : 'Новый альбом' }}</h3>
          <button class="rounded-lg p-1.5 text-ink-700/60 hover:bg-parchment-100" @click="editAlbum = null"><AppIcon name="close" :size="22" /></button>
        </header>
        <div class="space-y-4 p-5">
          <div>
            <label class="label">Название</label>
            <input v-model="editAlbum.title" class="input" placeholder="Название альбома" @keydown.enter="saveAlbum" autofocus />
          </div>
          <div>
            <label class="label">Описание</label>
            <textarea v-model="editAlbum.description" rows="3" class="input resize-y" placeholder="Необязательно"></textarea>
          </div>
        </div>
        <div class="flex justify-end gap-2 border-t border-parchment-200 px-5 py-3.5">
          <button class="btn-ghost" @click="editAlbum = null">Отмена</button>
          <button class="btn-primary" :disabled="savingAlbum || !editAlbum.title.trim()" @click="saveAlbum">{{ savingAlbum ? '…' : 'Сохранить' }}</button>
        </div>
      </div>
    </div>

    <!-- лайтбокс -->
    <div v-if="lb >= 0 && open" class="fixed inset-0 z-[60] flex items-center justify-center bg-ink-900/90 p-4" @click.self="lb = -1">
      <div class="absolute right-4 top-4 flex items-center gap-1">
        <button class="rounded-lg p-2 text-white/80 hover:bg-white/10" title="Повернуть (R)" @click.stop="lbRotate"><AppIcon name="rotate" :size="24" /></button>
        <button class="rounded-lg p-2 text-white/80 hover:bg-white/10" title="Закрыть" @click="lb = -1"><AppIcon name="close" :size="28" /></button>
      </div>
      <button v-if="open.photos.length > 1" class="absolute left-3 rounded-full p-3 text-white/80 hover:bg-white/10" title="Назад (←)" @click.stop="lbPrev"><AppIcon name="chevron" :size="28" class="rotate-90" /></button>
      <img :src="open.photos[lb].url" :style="{ transform: `rotate(${lbRot}deg)` }" class="max-h-[90vh] max-w-full rounded-lg object-contain transition-transform duration-200" :class="lbRot % 180 && 'max-h-[92vw] max-w-[92vh]'" />
      <button v-if="open.photos.length > 1" class="absolute right-3 rounded-full p-3 text-white/80 hover:bg-white/10" title="Вперёд (→)" @click.stop="lbNext"><AppIcon name="chevron" :size="28" class="-rotate-90" /></button>
    </div>
  </div>
</template>
