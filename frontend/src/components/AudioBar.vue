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

function seekAt(e) {
  const el = e.currentTarget
  const rect = el.getBoundingClientRect()
  const frac = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
  seek(frac * (player.duration || 0))
}
</script>

<template>
  <transition name="bar">
    <div v-if="player.visible" class="fixed inset-x-0 top-0 z-50 bg-ink-900 text-white shadow-lg">
      <div class="mx-auto flex h-14 max-w-6xl items-center gap-2 px-3 sm:gap-3 sm:px-5">
        <button class="rounded-full p-1.5 text-white/70 transition hover:bg-white/10 hover:text-white" title="Назад 5 сек" @click="skip(-5)">
          <AppIcon name="rewind" :size="22" />
        </button>
        <button class="rounded-full bg-white/10 p-2 transition hover:bg-white/20" :title="player.playing ? 'Пауза' : 'Играть'" @click="togglePlay">
          <AppIcon :name="player.playing ? 'pause' : 'play'" :size="22" />
        </button>
        <button class="rounded-full p-1.5 text-white/70 transition hover:bg-white/10 hover:text-white" title="Вперёд 5 сек" @click="skip(5)">
          <AppIcon name="forward" :size="22" />
        </button>

        <div class="min-w-0 flex-1">
          <div class="truncate text-sm font-medium">{{ player.label || 'Голосовое сообщение' }}</div>
        </div>

        <span class="shrink-0 text-sm tabular-nums text-white/70">{{ fmt(player.currentTime) }} / {{ fmt(player.duration) }}</span>
        <button class="rounded-full p-1.5 text-white/70 transition hover:bg-white/10 hover:text-white" :title="player.volume > 0 ? 'Выключить звук' : 'Включить звук'" @click="toggleMute">
          <AppIcon :name="player.volume > 0 ? 'volume' : 'volume-x'" :size="22" />
        </button>
        <button class="shrink-0 rounded-md px-2 py-1 text-sm font-semibold text-saffron-300 ring-1 ring-white/15 transition hover:bg-white/10" title="Скорость" @click="cycleRate">
          {{ rateLabel }}
        </button>
        <button class="rounded-full p-1.5 text-white/60 transition hover:bg-white/10 hover:text-white" title="Закрыть" @click="closePlayer">
          <AppIcon name="close" :size="22" />
        </button>
      </div>

      <!-- прогресс: клик — перемотать -->
      <div class="h-1 w-full cursor-pointer bg-white/15" @click="seekAt">
        <div class="h-full bg-saffron-400 transition-[width] duration-150" :style="{ width: progress + '%' }"></div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.bar-enter-active, .bar-leave-active { transition: transform .2s ease, opacity .2s ease; }
.bar-enter-from, .bar-leave-to { transform: translateY(-100%); opacity: 0; }
</style>
