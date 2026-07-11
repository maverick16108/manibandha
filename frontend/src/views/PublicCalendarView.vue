<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import client from '../api/client'
import PublicShell from '../components/PublicShell.vue'
import AppIcon from '../components/AppIcon.vue'

const router = useRouter()
const events = ref([])
const cursor = ref({ y: 2026, m: 7 })

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
function openEvent(id) { router.push({ name: 'public-event', params: { id }, query: { from: 'calendar' } }) }

// тап по дню на мобиле — прокрутить к событию в списке ниже и подсветить
const highlightId = ref(null)
function focusDay(y, m, d) {
  const evs = eventsOnDay(y, m, d)
  if (!evs.length) return
  const first = evs[0]
  highlightId.value = first.id
  nextTick(() => {
    document.getElementById('ev-' + first.id)?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  })
  setTimeout(() => { if (highlightId.value === first.id) highlightId.value = null }, 1800)
}

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
      <span class="text-ink-800">Календарь</span>
    </nav>
    <h1 class="mb-6 font-display text-3xl font-semibold text-ink-900 sm:text-4xl">Календарь событий</h1>

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

      <div class="grid grid-cols-7 gap-1 text-center text-xs uppercase tracking-wide text-ink-700/50">
        <div v-for="w in WD" :key="w" class="py-1">{{ w }}</div>
      </div>
      <div class="mt-1 grid grid-cols-7 gap-1">
        <template v-for="(week, wi) in weeks" :key="wi">
          <div v-for="(d, di) in week" :key="di"
               class="min-h-[44px] rounded-lg p-1 sm:min-h-[84px]"
               :class="[d ? 'border border-parchment-200 bg-white' : '', d && eventsOnDay(cursor.y, cursor.m, d).length ? 'cursor-pointer sm:cursor-default' : '']"
               @click="d && eventsOnDay(cursor.y, cursor.m, d).length && focusDay(cursor.y, cursor.m, d)">
            <template v-if="d">
              <div class="mb-1 text-xs font-medium"
                   :class="isToday(d) ? 'inline-flex h-5 w-5 items-center justify-center rounded-full bg-saffron-500 text-white' : 'text-ink-700/50'">{{ d }}</div>
              <!-- десктоп: названия -->
              <button v-for="e in eventsOnDay(cursor.y, cursor.m, d)" :key="e.id"
                      class="mb-0.5 hidden w-full whitespace-normal break-words rounded bg-saffron-500/15 px-1 py-0.5 text-left text-[11px] leading-tight text-saffron-800 hover:bg-saffron-500/25 sm:block"
                      @click.stop="openEvent(e.id)">{{ e.title }}</button>
              <!-- мобильный: точки -->
              <div v-if="eventsOnDay(cursor.y, cursor.m, d).length" class="flex flex-wrap gap-1 pt-0.5 sm:hidden">
                <span v-for="e in eventsOnDay(cursor.y, cursor.m, d)" :key="'dot' + e.id" class="h-2 w-2 rounded-full bg-saffron-500"></span>
              </div>
            </template>
          </div>
        </template>
      </div>
    </div>

    <!-- список событий месяца -->
    <div v-if="monthEvents.length" class="mt-6 space-y-2">
      <button v-for="e in monthEvents" :key="e.id" :id="'ev-' + e.id"
              class="flex w-full items-center gap-4 rounded-xl border bg-white px-4 py-3 text-left transition hover:border-saffron-300 hover:shadow-sm"
              :class="highlightId === e.id ? 'border-saffron-400 ring-2 ring-saffron-300' : 'border-parchment-200'"
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
  </PublicShell>
</template>
