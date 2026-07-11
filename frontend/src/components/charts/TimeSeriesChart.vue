<script setup>
import { computed } from 'vue'

// data: [{ period: 'YYYY-MM', pranama, harinama, brahman }]
const props = defineProps({ data: { type: Array, default: () => [] } })

const SERIES = [
  { key: 'pranama', label: 'Пранама', color: '#c8742a' },
  { key: 'harinama', label: 'Харинама', color: '#a4551b' },
  { key: 'brahman', label: 'Брахман', color: '#6d3f16' },
]
const MONTHS = ['', 'янв', 'фев', 'мар', 'апр', 'май', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек']

const max = computed(() => Math.max(1, ...props.data.flatMap((d) => SERIES.map((s) => d[s.key] || 0))))
function label(period) {
  const [y, m] = period.split('-')
  return `${MONTHS[+m]} ${y.slice(2)}`
}
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap gap-4">
      <span v-for="s in SERIES" :key="s.key" class="flex items-center gap-1.5 text-sm text-ink-700">
        <span class="h-3 w-3 rounded-sm" :style="{ background: s.color }"></span>{{ s.label }}
      </span>
    </div>

    <div v-if="data.length" class="flex items-end gap-3 overflow-x-auto pb-2">
      <div v-for="d in data" :key="d.period" class="flex shrink-0 flex-col items-center gap-1.5">
        <div class="flex h-40 items-end gap-1">
          <div v-for="s in SERIES" :key="s.key"
               class="w-3 rounded-t transition-[height]"
               :style="{ height: Math.max(d[s.key] ? 4 : 0, (d[s.key] || 0) / max * 160) + 'px', background: s.color }"
               :title="`${s.label}: ${d[s.key] || 0}`"></div>
        </div>
        <span class="whitespace-nowrap text-[10px] text-ink-700/60">{{ label(d.period) }}</span>
      </div>
    </div>
    <p v-else class="py-8 text-center text-sm text-ink-700/50">Нет данных по датам</p>
  </div>
</template>
