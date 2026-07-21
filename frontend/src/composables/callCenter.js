// Глобальный центр звонков (WebRTC 1:1 поверх чат-сокета). Живёт на уровне приложения (монтируется
// компонентом CallCenter.vue в AppLayout), поэтому входящие звонки приходят В ЛЮБОМ разделе, даже
// когда чат не открыт. Раньше вся логика жила в ChatView и работала только при открытом чате.
import { reactive, ref, computed, watch, nextTick } from 'vue'
import client from '../api/client'
import { chatState, sendCallSignal, onCallSignal, startDirect, sendMessageTo } from '../chat/store'
import { showToast } from './toast'

const RTC_FALLBACK = { iceServers: [{ urls: ['stun:stun.l.google.com:19302', 'stun:stun1.l.google.com:19302'] }] }
async function getRtcConfig() {
  try { const { data } = await client.get('/turn-credentials'); if (data?.iceServers?.length) return { iceServers: data.iceServers } } catch { /* fallback */ }
  return RTC_FALLBACK
}

export const call = reactive({ open: false, name: '', avatar: '', peerId: null, status: 'idle', localVideo: false, remoteVideo: false, video: false, fullscreen: false })
export const incoming = reactive({ open: false, name: '', avatar: '', video: false, from: null, offer: null })
export const callRemoteVideo = ref(null)
export const callLocalVideo = ref(null)
export const callStatusText = computed(() => call.status === 'connected' ? 'Соединено' : (call.status === 'calling' ? 'Вызов…' : 'Готов к звонку'))

let pc = null, localStream = null, remoteStream = null, remoteAudioEl = null, pendingIce = []
let makingOffer = false, politePeer = false

// какие звонки (по id звонящего) уже приняты/отклонены на ЛЮБОЙ вкладке — чтобы поздний offer
// (гонка сигналов) не открывал входящий заново, а «догоняющий» handled сработал.
const handledFrom = new Map()
function markHandled(from) { if (from != null) handledFrom.set(String(from), Date.now()) }
function isHandled(from) { const t = handledFrom.get(String(from)); return t != null && (Date.now() - t) < 15000 }

// вкладки одного браузера общаются напрямую — надёжно гасим входящий на других вкладках
let callBC = null
function onBCHandled(from) { markHandled(from); dropIncoming(from) }
try { callBC = new BroadcastChannel('mani-call'); callBC.onmessage = (e) => { const d = e.data; if (d === 'handled') dropIncoming(); else if (d && d.t === 'handled') onBCHandled(d.from) } } catch { /* нет поддержки */ }
function dropIncoming(from) { if (incoming.open && (from == null || String(incoming.from) === String(from))) { incoming.open = false; incoming.offer = null; stopRingtone() } }

export function toggleCallFullscreen() { call.fullscreen = !call.fullscreen }
function attachRemoteVideo() { if (callRemoteVideo.value && remoteStream) { callRemoteVideo.value.srcObject = remoteStream; callRemoteVideo.value.muted = true; callRemoteVideo.value.play?.().catch(() => {}) } }
function attachLocalVideo() { nextTick(() => { if (callLocalVideo.value && localStream) callLocalVideo.value.srcObject = localStream }) }
watch([() => call.status, () => call.remoteVideo], () => nextTick(attachRemoteVideo))
async function ensureLocalStream(wantVideo) {
  if (!localStream) localStream = await navigator.mediaDevices.getUserMedia({ audio: true, video: wantVideo })
  else if (wantVideo && !localStream.getVideoTracks().length) { const vs = await navigator.mediaDevices.getUserMedia({ video: true }); localStream.addTrack(vs.getVideoTracks()[0]) }
  attachLocalVideo()
  return localStream
}

// Исходящий звонок: вызывающий передаёт данные собеседника (определяет их сам — из активного чата).
export function startCall({ peerId, name, avatar, video }) {
  if (!peerId) { showToast('Не удалось определить собеседника'); return }
  call.peerId = peerId; call.name = name || ''; call.avatar = avatar || ''
  call.localVideo = !!video; call.video = !!video; call.remoteVideo = false
  call.status = 'idle-outgoing'; call.open = true
  call.outgoing = true; call.connectedAt = 0; call.eventSent = false; call.placed = false // для сообщения-события в чат
}
// событие о звонке в чат отправляет ВЫЗЫВАЮЩИЙ (чтобы не дублировать): @[call](status|длит.сек|видео)
// status: ok — ответили; cancel — вызывающий отменил до ответа; out — отклонили/не ответили
async function postCallEvent(peerId, status, dur, video) {
  try {
    const chatId = await startDirect(peerId)
    if (chatId) await sendMessageTo(chatId, `@[call](${status}|${dur}|${video ? 1 : 0})`)
  } catch { /* ignore */ }
}
// reasonIfNo — статус, если НЕ ответили (cancel — я отменил; out — отклонили/не ответили)
function postCallSummary(reasonIfNo) {
  if (!call.outgoing || !call.peerId || call.eventSent || !call.placed) return // не начали звонок — не пишем
  call.eventSent = true
  const answered = !!call.connectedAt
  const status = answered ? 'ok' : reasonIfNo
  const dur = answered ? Math.max(1, Math.round((Date.now() - call.connectedAt) / 1000)) : 0
  postCallEvent(call.peerId, status, dur, !!call.video)
}
function setupPc(peerId, cfg, polite) {
  politePeer = !!polite; makingOffer = false
  pc = new RTCPeerConnection(cfg || RTC_FALLBACK)
  pc.onicecandidate = (e) => { if (e.candidate) sendCallSignal({ to: peerId, subtype: 'ice', candidate: e.candidate }) }
  pc.ontrack = (e) => {
    remoteStream = e.streams[0]
    if (!remoteAudioEl) { remoteAudioEl = document.createElement('audio'); remoteAudioEl.autoplay = true; document.body.appendChild(remoteAudioEl) }
    remoteAudioEl.srcObject = remoteStream
    call.remoteVideo = remoteStream.getVideoTracks().length > 0
    nextTick(attachRemoteVideo)
    remoteStream.onaddtrack = remoteStream.onremovetrack = () => { call.remoteVideo = remoteStream.getVideoTracks().length > 0; nextTick(attachRemoteVideo) }
  }
  pc.onconnectionstatechange = () => {
    if (!pc) return
    if (pc.connectionState === 'connected') { call.status = 'connected'; if (!call.connectedAt) call.connectedAt = Date.now() }
    if (['failed', 'closed'].includes(pc.connectionState)) endCall()
  }
}
export async function placeCall() {
  call.status = 'calling'; call.placed = true // звонок реально начат (иначе «Отмена» до звонка не пишет событие)
  try { await ensureLocalStream(call.localVideo) }
  catch { showToast('Нет доступа к микрофону/камере'); endCall(); return }
  const rtcCfg = await getRtcConfig()
  setupPc(call.peerId, rtcCfg, false) // звонящий — «невежливый» (при коллизии не уступает)
  localStream.getTracks().forEach((t) => pc.addTrack(t, localStream))
  const offer = await pc.createOffer()
  await pc.setLocalDescription(offer)
  sendCallSignal({ to: call.peerId, subtype: 'offer', sdp: offer, video: call.localVideo }) // имя добавит сервер (from_name)
}
export async function acceptIncoming() {
  call.peerId = incoming.from; call.name = incoming.name; call.avatar = incoming.avatar
  call.video = incoming.video; call.localVideo = incoming.video; call.remoteVideo = false
  call.outgoing = false; call.connectedAt = 0; call.eventSent = false // принимающий сообщение НЕ шлёт
  const offer = incoming.offer; const callFrom = incoming.from; incoming.open = false
  markHandled(callFrom)
  sendCallSignal({ to: chatState.meId, subtype: 'handled', callFrom }) // погасить звонок на своих других вкладках
  callBC?.postMessage({ t: 'handled', from: callFrom })
  stopRingtone()
  call.open = true; call.status = 'calling'
  try { await ensureLocalStream(call.localVideo) }
  catch { showToast('Нет доступа к микрофону/камере'); endCall(); return }
  const rtcCfg = await getRtcConfig()
  setupPc(call.peerId, rtcCfg, true) // принимающий — «вежливый»
  localStream.getTracks().forEach((t) => pc.addTrack(t, localStream))
  await pc.setRemoteDescription(new RTCSessionDescription(offer))
  for (const c of pendingIce) { try { await pc.addIceCandidate(c) } catch { /* ignore */ } }
  pendingIce = []
  const answer = await pc.createAnswer()
  await pc.setLocalDescription(answer)
  sendCallSignal({ to: call.peerId, subtype: 'answer', sdp: answer })
  call.status = 'connected'
}
export function rejectIncoming() {
  const callFrom = incoming.from
  if (callFrom) sendCallSignal({ to: callFrom, subtype: 'reject' })
  markHandled(callFrom)
  sendCallSignal({ to: chatState.meId, subtype: 'handled', callFrom })
  callBC?.postMessage({ t: 'handled', from: callFrom })
  incoming.open = false; incoming.offer = null; stopRingtone()
}
export function endCall() {
  const to = call.peerId || incoming.from
  if (to && (call.open || incoming.open)) sendCallSignal({ to, subtype: 'end' })
  postCallSummary('cancel') // я завершил: если не ответили — «Отменённый»
  cleanupCall()
}
function cleanupCall() {
  try { pc && pc.close() } catch { /* ignore */ }
  pc = null; pendingIce = []; remoteStream = null; makingOffer = false; politePeer = false
  if (localStream) { localStream.getTracks().forEach((t) => t.stop()); localStream = null }
  if (remoteAudioEl) { try { remoteAudioEl.srcObject = null; remoteAudioEl.remove() } catch { /* ignore */ } remoteAudioEl = null }
  call.open = false; call.status = 'idle'; call.remoteVideo = false; call.localVideo = false; call.peerId = null; call.fullscreen = false
  incoming.open = false; incoming.offer = null
  stopRingtone()
}
async function renegotiate() {
  if (!pc) return
  try { makingOffer = true; const offer = await pc.createOffer(); await pc.setLocalDescription(offer); sendCallSignal({ to: call.peerId, subtype: 'offer', sdp: offer, renego: true }) }
  catch { /* ignore */ } finally { makingOffer = false }
}
export async function toggleCallVideo() {
  const want = !call.localVideo
  try { await ensureLocalStream(want) } catch { showToast('Нет доступа к камере'); return }
  call.localVideo = want
  const vt = localStream.getVideoTracks()[0]
  if (vt) vt.enabled = want
  if (pc && call.status === 'connected') {
    const sender = pc.getSenders().find((s) => s.track && s.track.kind === 'video')
    if (want && !sender && vt) { pc.addTrack(vt, localStream); await renegotiate() }
    else if (sender && sender.track) sender.track.enabled = want
  }
}
async function handleCallSignal(evt) {
  const sub = evt.subtype
  if (sub === 'offer') {
    if (evt.renego && pc) {
      const collision = makingOffer || pc.signalingState !== 'stable'
      if (!politePeer && collision) return
      try {
        if (collision) await Promise.all([pc.setLocalDescription({ type: 'rollback' }).catch(() => {}), pc.setRemoteDescription(new RTCSessionDescription(evt.sdp))])
        else await pc.setRemoteDescription(new RTCSessionDescription(evt.sdp))
        const answer = await pc.createAnswer(); await pc.setLocalDescription(answer)
        sendCallSignal({ to: evt.from, subtype: 'answer', sdp: answer })
      } catch { /* ignore */ }
      return
    }
    if (isHandled(evt.from)) return
    if (call.status === 'connected' || call.status === 'calling' || incoming.open) { sendCallSignal({ to: evt.from, subtype: 'busy' }); return }
    incoming.open = true; incoming.from = evt.from; incoming.name = evt.name || evt.from_name || 'Вызов'
    incoming.avatar = evt.from_avatar || ''; incoming.video = !!evt.video; incoming.offer = evt.sdp
  } else if (sub === 'answer') {
    if (pc && pc.signalingState === 'have-local-offer') { await pc.setRemoteDescription(new RTCSessionDescription(evt.sdp)); if (call.status !== 'connected') call.status = 'connected'; if (!call.connectedAt) call.connectedAt = Date.now() }
    for (const c of pendingIce) { try { await pc.addIceCandidate(c) } catch { /* ignore */ } }
    pendingIce = []
  } else if (sub === 'ice') {
    const cand = new RTCIceCandidate(evt.candidate)
    if (pc && pc.remoteDescription) { try { await pc.addIceCandidate(cand) } catch { /* ignore */ } } else pendingIce.push(cand)
  } else if (sub === 'handled') {
    markHandled(evt.callFrom)
    dropIncoming(evt.callFrom)
  } else if (sub === 'end' || sub === 'reject' || sub === 'busy') {
    if (sub === 'busy') showToast('Абонент занят')
    else if (sub === 'reject') showToast('Звонок отклонён')
    postCallSummary('out') // собеседник завершил/отклонил: если не ответили — «Исходящий» (недозвон)
    cleanupCall()
  }
}

// ── рингтон (Web Audio, без файлов) ─────────────────────────────────────────
let ringCtx = null, ringTimer = null
function startRingtone(isIncoming) {
  stopRingtone()
  try {
    ringCtx = new (window.AudioContext || window.webkitAudioContext)()
    if (ringCtx.state === 'suspended') ringCtx.resume().catch(() => {})
    const tone = (offset, freq, dur, vol) => {
      if (!ringCtx) return
      const o = ringCtx.createOscillator(), g = ringCtx.createGain()
      o.type = 'sine'; o.frequency.value = freq
      const t = ringCtx.currentTime + offset
      g.gain.setValueAtTime(0.0001, t)
      g.gain.exponentialRampToValueAtTime(vol, t + 0.03)
      g.gain.setValueAtTime(vol, t + dur - 0.05)
      g.gain.exponentialRampToValueAtTime(0.0001, t + dur)
      o.connect(g); g.connect(ringCtx.destination); o.start(t); o.stop(t + dur + 0.05)
    }
    const beep = () => {
      if (isIncoming) { tone(0, 540, 0.4, 0.22); tone(0.55, 680, 0.4, 0.22) }
      else { tone(0, 440, 0.45, 0.2); tone(0.6, 480, 0.45, 0.2) }
    }
    beep(); ringTimer = setInterval(beep, isIncoming ? 2400 : 3000)
  } catch { /* аудио недоступно */ }
}
function stopRingtone() { if (ringTimer) { clearInterval(ringTimer); ringTimer = null } if (ringCtx) { try { ringCtx.close() } catch { /* ignore */ } ringCtx = null } }
watch(() => call.status, (s) => { if (s === 'calling') startRingtone(false); else stopRingtone() })
watch(() => incoming.open, (v) => { v ? startRingtone(true) : stopRingtone() })

onCallSignal(handleCallSignal) // регистрируем ОДИН раз на уровне приложения
