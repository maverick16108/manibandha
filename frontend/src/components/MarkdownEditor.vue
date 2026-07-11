<script setup>
import { ref, nextTick } from 'vue'
import client from '../api/client'

const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: 'Текст… (поддерживается markdown)' },
  rows: { type: Number, default: 4 },
  submitOnEnter: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'submit'])

const textarea = ref(null)
const fileInput = ref(null)
const uploading = ref(false)
const dragOver = ref(false)

function setValue(v, caret) {
  emit('update:modelValue', v)
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
</script>

<template>
  <div>
    <div class="mb-1.5 flex flex-wrap items-center gap-1">
      <button type="button" class="md-btn font-bold" title="Жирный" @click="wrap('**', '**', 'текст')">B</button>
      <button type="button" class="md-btn italic" title="Курсив" @click="wrap('*', '*', 'текст')">I</button>
      <button type="button" class="md-btn line-through" title="Зачёркнутый" @click="wrap('~~', '~~', 'текст')">S</button>
      <button type="button" class="md-btn font-mono" title="Код" @click="wrap('`', '`', 'код')">&lt;/&gt;</button>
      <button type="button" class="md-btn" title="Список" @click="insert('\n- ')">• Список</button>
      <button type="button" class="md-btn" title="Ссылка" @click="wrap('[', '](https://)', 'текст')">🔗</button>
      <button type="button" class="md-btn" title="Картинка" :disabled="uploading" @click="fileInput.click()">
        {{ uploading ? '…' : '🖼 Фото' }}
      </button>
      <input ref="fileInput" type="file" accept="image/*" multiple class="hidden" @change="uploadFiles($event.target.files)" />
    </div>
    <textarea
      ref="textarea" :value="modelValue" :rows="rows" :placeholder="placeholder"
      class="input w-full resize-y transition-colors"
      :class="dragOver && 'border-saffron-400 ring-1 ring-saffron-400'"
      @input="emit('update:modelValue', $event.target.value)"
      @paste="onPaste" @keydown="onKeydown"
      @dragover.prevent="dragOver = true" @dragleave="dragOver = false" @drop="onDrop"></textarea>
    <p class="mt-1 text-xs text-ink-700/40">Фото можно вставить из буфера (Ctrl+V) или перетащить в поле</p>
  </div>
</template>

<style scoped>
.md-btn {
  @apply rounded-md border border-parchment-300 bg-white px-2 py-1 text-sm text-ink-700 transition-colors hover:bg-parchment-100 disabled:opacity-50;
}
</style>
