<script setup>
// Глобальный UI звонков: окно активного звонка + попап входящего. Монтируется в AppLayout, поэтому
// звонки приходят и отображаются в ЛЮБОМ разделе приложения (не только при открытом чате).
import { onMounted, onBeforeUnmount } from 'vue'
import AppIcon from './AppIcon.vue'
import { thumbUrl } from '../lib/format'
import { call, incoming, callRemoteVideo, callLocalVideo, callStatusText,
  placeCall, acceptIncoming, rejectIncoming, endCall, toggleCallVideo, toggleCallFullscreen } from '../composables/callCenter'

function initials(name) { return (name || '?').trim().split(/\s+/).slice(0, 2).map((w) => w[0]).join('').toUpperCase() }

// Escape: сначала выходим из полноэкранного режима звонка
function onKey(e) {
  if (e.key !== 'Escape') return
  if (call.fullscreen) { call.fullscreen = false; e.stopPropagation() }
}
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))
</script>

<template>
  <!-- Окно звонка (попап / на весь экран) -->
  <div v-if="call.open" class="fixed inset-0 z-[80] flex items-center justify-center bg-ink-900/60" :class="call.fullscreen ? 'p-0' : 'p-4'">
    <div class="relative flex flex-col items-center overflow-hidden bg-ink-900 text-white shadow-2xl"
         :class="call.fullscreen ? 'h-full w-full rounded-none' : 'w-full max-w-2xl rounded-2xl'">
      <template v-if="call.status === 'connected' && call.remoteVideo">
        <video ref="callRemoteVideo" autoplay playsinline class="w-full bg-black object-cover" :class="call.fullscreen ? 'h-full' : 'aspect-video max-h-[70vh]'"></video>
        <div class="pointer-events-none absolute left-0 right-0 top-0 bg-gradient-to-b from-black/50 to-transparent p-4 text-center">
          <div class="text-lg font-semibold">{{ call.name }}</div>
          <div class="text-xs text-white/70">{{ callStatusText }}</div>
        </div>
      </template>
      <div v-else class="flex flex-col items-center px-8 py-10" :class="call.fullscreen && 'flex-1 justify-center'">
        <div class="text-sm text-white/45">{{ callStatusText }}</div>
        <img v-if="call.avatar" :src="thumbUrl(call.avatar)" class="mt-6 rounded-full object-cover shadow-xl" :class="call.fullscreen ? 'h-56 w-56' : 'h-40 w-40'" />
        <span v-else class="mt-6 flex items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 font-semibold shadow-xl" :class="call.fullscreen ? 'h-56 w-56 text-7xl' : 'h-40 w-40 text-5xl'">{{ initials(call.name) }}</span>
        <div class="mt-5 text-2xl font-semibold">{{ call.name }}</div>
        <div v-if="call.status !== 'connected'" class="mt-2 text-center text-sm text-white/50">Если Вы хотите начать видеозвонок,<br>нажмите на значок камеры.</div>
      </div>
      <video v-show="call.localVideo" ref="callLocalVideo" autoplay playsinline muted class="absolute -scale-x-100 rounded-lg object-cover shadow-lg ring-2 ring-white/20"
             :class="call.fullscreen ? 'bottom-28 right-6 h-40 w-32' : 'right-4 top-4 h-28 w-24'"></video>
      <button v-if="call.status === 'connected'" class="absolute right-3 top-3 z-10 rounded-full bg-black/40 p-2 text-white transition hover:bg-black/60" :title="call.fullscreen ? 'Свернуть' : 'На весь экран'" @click="toggleCallFullscreen">
        <AppIcon :name="call.fullscreen ? 'minimize' : 'maximize'" :size="20" />
      </button>
      <div class="flex items-end justify-center gap-7" :class="[call.status === 'connected' && call.remoteVideo ? 'absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/60 to-transparent pb-8 pt-10' : 'pb-10']">
        <button class="flex flex-col items-center gap-2" @click="toggleCallVideo">
          <span class="flex h-12 w-12 items-center justify-center rounded-full text-white shadow-lg transition active:scale-95" :class="call.localVideo ? 'bg-sky-600' : 'bg-white/15 hover:bg-white/25'"><AppIcon name="video" :size="22" /></span>
          <span class="text-xs text-white/70">{{ call.localVideo ? 'Выкл. видео' : 'Вкл. видео' }}</span>
        </button>
        <button class="flex flex-col items-center gap-2" @click="endCall">
          <span class="flex h-12 w-12 items-center justify-center rounded-full bg-red-500 text-white shadow-lg transition hover:bg-red-600 active:scale-95"><AppIcon name="phone" :size="22" class="rotate-[135deg]" /></span>
          <span class="text-xs text-white/70">{{ call.status === 'connected' ? 'Завершить' : 'Отменить' }}</span>
        </button>
        <button v-if="call.status === 'idle-outgoing'" class="flex flex-col items-center gap-2" @click="placeCall">
          <span class="flex h-12 w-12 items-center justify-center rounded-full bg-sky-500 text-white shadow-lg transition hover:bg-sky-600 active:scale-95"><AppIcon name="phone" :size="22" /></span>
          <span class="text-xs text-white/70">Позвонить</span>
        </button>
      </div>
    </div>
  </div>

  <!-- Входящий звонок (попап) -->
  <div v-if="incoming.open" class="fixed inset-0 z-[81] flex items-center justify-center bg-ink-900/60 p-4">
    <div class="flex w-full max-w-sm flex-col items-center rounded-2xl bg-ink-900 p-8 text-white shadow-2xl">
      <div class="text-sm text-white/45">Входящий {{ incoming.video ? 'видеозвонок' : 'звонок' }}</div>
      <img v-if="incoming.avatar" :src="thumbUrl(incoming.avatar)" class="mt-6 h-32 w-32 rounded-full object-cover shadow-xl" />
      <span v-else class="mt-6 flex h-32 w-32 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-4xl font-semibold shadow-xl">{{ initials(incoming.name) }}</span>
      <div class="mt-4 text-xl font-semibold">{{ incoming.name }}</div>
      <div class="mt-8 flex items-end gap-12">
        <button class="flex flex-col items-center gap-2" @click="rejectIncoming">
          <span class="flex h-14 w-14 items-center justify-center rounded-full bg-red-500 text-white transition hover:bg-red-600 animate-pulse"><AppIcon name="phone" :size="24" class="rotate-[135deg]" /></span>
          <span class="text-xs text-white/60">Отклонить</span>
        </button>
        <button class="flex flex-col items-center gap-2" @click="acceptIncoming">
          <span class="flex h-14 w-14 items-center justify-center rounded-full bg-green-500 text-white transition hover:bg-green-600"><AppIcon name="phone" :size="24" /></span>
          <span class="text-xs text-white/60">Принять</span>
        </button>
      </div>
    </div>
  </div>
</template>
