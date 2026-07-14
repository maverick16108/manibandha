<script setup>
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Room, RoomEvent, Track } from 'livekit-client'
import client from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import ConfTile from '../components/ConfTile.vue'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Конференция')
const route = useRoute()
const router = useRouter()
const id = route.params.id
const isGuest = route.name === 'conference-guest'
const roomParam = route.params.room
const guestName = ref('')

const state = ref('connecting')
const errorMsg = ref('')
let room = null
const canPublish = ref(false)
const isHost = ref(false)
const myIdentity = ref('')
const micOn = ref(false)
const camOn = ref(false)
const micBusy = ref(false)  // идёт включение/выключение — не мигаем иконкой
const camBusy = ref(false)
const screenOn = ref(false)
// что мне сейчас разрешено публиковать (ведущий может забрать право прямо во время встречи)
const myAllowMic = ref(true)
const myAllowCam = ref(true)
const myAllowScreen = ref(true)
const handUp = ref(false)

const tiles = ref([])          // [{identity, name, isLocal, camOn, micOn, speaking}]
const screenSharer = ref(null)
const raised = reactive({})    // identity -> true
const activeSpeaker = ref(null)
const viewMode = ref('grid')   // grid | speaker
const pinnedId = ref(null)

function initials(name) { return (name || '?').trim()[0]?.toUpperCase() || '?' }

// LiveKit отдаёт canPublishSources то строками ("MICROPHONE"), то числами enum (1=CAMERA,2=MIC,3=SCREEN).
// Нормализуем к 'microphone' | 'camera' | 'screen_share', чтобы корректно читать разрешения.
const _NUM_SRC = { 1: 'camera', 2: 'microphone', 3: 'screen_share', 4: 'screen_share_audio' }
function normSrc(s) {
  if (typeof s === 'number') return _NUM_SRC[s] || ''
  return String(s).toLowerCase()
}
function hasSrc(srcs, target) { return (srcs || []).some((s) => normSrc(s) === target) }

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
    const perm = p.permissions
    const srcs = perm?.canPublishSources || []
    const allowAll = !!perm?.canPublish && srcs.length === 0
    list.push({
      identity: p.identity, name: p.name || 'Гость', isLocal,
      camOn: !!(cam && cam.track && !cam.isMuted),
      micOn: !!(mic && mic.track && !mic.isMuted),
      allowAudio: !!perm?.canPublish && (allowAll || hasSrc(srcs, 'microphone')),
      allowVideo: !!perm?.canPublish && (allowAll || hasSrc(srcs, 'camera')),
      speaking: p.isSpeaking,
    })
  }
  add(room.localParticipant, true)
  room.remoteParticipants.forEach((p) => add(p, false))
  tiles.value = list
  // мои актуальные права публикации (ведущий мог их изменить прямо во время встречи)
  if (!isHost.value) {
    const lp = room.localParticipant
    const perm = lp.permissions
    const srcs = perm?.canPublishSources || []
    const allowAll = !!perm?.canPublish && srcs.length === 0
    myAllowMic.value = !!perm?.canPublish && (allowAll || hasSrc(srcs, 'microphone'))
    myAllowCam.value = !!perm?.canPublish && (allowAll || hasSrc(srcs, 'camera'))
    myAllowScreen.value = !!perm?.canPublish && (allowAll || hasSrc(srcs, 'screen_share'))
    // забрали право — гасим локально, чтобы нельзя было включить обратно
    if (!myAllowMic.value && micOn.value && !micBusy.value) { micOn.value = false; room.localParticipant.setMicrophoneEnabled(false).catch(() => {}) }
    if (!myAllowCam.value && camOn.value && !camBusy.value) { camOn.value = false; room.localParticipant.setCameraEnabled(false).catch(() => {}) }
    if (!myAllowScreen.value && screenOn.value) { screenOn.value = false; room.localParticipant.setScreenShareEnabled(false).catch(() => {}) }
  }
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
    } else if (msg.type === 'chat') {
      messages.value.push({ name: participant?.name || 'Гость', text: msg.text })
      if (chatOpen.value) scrollChat(); else unread.value += 1
    }
  } catch { /* ignore */ }
}

async function connect() {
  state.value = 'connecting'
  try {
    const { data } = isGuest
      ? await client.post(`/conferences/guest/${roomParam}`, { name: guestName.value.trim() || 'Гость' })
      : await client.post(`/conferences/${id}/join`)
    canPublish.value = data.can_publish
    isHost.value = data.is_host
    myIdentity.value = data.identity
    allowAll.audio = data.mic_allowed !== false
    allowAll.video = data.cam_allowed !== false
    allowAll.screen = data.screen_allowed !== false
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
      .on(RoomEvent.ParticipantPermissionsChanged, refresh)
      .on(RoomEvent.DataReceived, onData)
      .on(RoomEvent.Disconnected, () => { if (state.value === 'connected') leave() })
    await room.connect(data.url, data.token)
    state.value = 'connected'
    if (canPublish.value) {
      camBusy.value = true; micBusy.value = true
      try { await room.localParticipant.setCameraEnabled(true); camOn.value = true } catch { /* нет доступа */ } finally { camBusy.value = false }
      try { await room.localParticipant.setMicrophoneEnabled(true); micOn.value = true } catch { /* нет доступа */ } finally { micBusy.value = false }
      loadDevices()
      try { navigator.mediaDevices.addEventListener('devicechange', loadDevices) } catch { /* ignore */ }
    }
    refresh()
  } catch (e) {
    state.value = 'error'
    errorMsg.value = e.response?.data?.detail || 'Не удалось подключиться к конференции'
  }
}

async function toggleMic() {
  if (!room || !canPublish.value || micBusy.value) return
  if (!micOn.value && !myAllowMic.value) return // право забрал ведущий — включить нельзя
  const next = !micOn.value
  micBusy.value = true // иконка не меняется, пока идёт переключение
  try { await room.localParticipant.setMicrophoneEnabled(next); micOn.value = next } catch { /* ignore */ } finally { micBusy.value = false }
  refresh()
}
async function toggleCam() {
  if (!room || !canPublish.value || camBusy.value) return
  if (!camOn.value && !myAllowCam.value) return // право забрал ведущий — включить нельзя
  const next = !camOn.value
  camBusy.value = true
  try { await room.localParticipant.setCameraEnabled(next); camOn.value = next } catch { /* ignore */ } finally { camBusy.value = false }
  refresh()
}
async function toggleScreen() {
  if (!room || !canPublish.value) return
  if (!screenOn.value && !myAllowScreen.value) return // право забрал ведущий
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

// ── модерация: разрешить/запретить публикацию ──
async function permit(identity, kind, allow, exceptId) {
  try { await client.post(`/conferences/${id}/permit`, { identity, kind, allow, except: exceptId || null }) }
  catch (e) { alert(e.response?.data?.detail || 'Не удалось') }
}
// состояние сегментных переключателей «всем» (действие + подсветка)
const allowAll = reactive({ audio: true, video: true, screen: true })
async function setAll(kind, allow) {
  allowAll[kind] = allow
  await permit('all', kind, allow)
  refresh()
}
function pinTile(identity) { pinnedId.value = pinnedId.value === identity ? null : identity }

// ── ширина ленты участников справа (тянется мышью; при большой ширине — в 2 столбца) ──
const stripW = ref(180)
// 2 столбца — только если тайлов хватает на второй столбец (иначе один пустует)
const stripTwoCols = computed(() => stripW.value > 300 && stripTiles.value.length >= 2)
function startStripResize(e) {
  const startX = e.touches ? e.touches[0].clientX : e.clientX
  const startW = stripW.value
  const move = (ev) => {
    const x = ev.touches ? ev.touches[0].clientX : ev.clientX
    stripW.value = Math.max(120, Math.min(window.innerWidth * 0.6, startW + (startX - x)))
    if (ev.cancelable) ev.preventDefault()
  }
  const up = () => {
    window.removeEventListener('mousemove', move); window.removeEventListener('mouseup', up)
    window.removeEventListener('touchmove', move); window.removeEventListener('touchend', up)
  }
  window.addEventListener('mousemove', move); window.addEventListener('mouseup', up)
  window.addEventListener('touchmove', move, { passive: false }); window.addEventListener('touchend', up)
}

// ── выбор устройств (несколько микрофонов/камер) ──
const mics = ref([])
const cams = ref([])
const curMic = ref('')
const curCam = ref('')
const micMenu = ref(false)
const camMenu = ref(false)
async function loadDevices() {
  try {
    const devs = await navigator.mediaDevices.enumerateDevices()
    mics.value = devs.filter((d) => d.kind === 'audioinput' && d.deviceId)
    cams.value = devs.filter((d) => d.kind === 'videoinput' && d.deviceId)
    if (room) {
      curMic.value = room.getActiveDevice?.('audioinput') || curMic.value
      curCam.value = room.getActiveDevice?.('videoinput') || curCam.value
    }
  } catch { /* нет доступа к списку устройств */ }
}
async function switchMic(deviceId) {
  micMenu.value = false
  try { await room?.switchActiveDevice('audioinput', deviceId); curMic.value = deviceId } catch { /* ignore */ }
}
async function switchCam(deviceId) {
  camMenu.value = false
  try { await room?.switchActiveDevice('videoinput', deviceId); curCam.value = deviceId } catch { /* ignore */ }
}
function devLabel(d, i, kind) { return d.label || `${kind} ${i + 1}` }

// ── полный экран ──
const rootEl = ref(null)
const isFs = ref(false)
function toggleFullscreen() {
  const el = rootEl.value
  if (!document.fullscreenElement) { el?.requestFullscreen?.().catch(() => {}); isFs.value = true }
  else { document.exitFullscreen?.(); isFs.value = false }
}

// ── чат ──
const chatOpen = ref(false)
const messages = ref([])
const chatInput = ref('')
const unread = ref(0)
function scrollChat() { nextTick(() => { const el = document.getElementById('conf-chat-scroll'); if (el) el.scrollTop = el.scrollHeight }) }
function sendChat() {
  const text = chatInput.value.trim()
  if (!text || !room) return
  try { room.localParticipant.publishData(new TextEncoder().encode(JSON.stringify({ type: 'chat', text })), { reliable: true }) } catch { /* ignore */ }
  messages.value.push({ name: 'Вы', text, self: true })
  chatInput.value = ''
  scrollChat()
}
function openChat() {
  chatOpen.value = true; unread.value = 0; scrollChat()
  nextTick(() => document.getElementById('conf-chat-input')?.focus()) // сразу фокус в поле ввода
}
// ссылки в сообщениях чата делаем кликабельными (с экранированием HTML)
function linkify(text) {
  const esc = String(text || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
  return esc.replace(/(https?:\/\/[^\s]+)/g, (u) => `<a href="${u}" target="_blank" rel="noopener noreferrer" class="underline underline-offset-2">${u}</a>`)
}

// переприкрепить видео при смене раскладки (иначе своё видео пропадает)
watch([viewMode, pinnedId, screenSharer, () => tiles.value.length], () => nextTick(attachAll))

function leave() {
  try { room?.disconnect() } catch { /* ignore */ }
  room = null
  router.push(isGuest ? '/' : { name: 'conference' })
}

// Esc — закрыть меню выбора устройства / чат; пробел — вкл/выкл микрофон (если не печатаем)
function onKey(e) {
  if (e.key === 'Escape' && (micMenu.value || camMenu.value)) { micMenu.value = false; camMenu.value = false; return }
  if (e.key === 'Escape' && chatOpen.value) { chatOpen.value = false; return }
  if (e.code !== 'Space' && e.key !== ' ') return
  const el = e.target
  if (el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable)) return
  if (state.value !== 'connected' || !canPublish.value) return
  e.preventDefault()
  toggleMic()
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

onMounted(() => {
  if (isGuest) state.value = 'prejoin'; else connect()
  document.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKey)
  try { navigator.mediaDevices.removeEventListener('devicechange', loadDevices) } catch { /* ignore */ }
  try { room?.disconnect() } catch { /* ignore */ }
})
</script>

<template>
  <div ref="rootEl" class="relative flex flex-col bg-parchment-100"
       :class="isGuest ? 'h-dvh p-3 sm:p-4' : 'mx-auto h-[calc(100dvh-4rem)] max-w-6xl -mt-4 -mb-4 sm:-mt-6 sm:-mb-6 lg:-mt-8 lg:-mb-8'">
    <!-- гость: ввод имени -->
    <div v-if="state === 'prejoin'" class="flex flex-1 items-center justify-center">
      <div class="card w-full max-w-sm p-6 text-center">
        <AppIcon name="video" :size="36" class="mx-auto mb-3 text-saffron-500" />
        <h2 class="font-display text-xl font-semibold text-ink-900">Вход в конференцию</h2>
        <p class="mb-4 mt-1 text-sm text-ink-700/60">Как вас представить?</p>
        <input v-model="guestName" class="input" placeholder="Ваше имя" @keydown.enter="connect" />
        <button class="btn-primary mt-4 w-full" @click="connect">Войти</button>
      </div>
    </div>
    <div v-else-if="state === 'connecting'" class="flex flex-1 items-center justify-center text-ink-700/60">
      <div class="text-center"><AppIcon name="video" :size="40" class="mx-auto mb-3 animate-pulse text-saffron-500" />Подключение…</div>
    </div>
    <div v-else-if="state === 'error'" class="flex flex-1 items-center justify-center">
      <div class="card max-w-md p-8 text-center"><p class="text-ink-800">{{ errorMsg }}</p><button class="btn-primary mt-4" @click="leave">К списку</button></div>
    </div>

    <template v-else>
      <!-- панель ведущего -->
      <div v-if="isHost" class="mb-2 flex flex-wrap items-center gap-x-4 gap-y-2 pt-3 text-sm">
        <span class="inline-flex items-center gap-2">
          <AppIcon name="volume" :size="15" class="text-ink-700/50" />
          <span class="text-ink-700/60">Звук всем</span>
          <button class="switch" :class="allowAll.audio && 'is-on'" role="switch" :aria-checked="allowAll.audio" title="Звук всем" @click="setAll('audio', !allowAll.audio)"><span class="switch-knob"></span></button>
        </span>
        <span class="inline-flex items-center gap-2">
          <AppIcon name="video" :size="15" class="text-ink-700/50" />
          <span class="text-ink-700/60">Видео всем</span>
          <button class="switch" :class="allowAll.video && 'is-on'" role="switch" :aria-checked="allowAll.video" title="Видео всем" @click="setAll('video', !allowAll.video)"><span class="switch-knob"></span></button>
        </span>
        <span class="inline-flex items-center gap-2">
          <AppIcon name="screen" :size="15" class="text-ink-700/50" />
          <span class="text-ink-700/60">Экран всем</span>
          <button class="switch" :class="allowAll.screen && 'is-on'" role="switch" :aria-checked="allowAll.screen" title="Экран всем" @click="setAll('screen', !allowAll.screen)"><span class="switch-knob"></span></button>
        </span>
      </div>

      <!-- крупный слот (экран/спикер) + лента участников справа с вертикальной прокруткой -->
      <div v-if="screenSharer || spotlightTile" class="mb-2 flex min-h-0 flex-1 gap-2 pt-2">
        <!-- крупно -->
        <div v-if="screenSharer" data-screen class="min-w-0 flex-1 overflow-hidden rounded-xl bg-ink-900">
          <video autoplay playsinline class="h-full w-full object-contain"></video>
        </div>
        <div v-else class="relative min-w-0 flex-1 overflow-hidden rounded-xl bg-ink-900" :class="spotlightTile.speaking && 'speaking'">
          <video :data-cam="spotlightTile.identity" autoplay playsinline :muted="spotlightTile.isLocal" class="h-full w-full object-cover" :class="!spotlightTile.camOn && 'hidden'"></video>
          <div v-if="!spotlightTile.camOn" class="flex h-full w-full items-center justify-center">
            <span class="flex h-24 w-24 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-3xl font-semibold text-white">{{ initials(spotlightTile.name) }}</span>
          </div>
          <div class="absolute bottom-2 left-2 flex items-center gap-1.5 rounded-md bg-black/50 px-2 py-1 text-sm text-white">
            <AppIcon v-if="!spotlightTile.micOn" name="mic-off" :size="18" class="text-red-400" />
            <span v-if="raised[spotlightTile.identity]" class="text-2xl leading-none">✋</span>
            <span>{{ spotlightTile.name }}<span v-if="spotlightTile.isLocal"> (вы)</span></span>
          </div>
        </div>
        <!-- ручка изменения ширины ленты -->
        <div v-if="stripTiles.length" class="w-1.5 shrink-0 cursor-col-resize rounded bg-parchment-300 transition hover:bg-saffron-400" title="Потяните, чтобы изменить ширину" @mousedown.prevent="startStripResize" @touchstart.prevent="startStripResize"></div>
        <!-- лента участников справа: тянется по ширине; при большой ширине — в 2 столбца -->
        <div v-if="stripTiles.length" class="shrink-0 gap-2 overflow-y-auto pr-0.5" :style="{ width: stripW + 'px' }"
             :class="stripTwoCols ? 'grid grid-cols-2 content-start' : 'flex flex-col'">
          <ConfTile v-for="t in stripTiles" :key="t.identity" :t="t" :raised="raised" :pinned-id="pinnedId" :is-host="isHost"
                    class="aspect-video shrink-0" @pin="pinTile" @permit="permit" />
        </div>
      </div>

      <!-- обычная сетка (нет крупного слота) -->
      <div v-else class="grid flex-1 gap-2 overflow-y-auto pt-2" :class="gridCols">
        <ConfTile v-for="t in tiles" :key="t.identity" :t="t" :raised="raised" :pinned-id="pinnedId" :is-host="isHost"
                  @pin="pinTile" @permit="permit" />
      </div>

      <audio v-for="t in tiles.filter((x) => !x.isLocal)" :key="'a' + t.identity" :data-audio="t.identity" autoplay></audio>

      <!-- клик вне меню выбора устройства — закрыть -->
      <div v-if="micMenu || camMenu" class="fixed inset-0 z-30" @click="micMenu = false; camMenu = false"></div>

      <!-- нижняя панель -->
      <div class="mt-2 flex shrink-0 items-center justify-center gap-3 pb-2">
        <template v-if="canPublish">
          <!-- микрофон + выбор устройства (скрыт, если ведущий забрал право на звук) -->
          <div v-if="myAllowMic" class="relative flex items-center">
            <button class="flex h-11 items-center justify-center transition" :class="[(micOn || micBusy) ? 'bg-parchment-200 text-ink-800 hover:bg-parchment-300' : 'bg-red-500 text-white', micBusy && 'animate-pulse', mics.length > 1 ? 'rounded-l-full pl-4 pr-3' : 'w-11 rounded-full']" title="Микрофон" @click="toggleMic"><AppIcon :name="(micOn || micBusy) ? 'volume' : 'mic-off'" :size="20" /></button>
            <button v-if="mics.length > 1" class="flex h-11 items-center justify-center rounded-r-full border-l border-parchment-50/60 pl-1.5 pr-2.5 transition" :class="(micOn || micBusy) ? 'bg-parchment-200 text-ink-800 hover:bg-parchment-300' : 'bg-red-500 text-white'" title="Выбрать микрофон" @click="micMenu = !micMenu; camMenu = false"><AppIcon name="chevron" :size="14" class="-rotate-90" /></button>
            <div v-if="micMenu" class="absolute bottom-14 left-0 z-40 min-w-[13rem] max-w-[16rem] overflow-hidden rounded-xl border border-parchment-200 bg-white py-1 shadow-xl">
              <button v-for="(d, di) in mics" :key="d.deviceId" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-parchment-100" :class="curMic === d.deviceId ? 'font-semibold text-saffron-700' : 'text-ink-800'" @click="switchMic(d.deviceId)">
                <AppIcon name="volume" :size="14" class="shrink-0 opacity-60" /><span class="truncate">{{ devLabel(d, di, 'Микрофон') }}</span>
              </button>
            </div>
          </div>
          <!-- камера + выбор устройства (скрыта, если ведущий забрал право на видео) -->
          <div v-if="myAllowCam" class="relative flex items-center">
            <button class="flex h-11 items-center justify-center transition" :class="[(camOn || camBusy) ? 'bg-parchment-200 text-ink-800 hover:bg-parchment-300' : 'bg-red-500 text-white', camBusy && 'animate-pulse', cams.length > 1 ? 'rounded-l-full pl-4 pr-3' : 'w-11 rounded-full']" title="Камера" @click="toggleCam"><AppIcon name="video" :size="20" /></button>
            <button v-if="cams.length > 1" class="flex h-11 items-center justify-center rounded-r-full border-l border-parchment-50/60 pl-1.5 pr-2.5 transition" :class="(camOn || camBusy) ? 'bg-parchment-200 text-ink-800 hover:bg-parchment-300' : 'bg-red-500 text-white'" title="Выбрать камеру" @click="camMenu = !camMenu; micMenu = false"><AppIcon name="chevron" :size="14" class="-rotate-90" /></button>
            <div v-if="camMenu" class="absolute bottom-14 left-0 z-40 min-w-[13rem] max-w-[16rem] overflow-hidden rounded-xl border border-parchment-200 bg-white py-1 shadow-xl">
              <button v-for="(d, di) in cams" :key="d.deviceId" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-parchment-100" :class="curCam === d.deviceId ? 'font-semibold text-saffron-700' : 'text-ink-800'" @click="switchCam(d.deviceId)">
                <AppIcon name="video" :size="14" class="shrink-0 opacity-60" /><span class="truncate">{{ devLabel(d, di, 'Камера') }}</span>
              </button>
            </div>
          </div>
          <button v-if="myAllowScreen" class="hidden h-11 w-11 items-center justify-center rounded-full transition sm:flex" :class="screenOn ? 'bg-saffron-500 text-white' : 'bg-parchment-200 text-ink-800 hover:bg-parchment-300'" title="Показать экран" @click="toggleScreen"><AppIcon name="screen" :size="20" /></button>
        </template>
        <!-- переключение раскладки: сетка / активный спикер (для всех) -->
        <button class="flex h-11 w-11 items-center justify-center rounded-full transition" :class="viewMode==='speaker' ? 'bg-saffron-500 text-white' : 'bg-parchment-200 text-ink-800 hover:bg-parchment-300'" :title="viewMode==='grid' ? 'Активный спикер' : 'Сетка'" @click="viewMode = viewMode==='grid' ? 'speaker' : 'grid'; pinnedId=null"><AppIcon :name="viewMode==='grid' ? 'user' : 'grid'" :size="20" /></button>
        <button class="flex h-11 w-11 items-center justify-center rounded-full text-xl transition" :class="handUp ? 'bg-saffron-500 text-white' : 'bg-parchment-200 hover:bg-parchment-300'" title="Поднять руку" @click="toggleHand">✋</button>
        <button class="relative flex h-11 w-11 items-center justify-center rounded-full bg-parchment-200 text-ink-800 transition hover:bg-parchment-300" title="Чат" @click="chatOpen ? (chatOpen=false) : openChat()">
          <AppIcon name="chat" :size="20" />
          <span v-if="unread" class="absolute -right-1 -top-1 flex h-5 min-w-[1.25rem] items-center justify-center rounded-full bg-red-500 px-1 text-xs font-semibold text-white">{{ unread }}</span>
        </button>
        <button class="hidden h-11 w-11 items-center justify-center rounded-full bg-parchment-200 text-ink-800 transition hover:bg-parchment-300 sm:flex" title="Во весь экран" @click="toggleFullscreen"><AppIcon name="expand" :size="20" /></button>
        <button class="flex h-11 items-center gap-2 rounded-full bg-red-500 px-5 text-white transition hover:bg-red-600" title="Выйти" @click="leave"><AppIcon name="logout" :size="18" /> Выйти</button>
      </div>

      <!-- чат -->
      <div v-if="chatOpen" class="absolute inset-y-0 right-0 z-30 flex w-full flex-col border-l border-parchment-200 bg-white shadow-xl sm:w-80">
        <div class="flex items-center justify-between border-b border-parchment-200 px-4 py-3">
          <span class="font-medium text-ink-900">Чат конференции</span>
          <button class="rounded-full p-1 text-ink-700/50 hover:bg-parchment-100" @click="chatOpen=false"><AppIcon name="close" :size="18" /></button>
        </div>
        <div id="conf-chat-scroll" class="flex-1 space-y-2 overflow-y-auto p-4">
          <div v-if="!messages.length" class="text-center text-sm text-ink-700/50">Сообщений пока нет</div>
          <div v-for="(m, mi) in messages" :key="mi" class="flex flex-col" :class="m.self ? 'items-end' : 'items-start'">
            <span class="text-xs text-ink-700/50">{{ m.name }}</span>
            <span class="max-w-[85%] whitespace-pre-wrap break-words rounded-2xl px-3 py-1.5 text-sm" :class="m.self ? 'bg-saffron-500 text-white' : 'bg-parchment-100 text-ink-800'" v-html="linkify(m.text)"></span>
          </div>
        </div>
        <div class="flex items-center gap-2 border-t border-parchment-200 p-3">
          <input id="conf-chat-input" v-model="chatInput" class="input flex-1" placeholder="Сообщение…" @keydown.enter="sendChat" />
          <button class="btn-primary" :disabled="!chatInput.trim()" @click="sendChat">→</button>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
/* ровная заметная рамка говорящего */
.speaking { outline: 3px solid #22c55e; outline-offset: -3px; box-shadow: 0 0 0 1px #22c55e, 0 0 14px rgba(34, 197, 94, 0.5); }

/* обычный переключатель вкл/выкл (в фирменных тонах) */
.switch { position: relative; flex: none; width: 42px; height: 24px; border-radius: 9999px; background: #d8cbb8; transition: background .18s ease; box-shadow: inset 0 1px 2px rgba(0,0,0,.08); }
.switch.is-on { background: #c8742a; }
.switch-knob { position: absolute; top: 3px; left: 3px; width: 18px; height: 18px; border-radius: 9999px; background: #fff; box-shadow: 0 1px 2px rgba(0,0,0,.28); transition: transform .18s ease; }
.switch.is-on .switch-knob { transform: translateX(18px); }
</style>
