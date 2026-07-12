<script setup>
import { computed } from 'vue'
import AppIcon from './AppIcon.vue'
import { player, togglePlay, skip, seek, cycleRate, toggleMute, closePlayer } from '../composables/audioPlayer'

function fmt(s) {
  if (!s || !isFinite(s)) return '0:00'
  const m = Math.floor(s / 60)
  return `${m}:${String(Math.floor(s % 60)).padStart(2, '0')}`
}
const progress = computed(() => (player.duration ? (player.currentTime / player.duration) * 100 : 0))
const rateLabel = computed(() => `${player.rate}×`)

function seekFrom(e, el) {
  const rect = el.getBoundingClientRect()
  const x = (e.touches ? e.touches[0].clientX : e.clientX)
  const frac = Math.max(0, Math.min(1, (x - rect.left) / rect.width))
  seek(frac * (player.duration || 0))
}
function onTrackDown(e) {
  const el = e.currentTarget
  seekFrom(e, el)
  const move = (ev) => { seekFrom(ev, el); if (ev.cancelable) ev.preventDefault() }
  const up = () => {
    window.removeEventListener('mousemove', move); window.removeEventListener('mouseup', up)
    window.removeEventListener('touchmove', move); window.removeEventListener('touchend', up)
  }
  window.addEventListener('mousemove', move); window.addEventListener('mouseup', up)
  window.addEventListener('touchmove', move, { passive: false }); window.addEventListener('touchend', up)
}
</script>

<template>
  <transition name="bar">
    <div v-if="player.visible" class="mb-2 shrink-0 overflow-hidden rounded-xl bg-ink-900 text-white shadow-md">
      <div class="flex items-center gap-1.5 px-2 py-2 sm:gap-2 sm:px-3">
        <button class="rounded-full p-1.5 text-white/70 transition hover:bg-white/10 hover:text-white" title="Назад 5 сек" @click="skip(-5)">
          <AppIcon name="rewind" :size="20" />
        </button>
        <button class="rounded-full bg-white/10 p-2 transition hover:bg-white/20" :title="player.playing ? 'Пауза' : 'Играть'" @click="togglePlay">
          <AppIcon :name="player.playing ? 'pause' : 'play'" :size="20" />
        </button>
        <button class="rounded-full p-1.5 text-white/70 transition hover:bg-white/10 hover:text-white" title="Вперёд 5 сек" @click="skip(5)">
          <AppIcon name="forward" :size="20" />
        </button>

        <div class="min-w-0 flex-1 px-1">
          <div class="truncate text-sm font-medium">{{ player.label || 'Голосовое сообщение' }}</div>
        </div>

        <span class="shrink-0 text-xs tabular-nums text-white/70 sm:text-sm">{{ fmt(player.currentTime) }} / {{ fmt(player.duration) }}</span>
        <button class="rounded-full p-1.5 text-white/70 transition hover:bg-white/10 hover:text-white" :title="player.volume > 0 ? 'Выключить звук' : 'Включить звук'" @click="toggleMute">
          <AppIcon :name="player.volume > 0 ? 'volume' : 'volume-x'" :size="20" />
        </button>
        <button class="shrink-0 rounded-md px-1.5 py-1 text-sm font-semibold text-saffron-300 ring-1 ring-white/15 transition hover:bg-white/10" title="Скорость" @click="cycleRate">
          {{ rateLabel }}
        </button>
        <button class="rounded-full p-1.5 text-white/60 transition hover:bg-white/10 hover:text-white" title="Закрыть" @click="closePlayer">
          <AppIcon name="close" :size="20" />
        </button>
      </div>

      <!-- перемотка: клик и перетаскивание -->
      <div class="group relative h-3 w-full cursor-pointer touch-none px-3 pb-2" @mousedown.prevent="onTrackDown" @touchstart.prevent="onTrackDown">
        <div class="relative h-1 w-full rounded-full bg-white/15">
          <div class="absolute left-0 top-0 h-full rounded-full bg-saffron-400" :style="{ width: progress + '%' }"></div>
          <div class="absolute top-1/2 h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-full bg-white opacity-0 shadow transition-opacity group-hover:opacity-100" :style="{ left: progress + '%' }"></div>
        </div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.bar-enter-active, .bar-leave-active { transition: max-height .25s ease, opacity .2s ease; max-height: 6rem; overflow: hidden; }
.bar-enter-from, .bar-leave-to { max-height: 0; opacity: 0; }
</style>
