<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import AppDatePicker from '../components/AppDatePicker.vue'
import AppSelect from '../components/AppSelect.vue'
import { confirmDialog } from '../composables/confirm'
import { showToast } from '../composables/toast'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Конференция')
const auth = useAuthStore()
const router = useRouter()
const canHost = computed(() => auth.can('conference.host'))

const items = ref([])
const loading = ref(true)

const showForm = ref(false)
const editingId = ref(null) // id редактируемой конференции (иначе создаём новую)
const form = ref({ title: '', description: '', mode: 'interactive', mic_allowed: true, cam_allowed: true, screen_allowed: true, guests_allowed: false, auto_record: false, host_id: null })
const schedDate = ref('')   // YYYY-MM-DD
const schedHour = ref(19)
const schedMin = ref(0)
const hourOptions = Array.from({ length: 24 }, (_, i) => ({ value: i, label: String(i).padStart(2, '0') }))
const minOptions = [0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55].map((m) => ({ value: m, label: String(m).padStart(2, '0') }))
const saving = ref(false)

async function load(silent = false) {
  if (!silent) loading.value = true
  try { const { data } = await client.get('/conferences'); items.value = data } finally { loading.value = false }
}
let poll = null
const nowTs = ref(Date.now())
let tick = null
const recEnabled = ref(false)
const moderators = ref([])
const moderatorOptions = computed(() => moderators.value.map((m) => ({ value: m.id, label: m.name })))
onMounted(async () => {
  load(); poll = setInterval(() => load(true), 4000); tick = setInterval(() => { nowTs.value = Date.now() }, 1000)
  try { const { data } = await client.get('/settings'); recEnabled.value = !!data.recording_enabled } catch { /* ignore */ }
  try { const { data } = await client.get('/conferences/moderators'); moderators.value = data.moderators || [] } catch { /* ignore */ }
})
onBeforeUnmount(() => { clearInterval(poll); clearInterval(tick) })

function elapsed(iso) {
  if (!iso) return ''
  let s = Math.max(0, Math.floor((nowTs.value - new Date(iso).getTime()) / 1000))
  const h = Math.floor(s / 3600); s -= h * 3600
  const m = Math.floor(s / 60); s -= m * 60
  const pad = (n) => String(n).padStart(2, '0')
  return h ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`
}
function pluralParts(n) {
  const a = Math.abs(n) % 100, b = a % 10
  if (a > 10 && a < 20) return 'участников'
  if (b > 1 && b < 5) return 'участника'
  if (b === 1) return 'участник'
  return 'участников'
}
function pInitials(name) { return (name || '?').trim()[0]?.toUpperCase() || '?' }

const live = computed(() => items.value.filter((c) => c.status === 'live'))
const scheduled = computed(() => items.value.filter((c) => c.status === 'scheduled'))
const ended = computed(() => items.value.filter((c) => c.status === 'ended'))

function fmt(iso) {
  if (!iso) return ''
  return new Date(iso).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
}
function modeLabel(m) { return m === 'broadcast' ? 'Трансляция' : 'Встреча' }

function resetForm() {
  showForm.value = false
  editingId.value = null
  form.value = { title: '', description: '', mode: 'interactive', mic_allowed: true, cam_allowed: true, screen_allowed: true, guests_allowed: false, auto_record: false, host_id: auth.user?.id ?? null }
  schedDate.value = ''; schedHour.value = 19; schedMin.value = 0
}
function startEdit(c) {
  editingId.value = c.id
  showForm.value = true
  form.value = {
    title: c.title || '', description: c.description || '', mode: c.mode || 'interactive',
    mic_allowed: c.mic_allowed !== false, cam_allowed: c.cam_allowed !== false,
    screen_allowed: c.screen_allowed !== false, guests_allowed: !!c.guests_allowed, auto_record: !!c.auto_record, host_id: c.host_id ?? null,
  }
  if (c.scheduled_at) {
    const d = new Date(c.scheduled_at)
    schedDate.value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    schedHour.value = d.getHours(); schedMin.value = d.getMinutes() - (d.getMinutes() % 5)
  } else { schedDate.value = '' }
  nextTick(() => document.querySelector('.conf-form')?.scrollIntoView({ behavior: 'smooth', block: 'center' }))
}
async function saveEdit() {
  if (!form.value.title.trim()) return
  saving.value = true
  try {
    const payload = {
      title: form.value.title.trim(), description: form.value.description.trim() || null, mode: form.value.mode,
      mic_allowed: form.value.mic_allowed, cam_allowed: form.value.cam_allowed,
      screen_allowed: form.value.screen_allowed, guests_allowed: form.value.guests_allowed, auto_record: form.value.auto_record, host_id: form.value.host_id || null,
    }
    if (schedDate.value) {
      const hh = String(schedHour.value).padStart(2, '0'); const mm = String(schedMin.value).padStart(2, '0')
      payload.scheduled_at = new Date(`${schedDate.value}T${hh}:${mm}:00`).toISOString()
    }
    await client.patch(`/conferences/${editingId.value}`, payload)
    resetForm()
    await load(true)
  } finally { saving.value = false }
}
function copyLink(c) {
  if (!c.code) return
  const url = `${location.origin}/c/${c.code}`
  navigator.clipboard?.writeText(url).then(() => showToast('Ссылка на конференцию скопирована')).catch(() => prompt('Ссылка на конференцию:', url))
}
// enter=true — начать сейчас и войти; enter=false — запланировать на дату
async function submit(enter) {
  if (!form.value.title.trim()) return
  saving.value = true
  try {
    const payload = { title: form.value.title.trim(), description: form.value.description.trim() || null, mode: form.value.mode, mic_allowed: form.value.mic_allowed, cam_allowed: form.value.cam_allowed, screen_allowed: form.value.screen_allowed, guests_allowed: form.value.guests_allowed, auto_record: form.value.auto_record, host_id: form.value.host_id || null }
    if (!enter && schedDate.value) {
      const hh = String(schedHour.value).padStart(2, '0')
      const mm = String(schedMin.value).padStart(2, '0')
      payload.scheduled_at = new Date(`${schedDate.value}T${hh}:${mm}:00`).toISOString()
    }
    const { data } = await client.post('/conferences', payload)
    resetForm()
    await load(true)
    if (enter) router.push({ name: 'conference-room', params: { id: data.id } })
  } finally { saving.value = false }
}
function enter(c) { router.push({ name: 'conference-room', params: { id: c.id } }) }
async function endConf(c) {
  if (!(await confirmDialog({ message: `Завершить «${c.title}»?`, confirmText: 'Завершить', danger: true }))) return
  await client.patch(`/conferences/${c.id}`, { status: 'ended' }); await load(true)
}
async function remove(c) {
  if (!(await confirmDialog({ message: `Удалить «${c.title}»?`, confirmText: 'Удалить', danger: true }))) return
  await client.delete(`/conferences/${c.id}`); await load(true)
}
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <div class="mb-6 flex items-center justify-between gap-3">
      <p class="text-ink-700/60">Онлайн-встречи и трансляции гуру с учениками</p>
      <div class="flex shrink-0 items-center gap-2">
        <button v-if="recEnabled" class="btn-outline" title="Архив записей" @click="router.push({ name: 'conference-recordings' })"><AppIcon name="play" :size="15" /> Записи</button>
        <button v-if="canHost" class="btn-primary" @click="editingId ? resetForm() : (showForm = !showForm)"><AppIcon name="video" :size="16" /> Создать</button>
      </div>
    </div>

    <div v-if="showForm" class="conf-form card mb-6 space-y-3 p-5">
      <div v-if="editingId" class="text-sm font-semibold text-saffron-700">Изменение конференции</div>
      <input v-model="form.title" class="input" placeholder="Название конференции" />
      <textarea v-model="form.description" rows="2" class="input resize-y" placeholder="Описание (необязательно)"></textarea>
      <div v-if="moderatorOptions.length > 1">
        <label class="label">Модератор конференции</label>
        <div class="sm:w-72"><AppSelect v-model="form.host_id" :options="moderatorOptions" /></div>
      </div>
      <div class="flex flex-wrap items-center gap-4">
        <label class="flex items-center gap-2 text-sm"><input type="radio" value="interactive" v-model="form.mode" /> Встреча (все с камерой)</label>
        <label class="flex items-center gap-2 text-sm"><input type="radio" value="broadcast" v-model="form.mode" /> Трансляция (вещает ведущий)</label>
      </div>
      <div v-if="form.mode === 'interactive'" class="flex flex-wrap items-center gap-4 rounded-lg bg-parchment-100 px-3 py-2">
        <span class="text-sm text-ink-700/60">Участникам по умолчанию:</span>
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="form.mic_allowed" /> микрофон</label>
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="form.cam_allowed" /> камера</label>
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="form.screen_allowed" /> показ экрана</label>
      </div>
      <label class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="form.guests_allowed" /> Разрешить вход гостям по ссылке (без авторизации)</label>
      <label v-if="recEnabled" class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="form.auto_record" /> Записывать встречу с самого начала</label>
      <div>
        <label class="label">Запланировать (необязательно)</label>
        <div class="flex flex-wrap items-center gap-2">
          <div class="w-44"><AppDatePicker v-model="schedDate" /></div>
          <template v-if="schedDate">
            <span class="text-ink-700/50">в</span>
            <div class="w-20"><AppSelect v-model="schedHour" :options="hourOptions" /></div>
            <span class="text-ink-700/60">:</span>
            <div class="w-20"><AppSelect v-model="schedMin" :options="minOptions" /></div>
          </template>
        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <template v-if="editingId">
          <button class="btn-primary" :disabled="saving || !form.title.trim()" @click="saveEdit">{{ saving ? '…' : 'Сохранить' }}</button>
        </template>
        <template v-else>
          <button class="btn-primary" :disabled="saving || !form.title.trim()" @click="submit(true)">
            <AppIcon name="video" :size="16" /> {{ saving ? '…' : 'Начать сейчас' }}
          </button>
          <button v-if="schedDate" class="btn-outline" :disabled="saving || !form.title.trim()" @click="submit(false)">Запланировать</button>
        </template>
        <button class="btn-ghost" @click="resetForm">Отмена</button>
      </div>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 3" :key="i" class="card space-y-2 p-4"><AppSkeleton w="w-56" /><AppSkeleton w="w-32" h="h-3" /></div>
    </div>

    <template v-else>
      <!-- идёт сейчас -->
      <div v-if="live.length" class="mb-6 space-y-3">
        <div class="text-sm font-semibold uppercase tracking-wide text-red-600">Сейчас в эфире</div>
        <div v-for="c in live" :key="c.id" class="card cursor-pointer border-red-400/40 bg-red-500/5 p-4 transition hover:border-red-400/70 hover:bg-red-500/10" @click="enter(c)">
          <div class="flex items-center justify-between gap-3">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="inline-flex h-2.5 w-2.5 animate-pulse rounded-full bg-red-500"></span>
                <h3 class="truncate font-display text-lg font-semibold text-ink-900">{{ c.title }}</h3>
                <span class="badge bg-parchment-200 text-ink-700">{{ modeLabel(c.mode) }}</span>
              </div>
              <div class="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-ink-700/60">
                <span>Ведущий: {{ c.host_name || '—' }}</span>
                <span v-if="c.started_at" class="inline-flex items-center gap-1 rounded-md bg-red-500/10 px-1.5 py-0.5 font-medium tabular-nums text-red-600">
                  <AppIcon name="clock" :size="13" /> {{ elapsed(c.started_at) }}
                </span>
              </div>
              <div v-if="c.participant_count" class="mt-2 flex items-center gap-2">
                <div class="flex -space-x-2">
                  <template v-for="(p, pi) in c.participants" :key="pi">
                    <img v-if="p.avatar_url" :src="p.avatar_url" :title="p.name" class="h-7 w-7 rounded-full border-2 border-parchment-50 object-cover" />
                    <span v-else :title="p.name" class="flex h-7 w-7 items-center justify-center rounded-full border-2 border-parchment-50 bg-gradient-to-br from-saffron-400 to-saffron-600 text-xs font-semibold text-white">{{ pInitials(p.name) }}</span>
                  </template>
                  <span v-if="c.participant_count > c.participants.length" class="flex h-7 min-w-[1.75rem] items-center justify-center rounded-full border-2 border-parchment-50 bg-parchment-200 px-1 text-xs font-semibold text-ink-700">+{{ c.participant_count - c.participants.length }}</span>
                </div>
                <span class="text-sm text-ink-700/60">{{ c.participant_count }} {{ pluralParts(c.participant_count) }}</span>
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <button v-if="c.can_host" class="btn-ghost p-2" title="Настройки конференции" @click.stop="startEdit(c)"><AppIcon name="settings" :size="17" /></button>
              <button class="btn-ghost text-sm" title="Скопировать ссылку на конференцию" @click.stop="copyLink(c)"><AppIcon name="link" :size="15" /> Ссылка</button>
              <button class="btn-primary" @click.stop="enter(c)">Войти</button>
              <button v-if="c.can_host" class="btn-ghost" @click.stop="endConf(c)">Завершить</button>
            </div>
          </div>
        </div>
      </div>

      <!-- запланированные -->
      <div v-if="scheduled.length" class="mb-6 space-y-3">
        <div class="text-sm font-semibold uppercase tracking-wide text-ink-700/50">Запланированные</div>
        <div v-for="c in scheduled" :key="c.id" class="card cursor-pointer p-4 transition hover:border-saffron-400/50 hover:bg-parchment-100" @click="enter(c)">
          <div class="flex items-center justify-between gap-3">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <h3 class="truncate font-display text-lg font-semibold text-ink-900">{{ c.title }}</h3>
                <span class="badge bg-parchment-200 text-ink-700">{{ modeLabel(c.mode) }}</span>
              </div>
              <div class="mt-0.5 text-sm text-ink-700/60">
                Ведущий: {{ c.host_name || '—' }}<span v-if="c.scheduled_at"> · {{ fmt(c.scheduled_at) }}</span>
              </div>
              <p v-if="c.description" class="mt-1 text-sm text-ink-700/70">{{ c.description }}</p>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <button v-if="c.can_host" class="btn-ghost p-2" title="Настройки конференции" @click.stop="startEdit(c)"><AppIcon name="settings" :size="17" /></button>
              <button class="btn-ghost text-sm" title="Скопировать ссылку на конференцию" @click.stop="copyLink(c)"><AppIcon name="link" :size="15" /> Ссылка</button>
              <button class="btn-outline" @click.stop="enter(c)">{{ c.can_host ? 'Начать' : 'Войти' }}</button>
              <button v-if="c.can_host" class="text-ink-700/40 hover:text-red-600" @click.stop="remove(c)"><AppIcon name="trash" :size="16" /></button>
            </div>
          </div>
        </div>
      </div>

      <!-- прошедшие -->
      <div v-if="ended.length" class="space-y-2">
        <div class="flex items-center gap-2 text-sm font-semibold uppercase tracking-wide text-ink-700/70">
          <span>Прошедшие</span>
          <span class="h-px flex-1 bg-parchment-300"></span>
        </div>
        <div v-for="c in ended" :key="c.id" class="flex cursor-pointer items-center justify-between gap-3 rounded-lg border border-parchment-200 px-4 py-2.5 transition hover:border-parchment-300 hover:bg-parchment-100" @click="enter(c)">
          <div class="min-w-0">
            <span class="truncate font-medium text-ink-700">{{ c.title }}</span>
            <span class="ml-2 text-xs text-ink-700/50">{{ fmt(c.started_at || c.created_at) }}</span>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <button class="btn-ghost text-sm" @click.stop="enter(c)">Подключиться</button>
            <button v-if="c.can_host" class="rounded-lg p-1.5 text-ink-700/40 transition hover:bg-red-50 hover:text-red-600" title="Удалить" @click.stop="remove(c)"><AppIcon name="trash" :size="20" /></button>
          </div>
        </div>
      </div>

      <div v-if="!live.length && !scheduled.length && !ended.length" class="card p-10 text-center text-ink-700/50">
        Конференций пока нет.<span v-if="canHost"> Создайте первую.</span>
      </div>
    </template>
  </div>
</template>
