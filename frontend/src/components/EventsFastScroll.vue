<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'

// Быстрый скроллер ленты (как в Telegram): тянешь ползунок — страница листается,
// рядом всплывает подпись (месяц) текущей позиции. Работает с прокруткой окна.
const props = defineProps({
  // якоря в порядке ленты: [{ id: 'evl-12', label: 'Июль 2026' }]
  points: { type: Array, default: () => [] },
})

const THUMB = 34
const track = ref(null)
const thumbTop = ref(0)
const dragging = ref(false)
const label = ref('')

function metrics() {
  const max = document.documentElement.scrollHeight - window.innerHeight
  return { y: window.scrollY, max: Math.max(1, max) }
}
function trackH() { return track.value ? track.value.clientHeight : 0 }

function updateThumb() {
  const { y, max } = metrics()
  thumbTop.value = (y / max) * (trackH() - THUMB)
  if (dragging.value) updateLabel()
}
function updateLabel() {
  let l = props.points[0]?.label || ''
  for (const p of props.points) {
    const el = document.getElementById(p.id)
    if (el && el.getBoundingClientRect().top - 90 <= 0) l = p.label
  }
  label.value = l
}
function onWin() { if (!dragging.value) updateThumb() }

function scrollFrom(clientY) {
  const rect = track.value.getBoundingClientRect()
  const frac = Math.max(0, Math.min(1, (clientY - rect.top) / rect.height))
  const { max } = metrics()
  window.scrollTo({ top: frac * max })
  thumbTop.value = frac * (trackH() - THUMB)
  updateLabel()
}
function onDown(e) {
  dragging.value = true
  scrollFrom(e.touches ? e.touches[0].clientY : e.clientY)
  const move = (ev) => { scrollFrom(ev.touches ? ev.touches[0].clientY : ev.clientY); if (ev.cancelable) ev.preventDefault() }
  const up = () => {
    dragging.value = false
    window.removeEventListener('mousemove', move); window.removeEventListener('mouseup', up)
    window.removeEventListener('touchmove', move); window.removeEventListener('touchend', up)
  }
  window.addEventListener('mousemove', move); window.addEventListener('mouseup', up)
  window.addEventListener('touchmove', move, { passive: false }); window.addEventListener('touchend', up)
}

let resizeObs
onMounted(() => {
  window.addEventListener('scroll', onWin, { passive: true })
  window.addEventListener('resize', onWin)
  resizeObs = new ResizeObserver(onWin)
  if (track.value) resizeObs.observe(document.body)
  updateThumb()
})
onBeforeUnmount(() => {
  window.removeEventListener('scroll', onWin)
  window.removeEventListener('resize', onWin)
  if (resizeObs) resizeObs.disconnect()
})
</script>

<template>
  <div class="relative h-full w-8 select-none">
    <!-- дорожка -->
    <div ref="track" class="absolute inset-y-2 left-1/2 w-1 -translate-x-1/2 rounded-full bg-parchment-300/70"></div>
    <!-- ползунок -->
    <div class="absolute left-1/2 -translate-x-1/2 cursor-grab touch-none rounded-full bg-saffron-500 shadow ring-2 ring-white transition-colors active:cursor-grabbing"
         :class="dragging ? 'w-3' : 'w-2 hover:w-3'"
         :style="{ top: (thumbTop + 8) + 'px', height: THUMB + 'px' }"
         @mousedown.prevent="onDown" @touchstart.prevent="onDown"></div>
    <!-- подпись даты при перетаскивании -->
    <transition name="fs">
      <div v-if="dragging && label" class="pointer-events-none absolute right-full mr-2 whitespace-nowrap rounded-lg bg-ink-900 px-3 py-1 text-sm font-semibold text-white shadow-lg"
           :style="{ top: (thumbTop + 8) + 'px' }">{{ label }}</div>
    </transition>
  </div>
</template>

<style scoped>
.fs-enter-active, .fs-leave-active { transition: opacity .12s ease; }
.fs-enter-from, .fs-leave-to { opacity: 0; }
</style>
