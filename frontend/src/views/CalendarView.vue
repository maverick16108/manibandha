<script setup>
import { ref, computed, onMounted, onActivated, onBeforeUnmount, watch, nextTick } from 'vue'
defineOptions({ name: 'CalendarView' })
import { RouterLink } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import { confirmDialog } from '../composables/confirm'
import AppSkeleton from '../components/AppSkeleton.vue'
import AppIcon from '../components/AppIcon.vue'
import AppDatePicker from '../components/AppDatePicker.vue'
import EventsMap from '../components/EventsMap.vue'
import EventsFastScroll from '../components/EventsFastScroll.vue'
import { renderMarkdown } from '../lib/markdown'
import { extractImageUrls, preloadImages } from '../lib/preload'
import { formatDate } from '../lib/format'
import { usePageTitle } from '../composables/pageTitle'
import { onEscape } from '../composables/useEscape'

usePageTitle('События')
onEscape(() => { if (selected.value) selected.value = null })

const auth = useAuthStore()
const events = ref([])
const loading = ref(true)
const mode = ref('list') // 'list' | 'calendar' | 'map'
const selected = ref(null)

const now = new Date()
const cursor = ref({ y: now.getFullYear(), m: now.getMonth() + 1 })

// период карты — по умолчанию год вперёд
const pad = (n) => String(n).padStart(2, '0')
const isoToday = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
const isoNextYear = `${now.getFullYear() + 1}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
const mapFrom = ref(isoToday)
const mapTo = ref(isoNextYear)
const mapEvents = computed(() => events.value.filter(
  (e) => e.starts_on && e.starts_on >= mapFrom.value && e.starts_on <= mapTo.value,
))

const MONTHS = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']
const WD = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']

function todayStr() {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
}
const today = todayStr()
function isNow(e) { return e.starts_on <= today && (e.ends_on || e.starts_on) >= today }
function dateRange(e) {
  return e.ends_on && e.ends_on !== e.starts_on ? `${formatDate(e.starts_on)} — ${formatDate(e.ends_on)}` : formatDate(e.starts_on)
}
const current = computed(() => events.value.find(isNow))

// ── календарная сетка ──
function eventsOnDay(y, m, d) {
  const iso = `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
  return events.value.filter((e) => e.starts_on && iso >= e.starts_on && iso <= (e.ends_on || e.starts_on))
}
const weeks = computed(() => {
  const { y, m } = cursor.value
  const startDow = (new Date(y, m - 1, 1).getDay() + 6) % 7
  const daysInMonth = new Date(y, m, 0).getDate()
  const cells = []
  for (let i = 0; i < startDow; i++) cells.push(null)
  for (let d = 1; d <= daysInMonth; d++) cells.push(d)
  while (cells.length % 7) cells.push(null)
  const w = []
  for (let i = 0; i < cells.length; i += 7) w.push(cells.slice(i, i + 7))
  return w
})
function prevMonth() { let { y, m } = cursor.value; m--; if (m < 1) { m = 12; y-- } cursor.value = { y, m } }
function nextMonth() { let { y, m } = cursor.value; m++; if (m > 12) { m = 1; y++ } cursor.value = { y, m } }
function isTodayCell(d) { return d && cursor.value.y === now.getFullYear() && cursor.value.m === now.getMonth() + 1 && d === now.getDate() }

async function load() {
  loading.value = true
  try {
    const { data } = await client.get('/events')
    await preloadImages(data.flatMap((e) => extractImageUrls(e.description))) // фото вперёд — без скачков
    events.value = data
  } finally {
    loading.value = false
  }
}
async function remove(e) {
  if (!(await confirmDialog({ message: `Удалить событие «${e.title}»?` }))) return
  await client.delete(`/events/${e.id}`)
  selected.value = null
  await load()
}

// ── лента: от сегодня вниз по возрастанию даты ──
const listFeed = computed(() => events.value
  .filter((e) => e.starts_on && (e.ends_on || e.starts_on) >= today)
  .sort((a, b) => (a.starts_on || '').localeCompare(b.starts_on || '')))
const feedPoints = computed(() => listFeed.value.map((e) => {
  const [y, m] = e.starts_on.split('-')
  return { id: `ev-${e.id}`, label: `${MONTHS[+m - 1]} ${y}` }
}))

onMounted(load)
// keep-alive: onMounted срабатывает один раз — при ВОЗВРАТЕ (например, после «Изменить/Сохранить»)
// перезагружаем список, чтобы изменения были видны сразу. Первую активацию пропускаем (не грузим дважды).
let firstActivate = true
onActivated(() => { if (firstActivate) { firstActivate = false; return } load() })
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <div class="mb-6 flex flex-wrap items-center justify-between gap-3">
      <p class="text-ink-700/60">Где находится гуру и что происходит</p>
      <div class="flex flex-wrap items-center gap-2">
        <!-- переключатель вида -->
        <div class="flex rounded-lg border border-parchment-300 p-0.5">
          <button class="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm transition"
                  :class="mode === 'list' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="mode = 'list'">
            <AppIcon name="reports" :size="15" /> Список
          </button>
          <button class="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm transition"
                  :class="mode === 'calendar' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="mode = 'calendar'">
            <AppIcon name="calendar" :size="15" /> Календарь
          </button>
          <button class="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm transition"
                  :class="mode === 'map' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="mode = 'map'">
            <AppIcon name="pin" :size="15" /> Карта
          </button>
        </div>
        <RouterLink v-if="auth.isStaff" :to="{ name: 'event-new' }" class="btn-primary shrink-0 whitespace-nowrap">+ Событие</RouterLink>
      </div>
    </div>

    <!-- current location banner -->
    <div v-if="current" class="card mb-6 border-saffron-400/50 bg-saffron-500/5 p-5">
      <div class="mb-1 text-sm uppercase tracking-wide text-saffron-700">Сейчас</div>
      <div class="font-display text-2xl text-ink-900">{{ current.title }}</div>
      <div v-if="current.location" class="text-ink-700">📍 {{ current.location }} · {{ dateRange(current) }}</div>
    </div>

    <div v-if="loading" class="space-y-4">
      <div v-for="i in 3" :key="i" class="card space-y-3 p-5"><AppSkeleton w="w-56" h="h-6" /><AppSkeleton /><AppSkeleton w="w-2/3" /></div>
    </div>

    <!-- LIST -->
    <div v-else-if="mode === 'list'" class="lg:flex lg:items-start lg:gap-6">
      <div class="min-w-0 flex-1 space-y-4">
        <div v-for="e in listFeed" :id="`ev-${e.id}`" :key="e.id" class="card scroll-mt-24 p-5" :class="isNow(e) && 'border-saffron-400/50'">
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="font-display text-xl font-semibold text-ink-900">{{ e.title }}</h3>
                <span v-if="isNow(e)" class="badge bg-saffron-500 text-white">Сейчас</span>
              </div>
              <div class="mt-0.5 text-sm text-ink-700/70">
                <span v-if="e.location">📍 {{ e.location }} · </span>{{ dateRange(e) }}
              </div>
            </div>
            <div v-if="auth.isStaff" class="flex shrink-0 gap-2">
              <RouterLink :to="{ name: 'event-edit', params: { id: e.id } }" class="btn-ghost">Изменить</RouterLink>
              <button class="text-ink-700/40 hover:text-red-600" @click="remove(e)">✕</button>
            </div>
          </div>
          <div v-if="e.description" class="markdown-body mt-3 text-ink-700" v-html="renderMarkdown(e.description)"></div>
        </div>
        <div v-if="!listFeed.length" class="card p-8 text-center text-ink-700/50">Предстоящих событий нет</div>
      </div>

      <!-- быстрый скроллер по датам -->
      <aside v-if="feedPoints.length > 3" class="sticky top-20 hidden h-[calc(100vh-6rem)] w-8 shrink-0 lg:block">
        <EventsFastScroll :points="feedPoints" />
      </aside>
    </div>

    <!-- CALENDAR -->
    <div v-else-if="mode === 'calendar'" class="card p-4 sm:p-6">
      <div class="mb-4 flex items-center justify-between">
        <button class="flex h-9 w-9 items-center justify-center rounded-full border border-parchment-300 text-ink-700 hover:bg-parchment-100" @click="prevMonth">
          <AppIcon name="chevron" :size="18" class="rotate-90" />
        </button>
        <div class="font-display text-xl font-semibold text-ink-900">{{ MONTHS[cursor.m - 1] }} {{ cursor.y }}</div>
        <button class="flex h-9 w-9 items-center justify-center rounded-full border border-parchment-300 text-ink-700 hover:bg-parchment-100" @click="nextMonth">
          <AppIcon name="chevron" :size="18" class="-rotate-90" />
        </button>
      </div>
      <div class="grid grid-cols-7 gap-1 text-center text-xs font-semibold uppercase tracking-wide text-ink-700/80">
        <div v-for="w in WD" :key="w" class="py-1">{{ w }}</div>
      </div>
      <div class="mt-1 grid grid-cols-7 gap-1">
        <template v-for="(week, wi) in weeks" :key="wi">
          <div v-for="(d, di) in week" :key="di" class="relative min-h-[44px] rounded-lg p-1 sm:min-h-[88px]"
               :class="d ? 'border border-parchment-300 bg-white' : ''">
            <template v-if="d">
              <div class="mb-1 text-sm font-semibold"
                   :class="isTodayCell(d) ? 'inline-flex h-6 w-6 items-center justify-center rounded-full bg-saffron-500 text-white' : 'text-ink-800'">{{ d }}</div>
              <button v-for="e in eventsOnDay(cursor.y, cursor.m, d)" :key="e.id"
                      class="relative z-10 mb-0.5 hidden w-full whitespace-normal break-words rounded bg-saffron-500/15 px-1 py-0.5 text-left text-[11px] leading-tight text-saffron-800 hover:bg-saffron-500/25 sm:block"
                      @click="selected = e">{{ e.title }}</button>
              <div v-if="eventsOnDay(cursor.y, cursor.m, d).length" class="flex flex-wrap gap-1 pt-0.5 sm:hidden">
                <span v-for="e in eventsOnDay(cursor.y, cursor.m, d)" :key="'dot' + e.id" class="h-2 w-2 rounded-full bg-saffron-500"></span>
              </div>
              <button v-if="eventsOnDay(cursor.y, cursor.m, d).length" type="button"
                      class="absolute inset-0 sm:hidden" aria-label="События дня"
                      @click="selected = eventsOnDay(cursor.y, cursor.m, d)[0]"></button>
            </template>
          </div>
        </template>
      </div>
    </div>

    <!-- MAP -->
    <div v-else-if="mode === 'map'">
      <div class="card mb-4 flex flex-wrap items-end gap-3 p-4">
        <div>
          <label class="label">С</label>
          <div class="w-40"><AppDatePicker v-model="mapFrom" /></div>
        </div>
        <div>
          <label class="label">По</label>
          <div class="w-40"><AppDatePicker v-model="mapTo" /></div>
        </div>
        <p class="ml-auto text-sm text-ink-700/60">Маршрут гуру · событий: <b class="text-ink-900">{{ mapEvents.length }}</b></p>
      </div>
      <!-- карта во всю ширину и до низа экрана (отрицательные поля гасят паддинги main) -->
      <div class="-mx-4 -mb-4 h-[calc(100dvh-15rem)] sm:-mx-6 sm:-mb-6 lg:-mx-8 lg:-mb-8">
        <EventsMap :events="mapEvents" @open="selected = $event" />
      </div>
    </div>

    <!-- event modal (from calendar) -->
    <teleport to="body">
      <div v-if="selected" class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-ink-900/50 p-4 sm:p-8" @click.self="selected = null">
        <div class="card w-full max-w-2xl p-6">
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="font-display text-2xl font-semibold text-ink-900">{{ selected.title }}</h3>
                <span v-if="isNow(selected)" class="badge bg-saffron-500 text-white">Сейчас</span>
              </div>
              <div class="mt-1 text-sm text-ink-700/70"><span v-if="selected.location">📍 {{ selected.location }} · </span>{{ dateRange(selected) }}</div>
            </div>
            <button class="text-ink-700/40 hover:text-ink-800" @click="selected = null"><AppIcon name="close" :size="20" /></button>
          </div>
          <div v-if="selected.description" class="markdown-body mt-4 text-ink-700" v-html="renderMarkdown(selected.description)"></div>
          <div v-if="auth.isStaff" class="mt-5 flex gap-2 border-t border-parchment-200 pt-4">
            <RouterLink :to="{ name: 'event-edit', params: { id: selected.id } }" class="btn-outline">Изменить</RouterLink>
            <button class="btn border border-red-200 text-red-600 hover:bg-red-50" @click="remove(selected)">Удалить</button>
          </div>
        </div>
      </div>
    </teleport>
  </div>
</template>

<style scoped>
.markdown-body :deep(a) { text-decoration: underline; color: #a85e1f; }
.markdown-body :deep(img) { max-height: 22rem; border-radius: 0.5rem; margin: 0.5rem 0; }
.markdown-body :deep(ul) { list-style: disc; margin-left: 1.25rem; }
</style>
