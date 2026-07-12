// Единый аудиоплеер для голосовых сообщений: один общий <audio> + реактивное состояние.
// Управляется из верхней панели (AudioBar), запускается кликом по сообщению.
import { reactive } from 'vue'

const audio = typeof Audio !== 'undefined' ? new Audio() : null

export const player = reactive({
  src: '',
  label: '',
  playing: false,
  currentTime: 0,
  duration: 0,
  volume: 1,
  rate: 1,
  visible: false,
})

const RATES = [1, 1.5, 2]

if (audio) {
  audio.addEventListener('timeupdate', () => { player.currentTime = audio.currentTime || 0 })
  audio.addEventListener('loadedmetadata', () => { player.duration = audio.duration || 0 })
  audio.addEventListener('durationchange', () => { player.duration = audio.duration || 0 })
  audio.addEventListener('play', () => { player.playing = true })
  audio.addEventListener('pause', () => { player.playing = false })
  audio.addEventListener('ended', () => { player.playing = false; player.currentTime = 0 })
}

export function playAudio(src, label = '') {
  if (!audio) return
  if (player.src === src) { togglePlay(); return } // тот же трек — пауза/продолжить
  player.src = src
  player.label = label
  player.visible = true
  audio.src = src
  audio.playbackRate = player.rate
  audio.volume = player.volume
  audio.currentTime = 0
  player.currentTime = 0
  player.duration = 0
  audio.play().catch(() => {})
}

export function togglePlay() {
  if (!audio || !player.src) return
  if (audio.paused) audio.play().catch(() => {}); else audio.pause()
}

export function seek(t) {
  if (!audio) return
  const d = audio.duration || 0
  audio.currentTime = Math.max(0, Math.min(t, d))
  player.currentTime = audio.currentTime
}

export function skip(delta) { if (audio) seek((audio.currentTime || 0) + delta) }

export function setRate(r) { player.rate = r; if (audio) audio.playbackRate = r }
export function cycleRate() {
  const i = RATES.indexOf(player.rate)
  setRate(RATES[(i + 1) % RATES.length])
}

export function setVolume(v) { player.volume = v; if (audio) audio.volume = v }
export function toggleMute() { setVolume(player.volume > 0 ? 0 : 1) }

export function closePlayer() {
  if (audio) { audio.pause(); audio.removeAttribute('src'); audio.load() }
  player.visible = false
  player.src = ''
  player.label = ''
  player.playing = false
  player.currentTime = 0
  player.duration = 0
}
