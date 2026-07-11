<script setup>
import { computed, ref } from 'vue'

// data: [{ label, value, color }]
const props = defineProps({
  data: { type: Array, default: () => [] },
  size: { type: Number, default: 180 },
  thickness: { type: Number, default: 22 },
  centerLabel: { type: String, default: '' },
  clickable: { type: Boolean, default: false },
})
const emit = defineEmits(['select'])

const total = computed(() => props.data.reduce((s, d) => s + d.value, 0))
const r = computed(() => (props.size - props.thickness) / 2)
const c = computed(() => 2 * Math.PI * r.value)
const GAP = 2 // px surface gap between segments

const hover = ref(-1)

const segments = computed(() => {
  let acc = 0
  return props.data.map((d, i) => {
    const frac = total.value ? d.value / total.value : 0
    const len = Math.max(0, frac * c.value - GAP)
    const seg = { ...d, i, dash: `${len} ${c.value - len}`, offset: -acc * c.value, pct: Math.round(frac * 100) }
    acc += frac
    return seg
  })
})
</script>

<template>
  <div class="flex flex-col items-center gap-5 sm:flex-row sm:items-center sm:gap-8">
    <div class="relative shrink-0" :style="{ width: size + 'px', height: size + 'px' }">
      <svg :width="size" :height="size" :viewBox="`0 0 ${size} ${size}`" class="-rotate-90 overflow-visible">
        <circle :cx="size / 2" :cy="size / 2" :r="r" fill="none" stroke="currentColor"
                class="text-parchment-200" :stroke-width="thickness" />
        <circle
          v-for="s in segments" :key="s.i"
          :cx="size / 2" :cy="size / 2" :r="r" fill="none"
          :stroke="s.color" :stroke-width="hover === s.i ? thickness + 3 : thickness"
          :stroke-dasharray="s.dash" :stroke-dashoffset="s.offset" stroke-linecap="butt"
          class="cursor-pointer transition-[stroke-width]"
          @mouseenter="hover = s.i" @mouseleave="hover = -1"
          @click="clickable && emit('select', s)"
        >
          <title>{{ s.label }}: {{ s.value }} ({{ s.pct }}%)</title>
        </circle>
      </svg>
      <div class="absolute inset-0 flex flex-col items-center justify-center">
        <span class="font-display text-3xl font-semibold text-ink-900">{{ total }}</span>
        <span v-if="centerLabel" class="text-xs text-ink-700/60">{{ centerLabel }}</span>
      </div>
    </div>

    <ul class="w-full space-y-2">
      <li v-for="s in segments" :key="s.i"
          class="flex items-center gap-2.5 rounded-md px-2 py-1 text-sm transition-colors"
          :class="[hover === s.i && 'bg-parchment-100', clickable && 'cursor-pointer']"
          @mouseenter="hover = s.i" @mouseleave="hover = -1"
          @click="clickable && emit('select', s)">
        <span class="h-3 w-3 shrink-0 rounded-sm" :style="{ background: s.color }"></span>
        <span class="flex-1 truncate text-ink-700">{{ s.label }}</span>
        <span class="font-medium text-ink-900">{{ s.value }}</span>
        <span class="w-10 text-right text-xs text-ink-700/50">{{ s.pct }}%</span>
      </li>
    </ul>
  </div>
</template>
