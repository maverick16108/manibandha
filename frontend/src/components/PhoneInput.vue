<script setup>
import { computed } from 'vue'

// Stores a single number as E.164 ('+79048042771'), displays it formatted.
// Falls back to free-text passthrough when the value holds several numbers.
const props = defineProps({ modelValue: { type: String, default: '' } })
const emit = defineEmits(['update:modelValue'])

const digits = (s) => (s || '').replace(/\D/g, '')
const isMulti = (v) => /[,;]/.test(v || '')

function toE164(d) {
  let x = d
  if (x.startsWith('8')) x = '7' + x.slice(1)
  if (x && x[0] !== '7') x = '7' + x
  x = x.slice(0, 11)
  return x ? '+' + x : ''
}
function format(v) {
  const d = digits(v)
  if (!d) return ''
  const n = d[0] === '7' || d[0] === '8' ? d.slice(1) : d
  let out = '+7'
  if (n.length) out += ' ' + n.slice(0, 3)
  if (n.length > 3) out += ' ' + n.slice(3, 6)
  if (n.length > 6) out += '-' + n.slice(6, 8)
  if (n.length > 8) out += '-' + n.slice(8, 10)
  return out
}

const display = computed(() => (isMulti(props.modelValue) ? props.modelValue : format(props.modelValue)))

function onInput(e) {
  const v = e.target.value
  if (isMulti(v)) emit('update:modelValue', v)
  else emit('update:modelValue', toE164(digits(v)))
}
</script>

<template>
  <input :value="display" @input="onInput" type="tel" inputmode="tel" class="input" placeholder="+7 900 000-00-00" />
</template>
