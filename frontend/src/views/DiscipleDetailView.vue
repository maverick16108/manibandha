<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import client from '../api/client'
import { invalidatePrefix } from '../composables/apiCache'
import { useAuthStore } from '../stores/auth'
import { confirmDialog } from '../composables/confirm'
import AppSelect from '../components/AppSelect.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import AppIcon from '../components/AppIcon.vue'
import { STATUS_LABELS, STATUS_ORDER, STATUS_BADGE, MARITAL_LABELS, GENDER_LABELS, formatDate, formatPhone, phoneList, thumbUrl, imgFull } from '../lib/format'
import { usePageTitle } from '../composables/pageTitle'
import { openLightbox } from '../composables/lightbox'

const statusOptions = STATUS_ORDER.map((s) => ({ value: s, label: STATUS_LABELS[s] }))

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const id = computed(() => route.params.id)

const d = ref(null)
const stats = ref(null)
const loading = ref(true)
const newItem = reactive({ title: '', target: 'harinama' })

// имя ученика; если не заполнено (напр. только что зарегистрировался) — показываем телефон
const nameIsPhone = computed(() => !!d.value && !d.value.spiritual_name && !d.value.material_name)
const displayName = computed(() => {
  const v = d.value
  if (!v) return ''
  return v.spiritual_name || v.material_name || (v.phone ? formatPhone(v.phone) : '—')
})
const isSelf = computed(() => String(auth.user?.disciple_id || '') === String(d.value?.id || ''))

usePageTitle(() => displayName.value)

async function load() {
  const { data } = await client.get(`/disciples/${id.value}`)
  d.value = data
  try {
    const s = await client.get('/threads/stats', { params: { disciple_id: id.value } })
    stats.value = s.data
  } catch { stats.value = null }
}

async function addItem() {
  if (!newItem.title.trim()) return
  await client.post(`/disciples/${id.value}/checklist`, { title: newItem.title, target: newItem.target })
  newItem.title = ''
  await load()
}

async function toggleItem(item) {
  await client.patch(`/disciples/${id.value}/checklist/${item.id}`, { is_done: !item.is_done })
  await load()
}

async function removeItem(item) {
  await client.delete(`/disciples/${id.value}/checklist/${item.id}`)
  await load()
}

async function remove() {
  if (!(await confirmDialog({ message: 'Удалить ученика безвозвратно?' }))) return
  await client.delete(`/disciples/${id.value}`)
  invalidatePrefix('/reports'); invalidatePrefix('/disciples') // обновить дашборд и список
  router.push({ name: 'disciples' })
}

// ── заметки куратора (дата + текст) ──
const notes = ref([])
const newNote = ref('')
const savingNote = ref(false)
const canNote = computed(() => auth.can('disciples.note'))
async function loadNotes() {
  if (!canNote.value) return
  try { const { data } = await client.get(`/disciples/${id.value}/notes`); notes.value = data } catch { notes.value = [] }
}
async function addNote() {
  const t = newNote.value.trim()
  if (!t) return
  savingNote.value = true
  try {
    const { data } = await client.post(`/disciples/${id.value}/notes`, { text: t })
    notes.value.unshift(data); newNote.value = ''
  } finally { savingNote.value = false }
}
async function deleteNote(n) {
  if (!(await confirmDialog({ message: 'Удалить заметку?', confirmText: 'Удалить', danger: true }))) return
  await client.delete(`/disciples/${id.value}/notes/${n.id}`)
  notes.value = notes.value.filter((x) => x.id !== n.id)
}

// ── файлы анкеты ──
const files = ref([])
const fileInput = ref(null)
const uploadingFile = ref(false)
const fileDragOver = ref(false)
const canEdit = computed(() => auth.can('disciples.edit'))
async function loadFiles() {
  try { const { data } = await client.get(`/disciples/${id.value}/files`); files.value = data } catch { files.value = [] }
}
async function uploadDiscipleFiles(list) {
  const arr = Array.from(list || [])
  if (!arr.length) return
  uploadingFile.value = true
  try {
    for (const f of arr) {
      const fd = new FormData(); fd.append('file', f)
      const { data } = await client.post(`/disciples/${id.value}/files`, fd, { headers: { 'Content-Type': 'multipart/form-data' } })
      files.value.unshift(data)
    }
  } catch (err) { alert(err.response?.data?.detail || 'Не удалось загрузить файл') }
  finally { uploadingFile.value = false; if (fileInput.value) fileInput.value.value = '' }
}
function onFilePick(e) { uploadDiscipleFiles(e.target.files) }
function onFileDrop(e) { fileDragOver.value = false; if (canEdit.value) uploadDiscipleFiles(e.dataTransfer?.files) }
async function deleteFile(f) {
  if (!(await confirmDialog({ message: `Удалить файл «${f.name}»?`, confirmText: 'Удалить', danger: true }))) return
  await client.delete(`/disciples/${id.value}/files/${f.id}`)
  files.value = files.value.filter((x) => x.id !== f.id)
}
function fmtSize(b) {
  if (!b) return ''
  if (b < 1024) return `${b} Б`
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(0)} КБ`
  return `${(b / 1024 / 1024).toFixed(1)} МБ`
}

onMounted(async () => {
  try { await load() } finally { loading.value = false }
  loadNotes(); loadFiles()
})
</script>

<template>
  <div v-if="loading" class="mx-auto max-w-6xl space-y-6">
    <div class="card flex gap-6 p-6">
      <AppSkeleton w="w-28" h="h-28" rounded="rounded-xl" />
      <div class="flex-1 space-y-3"><AppSkeleton w="w-56" h="h-8" /><AppSkeleton w="w-32" /><AppSkeleton w="w-24" h="h-5" rounded="rounded-full" /></div>
    </div>
    <div class="grid gap-6 lg:grid-cols-2">
      <div class="card space-y-3 p-6"><AppSkeleton w="w-32" h="h-5" /><AppSkeleton v-for="j in 6" :key="j" /></div>
      <div class="card space-y-3 p-6"><AppSkeleton w="w-32" h="h-5" /><AppSkeleton v-for="j in 6" :key="j" /></div>
    </div>
  </div>
  <div v-else-if="d" class="mx-auto max-w-6xl">

    <!-- Ожидание апрува (для самого кандидата) -->
    <div v-if="isSelf && d.is_approved === false" class="card mb-6 border-saffron-400/50 bg-saffron-500/5 p-5">
      <div class="flex items-start gap-3">
        <AppIcon name="chat" :size="22" class="mt-0.5 shrink-0 text-saffron-600" />
        <div>
          <div class="font-medium text-ink-900">Заявка на рассмотрении</div>
          <p class="mt-1 text-sm text-ink-700/80">
            Ожидайте — с Вами свяжется куратор для общения и завершения регистрации.
            Пока можно заполнить анкету и написать в чат.
          </p>
        </div>
      </div>
    </div>

    <!-- Header -->
    <div class="card mb-6 p-6">
      <div class="flex flex-wrap items-start gap-6">
        <img v-if="d.photo_url" :src="thumbUrl(d.photo_url)" @error="imgFull($event, d.photo_url)" class="photo-bw h-28 w-28 cursor-zoom-in rounded-xl object-cover" @click="openLightbox(d.photo_url)" />
        <div v-else class="flex h-28 w-28 items-center justify-center rounded-xl bg-parchment-200 text-4xl text-ink-700">
          {{ (d.spiritual_name || d.material_name || '?')[0] }}
        </div>
        <div class="flex-1">
          <h1 class="font-semibold text-ink-900" :class="nameIsPhone ? 'text-2xl tabular-nums' : 'font-display text-3xl'">{{ displayName }}</h1>
          <p v-if="d.spiritual_name" class="text-ink-700/70">{{ d.material_name }}</p>
          <div class="mt-3 flex flex-wrap items-center gap-2">
            <span class="badge" :class="STATUS_BADGE[d.initiation_status]">{{ STATUS_LABELS[d.initiation_status] }}</span>
            <span v-if="d.ready_for_pranama" class="badge bg-orange-100 text-orange-800">Готов к пранаме</span>
            <span v-if="d.ready_for_initiation" class="badge bg-saffron-500/15 text-saffron-700">Готов к инициации</span>
          </div>
          <div v-if="stats" class="mt-3 flex flex-wrap gap-4 text-sm text-ink-700/70">
            <span>Отчётов: <b class="text-ink-900">{{ stats.reports }}</b></span>
            <span>Вопросов: <b class="text-ink-900">{{ stats.questions }}</b></span>
            <span>Сообщений от ученика: <b class="text-ink-900">{{ stats.messages }}</b></span>
          </div>
        </div>
        <div v-if="auth.canEdit || isSelf" class="flex gap-2">
          <RouterLink :to="{ name: 'disciple-edit', params: { id: d.id } }" class="btn-outline">Редактировать</RouterLink>
          <button v-if="auth.isStaff" class="btn border border-red-200 text-red-600 hover:bg-red-50" @click="remove">Удалить</button>
        </div>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-2">
      <!-- Info -->
      <div class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Данные</h3>
        <dl class="space-y-2 text-sm">
          <div class="flex justify-between gap-4"><dt class="text-ink-700/60">Телефон</dt>
            <dd class="text-right">
              <template v-if="phoneList(d.phone).length">
                <a v-for="(p, i) in phoneList(d.phone)" :key="i" :href="`tel:${p.tel}`"
                   class="block text-saffron-700 hover:underline">{{ p.display }}</a>
              </template>
              <span v-else class="text-ink-800">—</span>
            </dd>
          </div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Email</dt><dd class="text-ink-800">{{ d.email || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Мессенджер</dt><dd class="text-ink-800">{{ d.messenger || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Страна / город</dt><dd class="text-ink-800">{{ d.country || '—' }}<span v-if="d.city">, {{ d.city }}</span></dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Область</dt><dd class="text-ink-800">{{ d.region || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Пол</dt><dd class="text-ink-800">{{ GENDER_LABELS[d.gender] || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Семейное положение</dt><dd class="text-ink-800">{{ MARITAL_LABELS[d.marital_status] || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Дата рождения</dt><dd class="text-ink-800">{{ formatDate(d.date_of_birth) }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Куратор</dt><dd class="text-ink-800">{{ d.mentor?.name || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Наставник</dt><dd class="text-ink-800">{{ d.mentor_name || '—' }}</dd></div>
        </dl>
      </div>

      <!-- Initiation + service -->
      <div class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Инициация и служение</h3>
        <dl class="space-y-2 text-sm">
          <div class="flex justify-between"><dt class="text-ink-700/60">Пранама-мантра</dt><dd class="text-ink-800">{{ formatDate(d.pranama_date) }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Харинама</dt><dd class="text-ink-800">{{ formatDate(d.harinama_date) }}<span v-if="d.harinama_name"> · {{ d.harinama_name }}</span></dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Вторая инициация</dt><dd class="text-ink-800">{{ formatDate(d.brahman_date) }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Кто рекомендовал</dt><dd class="text-ink-800">{{ d.recommended_by || '—' }}</dd></div>
          <div class="flex justify-between"><dt class="text-ink-700/60">Дата заявки</dt><dd class="text-ink-800">{{ formatDate(d.application_date) }}</dd></div>
        </dl>
        <div v-if="d.seva" class="mt-4"><div class="label">Служение</div><p class="text-sm text-ink-700">{{ d.seva }}</p></div>
        <div v-if="d.current_activity" class="mt-3"><div class="label">Деятельность</div><p class="text-sm text-ink-700">{{ d.current_activity }}</p></div>
        <div v-if="d.notes" class="mt-3"><div class="label">Примечания</div><p class="text-sm text-ink-700">{{ d.notes }}</p></div>
      </div>
    </div>

    <!-- Заметки куратора -->
    <div v-if="canNote" class="card mt-6 p-6">
      <h3 class="mb-4 font-display text-xl text-ink-900">Заметки</h3>
      <div class="mb-4 flex flex-col gap-2 sm:flex-row">
        <textarea v-model="newNote" rows="2" class="input flex-1 resize-y" placeholder="Новая заметка об ученике…"></textarea>
        <button class="btn-primary shrink-0 self-start" :disabled="savingNote || !newNote.trim()" @click="addNote">{{ savingNote ? '…' : 'Добавить' }}</button>
      </div>
      <div v-if="!notes.length" class="text-sm text-ink-700/50">Заметок пока нет</div>
      <ul v-else class="space-y-3">
        <li v-for="n in notes" :key="n.id" class="rounded-lg border border-parchment-200 bg-parchment-50 p-3">
          <div class="mb-1 flex items-center justify-between gap-2 text-xs text-ink-700/50">
            <span>{{ n.author_name || 'Аноним' }} · {{ formatDate(n.created_at) }}</span>
            <button class="text-red-500/70 hover:text-red-600" @click="deleteNote(n)">Удалить</button>
          </div>
          <p class="whitespace-pre-wrap text-sm text-ink-800">{{ n.text }}</p>
        </li>
      </ul>
    </div>

    <!-- Файлы анкеты -->
    <div class="card mt-6 p-6 transition" :class="fileDragOver && 'ring-2 ring-saffron-400 ring-offset-2'"
         @dragover.prevent="canEdit && (fileDragOver = true)" @dragleave="fileDragOver = false" @drop.prevent="onFileDrop">
      <div class="mb-4 flex items-center justify-between gap-3">
        <h3 class="font-display text-xl text-ink-900">Файлы</h3>
        <button v-if="canEdit" class="btn-outline shrink-0" :disabled="uploadingFile" @click="fileInput.click()">
          <AppIcon name="reports" :size="16" /> {{ uploadingFile ? 'Загрузка…' : 'Добавить файл' }}
        </button>
        <input ref="fileInput" type="file" multiple class="hidden" @change="onFilePick" />
      </div>
      <div v-if="canEdit && !files.length" class="rounded-lg border-2 border-dashed py-6 text-center text-sm transition"
           :class="fileDragOver ? 'border-saffron-500 bg-saffron-500/10 text-saffron-700' : 'border-parchment-300 text-ink-700/50'">
        {{ fileDragOver ? 'Отпустите файлы' : 'Перетащите файлы сюда или нажмите «Добавить файл»' }}
      </div>
      <div v-else-if="!files.length" class="text-sm text-ink-700/50">Файлов пока нет</div>
      <ul v-else class="divide-y divide-parchment-100">
        <li v-for="f in files" :key="f.id" class="flex items-center gap-3 py-2.5">
          <AppIcon name="reports" :size="18" class="shrink-0 text-saffron-600" />
          <a :href="f.url" target="_blank" rel="noopener" :download="f.name" class="min-w-0 flex-1">
            <span class="block truncate font-medium text-ink-800 hover:text-saffron-700 hover:underline">{{ f.name }}</span>
            <span class="text-xs text-ink-700/50">{{ fmtSize(f.size) }}<span v-if="f.uploaded_by_name"> · {{ f.uploaded_by_name }}</span> · {{ formatDate(f.created_at) }}</span>
          </a>
          <button v-if="canEdit" class="shrink-0 text-ink-700/40 hover:text-red-600" title="Удалить" @click="deleteFile(f)"><AppIcon name="trash" :size="16" /></button>
        </li>
      </ul>
    </div>

  </div>
</template>
