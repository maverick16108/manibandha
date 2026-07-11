<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import AppIcon from './AppIcon.vue'

const props = defineProps({
  modelValue: { type: [String, Number, null], default: '' },
  options: { type: Array, default: () => [] }, // [{value,label}] or [string]
  placeholder: { type: String, default: 'Выберите…' },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const root = ref(null)
const open = ref(false)
const activeIdx = ref(-1)
const search = ref('')
const searchEl = ref(null)
const listEl = ref(null)

const norm = computed(() =>
  props.options.map((o) => (typeof o === 'object' ? o : { value: o, label: String(o) })),
)
const showSearch = computed(() => norm.value.length > 7)
const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return q ? norm.value.filter((o) => o.label.toLowerCase().includes(q)) : norm.value
})
const selected = computed(() => norm.value.find((o) => o.value === props.modelValue) || null)
const displayLabel = computed(() => selected.value?.label ?? '')

watch(filtered, () => { activeIdx.value = filtered.value.length ? 0 : -1 })

function toggle() {
  if (props.disabled) return
  open.value = !open.value
  if (open.value) {
    search.value = ''
    activeIdx.value = Math.max(0, filtered.value.findIndex((o) => o.value === props.modelValue))
    nextTick(() => { if (showSearch.value) searchEl.value?.focus(); scrollActive() })
  }
}
function close() { open.value = false }
function pick(opt) { emit('update:modelValue', opt.value); close() }

function onKeydown(e) {
  if (props.disabled) return
  if (!open.value && (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown')) {
    e.preventDefault(); toggle(); return
  }
  if (!open.value) return
  if (e.key === 'Escape') { e.preventDefault(); close() }
  else if (e.key === 'ArrowDown') { e.preventDefault(); activeIdx.value = Math.min(filtered.value.length - 1, activeIdx.value + 1); scrollActive() }
  else if (e.key === 'ArrowUp') { e.preventDefault(); activeIdx.value = Math.max(0, activeIdx.value - 1); scrollActive() }
  else if (e.key === 'Enter') { e.preventDefault(); if (filtered.value[activeIdx.value]) pick(filtered.value[activeIdx.value]) }
}
function scrollActive() {
  nextTick(() => listEl.value?.children[activeIdx.value]?.scrollIntoView({ block: 'nearest' }))
}

function onDocClick(e) { if (root.value && !root.value.contains(e.target)) close() }
onMounted(() => document.addEventListener('mousedown', onDocClick))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDocClick))
</script>

<template>
  <div ref="root" class="relative" @keydown="onKeydown">
    <button
      type="button" :disabled="disabled"
      class="flex w-full items-center justify-between gap-2 rounded-md border bg-white px-3 py-2 text-left text-sm transition-colors focus:outline-none focus:ring-1 focus:ring-saffron-400"
      :class="[
        open ? 'border-saffron-400 ring-1 ring-saffron-400' : 'border-parchment-300',
        disabled ? 'cursor-not-allowed opacity-60' : 'hover:border-saffron-400/60',
      ]"
      @click="toggle"
    >
      <span :class="displayLabel ? 'text-ink-800' : 'text-ink-700/40'" class="truncate">{{ displayLabel || placeholder }}</span>
      <AppIcon name="chevron" :size="16" class="shrink-0 text-ink-700/50 transition-transform" :class="open && 'rotate-180'" />
    </button>

    <transition
      enter-active-class="transition duration-100 ease-out" enter-from-class="opacity-0 -translate-y-1"
      leave-active-class="transition duration-75 ease-in" leave-to-class="opacity-0 -translate-y-1"
    >
      <div v-if="open" class="absolute z-40 mt-1 w-full overflow-hidden rounded-md border border-parchment-300 bg-white shadow-lg">
        <div v-if="showSearch" class="border-b border-parchment-100 p-1.5">
          <div class="relative">
            <AppIcon name="search" :size="14" class="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-700/40" />
            <input ref="searchEl" v-model="search" type="text" placeholder="Поиск…"
                   class="w-full rounded border border-parchment-200 bg-parchment-50 py-1.5 pl-8 pr-2 text-sm focus:border-saffron-400 focus:outline-none" />
          </div>
        </div>
        <ul ref="listEl" class="max-h-60 overflow-auto py-1">
          <li
            v-for="(opt, i) in filtered" :key="opt.value"
            class="flex cursor-pointer items-center justify-between px-3 py-2 text-sm"
            :class="i === activeIdx ? 'bg-saffron-500/10 text-saffron-700' : 'text-ink-800 hover:bg-parchment-100'"
            @mouseenter="activeIdx = i" @click="pick(opt)"
          >
            <span class="truncate">{{ opt.label }}</span>
            <AppIcon v-if="opt.value === modelValue" name="check" :size="16" class="shrink-0 text-saffron-600" />
          </li>
          <li v-if="!filtered.length" class="px-3 py-2 text-sm text-ink-700/40">Ничего не найдено</li>
        </ul>
      </div>
    </transition>
  </div>
</template>
