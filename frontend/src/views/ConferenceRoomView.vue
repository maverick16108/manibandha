<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Room, RoomEvent, Track } from 'livekit-client'
import client from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Конференция')
const route = useRoute()
const router = useRouter()
const id = route.params.id

const state = ref('connecting') // connecting | connected | error
const errorMsg = ref('')
let room = null
const canPublish = ref(false)
const mode = ref('interactive')
const micOn = ref(false)
const camOn = ref(false)
const screenOn = ref(false)

const tiles = ref([]) // [{ identity, name, isLocal, camOn, micOn, speaking }]
const screenSharer = ref(null) // identity, если кто-то шарит экран

function initials(name) { return (name || '?').trim()[0]?.toUpperCase() || '?' }

function refresh() {
  if (!room) return
  const list = []
  const add = (p, isLocal) => {
    const cam = p.getTrackPublication(Track.Source.Camera)
    const mic = p.getTrackPublication(Track.Source.Microphone)
    list.push({
      identity: p.identity, name: p.name || 'Гость', isLocal,
      camOn: !!(cam && cam.track && !cam.isMuted),
      micOn: !!(mic && mic.track && !mic.isMuted),
      speaking: p.isSpeaking,
    })
  }
  add(room.localParticipant, true)
  room.remoteParticipants.forEach((p) => add(p, false))
  tiles.value = list
  // экран
  let sharer = null
  const all = [room.localParticipant, ...room.remoteParticipants.values()]
  for (const p of all) {
    const sp = p.getTrackPublication(Track.Source.ScreenShare)
    if (sp && sp.track) { sharer = p.identity; break }
  }
  screenSharer.value = sharer
  nextTick(attachAll)
}

function attachAll() {
  if (!room) return
  const all = [room.localParticipant, ...room.remoteParticipants.values()]
  all.forEach((p) => {
    const tileVid = document.querySelector(`[data-tile="${p.identity}"] video`)
    const cam = p.getTrackPublication(Track.Source.Camera)
    if (tileVid && cam && cam.track && !cam.isMuted) cam.track.attach(tileVid)
    // звук удалённых
    if (!p.isLocal) {
      const aEl = document.querySelector(`[data-audio="${p.identity}"]`)
      const mic = p.getTrackPublication(Track.Source.Microphone)
      if (aEl && mic && mic.track) mic.track.attach(aEl)
    }
    // экран
    if (screenSharer.value === p.identity) {
      const sEl = document.querySelector('[data-screen] video')
      const sp = p.getTrackPublication(Track.Source.ScreenShare)
      if (sEl && sp && sp.track) sp.track.attach(sEl)
    }
  })
}

async function connect() {
  try {
    const { data } = await client.post(`/conferences/${id}/join`)
    canPublish.value = data.can_publish
    mode.value = data.mode
    room = new Room({ adaptiveStream: true, dynacast: true })
    room
      .on(RoomEvent.ParticipantConnected, refresh)
      .on(RoomEvent.ParticipantDisconnected, refresh)
      .on(RoomEvent.TrackSubscribed, refresh)
      .on(RoomEvent.TrackUnsubscribed, refresh)
      .on(RoomEvent.TrackMuted, refresh)
      .on(RoomEvent.TrackUnmuted, refresh)
      .on(RoomEvent.LocalTrackPublished, refresh)
      .on(RoomEvent.LocalTrackUnpublished, refresh)
      .on(RoomEvent.ActiveSpeakersChanged, refresh)
      .on(RoomEvent.Disconnected, () => { if (state.value === 'connected') leave() })
    await room.connect(data.url, data.token)
    state.value = 'connected'
    if (canPublish.value) {
      try {
        await room.localParticipant.setCameraEnabled(true)
        await room.localParticipant.setMicrophoneEnabled(true)
        camOn.value = true; micOn.value = true
      } catch { /* нет доступа к камере/микрофону — можно смотреть */ }
    }
    refresh()
  } catch (e) {
    state.value = 'error'
    errorMsg.value = e.response?.data?.detail || 'Не удалось подключиться к конференции'
  }
}

async function toggleMic() {
  if (!room || !canPublish.value) return
  micOn.value = !micOn.value
  try { await room.localParticipant.setMicrophoneEnabled(micOn.value) } catch { micOn.value = !micOn.value }
  refresh()
}
async function toggleCam() {
  if (!room || !canPublish.value) return
  camOn.value = !camOn.value
  try { await room.localParticipant.setCameraEnabled(camOn.value) } catch { camOn.value = !camOn.value }
  refresh()
}
async function toggleScreen() {
  if (!room || !canPublish.value) return
  screenOn.value = !screenOn.value
  try { await room.localParticipant.setScreenShareEnabled(screenOn.value) } catch { screenOn.value = !screenOn.value }
  refresh()
}
function leave() {
  try { room?.disconnect() } catch { /* ignore */ }
  room = null
  router.push({ name: 'conference' })
}

const gridCols = computed(() => {
  const n = tiles.value.length
  if (n <= 1) return 'grid-cols-1'
  if (n <= 4) return 'grid-cols-2'
  if (n <= 9) return 'grid-cols-3'
  return 'grid-cols-4'
})

onMounted(connect)
onBeforeUnmount(() => { try { room?.disconnect() } catch { /* ignore */ } })
</script>

<template>
  <div class="mx-auto flex h-[calc(100dvh-4rem)] max-w-6xl flex-col -mt-4 -mb-4 sm:-mt-6 sm:-mb-6 lg:-mt-8 lg:-mb-8">
    <div v-if="state === 'connecting'" class="flex flex-1 items-center justify-center text-ink-700/60">
      <div class="text-center">
        <AppIcon name="video" :size="40" class="mx-auto mb-3 animate-pulse text-saffron-500" />
        Подключение к конференции…
      </div>
    </div>
    <div v-else-if="state === 'error'" class="flex flex-1 items-center justify-center">
      <div class="card max-w-md p-8 text-center">
        <p class="text-ink-800">{{ errorMsg }}</p>
        <button class="btn-primary mt-4" @click="leave">К списку конференций</button>
      </div>
    </div>

    <template v-else>
      <!-- шаринг экрана (крупно) -->
      <div v-if="screenSharer" data-screen class="mb-2 flex-1 overflow-hidden rounded-xl bg-ink-900">
        <video autoplay playsinline class="h-full w-full object-contain"></video>
      </div>

      <!-- сетка участников -->
      <div class="grid flex-1 gap-2 overflow-y-auto" :class="[gridCols, screenSharer && 'max-h-40 flex-none']">
        <div v-for="t in tiles" :key="t.identity" :data-tile="t.identity"
             class="relative flex items-center justify-center overflow-hidden rounded-xl bg-ink-900 ring-2 transition"
             :class="t.speaking ? 'ring-saffron-400' : 'ring-transparent'">
          <video autoplay playsinline :muted="t.isLocal" class="h-full w-full object-cover" :class="!t.camOn && 'hidden'"></video>
          <div v-if="!t.camOn" class="flex h-full w-full items-center justify-center">
            <span class="flex h-16 w-16 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-2xl font-semibold text-white">{{ initials(t.name) }}</span>
          </div>
          <div class="absolute bottom-2 left-2 flex items-center gap-1 rounded-md bg-black/50 px-2 py-0.5 text-xs text-white">
            <AppIcon v-if="!t.micOn" name="mic-off" :size="12" class="text-red-400" />
            <span>{{ t.name }}<span v-if="t.isLocal"> (вы)</span></span>
          </div>
        </div>
      </div>

      <!-- аудио удалённых участников -->
      <audio v-for="t in tiles.filter((x) => !x.isLocal)" :key="'a' + t.identity" :data-audio="t.identity" autoplay></audio>

      <!-- панель управления -->
      <div class="mt-2 flex shrink-0 items-center justify-center gap-3 pb-2">
        <template v-if="canPublish">
          <button class="flex h-11 w-11 items-center justify-center rounded-full transition" :class="micOn ? 'bg-parchment-200 text-ink-800 hover:bg-parchment-300' : 'bg-red-500 text-white'" title="Микрофон" @click="toggleMic">
            <AppIcon :name="micOn ? 'volume' : 'mic-off'" :size="20" />
          </button>
          <button class="flex h-11 w-11 items-center justify-center rounded-full transition" :class="camOn ? 'bg-parchment-200 text-ink-800 hover:bg-parchment-300' : 'bg-red-500 text-white'" title="Камера" @click="toggleCam">
            <AppIcon name="video" :size="20" />
          </button>
          <button class="hidden h-11 w-11 items-center justify-center rounded-full transition sm:flex" :class="screenOn ? 'bg-saffron-500 text-white' : 'bg-parchment-200 text-ink-800 hover:bg-parchment-300'" title="Показать экран" @click="toggleScreen">
            <AppIcon name="reports" :size="20" />
          </button>
        </template>
        <span v-else class="text-sm text-ink-700/60">Вы смотрите трансляцию</span>
        <button class="flex h-11 items-center gap-2 rounded-full bg-red-500 px-5 text-white transition hover:bg-red-600" title="Выйти" @click="leave">
          <AppIcon name="logout" :size="18" /> Выйти
        </button>
      </div>
    </template>
  </div>
</template>
