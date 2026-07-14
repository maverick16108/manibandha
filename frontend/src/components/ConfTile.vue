<script setup>
import AppIcon from './AppIcon.vue'

defineProps({
  t: { type: Object, required: true },
  raised: { type: Object, default: () => ({}) },
  pinnedId: { type: [String, null], default: null },
  isHost: { type: Boolean, default: false },
})
const emit = defineEmits(['pin', 'permit'])
function initials(name) { return (name || '?').trim()[0]?.toUpperCase() || '?' }
</script>

<template>
  <div :data-tile="t.identity" class="group relative flex items-center justify-center overflow-hidden rounded-xl bg-ink-900" :class="t.speaking && 'speaking'">
    <video :data-cam="t.identity" autoplay playsinline :muted="t.isLocal" class="h-full w-full object-cover" :class="!t.camOn && 'hidden'"></video>
    <div v-if="!t.camOn" class="flex h-full w-full items-center justify-center">
      <span class="flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-xl font-semibold text-white">{{ initials(t.name) }}</span>
    </div>
    <div class="absolute bottom-1.5 left-1.5 flex items-center gap-1 rounded-md bg-black/50 px-1.5 py-0.5 text-xs text-white">
      <AppIcon v-if="!t.micOn" name="mic-off" :size="17" class="text-red-400" />
      <span v-if="raised[t.identity]" class="text-lg leading-none">✋</span>
      <span class="max-w-[9rem] truncate sm:max-w-[16rem]">{{ t.name }}<span v-if="t.isLocal"> (вы)</span></span>
    </div>
    <div class="absolute right-2 top-2 flex gap-1.5 opacity-100 transition sm:opacity-70 sm:group-hover:opacity-100">
      <button class="rounded-lg bg-black/50 p-2 text-white hover:bg-black/70" :class="pinnedId===t.identity && 'bg-saffron-500/90'" :title="pinnedId===t.identity ? 'Открепить' : 'На весь экран'" @click="emit('pin', t.identity)"><AppIcon name="pushpin" :size="20" /></button>
      <template v-if="isHost && !t.isLocal">
        <button class="rounded-lg p-2 text-white hover:bg-black/70" :class="t.allowAudio ? 'bg-black/50' : 'bg-red-500/90'" :title="t.allowAudio ? 'Запретить звук' : 'Разрешить звук'" @click.stop="emit('permit', t.identity, 'audio', !t.allowAudio)"><AppIcon :name="t.allowAudio ? 'volume' : 'mic-off'" :size="20" /></button>
        <button class="rounded-lg p-2 text-white hover:bg-black/70" :class="t.allowVideo ? 'bg-black/50' : 'bg-red-500/90'" :title="t.allowVideo ? 'Запретить видео' : 'Разрешить видео'" @click.stop="emit('permit', t.identity, 'video', !t.allowVideo)"><AppIcon name="video" :size="20" /></button>
      </template>
    </div>
  </div>
</template>
