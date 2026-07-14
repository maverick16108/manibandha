<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import AppDatePicker from '../components/AppDatePicker.vue'
import AppSelect from '../components/AppSelect.vue'
import { confirmDialog } from '../composables/confirm'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Конференция')
const auth = useAuthStore()
const router = useRouter()
const canHost = computed(() => auth.can('conference.host'))

const items = ref([])
const loading = ref(true)

const showForm = ref(false)
const form = ref({ title: '', description: '', mode: 'interactive' })
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
onMounted(() => { load(); poll = setInterval(() => load(true), 15000) })
onBeforeUnmount(() => clearInterval(poll))

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
  form.value = { title: '', description: '', mode: 'interactive' }
  schedDate.value = ''; schedHour.value = 19; schedMin.value = 0
}
// enter=true — начать сейчас и войти; enter=false — запланировать на дату
async function submit(enter) {
  if (!form.value.title.trim()) return
  saving.value = true
  try {
    const payload = { title: form.value.title.trim(), description: form.value.description.trim() || null, mode: form.value.mode }
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
  <div class="mx-auto max-w-4xl">
    <div class="mb-6 flex items-center justify-between gap-3">
      <p class="text-ink-700/60">Онлайн-встречи и трансляции гуру с учениками</p>
      <button v-if="canHost" class="btn-primary shrink-0" @click="showForm = !showForm"><AppIcon name="video" :size="16" /> Создать</button>
    </div>

    <div v-if="showForm" class="card mb-6 space-y-3 p-5">
      <input v-model="form.title" class="input" placeholder="Название конференции" />
      <textarea v-model="form.description" rows="2" class="input resize-y" placeholder="Описание (необязательно)"></textarea>
      <div class="flex flex-wrap items-center gap-4">
        <label class="flex items-center gap-2 text-sm"><input type="radio" value="interactive" v-model="form.mode" /> Встреча (все с камерой)</label>
        <label class="flex items-center gap-2 text-sm"><input type="radio" value="broadcast" v-model="form.mode" /> Трансляция (вещает ведущий)</label>
      </div>
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
        <button class="btn-primary" :disabled="saving || !form.title.trim()" @click="submit(true)">
          <AppIcon name="video" :size="16" /> {{ saving ? '…' : 'Начать сейчас' }}
        </button>
        <button v-if="schedDate" class="btn-outline" :disabled="saving || !form.title.trim()" @click="submit(false)">Запланировать</button>
        <button class="btn-ghost" @click="showForm = false">Отмена</button>
      </div>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 3" :key="i" class="card space-y-2 p-4"><AppSkeleton w="w-56" /><AppSkeleton w="w-32" h="h-3" /></div>
    </div>

    <template v-else>
      <!-- идёт сейчас -->
      <div v-if="live.length" class="mb-6 space-y-3">
        <div class="text-sm font-semibold uppercase tracking-wide text-red-600">Сейчас в эфире</div>
        <div v-for="c in live" :key="c.id" class="card border-red-400/40 bg-red-500/5 p-4">
          <div class="flex items-center justify-between gap-3">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="inline-flex h-2.5 w-2.5 animate-pulse rounded-full bg-red-500"></span>
                <h3 class="truncate font-display text-lg font-semibold text-ink-900">{{ c.title }}</h3>
                <span class="badge bg-parchment-200 text-ink-700">{{ modeLabel(c.mode) }}</span>
              </div>
              <div class="mt-0.5 text-sm text-ink-700/60">Ведущий: {{ c.host_name || '—' }} · с {{ fmt(c.started_at) }}</div>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <button class="btn-primary" @click="enter(c)">Войти</button>
              <button v-if="c.can_host" class="btn-ghost" @click="endConf(c)">Завершить</button>
            </div>
          </div>
        </div>
      </div>

      <!-- запланированные -->
      <div v-if="scheduled.length" class="mb-6 space-y-3">
        <div class="text-sm font-semibold uppercase tracking-wide text-ink-700/50">Запланированные</div>
        <div v-for="c in scheduled" :key="c.id" class="card p-4">
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
              <button class="btn-outline" @click="enter(c)">{{ c.can_host ? 'Начать' : 'Войти' }}</button>
              <button v-if="c.can_host" class="text-ink-700/40 hover:text-red-600" @click="remove(c)"><AppIcon name="trash" :size="16" /></button>
            </div>
          </div>
        </div>
      </div>

      <!-- прошедшие -->
      <div v-if="ended.length" class="space-y-2">
        <div class="text-sm font-semibold uppercase tracking-wide text-ink-700/40">Прошедшие</div>
        <div v-for="c in ended" :key="c.id" class="flex items-center justify-between gap-3 rounded-lg border border-parchment-200 px-4 py-2.5">
          <div class="min-w-0">
            <span class="truncate font-medium text-ink-700">{{ c.title }}</span>
            <span class="ml-2 text-xs text-ink-700/50">{{ fmt(c.started_at || c.created_at) }}</span>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <button class="btn-ghost text-sm" @click="enter(c)">Подключиться</button>
            <button v-if="c.can_host" class="rounded-lg p-1.5 text-ink-700/40 transition hover:bg-red-50 hover:text-red-600" title="Удалить" @click="remove(c)"><AppIcon name="trash" :size="20" /></button>
          </div>
        </div>
      </div>

      <div v-if="!live.length && !scheduled.length && !ended.length" class="card p-10 text-center text-ink-700/50">
        Конференций пока нет.<span v-if="canHost"> Создайте первую.</span>
      </div>
    </template>
  </div>
</template>
