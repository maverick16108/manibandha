<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Room, RoomEvent, Track, DataPacket_Kind } from 'livekit-client'
import client from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Конференция')
const route = useRoute()
const router = useRouter()
const id = route.params.id

const state = ref('connecting')
const errorMsg = ref('')
let room = null
const canPublish = ref(false)
const isHost = ref(false)
const myIdentity = ref('')
const micOn = ref(false)
const camOn = ref(false)
const screenOn = ref(false)
const handUp = ref(false)

const tiles = ref([])          // [{identity, name, isLocal, camOn, micOn, speaking}]
const screenSharer = ref(null)
const raised = reactive({})    // identity -> true
const activeSpeaker = ref(null)
const viewMode = ref('grid')   // grid | speaker
const pinnedId = ref(null)

function initials(name) { return (name || '?').trim()[0]?.toUpperCase() || '?' }

const spotlightId = computed(() => {
  if (screenSharer.value) return null // экран занимает крупный слот
  if (pinnedId.value && tiles.value.some((t) => t.identity === pinnedId.value)) return pinnedId.value
  if (viewMode.value === 'speaker') return activeSpeaker.value || tiles.value[0]?.identity || null
  return null
})
const spotlightTile = computed(() => tiles.value.find((t) => t.identity === spotlightId.value) || null)

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
    const cam = p.getTrackPublication(Track.Source.Camera)
    document.querySelectorAll(`[data-cam="${p.identity}"]`).forEach((el) => {
      if (cam && cam.track && !cam.isMuted) cam.track.attach(el)
    })
    if (!p.isLocal) {
      const aEl = document.querySelector(`[data-audio="${p.identity}"]`)
      const mic = p.getTrackPublication(Track.Source.Microphone)
      if (aEl && mic && mic.track) mic.track.attach(aEl)
    }
    if (screenSharer.value === p.identity) {
      const sEl = document.querySelector('[data-screen] video')
      const sp = p.getTrackPublication(Track.Source.ScreenShare)
      if (sEl && sp && sp.track) sp.track.attach(sEl)
    }
  })
}

function onData(payload, participant) {
  try {
    const msg = JSON.parse(new TextDecoder().decode(payload))
    if (msg.type === 'hand' && participant) {
      if (msg.raised) raised[participant.identity] = true
      else delete raised[participant.identity]
    }
  } catch { /* ignore */ }
}

async function connect() {
  try {
    const { data } = await client.post(`/conferences/${id}/join`)
    canPublish.value = data.can_publish
    isHost.value = data.is_host
    myIdentity.value = data.identity
    room = new Room({ adaptiveStream: true, dynacast: true })
    room
      .on(RoomEvent.ParticipantConnected, refresh)
      .on(RoomEvent.ParticipantDisconnected, (p) => { delete raised[p.identity]; if (pinnedId.value === p.identity) pinnedId.value = null; refresh() })
      .on(RoomEvent.TrackSubscribed, refresh)
      .on(RoomEvent.TrackUnsubscribed, refresh)
      .on(RoomEvent.TrackMuted, refresh)
      .on(RoomEvent.TrackUnmuted, refresh)
      .on(RoomEvent.LocalTrackPublished, refresh)
      .on(RoomEvent.LocalTrackUnpublished, refresh)
      .on(RoomEvent.ActiveSpeakersChanged, (speakers) => { activeSpeaker.value = speakers[0]?.identity || activeSpeaker.value; refresh() })
      .on(RoomEvent.DataReceived, onData)
      .on(RoomEvent.Disconnected, () => { if (state.value === 'connected') leave() })
    await room.connect(data.url, data.token)
    state.value = 'connected'
    if (canPublish.value) {
      try {
        await room.localParticipant.setCameraEnabled(true)
        await room.localParticipant.setMicrophoneEnabled(true)
        camOn.value = true; micOn.value = true
      } catch { /* нет доступа — можно смотреть */ }
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
async function toggleHand() {
  handUp.value = !handUp.value
  if (handUp.value) raised[myIdentity.value] = true; else delete raised[myIdentity.value]
  try {
    const data = new TextEncoder().encode(JSON.stringify({ type: 'hand', raised: handUp.value }))
    await room.localParticipant.publishData(data, { reliable: true })
  } catch { /* ignore */ }
}

// ── модерация (для ведущего) ──
async function moderate(identity, kind, muted) {
  try { await client.post(`/conferences/${id}/mute`, { identity, kind, muted }) } catch (e) { alert(e.response?.data?.detail || 'Не удалось') }
}
function pinTile(identity) { pinnedId.value = pinnedId.value === identity ? null : identity }

function leave() {
  try { room?.disconnect() } catch { /* ignore */ }
  room = null
  router.push({ name: 'conference' })
}

const stripTiles = computed(() => (spotlightId.value || screenSharer.value)
  ? tiles.value.filter((t) => t.identity !== spotlightId.value)
  : tiles.value)
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
      <div class="text-center"><AppIcon name="video" :size="40" class="mx-auto mb-3 animate-pulse text-saffron-500" />Подключение…</div>
    </div>
    <div v-else-if="state === 'error'" class="flex flex-1 items-center justify-center">
      <div class="card max-w-md p-8 text-center"><p class="text-ink-800">{{ errorMsg }}</p><button class="btn-primary mt-4" @click="leave">К списку</button></div>
    </div>

    <template v-else>
      <!-- панель ведущего -->
      <div v-if="isHost" class="mb-2 flex flex-wrap items-center gap-2 text-sm">
        <span class="font-medium text-ink-700/60">Модерация:</span>
        <button class="rounded-md border border-parchment-300 px-2.5 py-1 text-ink-700 hover:bg-parchment-100" @click="moderate('all','audio',true)">Выкл. звук всем</button>
        <button class="rounded-md border border-parchment-300 px-2.5 py-1 text-ink-700 hover:bg-parchment-100" @click="moderate('all','video',true)">Выкл. видео всем</button>
        <div class="ml-auto flex gap-1">
          <button class="rounded-md px-2.5 py-1 transition" :class="viewMode==='grid' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="viewMode='grid'; pinnedId=null">Сетка</button>
          <button class="rounded-md px-2.5 py-1 transition" :class="viewMode==='speaker' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="viewMode='speaker'; pinnedId=null">Активный спикер</button>
        </div>
      </div>
      <div v-else class="mb-2 flex justify-end gap-1 text-sm">
        <button class="rounded-md px-2.5 py-1 transition" :class="viewMode==='grid' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="viewMode='grid'; pinnedId=null">Сетка</button>
        <button class="rounded-md px-2.5 py-1 transition" :class="viewMode==='speaker' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="viewMode='speaker'; pinnedId=null">Активный спикер</button>
      </div>

      <!-- крупно: экран или спотлайт -->
      <div v-if="screenSharer" data-screen class="mb-2 flex-1 overflow-hidden rounded-xl bg-ink-900">
        <video autoplay playsinline class="h-full w-full object-contain"></video>
      </div>
      <div v-else-if="spotlightTile" class="relative mb-2 flex-1 overflow-hidden rounded-xl bg-ink-900"
           :class="spotlightTile.speaking && 'speaking'">
        <video :data-cam="spotlightTile.identity" autoplay playsinline :muted="spotlightTile.isLocal" class="h-full w-full object-cover" :class="!spotlightTile.camOn && 'hidden'"></video>
        <div v-if="!spotlightTile.camOn" class="flex h-full w-full items-center justify-center">
          <span class="flex h-24 w-24 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-3xl font-semibold text-white">{{ initials(spotlightTile.name) }}</span>
        </div>
        <div class="absolute bottom-2 left-2 flex items-center gap-1 rounded-md bg-black/50 px-2 py-1 text-sm text-white">
          <AppIcon v-if="!spotlightTile.micOn" name="mic-off" :size="14" class="text-red-400" />
          <span v-if="raised[spotlightTile.identity]">✋</span>
          <span>{{ spotlightTile.name }}<span v-if="spotlightTile.isLocal"> (вы)</span></span>
        </div>
      </div>

      <!-- сетка / лента миниатюр -->
      <div class="grid gap-2 overflow-y-auto"
           :class="(screenSharer || spotlightTile) ? 'max-h-32 shrink-0 grid-flow-col auto-cols-[9rem]' : ['flex-1', gridCols]">
        <div v-for="t in ((screenSharer || spotlightTile) ? stripTiles : tiles)" :key="t.identity" :data-tile="t.identity"
             class="group relative flex items-center justify-center overflow-hidden rounded-xl bg-ink-900"
             :class="t.speaking && 'speaking'">
          <video :data-cam="t.identity" autoplay playsinline :muted="t.isLocal" class="h-full w-full object-cover" :class="!t.camOn && 'hidden'"></video>
          <div v-if="!t.camOn" class="flex h-full w-full items-center justify-center">
            <span class="flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-xl font-semibold text-white">{{ initials(t.name) }}</span>
          </div>
          <div class="absolute bottom-1.5 left-1.5 flex items-center gap-1 rounded-md bg-black/50 px-1.5 py-0.5 text-xs text-white">
            <AppIcon v-if="!t.micOn" name="mic-off" :size="12" class="text-red-400" />
            <span v-if="raised[t.identity]">✋</span>
            <span class="max-w-[6rem] truncate">{{ t.name }}<span v-if="t.isLocal"> (вы)</span></span>
          </div>
          <!-- действия наведения: закрепить + (для ведущего) выкл. видео/звук -->
          <div class="absolute right-1.5 top-1.5 flex gap-1 opacity-0 transition group-hover:opacity-100">
            <button class="rounded-md bg-black/50 p-1 text-white hover:bg-black/70" :title="pinnedId===t.identity ? 'Открепить' : 'На весь экран'" @click="pinTile(t.identity)"><AppIcon name="pin" :size="14" /></button>
            <template v-if="isHost && !t.isLocal">
              <button class="rounded-md bg-black/50 p-1 text-white hover:bg-black/70" :title="t.micOn ? 'Выкл. звук' : 'Вкл. звук'" @click="moderate(t.identity,'audio',t.micOn)"><AppIcon :name="t.micOn ? 'volume' : 'mic-off'" :size="14" /></button>
              <button class="rounded-md bg-black/50 p-1 text-white hover:bg-black/70" :title="t.camOn ? 'Выкл. видео' : 'Вкл. видео'" @click="moderate(t.identity,'video',t.camOn)"><AppIcon name="video" :size="14" /></button>
            </template>
          </div>
        </div>
      </div>

      <audio v-for="t in tiles.filter((x) => !x.isLocal)" :key="'a' + t.identity" :data-audio="t.identity" autoplay></audio>

      <!-- нижняя панель -->
      <div class="mt-2 flex shrink-0 items-center justify-center gap-3 pb-2">
        <template v-if="canPublish">
          <button class="flex h-11 w-11 items-center justify-center rounded-full transition" :class="micOn ? 'bg-parchment-200 text-ink-800 hover:bg-parchment-300' : 'bg-red-500 text-white'" title="Микрофон" @click="toggleMic"><AppIcon :name="micOn ? 'volume' : 'mic-off'" :size="20" /></button>
          <button class="flex h-11 w-11 items-center justify-center rounded-full transition" :class="camOn ? 'bg-parchment-200 text-ink-800 hover:bg-parchment-300' : 'bg-red-500 text-white'" title="Камера" @click="toggleCam"><AppIcon name="video" :size="20" /></button>
          <button class="hidden h-11 w-11 items-center justify-center rounded-full transition sm:flex" :class="screenOn ? 'bg-saffron-500 text-white' : 'bg-parchment-200 text-ink-800 hover:bg-parchment-300'" title="Показать экран" @click="toggleScreen"><AppIcon name="screen" :size="20" /></button>
        </template>
        <button class="flex h-11 w-11 items-center justify-center rounded-full text-xl transition" :class="handUp ? 'bg-saffron-500 text-white' : 'bg-parchment-200 hover:bg-parchment-300'" title="Поднять руку" @click="toggleHand">✋</button>
        <button class="flex h-11 items-center gap-2 rounded-full bg-red-500 px-5 text-white transition hover:bg-red-600" title="Выйти" @click="leave"><AppIcon name="logout" :size="18" /> Выйти</button>
      </div>
    </template>
  </div>
</template>

<style scoped>
/* ровная заметная рамка говорящего */
.speaking { outline: 3px solid #22c55e; outline-offset: -3px; box-shadow: 0 0 0 1px #22c55e, 0 0 14px rgba(34, 197, 94, 0.5); }
</style>
