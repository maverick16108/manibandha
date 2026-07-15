<script setup>
import { ref, onMounted } from 'vue'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { backTarget } from '../composables/backTarget'
import { confirmDialog } from '../composables/confirm'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Записи конференций')
const auth = useAuthStore()

const loading = ref(true)
const recordings = ref([])
const playing = ref(null)
const editing = ref(null)
const editForm = ref({ title: '', description: '' })
const saving = ref(false)

async function load() {
  loading.value = true
  try { const { data } = await client.get('/conferences/recordings'); recordings.value = data.recordings || [] } finally { loading.value = false }
}
onMounted(() => { backTarget.value = { name: 'conference' }; load() })

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
    <p class="mb-6 text-ink-700/60">Записи прошедших конференций — можно смотреть, скачивать и подписывать</p>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 3" :key="i" class="card space-y-2 p-4"><AppSkeleton w="w-56" /><AppSkeleton w="w-40" h="h-3" /></div>
    </div>

    <div v-else-if="!recordings.length" class="card p-10 text-center text-ink-700/50">Записей пока нет</div>

    <div v-else class="space-y-3">
      <div v-for="r in recordings" :key="r.id" class="card p-4 sm:p-5">
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
