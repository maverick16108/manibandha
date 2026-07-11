<script setup>
import { computed } from 'vue'

// data: [{ label, value, color? }]
const props = defineProps({
  data: { type: Array, default: () => [] },
  color: { type: String, default: '#c8742a' }, // default single-series hue
  clickable: { type: Boolean, default: false },
})
const emit = defineEmits(['select'])
const max = computed(() => Math.max(1, ...props.data.map((d) => d.value)))
</script>

<template>
  <ul class="space-y-2.5">
    <li v-for="(d, i) in data" :key="i" class="group flex items-center gap-3"
        :class="clickable && 'cursor-pointer rounded-md px-1 -mx-1 hover:bg-parchment-100'"
        @click="clickable && emit('select', d)">
      <span class="w-32 shrink-0 truncate text-sm text-ink-700 sm:w-36" :title="d.label">{{ d.label }}</span>
      <div class="relative h-6 flex-1 overflow-hidden rounded bg-parchment-100">
        <div class="h-full rounded transition-[width] duration-500"
             :style="{ width: (d.value / max * 100) + '%', background: d.color || color }"
             :title="`${d.label}: ${d.value}`"></div>
      </div>
      <span class="w-9 shrink-0 text-right text-sm font-medium text-ink-900">{{ d.value }}</span>
    </li>
    <li v-if="!data.length" class="text-sm text-ink-700/50">Нет данных</li>
  </ul>
</template>
