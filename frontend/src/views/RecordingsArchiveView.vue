<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import AppDatePicker from '../components/AppDatePicker.vue'
import { backTarget } from '../composables/backTarget'
import { confirmDialog } from '../composables/confirm'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Записи конференций')
const auth = useAuthStore()

const loading = ref(true)
const recordings = ref([])
const playing = ref(null)
const search = ref('')
const dateFrom = ref('')
const dateTo = ref('')

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return recordings.value.filter((r) => {
    if (q && ![r.title, r.description, r.conference_title].some((s) => (s || '').toLowerCase().includes(q))) return false
    const d = (r.started_at || '').slice(0, 10)
    if (dateFrom.value && d < dateFrom.value) return false
    if (dateTo.value && d > dateTo.value) return false
    return true
  })
})
function resetFilters() { search.value = ''; dateFrom.value = ''; dateTo.value = '' }
const editing = ref(null)
const editForm = ref({ title: '', description: '' })
const saving = ref(false)

async function load() {
  loading.value = true
  try { const { data } = await client.get('/conferences/recordings'); recordings.value = data.recordings || [] } finally { loading.value = false }
}
// поиск при наборе в любом месте страницы
const searchInput = ref(null)
function onDocType(e) {
  if (e.ctrlKey || e.metaKey || e.altKey) return
  const t = e.target
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
  if (editing.value !== null) return
  if (e.key === 'Escape') { search.value = ''; return }
  if (e.key === 'Backspace') { if (search.value) { search.value = search.value.slice(0, -1); e.preventDefault() }; return }
  if (e.key.length === 1) { search.value += e.key; e.preventDefault(); nextTick(() => searchInput.value?.focus()) }
}
onMounted(() => { backTarget.value = { name: 'conference' }; load(); document.addEventListener('keydown', onDocType) })
onBeforeUnmount(() => document.removeEventListener('keydown', onDocType))

function recUrl(r) { return `${r.url}?token=${encodeURIComponent(auth.token)}` }
function fmt(iso) { return iso ? new Date(iso).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' }) : '' }
function fmtDur(ms) {
  let s = Math.floor((ms || 0) / 1000); const h = Math.floor(s / 3600); s -= h * 3600
  const m = Math.floor(s / 60); s -= m * 60; const pad = (n) => String(n).padStart(2, '0')
  return h ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`
}
function fmtSize(b) { const mb = (b || 0) / 1048576; return mb >= 1024 ? `${(mb / 1024).toFixed(1)} ГБ` : `${Math.max(1, Math.round(mb))} МБ` }

function startEdit(r) { editing.value = r.id; editForm.value = { title: r.title || '', description: r.description || '' } }
function cancelEdit() { editing.value = null }
async function saveEdit(r) {
  saving.value = true
  try {
    const { data } = await client.patch(`/conferences/recordings/${r.id}`, { title: editForm.value.title, description: editForm.value.description })
    Object.assign(r, { title: data.title, description: data.description })
    editing.value = null
  } catch (e) { alert(e.response?.data?.detail || 'Не удалось сохранить') } finally { saving.value = false }
}
async function remove(r) {
  const ok = await confirmDialog({ message: `Удалить запись «${r.title}»? Файл будет удалён с сервера безвозвратно.`, confirmText: 'Удалить', danger: true })
  if (!ok) return
  try {
    await client.delete(`/conferences/recordings/${r.id}`)
    recordings.value = recordings.value.filter((x) => x.id !== r.id)
    if (playing.value === r.id) playing.value = null
  } catch (e) { alert(e.response?.data?.detail || 'Не удалось удалить') }
}
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <p class="mb-4 text-ink-700/60">Записи прошедших конференций — можно смотреть, скачивать и подписывать</p>

    <!-- поиск + фильтр по дате -->
    <div v-if="!loading && recordings.length" class="mb-4 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
      <div class="flex items-center gap-2 rounded-md border border-parchment-300 bg-white px-3 py-2 sm:w-72">
        <AppIcon name="search" :size="16" class="shrink-0 text-ink-700/40" />
        <input ref="searchInput" v-model="search" class="w-full bg-transparent text-sm text-ink-800 outline-none placeholder:text-ink-700/40" placeholder="Поиск по названию, описанию" />
      </div>
      <div class="flex items-center gap-2">
        <span class="text-sm text-ink-700/60">с</span>
        <div class="w-40"><AppDatePicker v-model="dateFrom" /></div>
        <span class="text-sm text-ink-700/60">по</span>
        <div class="w-40"><AppDatePicker v-model="dateTo" /></div>
      </div>
      <button v-if="search || dateFrom || dateTo" class="btn-ghost text-sm" @click="resetFilters">Сбросить</button>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 3" :key="i" class="card space-y-2 p-4"><AppSkeleton w="w-56" /><AppSkeleton w="w-40" h="h-3" /></div>
    </div>

    <div v-else-if="!recordings.length" class="card p-10 text-center text-ink-700/50">Записей пока нет</div>
    <div v-else-if="!filtered.length" class="card p-10 text-center text-ink-700/50">Ничего не найдено</div>

    <div v-else class="space-y-3">
      <div v-for="r in filtered" :key="r.id" class="card p-4 sm:p-5">
        <!-- режим просмотра -->
        <template v-if="editing !== r.id">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <h3 class="truncate font-display text-lg font-semibold text-ink-900">{{ r.title }}</h3>
              <div class="mt-0.5 text-xs text-ink-700/50">
                <span v-if="r.conference_title && r.conference_title !== r.title">{{ r.conference_title }} · </span>{{ fmt(r.started_at) }} · {{ fmtDur(r.duration_ms) }} · {{ fmtSize(r.size_bytes) }}
              </div>
              <p v-if="r.description" class="mt-1.5 whitespace-pre-wrap text-sm text-ink-700/80">{{ r.description }}</p>
            </div>
            <div class="flex shrink-0 items-center gap-1.5">
              <button class="btn-outline text-sm" @click="playing = playing === r.id ? null : r.id"><AppIcon name="play" :size="14" /> {{ playing === r.id ? 'Скрыть' : 'Смотреть' }}</button>
              <a :href="recUrl(r)" download class="btn-ghost p-2" title="Скачать"><AppIcon name="download" :size="18" /></a>
              <button v-if="r.can_edit" class="btn-ghost p-2" title="Изменить название и описание" @click="startEdit(r)"><AppIcon name="edit" :size="18" /></button>
              <button v-if="r.can_edit" class="rounded-lg p-2 text-ink-700/40 transition hover:bg-red-50 hover:text-red-600" title="Удалить запись" @click="remove(r)"><AppIcon name="trash" :size="18" /></button>
            </div>
          </div>
          <video v-if="playing === r.id" :src="recUrl(r)" controls autoplay class="mt-3 w-full rounded-lg bg-ink-900"></video>
        </template>

        <!-- режим редактирования -->
        <div v-else class="space-y-3">
          <div>
            <label class="label">Название записи</label>
            <input v-model="editForm.title" class="input" :placeholder="r.conference_title || 'Название'" />
          </div>
          <div>
            <label class="label">Описание</label>
            <textarea v-model="editForm.description" rows="3" class="input resize-y" placeholder="О чём эта запись (необязательно)"></textarea>
          </div>
          <div class="flex gap-2">
            <button class="btn-primary" :disabled="saving" @click="saveEdit(r)">{{ saving ? '…' : 'Сохранить' }}</button>
            <button class="btn-ghost" @click="cancelEdit">Отмена</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
