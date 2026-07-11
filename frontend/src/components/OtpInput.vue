<script setup>
import { ref, watch, onMounted, nextTick } from 'vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  length: { type: Number, default: 4 },
})
const emit = defineEmits(['update:modelValue', 'complete'])

const boxes = ref([])            // digit per box
const inputs = ref([])           // refs to input elements

function syncFromModel(v) {
  const digits = (v || '').replace(/\D/g, '').slice(0, props.length).split('')
  boxes.value = Array.from({ length: props.length }, (_, i) => digits[i] || '')
}
syncFromModel(props.modelValue)

// Keep boxes in sync when the parent clears/changes the value externally.
watch(
  () => props.modelValue,
  (v) => {
    if (v === boxes.value.join('')) return
    syncFromModel(v)
  },
)

function emitValue() {
  const value = boxes.value.join('')
  emit('update:modelValue', value)
  if (value.length === props.length && !value.includes('')) emit('complete', value)
}

function focusBox(i) {
  nextTick(() => inputs.value[i]?.focus())
}

function onInput(i, e) {
  const raw = e.target.value.replace(/\D/g, '')
  if (!raw) {
    boxes.value[i] = ''
    e.target.value = ''
    emitValue()
    return
  }
  // Take the last typed digit (handles overwrite in a filled box).
  boxes.value[i] = raw[raw.length - 1]
  e.target.value = boxes.value[i]
  emitValue()
  if (i < props.length - 1) focusBox(i + 1)
}

function onKeydown(i, e) {
  if (e.key === 'Backspace') {
    if (boxes.value[i]) {
      boxes.value[i] = ''
      emitValue()
    } else if (i > 0) {
      e.preventDefault()
      boxes.value[i - 1] = ''
      emitValue()
      focusBox(i - 1)
    }
  } else if (e.key === 'ArrowLeft' && i > 0) {
    e.preventDefault()
    focusBox(i - 1)
  } else if (e.key === 'ArrowRight' && i < props.length - 1) {
    e.preventDefault()
    focusBox(i + 1)
  }
}

function onPaste(i, e) {
  e.preventDefault()
  const digits = (e.clipboardData?.getData('text') || '').replace(/\D/g, '')
  if (!digits) return
  for (let k = 0; k < props.length; k++) {
    boxes.value[k] = digits[k] || ''
  }
  emitValue()
  const next = Math.min(digits.length, props.length - 1)
  focusBox(next)
}

onMounted(() => focusBox(0))
</script>

<template>
  <div class="flex justify-center gap-2 sm:gap-3">
    <input
      v-for="(box, i) in boxes"
      :key="i"
      ref="inputs"
      :value="box"
      type="text"
      inputmode="numeric"
      autocomplete="one-time-code"
      maxlength="1"
      class="h-14 w-12 rounded-lg border border-parchment-300 bg-parchment-50 text-center text-2xl font-semibold text-ink-900 transition focus:border-saffron-400 focus:outline-none focus:ring-2 focus:ring-saffron-400/40 sm:w-14"
      @input="onInput(i, $event)"
      @keydown="onKeydown(i, $event)"
      @paste="onPaste(i, $event)"
      @focus="$event.target.select()"
    />
  </div>
</template>
