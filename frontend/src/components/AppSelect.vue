<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import AppIcon from './AppIcon.vue'

const props = defineProps({
  modelValue: { type: [String, Number, null], default: '' },
  // options: array of { value, label } OR array of strings
  options: { type: Array, default: () => [] },
  placeholder: { type: String, default: 'Выберите…' },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const root = ref(null)
const open = ref(false)
const activeIdx = ref(-1)

const norm = computed(() =>
  props.options.map((o) => (typeof o === 'object' ? o : { value: o, label: String(o) })),
)
const selected = computed(() => norm.value.find((o) => o.value === props.modelValue) || null)
const displayLabel = computed(() => selected.value?.label ?? '')

function toggle() {
  if (props.disabled) return
  open.value = !open.value
  if (open.value) {
    activeIdx.value = Math.max(0, norm.value.findIndex((o) => o.value === props.modelValue))
    nextTick(scrollActive)
  }
}
function close() {
  open.value = false
}
function pick(opt) {
  emit('update:modelValue', opt.value)
  close()
}
function onKeydown(e) {
  if (props.disabled) return
  if (!open.value && (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown')) {
    e.preventDefault()
    toggle()
    return
  }
  if (!open.value) return
  if (e.key === 'Escape') { close(); return }
  if (e.key === 'ArrowDown') { e.preventDefault(); activeIdx.value = Math.min(norm.value.length - 1, activeIdx.value + 1); scrollActive() }
  else if (e.key === 'ArrowUp') { e.preventDefault(); activeIdx.value = Math.max(0, activeIdx.value - 1); scrollActive() }
  else if (e.key === 'Enter') { e.preventDefault(); if (norm.value[activeIdx.value]) pick(norm.value[activeIdx.value]) }
}
const listEl = ref(null)
function scrollActive() {
  nextTick(() => {
    const el = listEl.value?.children[activeIdx.value]
    if (el) el.scrollIntoView({ block: 'nearest' })
  })
}

function onDocClick(e) {
  if (root.value && !root.value.contains(e.target)) close()
}
onMounted(() => document.addEventListener('mousedown', onDocClick))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDocClick))
</script>

<template>
  <div ref="root" class="relative" @keydown="onKeydown">
    <button
      type="button"
      :disabled="disabled"
      class="flex w-full items-center justify-between gap-2 rounded-md border bg-white px-3 py-2 text-left text-sm transition-colors focus:outline-none focus:ring-1 focus:ring-saffron-400"
      :class="[
        open ? 'border-saffron-400 ring-1 ring-saffron-400' : 'border-parchment-300',
        disabled ? 'cursor-not-allowed opacity-60' : 'hover:border-saffron-400/60',
      ]"
      @click="toggle"
    >
      <span :class="displayLabel ? 'text-ink-800' : 'text-ink-700/40'" class="truncate">
        {{ displayLabel || placeholder }}
      </span>
      <AppIcon name="chevron" :size="16" class="shrink-0 text-ink-700/50 transition-transform" :class="open && 'rotate-180'" />
    </button>

    <transition
      enter-active-class="transition duration-100 ease-out"
      enter-from-class="opacity-0 -translate-y-1"
      leave-active-class="transition duration-75 ease-in"
      leave-to-class="opacity-0 -translate-y-1"
    >
      <ul
        v-if="open"
        ref="listEl"
        class="absolute z-40 mt-1 max-h-64 w-full overflow-auto rounded-md border border-parchment-300 bg-white py-1 shadow-lg"
      >
        <li
          v-for="(opt, i) in norm"
          :key="opt.value"
          class="flex cursor-pointer items-center justify-between px-3 py-2 text-sm"
          :class="[
            i === activeIdx ? 'bg-saffron-500/10 text-saffron-700' : 'text-ink-800 hover:bg-parchment-100',
          ]"
          @mouseenter="activeIdx = i"
          @click="pick(opt)"
        >
          <span class="truncate">{{ opt.label }}</span>
          <AppIcon v-if="opt.value === modelValue" name="check" :size="16" class="shrink-0 text-saffron-600" />
        </li>
        <li v-if="!norm.length" class="px-3 py-2 text-sm text-ink-700/40">Нет вариантов</li>
      </ul>
    </transition>
  </div>
</template>
