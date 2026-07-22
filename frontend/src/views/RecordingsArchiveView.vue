<script setup>
import { ref, computed, onMounted, onActivated, onBeforeUnmount, nextTick } from 'vue'
defineOptions({ name: 'RecordingsArchiveView' })
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
  if (e.key === 'Escape' && parts.value) { closeParticipants(); return }
  if (e.key === 'Escape' && editing.value) { cancelEdit(); return }
  const t = e.target
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
  if (editing.value !== null) return
  if (e.key === 'Escape') { search.value = ''; return }
  if (e.key === 'Backspace') { if (search.value) { search.value = search.value.slice(0, -1); e.preventDefault() }; return }
  if (e.key.length === 1) { search.value += e.key; e.preventDefault(); nextTick(() => searchInput.value?.focus()) }
}
onMounted(() => { backTarget.value = { name: 'conference' }; load(); document.addEventListener('keydown', onDocType) })
// keep-alive: обновляем список записей при возврате (первую активацию пропускаем)
let firstActivate = true
onActivated(() => { if (firstActivate) { firstActivate = false; return } load() })
onBeforeUnmount(() => document.removeEventListener('keydown', onDocType))

function recUrl(r) { return `${r.url}?token=${encodeURIComponent(auth.token)}` }
function fmt(iso) { return iso ? new Date(iso).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' }) : '' }
function fmtDur(ms) {
  let s = Math.floor((ms || 0) / 1000); const h = Math.floor(s / 3600); s -= h * 3600
  const m = Math.floor(s / 60); s -= m * 60; const pad = (n) => String(n).padStart(2, '0')
  return h ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`
}
function fmtSize(b) { const mb = (b || 0) / 1048576; return mb >= 1024 ? `${(mb / 1024).toFixed(1)} ГБ` : `${Math.max(1, Math.round(mb))} МБ` }
function initials(name) { return (name || '?').trim().split(/\s+/).slice(0, 2).map((w) => w[0]).join('').toUpperCase() }

// ── кто был на созвоне ──
const parts = ref(null)        // { title, list } | null — открытый попап
const partsLoading = ref(false)
async function openParticipants(r) {
  parts.value = { title: r.conference_title || r.title, list: [] }
  partsLoading.value = true
  try { const { data } = await client.get(`/conferences/${r.conference_id}/participants`); if (parts.value) parts.value.list = data.participants || [] }
  catch { if (parts.value) parts.value.list = [] } finally { partsLoading.value = false }
}
function closeParticipants() { parts.value = null }

function startEdit(r) { editing.value = r; editForm.value = { title: r.title || '', description: r.description || '' } }
function cancelEdit() { editing.value = null }
// закрытие попапа по клику на фон — только если нажатие И началось, и закончилось на фоне
let editBgDownFlag = false
function editBgDown(e) { editBgDownFlag = e.target === e.currentTarget }
function editBgClick(e) { if (editBgDownFlag && e.target === e.currentTarget) cancelEdit(); editBgDownFlag = false }
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
  <div class="w-full">
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

    <!-- таблица записей на всю ширину -->
    <div v-else class="card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full min-w-[860px] border-collapse text-left">
          <thead class="border-b border-parchment-200 bg-parchment-50 text-xs uppercase tracking-wide text-ink-700/55">
            <tr>
              <th class="px-5 py-3 font-semibold">Название</th>
              <th class="px-4 py-3 font-semibold">Дата и время</th>
              <th class="px-4 py-3 font-semibold">Длительность</th>
              <th class="px-4 py-3 font-semibold">Размер</th>
              <th class="px-4 py-3 font-semibold">Кто записал</th>
              <th class="px-5 py-3 text-right font-semibold">Действия</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="r in filtered" :key="r.id">
              <tr class="border-b border-parchment-100 align-top transition hover:bg-parchment-50/60">
                <td class="px-5 py-3.5">
                  <div class="font-medium text-ink-900">{{ r.title }}</div>
                  <div v-if="r.conference_title && r.conference_title !== r.title" class="text-xs text-ink-700/45">{{ r.conference_title }}</div>
                  <p v-if="r.description" class="mt-0.5 max-w-md truncate text-sm text-ink-700/70">{{ r.description }}</p>
                </td>
                <td class="whitespace-nowrap px-4 py-3.5 text-sm text-ink-700">{{ fmt(r.started_at) }}</td>
                <td class="whitespace-nowrap px-4 py-3.5 text-sm tabular-nums text-ink-700">{{ fmtDur(r.duration_ms) }}</td>
                <td class="whitespace-nowrap px-4 py-3.5 text-sm tabular-nums text-ink-700">{{ fmtSize(r.size_bytes) }}</td>
                <td class="px-4 py-3.5 text-sm text-ink-700">
                  <span v-if="r.recorded_by" class="inline-flex items-center gap-2">
                    <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-[11px] font-semibold text-white">{{ initials(r.recorded_by) }}</span>
                    <span class="whitespace-nowrap">{{ r.recorded_by }}</span>
                  </span>
                  <span v-else class="text-ink-700/40">—</span>
                </td>
                <td class="px-5 py-3.5">
                  <div class="flex items-center justify-end gap-1">
                    <button class="btn-ghost whitespace-nowrap text-sm" title="Кто был на созвоне" @click="openParticipants(r)"><AppIcon name="users" :size="16" /> Кто был</button>
                    <button class="btn-outline whitespace-nowrap text-sm" @click="playing = playing === r.id ? null : r.id"><AppIcon name="play" :size="14" /> {{ playing === r.id ? 'Скрыть' : 'Смотреть' }}</button>
                    <a :href="recUrl(r)" download class="btn-ghost p-2" title="Скачать"><AppIcon name="download" :size="18" /></a>
                    <button v-if="r.can_edit" class="btn-ghost p-2" title="Изменить название и описание" @click="startEdit(r)"><AppIcon name="edit" :size="18" /></button>
                    <button v-if="r.can_edit" class="rounded-lg p-2 text-ink-700/40 transition hover:bg-red-50 hover:text-red-600" title="Удалить запись" @click="remove(r)"><AppIcon name="trash" :size="18" /></button>
                  </div>
                </td>
              </tr>
              <!-- развёрнутый плеер -->
              <tr v-if="playing === r.id" class="border-b border-parchment-100 bg-ink-900/[0.03]">
                <td colspan="6" class="px-5 py-4">
                  <video :src="recUrl(r)" controls autoplay class="max-h-[70vh] w-full rounded-lg bg-ink-900"></video>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>

    <!-- попап редактирования названия/описания записи -->
    <div v-if="editing" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-4" @mousedown="editBgDown" @click="editBgClick">
      <div class="w-full max-w-lg overflow-hidden rounded-2xl bg-white shadow-xl">
        <header class="flex items-center justify-between border-b border-parchment-200 px-5 py-3.5">
          <h3 class="font-medium text-ink-900">Редактирование записи</h3>
          <button class="rounded-lg p-1.5 text-ink-700/60 transition hover:bg-parchment-100" title="Закрыть" @click="cancelEdit"><AppIcon name="close" :size="22" /></button>
        </header>
        <div class="space-y-4 p-5">
          <div>
            <label class="label">Название записи</label>
            <input v-model="editForm.title" class="input" :placeholder="editing.conference_title || 'Название'" @keydown.enter="saveEdit(editing)" />
          </div>
          <div>
            <label class="label">Описание</label>
            <textarea v-model="editForm.description" rows="4" class="input resize-y" placeholder="О чём эта запись (необязательно)"></textarea>
          </div>
        </div>
        <div class="flex justify-end gap-2 border-t border-parchment-200 px-5 py-3.5">
          <button class="btn-ghost" @click="cancelEdit">Отмена</button>
          <button class="btn-primary" :disabled="saving" @click="saveEdit(editing)">{{ saving ? '…' : 'Сохранить' }}</button>
        </div>
      </div>
    </div>

    <!-- попап «кто был на созвоне» -->
    <div v-if="parts" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-4" @click.self="closeParticipants">
      <div class="flex max-h-[80vh] w-full max-w-md flex-col overflow-hidden rounded-2xl bg-white shadow-xl">
        <header class="flex items-center justify-between gap-2 border-b border-parchment-200 px-5 py-3.5">
          <div class="min-w-0">
            <h3 class="font-medium text-ink-900">Кто был на созвоне</h3>
            <div v-if="parts.title" class="truncate text-xs text-ink-700/50">{{ parts.title }}</div>
          </div>
          <button class="rounded-lg p-1.5 text-ink-700/60 transition hover:bg-parchment-100" title="Закрыть" @click="closeParticipants"><AppIcon name="close" :size="22" /></button>
        </header>
        <div class="flex-1 overflow-y-auto p-2">
          <p v-if="partsLoading" class="p-6 text-center text-sm text-ink-700/50">Загрузка…</p>
          <p v-else-if="!parts.list.length" class="p-6 text-center text-sm text-ink-700/50">Нет данных об участниках</p>
          <div v-else>
            <div v-for="(p, i) in parts.list" :key="i" class="flex items-center gap-3 rounded-lg px-3 py-2 hover:bg-parchment-50">
              <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-sm font-semibold text-white" :class="p.is_guest ? 'bg-gradient-to-br from-sage-400 to-sage-600' : 'bg-gradient-to-br from-saffron-400 to-saffron-600'">{{ initials(p.name || 'Гость') }}</span>
              <span class="min-w-0 flex-1 truncate text-[15px] text-ink-900">{{ p.name || 'Без имени' }}</span>
              <span v-if="p.is_guest" class="badge shrink-0 bg-parchment-200 text-ink-700/70">гость</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
