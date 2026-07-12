<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter, useRoute, RouterLink } from 'vue-router'
import client from '../api/client'
import PublicShell from '../components/PublicShell.vue'
import AppIcon from '../components/AppIcon.vue'
import AppDatePicker from '../components/AppDatePicker.vue'
import EventsMap from '../components/EventsMap.vue'
import EventsFastScroll from '../components/EventsFastScroll.vue'
import { renderMarkdown } from '../lib/markdown'

const router = useRouter()
const route = useRoute()
const events = ref([])
const cursor = ref({ y: 2026, m: 7 })

// вид: список (по умолчанию — развёрнутая лента), календарь или карта маршрута
const mode = ref(route.query.view === 'map' ? 'map' : route.query.view === 'calendar' ? 'calendar' : 'list')

// период карты — по умолчанию год вперёд
const pad = (n) => String(n).padStart(2, '0')
const nowMap = new Date()
const isoToday = `${nowMap.getFullYear()}-${pad(nowMap.getMonth() + 1)}-${pad(nowMap.getDate())}`
const isoNextYear = `${nowMap.getFullYear() + 1}-${pad(nowMap.getMonth() + 1)}-${pad(nowMap.getDate())}`
const mapFrom = ref(isoToday)
const mapTo = ref(isoNextYear)
const mapEvents = computed(() => events.value.filter(
  (e) => e.starts_on && e.starts_on >= mapFrom.value && e.starts_on <= mapTo.value,
))

const MONTHS = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']
const WD = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']
const today = new Date()

function eventsOnDay(y, m, d) {
  const iso = `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
  return events.value.filter((e) => e.starts_on && iso >= e.starts_on && iso <= (e.ends_on || e.starts_on))
}

const weeks = computed(() => {
  const { y, m } = cursor.value
  const startDow = (new Date(y, m - 1, 1).getDay() + 6) % 7 // Пн = 0
  const daysInMonth = new Date(y, m, 0).getDate()
  const cells = []
  for (let i = 0; i < startDow; i++) cells.push(null)
  for (let d = 1; d <= daysInMonth; d++) cells.push(d)
  while (cells.length % 7) cells.push(null)
  const w = []
  for (let i = 0; i < cells.length; i += 7) w.push(cells.slice(i, i + 7))
  return w
})

// события выбранного месяца — список под сеткой
const monthEvents = computed(() => {
  const { y, m } = cursor.value
  const pref = `${y}-${String(m).padStart(2, '0')}`
  return events.value
    .filter((e) => (e.starts_on || '').startsWith(pref) || (e.ends_on || '').startsWith(pref))
    .sort((a, b) => (a.starts_on || '').localeCompare(b.starts_on || ''))
})

const MON_SHORT = ['янв', 'фев', 'мар', 'апр', 'май', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек']
function fmtShort(iso) { if (!iso) return ''; const [, mm, dd] = iso.split('-'); return `${+dd} ${MON_SHORT[+mm - 1]}` }
function range(e) { const s = fmtShort(e.starts_on); return e.ends_on && e.ends_on !== e.starts_on ? `${s} — ${fmtShort(e.ends_on)}` : s }

function prevMonth() { let { y, m } = cursor.value; m--; if (m < 1) { m = 12; y-- } cursor.value = { y, m } }
function nextMonth() { let { y, m } = cursor.value; m++; if (m > 12) { m = 1; y++ } cursor.value = { y, m } }
function isToday(d) { return d && cursor.value.y === today.getFullYear() && cursor.value.m === today.getMonth() + 1 && d === today.getDate() }
function openEvent(id, from = 'calendar') { router.push({ name: 'public-event', params: { id }, query: { from } }) }

// тап по дню — прокрутить к событиям дня в списке ниже и подсветить все
const highlightIds = ref([])
function focusDay(y, m, d) {
  const evs = eventsOnDay(y, m, d)
  if (!evs.length) return
  const ids = evs.map((e) => e.id)
  highlightIds.value = ids
  nextTick(() => {
    document.getElementById('ev-' + ids[0])?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  })
  setTimeout(() => { if (highlightIds.value === ids) highlightIds.value = [] }, 2000)
}

// ── лента и быстрый скроллер по датам (режим «Список») ──
const feed = computed(() => [...events.value].sort((a, b) => (a.starts_on || '').localeCompare(b.starts_on || '')))
const feedPoints = computed(() => feed.value.filter((e) => e.starts_on).map((e) => {
  const [y, m] = e.starts_on.split('-')
  return { id: `evl-${e.id}`, label: `${MONTHS[+m - 1]} ${y}` }
}))

onMounted(async () => {
  cursor.value = { y: today.getFullYear(), m: today.getMonth() + 1 }
  try { const { data } = await client.get('/events/public'); events.value = data } catch { /* пусто */ }
})
</script>

<template>
  <PublicShell>
    <nav class="mb-4 flex items-center gap-1.5 text-sm text-ink-700/60">
      <RouterLink to="/" class="hover:text-saffron-700">Главная</RouterLink>
      <span class="text-ink-700/30">/</span>
      <span class="text-ink-800">{{ mode === 'map' ? 'Карта' : mode === 'list' ? 'Список' : 'Календарь' }}</span>
    </nav>
    <h1 class="mb-6 font-display text-3xl font-semibold text-ink-900 sm:text-4xl">Календарь событий</h1>

    <!-- переключатель вида -->
    <div class="mb-6 flex rounded-lg border border-parchment-300 p-0.5 sm:w-max">
      <button class="flex flex-1 items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-sm transition sm:flex-none"
              :class="mode === 'list' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="mode = 'list'">
        <AppIcon name="reports" :size="15" /> Список
      </button>
      <button class="flex flex-1 items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-sm transition sm:flex-none"
              :class="mode === 'calendar' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="mode = 'calendar'">
        <AppIcon name="calendar" :size="15" /> Календарь
      </button>
      <button class="flex flex-1 items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-sm transition sm:flex-none"
              :class="mode === 'map' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="mode = 'map'">
        <AppIcon name="pin" :size="15" /> Карта
      </button>
    </div>

    <!-- ЛЕНТА событий с навигатором по месяцам -->
    <template v-if="mode === 'list'">
      <div class="lg:flex lg:items-start lg:gap-6">
        <div class="min-w-0 flex-1 space-y-4">
          <article v-for="e in feed" :id="'evl-' + e.id" :key="e.id" class="card scroll-mt-20 p-5">
            <h3 class="font-display text-xl font-semibold text-ink-900">{{ e.title }}</h3>
            <div class="mt-0.5 text-sm text-ink-700/70"><span v-if="e.location">📍 {{ e.location }} · </span>{{ range(e) }}</div>
            <div v-if="e.description" class="markdown-body mt-3 text-ink-700" v-html="renderMarkdown(e.description)"></div>
          </article>
          <p v-if="!feed.length" class="card p-8 text-center text-ink-700/50">Событий пока нет</p>
        </div>
        <aside v-if="feedPoints.length > 3" class="sticky top-6 hidden h-[calc(100vh-3rem)] w-8 shrink-0 lg:block">
          <EventsFastScroll :points="feedPoints" />
        </aside>
      </div>
    </template>

    <template v-if="mode === 'calendar'">
    <div class="card p-4 sm:p-6">
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
          <div v-for="(d, di) in week" :key="di"
               class="relative min-h-[44px] rounded-lg p-1 sm:min-h-[84px]"
               :class="d ? 'border border-parchment-300 bg-white' : ''">
            <template v-if="d">
              <div class="mb-1 text-sm font-semibold"
                   :class="isToday(d) ? 'inline-flex h-6 w-6 items-center justify-center rounded-full bg-saffron-500 text-white' : 'text-ink-800'">{{ d }}</div>
              <!-- десктоп: названия -->
              <button v-for="e in eventsOnDay(cursor.y, cursor.m, d)" :key="e.id"
                      class="relative z-10 mb-0.5 hidden w-full whitespace-normal break-words rounded bg-saffron-500/15 px-1 py-0.5 text-left text-[11px] leading-tight text-saffron-800 hover:bg-saffron-500/25 sm:block"
                      @click="openEvent(e.id)">{{ e.title }}</button>
              <!-- мобильный: точки -->
              <div v-if="eventsOnDay(cursor.y, cursor.m, d).length" class="flex flex-wrap gap-1 pt-0.5 sm:hidden">
                <span v-for="e in eventsOnDay(cursor.y, cursor.m, d)" :key="'dot' + e.id" class="h-2 w-2 rounded-full bg-saffron-500"></span>
              </div>
              <!-- мобильный: тап по всей ячейке -->
              <button v-if="eventsOnDay(cursor.y, cursor.m, d).length" type="button"
                      class="absolute inset-0 sm:hidden" aria-label="События дня"
                      @click="focusDay(cursor.y, cursor.m, d)"></button>
            </template>
          </div>
        </template>
      </div>
    </div>

    <!-- список событий месяца -->
    <div v-if="monthEvents.length" class="mt-6 space-y-2">
      <button v-for="e in monthEvents" :key="e.id" :id="'ev-' + e.id"
              class="flex w-full items-center gap-4 rounded-xl border bg-white px-4 py-3 text-left transition hover:border-saffron-300 hover:shadow-sm"
              :class="highlightIds.includes(e.id) ? 'border-saffron-400 ring-2 ring-saffron-300' : 'border-parchment-200'"
              @click="openEvent(e.id)">
        <span class="inline-flex w-28 shrink-0 items-center gap-1.5 text-sm font-medium text-saffron-700">
          <AppIcon name="calendar" :size="15" /> {{ range(e) }}
        </span>
        <span class="min-w-0 flex-1">
          <span class="block truncate font-medium text-ink-900">{{ e.title }}</span>
          <span v-if="e.location" class="block truncate text-sm text-ink-700/60">{{ e.location }}</span>
        </span>
        <AppIcon name="chevron" :size="16" class="-rotate-90 shrink-0 text-ink-700/40" />
      </button>
    </div>
    <p v-else class="mt-6 text-center text-ink-700/50">В этом месяце событий нет</p>
    </template>

    <!-- карта маршрута -->
    <template v-else-if="mode === 'map'">
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
      <EventsMap :events="mapEvents" @open="openEvent($event.id, 'map')" />
    </template>
  </PublicShell>
</template>

<style scoped>
.markdown-body :deep(img) { max-width: 100%; border-radius: 0.75rem; margin: 0.5rem 0; }
.markdown-body :deep(a) { text-decoration: underline; }
.markdown-body :deep(ul) { list-style: disc; padding-left: 1.25rem; margin: 0.5rem 0; }
.markdown-body :deep(ol) { list-style: decimal; padding-left: 1.35rem; margin: 0.5rem 0; }
.markdown-body :deep(p) { margin: 0.5rem 0; }
</style>
