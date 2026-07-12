<script setup>
import { ref, nextTick, onMounted, onBeforeUnmount, watch } from 'vue'
import client from '../api/client'
import AppIcon from './AppIcon.vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: 'Текст… (поддерживается markdown)' },
  rows: { type: Number, default: 4 },
  submitOnEnter: { type: Boolean, default: false },
  // печать в любом месте страницы попадает в это поле (для чатов/сообщений)
  typeAnywhere: { type: Boolean, default: false },
  // ключ черновика на сервере (напр. 'thread:12', 'new:question'); '' — не сохранять
  draftScope: { type: String, default: '' },
  // скрыть подсказку про фото (для компактного чата)
  hideHint: { type: Boolean, default: false },
  // доп. класс высоты для поля (напр. 'min-h-[42vh]')
  heightClass: { type: String, default: '' },
  // разрешить запись голосовых сообщений (кнопка микрофона)
  voice: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'submit'])

const textarea = ref(null)
const fileInput = ref(null)
const uploading = ref(false)
const dragOver = ref(false)
const showPreview = ref(false)

import { renderMarkdown } from '../lib/markdown'
const previewHtml = () => renderMarkdown(props.modelValue || '')

// авто-рост поля до половины экрана; выше — пользователь тянет сам (resize-y)
function autoGrow() {
  const el = textarea.value
  if (!el) return
  const max = window.innerHeight * 0.5
  if (el.scrollHeight > el.clientHeight && el.clientHeight < max) {
    el.style.height = Math.min(el.scrollHeight, max) + 'px'
  }
}
onMounted(autoGrow)

// ── черновик на сервере (автосохранение) ──
let draftTimer = null
let draftLoaded = false
async function loadDraft() {
  if (!props.draftScope) { draftLoaded = true; return }
  try {
    const { data } = await client.get(`/drafts/${encodeURIComponent(props.draftScope)}`)
    if (data.body && !props.modelValue) emit('update:modelValue', data.body)
  } catch { /* нет черновика */ }
  draftLoaded = true
}
function scheduleDraftSave() {
  if (!props.draftScope || !draftLoaded) return
  clearTimeout(draftTimer)
  draftTimer = setTimeout(saveDraft, 600)
}
async function saveDraft() {
  if (!props.draftScope) return
  const v = props.modelValue || ''
  try {
    if (v) await client.put(`/drafts/${encodeURIComponent(props.draftScope)}`, { body: v })
    else await client.delete(`/drafts/${encodeURIComponent(props.draftScope)}`)
  } catch { /* игнор */ }
}
onMounted(loadDraft)

// когда поле очистили (напр. после отправки) — вернуть стандартную высоту + сохранить черновик
watch(() => props.modelValue, (v) => {
  const el = textarea.value
  if (el) { if (!v) el.style.height = ''; else nextTick(autoGrow) }
  scheduleDraftSave()
})

// ручное растягивание за верхний хват (тянуть вверх — поле выше)
let resizing = false
let startY = 0
let startH = 0
function onResizeMove(e) {
  if (!resizing) return
  const y = e.touches ? e.touches[0].clientY : e.clientY
  const el = textarea.value
  if (!el) return
  el.style.height = Math.max(64, Math.min(window.innerHeight * 0.85, startH + (startY - y))) + 'px'
  if (e.cancelable) e.preventDefault()
}
function stopResize() {
  resizing = false
  window.removeEventListener('mousemove', onResizeMove)
  window.removeEventListener('mouseup', stopResize)
  window.removeEventListener('touchmove', onResizeMove)
  window.removeEventListener('touchend', stopResize)
}
function startResize(e) {
  const el = textarea.value
  if (!el) return
  resizing = true
  startY = e.touches ? e.touches[0].clientY : e.clientY
  startH = el.offsetHeight
  window.addEventListener('mousemove', onResizeMove)
  window.addEventListener('mouseup', stopResize)
  window.addEventListener('touchmove', onResizeMove, { passive: false })
  window.addEventListener('touchend', stopResize)
}

// печать в любом месте страницы → в это поле
function onDocType(e) {
  if (e.ctrlKey || e.metaKey || e.altKey || showPreview.value) return
  const t = e.target
  const tag = (t.tagName || '').toLowerCase()
  if (tag === 'input' || tag === 'textarea' || tag === 'select' || t.isContentEditable) return
  const el = textarea.value
  if (!el) return
  if (e.key.length === 1 && e.key !== ' ') {
    e.preventDefault()
    emit('update:modelValue', (props.modelValue || '') + e.key)
    nextTick(() => { el.focus(); el.selectionStart = el.selectionEnd = el.value.length; autoGrow() })
  } else if (e.key === 'Backspace' && props.modelValue) {
    e.preventDefault()
    emit('update:modelValue', props.modelValue.slice(0, -1))
    nextTick(() => { el.focus(); el.selectionStart = el.selectionEnd = el.value.length })
  }
}
onMounted(() => { if (props.typeAnywhere) document.addEventListener('keydown', onDocType) })
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onDocType)
  stopResize()
  if (draftTimer) { clearTimeout(draftTimer); saveDraft() }
})

function setValue(v, caret) {
  emit('update:modelValue', v)
  nextTick(autoGrow)
  if (caret != null) nextTick(() => { const el = textarea.value; if (el) { el.focus(); el.selectionStart = el.selectionEnd = caret } })
}
function wrap(before, after = before, placeholder = '') {
  const el = textarea.value
  const s = el?.selectionStart ?? props.modelValue.length
  const e = el?.selectionEnd ?? props.modelValue.length
  const sel = props.modelValue.slice(s, e) || placeholder
  const v = props.modelValue.slice(0, s) + before + sel + after + props.modelValue.slice(e)
  emit('update:modelValue', v)
  nextTick(() => { el.focus(); el.selectionStart = s + before.length; el.selectionEnd = s + before.length + sel.length })
}
function insert(text) {
  const el = textarea.value
  const pos = el?.selectionStart ?? props.modelValue.length
  setValue(props.modelValue.slice(0, pos) + text + props.modelValue.slice(pos), pos + text.length)
}

async function uploadFiles(files) {
  const imgs = Array.from(files).filter((f) => f.type.startsWith('image/'))
  if (!imgs.length) return
  uploading.value = true
  try {
    const fd = new FormData()
    imgs.forEach((f) => fd.append('files', f))
    const { data } = await client.post('/uploads', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    data.urls.forEach((u) => insert(`\n![](${u})\n`))
  } finally {
    uploading.value = false
    if (fileInput.value) fileInput.value.value = ''
  }
}
function onPaste(e) {
  const imgs = Array.from(e.clipboardData?.items || []).filter((i) => i.type.startsWith('image/')).map((i) => i.getAsFile()).filter(Boolean)
  if (imgs.length) { e.preventDefault(); uploadFiles(imgs) }
}
function onDrop(e) {
  dragOver.value = false
  const files = Array.from(e.dataTransfer?.files || []).filter((f) => f.type.startsWith('image/'))
  if (files.length) { e.preventDefault(); uploadFiles(files) }
}
function onKeydown(e) {
  if (props.submitOnEnter && e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); emit('submit') }
}

// ── запись голосовых ──
const recording = ref(false)
const recSeconds = ref(0)
let mediaRecorder = null
let recChunks = []
let recStream = null
let recTimer = null
let recStart = 0
let recCanceled = false

function onRecEscape(e) { if (e.key === 'Escape' && recording.value) { e.preventDefault(); e.stopPropagation(); cancelRec() } }

function fmtRec(s) {
  const m = Math.floor(s / 60)
  return `${m}:${String(s % 60).padStart(2, '0')}`
}
function pickMime() {
  const cands = ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4', 'audio/ogg']
  for (const c of cands) { if (window.MediaRecorder && MediaRecorder.isTypeSupported(c)) return c }
  return ''
}
async function startRec() {
  if (recording.value) return
  if (!navigator.mediaDevices?.getUserMedia || !window.MediaRecorder) { alert('Запись не поддерживается этим браузером'); return }
  try {
    recStream = await navigator.mediaDevices.getUserMedia({ audio: true })
  } catch { alert('Нет доступа к микрофону'); return }
  recChunks = []
  recCanceled = false
  const mime = pickMime()
  mediaRecorder = new MediaRecorder(recStream, mime ? { mimeType: mime } : undefined)
  mediaRecorder.ondataavailable = (e) => { if (e.data && e.data.size) recChunks.push(e.data) }
  mediaRecorder.onstop = onRecStop
  mediaRecorder.start()
  recording.value = true
  recSeconds.value = 0
  recStart = Date.now()
  clearInterval(recTimer)
  // счёт от метки времени — не зависит от числа тиков (никаких «скачков по 2»)
  recTimer = setInterval(() => {
    recSeconds.value = Math.floor((Date.now() - recStart) / 1000)
    if (recSeconds.value >= 300) stopRec()
  }, 250)
  document.addEventListener('keydown', onRecEscape, true) // Esc — отменить запись
}
function cleanupRec() {
  clearInterval(recTimer); recTimer = null
  document.removeEventListener('keydown', onRecEscape, true)
  recording.value = false
  if (recStream) { recStream.getTracks().forEach((t) => t.stop()); recStream = null }
}
async function onRecStop() {
  const mime = mediaRecorder?.mimeType || 'audio/webm'
  cleanupRec()
  if (recCanceled || !recChunks.length) { recChunks = []; return }
  const blob = new Blob(recChunks, { type: mime })
  recChunks = []
  uploading.value = true
  try {
    const ext = mime.includes('mp4') ? 'm4a' : mime.includes('ogg') ? 'ogg' : 'webm'
    const fd = new FormData()
    fd.append('files', blob, `voice.${ext}`)
    const { data } = await client.post('/uploads', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    const url = data.urls?.[0]
    if (url) {
      const base = props.modelValue ? props.modelValue.replace(/\s*$/, '') + '\n' : ''
      emit('update:modelValue', base + `@[audio](${url})`)
      nextTick(() => emit('submit')) // голосовое отправляется сразу
    }
  } finally {
    uploading.value = false
  }
}
function stopRec() { if (mediaRecorder && mediaRecorder.state !== 'inactive') mediaRecorder.stop() }
function cancelRec() { recCanceled = true; stopRec() }
function toggleRec() { if (recording.value) stopRec(); else startRec() }
onBeforeUnmount(() => { if (recording.value) { recCanceled = true; stopRec() } cleanupRec() })
</script>

<template>
  <div>
    <div class="mb-1.5 flex flex-wrap items-center gap-1">
      <button type="button" class="md-btn font-bold" title="Жирный" @click="wrap('**', '**', 'текст')">B</button>
      <button type="button" class="md-btn italic" title="Курсив" @click="wrap('*', '*', 'текст')">I</button>
      <button type="button" class="md-btn line-through" title="Зачёркнутый" @click="wrap('~~', '~~', 'текст')">S</button>
      <button type="button" class="md-btn font-mono" title="Код" @click="wrap('`', '`', 'код')">&lt;/&gt;</button>
      <button type="button" class="md-btn" title="Список" @click="insert('\n- ')">• Список</button>
      <button type="button" class="md-btn" title="Ссылка" @click="wrap('[', '](https://)', 'текст')">
        <AppIcon name="link" :size="16" />
      </button>
      <button type="button" class="md-btn inline-flex items-center gap-1" title="Картинка" :disabled="uploading" @click="fileInput.click()">
        <AppIcon name="image" :size="16" /> {{ uploading ? '…' : 'Фото' }}
      </button>
      <template v-if="voice">
        <button type="button" class="md-btn inline-flex items-center gap-1"
                :class="recording && 'animate-pulse bg-red-500/15 text-red-600 ring-1 ring-red-400'"
                :title="recording ? 'Остановить и отправить' : 'Записать голосовое'"
                :disabled="uploading" @click="toggleRec">
          <AppIcon name="mic" :size="16" />
          <span v-if="recording" class="tabular-nums">{{ fmtRec(recSeconds) }}</span>
        </button>
        <button v-if="recording" type="button" class="md-btn text-ink-700/50" title="Отменить запись" @click="cancelRec">✕</button>
      </template>
      <button type="button" class="md-btn inline-flex items-center gap-1 sm:ml-auto"
              :class="showPreview && 'bg-saffron-500/15 text-saffron-700'"
              title="Предпросмотр" @click="showPreview = !showPreview">
        <AppIcon name="eye" :size="16" /> {{ showPreview ? 'Редактор' : 'Превью' }}
      </button>
      <input ref="fileInput" type="file" accept="image/*" multiple class="hidden" @change="uploadFiles($event.target.files)" />
    </div>
    <div v-if="showPreview"
         class="markdown-body input min-h-[8rem] w-full overflow-auto bg-parchment-50"
         v-html="previewHtml() || '<span class=\'text-ink-700/40\'>Пусто</span>'"></div>
    <div v-show="!showPreview" class="relative">
      <!-- хват сверху: тянуть, чтобы увеличить поле -->
      <div class="absolute left-1/2 top-0 z-10 flex h-4 w-16 -translate-x-1/2 -translate-y-1/2 cursor-ns-resize items-center justify-center rounded-full bg-parchment-100 ring-1 ring-parchment-300 hover:bg-parchment-200"
           title="Потяните, чтобы изменить высоту"
           @mousedown="startResize" @touchstart.prevent="startResize">
        <span class="h-1 w-6 rounded-full bg-ink-700/30"></span>
      </div>
      <textarea
        ref="textarea" :value="modelValue" :rows="rows" :placeholder="placeholder"
        class="input w-full resize-none transition-colors"
        :class="[dragOver && 'border-saffron-400 ring-1 ring-saffron-400', heightClass]"
        @input="emit('update:modelValue', $event.target.value); autoGrow()"
        @paste="onPaste" @keydown="onKeydown"
        @dragover.prevent="dragOver = true" @dragleave="dragOver = false" @drop="onDrop"></textarea>
    </div>
    <p v-if="!showPreview && !hideHint" class="mt-1 text-xs text-ink-700/40">Фото можно вставить из буфера (Ctrl+V) или перетащить в поле</p>
  </div>
</template>

<style scoped>
.md-btn {
  @apply rounded-md border border-parchment-300 bg-white px-2 py-1 text-sm text-ink-700 transition-colors hover:bg-parchment-100 disabled:opacity-50;
}
.markdown-body :deep(a) { text-decoration: underline; }
.markdown-body :deep(ul) { margin: 0.25rem 0; padding-left: 1.1rem; list-style: disc; }
.markdown-body :deep(img) { max-height: 18rem; border-radius: 0.5rem; margin: 0.35rem 0; }
.markdown-body :deep(code) { background: rgba(0,0,0,.06); padding: 0 .25rem; border-radius: .25rem; }
</style>
