<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import AppIcon from './AppIcon.vue'

// modelValue: 'YYYY-MM-DD' | '' (same format as native <input type=date>)
const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: 'дд.мм.гггг' },
})
const emit = defineEmits(['update:modelValue'])

const MONTHS = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']
const WEEK = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']

const root = ref(null)
const open = ref(false)
const view = ref('days') // 'days' | 'years'

function parse(v) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(v || '')
  return m ? { y: +m[1], mo: +m[2] - 1, d: +m[3] } : null
}
const now = new Date()
const viewY = ref(now.getFullYear())
const viewMo = ref(now.getMonth())

const display = computed(() => {
  const p = parse(props.modelValue)
  return p ? `${String(p.d).padStart(2, '0')}.${String(p.mo + 1).padStart(2, '0')}.${p.y}` : ''
})

const grid = computed(() => {
  const first = new Date(viewY.value, viewMo.value, 1)
  const offset = (first.getDay() + 6) % 7 // Monday-first
  const daysInMonth = new Date(viewY.value, viewMo.value + 1, 0).getDate()
  const cells = []
  for (let i = 0; i < offset; i++) cells.push(null)
  for (let d = 1; d <= daysInMonth; d++) cells.push(d)
  return cells
})
const years = computed(() => Array.from({ length: 12 }, (_, i) => viewY.value - 6 + i))

function isSel(d) {
  const p = parse(props.modelValue)
  return p && p.y === viewY.value && p.mo === viewMo.value && p.d === d
}
function isToday(d) {
  return now.getFullYear() === viewY.value && now.getMonth() === viewMo.value && now.getDate() === d
}
function pick(d) {
  const v = `${viewY.value}-${String(viewMo.value + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`
  emit('update:modelValue', v)
  open.value = false
}
function prevMonth() { if (viewMo.value === 0) { viewMo.value = 11; viewY.value-- } else viewMo.value-- }
function nextMonth() { if (viewMo.value === 11) { viewMo.value = 0; viewY.value++ } else viewMo.value++ }
function pickYear(y) { viewY.value = y; view.value = 'days' }
function today() { const t = new Date(); pick(t.getDate()); viewY.value = t.getFullYear(); viewMo.value = t.getMonth() }
function clear() { emit('update:modelValue', ''); open.value = false }

function toggle() {
  open.value = !open.value
  if (open.value) {
    const p = parse(props.modelValue)
    if (p) { viewY.value = p.y; viewMo.value = p.mo }
    view.value = 'days'
  }
}
function onDoc(e) { if (root.value && !root.value.contains(e.target)) open.value = false }
onMounted(() => document.addEventListener('mousedown', onDoc))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDoc))
</script>

<template>
  <div ref="root" class="relative">
    <button type="button"
      class="flex w-full items-center justify-between gap-2 rounded-md border bg-white px-3 py-2 text-left text-sm transition-colors focus:outline-none focus:ring-1 focus:ring-saffron-400"
      :class="open ? 'border-saffron-400 ring-1 ring-saffron-400' : 'border-parchment-300 hover:border-saffron-400/60'"
      @click="toggle">
      <span :class="display ? 'text-ink-800' : 'text-ink-700/40'">{{ display || placeholder }}</span>
      <AppIcon name="calendar" :size="16" class="shrink-0 text-ink-700/50" />
    </button>

    <transition enter-active-class="transition duration-100 ease-out" enter-from-class="opacity-0 -translate-y-1"
      leave-active-class="transition duration-75 ease-in" leave-to-class="opacity-0 -translate-y-1">
      <div v-if="open" class="absolute z-40 mt-1 w-72 rounded-lg border border-parchment-300 bg-white p-3 shadow-lg">
        <!-- header -->
        <div class="mb-2 flex items-center justify-between">
          <button type="button" class="rounded p-1 text-ink-700 hover:bg-parchment-100" @click="view === 'days' ? prevMonth() : (viewY -= 12)">
            <AppIcon name="chevron" :size="18" class="rotate-90" />
          </button>
          <button type="button" class="rounded px-2 py-1 font-medium text-ink-900 hover:bg-parchment-100" @click="view = view === 'days' ? 'years' : 'days'">
            <span v-if="view === 'days'">{{ MONTHS[viewMo] }} {{ viewY }}</span>
            <span v-else>{{ years[0] }}–{{ years[11] }}</span>
          </button>
          <button type="button" class="rounded p-1 text-ink-700 hover:bg-parchment-100" @click="view === 'days' ? nextMonth() : (viewY += 12)">
            <AppIcon name="chevron" :size="18" class="-rotate-90" />
          </button>
        </div>

        <!-- days -->
        <template v-if="view === 'days'">
          <div class="mb-1 grid grid-cols-7 text-center text-xs text-ink-700/50">
            <span v-for="w in WEEK" :key="w">{{ w }}</span>
          </div>
          <div class="grid grid-cols-7 gap-0.5">
            <template v-for="(d, i) in grid" :key="i">
              <span v-if="d === null"></span>
              <button v-else type="button"
                class="flex h-8 items-center justify-center rounded text-sm transition-colors"
                :class="isSel(d) ? 'bg-saffron-500 text-white' : isToday(d) ? 'border border-saffron-400 text-saffron-700' : 'text-ink-800 hover:bg-parchment-100'"
                @click="pick(d)">{{ d }}</button>
            </template>
          </div>
        </template>

        <!-- years -->
        <div v-else class="grid grid-cols-3 gap-1">
          <button v-for="y in years" :key="y" type="button"
            class="rounded py-2 text-sm transition-colors"
            :class="y === viewY ? 'bg-saffron-500 text-white' : 'text-ink-800 hover:bg-parchment-100'"
            @click="pickYear(y)">{{ y }}</button>
        </div>

        <div class="mt-2 flex justify-between border-t border-parchment-100 pt-2 text-sm">
          <button type="button" class="text-ink-700/60 hover:text-red-600" @click="clear">Очистить</button>
          <button type="button" class="text-saffron-600 hover:text-saffron-700" @click="today">Сегодня</button>
        </div>
      </div>
    </transition>
  </div>
</template>
