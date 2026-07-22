<script setup>
import { computed, ref } from 'vue'

// data: [{ period: 'YYYY-MM', pranama, harinama, brahman }]
const props = defineProps({ data: { type: Array, default: () => [] } })
const emit = defineEmits(['select'])

const SERIES = [
  { key: 'pranama', label: 'Пранама', color: '#c8742a' },
  { key: 'harinama', label: 'Харинама', color: '#a4551b' },
  { key: 'brahman', label: 'Брахман', color: '#6d3f16' },
]
const MONTHS = ['', 'янв', 'фев', 'мар', 'апр', 'май', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек']
function monthLabel(period) {
  const [y, m] = period.split('-')
  return `${MONTHS[+m]} ${y.slice(2)}`
}

// SVG geometry (viewBox units; scales to container width)
const W = 720, H = 260
const padL = 26, padR = 14, padT = 12, padB = 30
const plotW = W - padL - padR
const plotH = H - padT - padB

const hover = ref(-1)

// smooth curve through points (Catmull-Rom -> cubic bezier), clamped to the plot band
const clampY = (v) => Math.max(padT, Math.min(padT + plotH, v))
function smoothPath(pts) {
  if (!pts.length) return ''
  if (pts.length === 1) return `M ${pts[0].x},${pts[0].y}`
  let d = `M ${pts[0].x},${pts[0].y}`
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i - 1] || pts[i]
    const p1 = pts[i]
    const p2 = pts[i + 1]
    const p3 = pts[i + 2] || p2
    const c1x = p1.x + (p2.x - p0.x) / 6
    const c1y = clampY(p1.y + (p2.y - p0.y) / 6)
    const c2x = p2.x - (p3.x - p1.x) / 6
    const c2y = clampY(p2.y - (p3.y - p1.y) / 6)
    d += ` C ${c1x},${c1y} ${c2x},${c2y} ${p2.x},${p2.y}`
  }
  return d
}

const geom = computed(() => {
  const n = props.data.length
  const maxVal = Math.max(1, ...props.data.flatMap((d) => SERIES.map((s) => d[s.key] || 0)))
  const step = Math.max(1, Math.ceil(maxVal / 4))
  const niceMax = step * Math.ceil(maxVal / step)
  const ticks = []
  for (let t = 0; t <= niceMax; t += step) ticks.push(t)

  const x = (i) => (n <= 1 ? padL + plotW / 2 : padL + (i / (n - 1)) * plotW)
  const y = (v) => padT + plotH * (1 - v / niceMax)

  const lines = SERIES.map((s) => ({
    ...s,
    points: props.data.map((d, i) => ({ x: x(i), y: y(d[s.key] || 0), v: d[s.key] || 0 })),
    poly: props.data.map((d, i) => `${x(i)},${y(d[s.key] || 0)}`).join(' '),
  }))
  return { n, niceMax, ticks, x, y, lines }
})

// тултип: что произошло в наведённый месяц (получение пранамы / инициации) — по сериям с count > 0
const TIPW = 172
const tip = computed(() => {
  const i = hover.value
  if (i < 0 || !props.data[i]) return null
  const d = props.data[i]
  const rows = SERIES.filter((s) => (d[s.key] || 0) > 0).map((s) => ({ color: s.color, text: `${s.label}: ${d[s.key]}` }))
  const h = rows.length ? 32 + rows.length * 16 : 44
  const gx = geom.value.x(i)
  const x = Math.max(4, Math.min(W - TIPW - 4, gx - TIPW / 2))
  return { x, y: padT + 4, h, month: monthLabel(d.period), rows }
})
</script>

<template>
  <div>
    <div class="mb-3 flex flex-wrap gap-4">
      <span v-for="s in SERIES" :key="s.key" class="flex items-center gap-1.5 text-sm text-ink-700">
        <span class="h-3 w-3 rounded-sm" :style="{ background: s.color }"></span>{{ s.label }}
      </span>
    </div>

    <svg v-if="data.length" :viewBox="`0 0 ${W} ${H}`" class="w-full" :style="{ height: 'auto' }" @mouseleave="hover = -1">
      <!-- gridlines + y labels -->
      <g>
        <template v-for="t in geom.ticks" :key="t">
          <line :x1="padL" :x2="W - padR" :y1="geom.y(t)" :y2="geom.y(t)" stroke="#efe6d6" stroke-width="1" />
          <text :x="padL - 6" :y="geom.y(t) + 3" text-anchor="end" font-size="10" fill="#9c8f7c">{{ t }}</text>
        </template>
      </g>

      <!-- hover guide -->
      <line v-if="hover >= 0" :x1="geom.x(hover)" :x2="geom.x(hover)" :y1="padT" :y2="padT + plotH"
            stroke="#c8742a" stroke-width="1" stroke-dasharray="3 3" opacity="0.5" />

      <!-- series lines + dots -->
      <g v-for="s in geom.lines" :key="s.key">
        <path :d="smoothPath(s.points)" fill="none" :stroke="s.color" stroke-width="3.5" stroke-linejoin="round" stroke-linecap="round" />
        <circle v-for="(p, i) in s.points" :key="i" :cx="p.x" :cy="p.y"
                :r="hover === i ? 5 : 3" :fill="s.color" stroke="#fff" stroke-width="1.5">
          <title>{{ monthLabel(data[i].period) }} · {{ s.label }}: {{ p.v }}</title>
        </circle>
      </g>

      <!-- x labels + hover hit areas -->
      <g>
        <text v-for="(d, i) in data" :key="'l' + i" :x="geom.x(i)" :y="H - 10" text-anchor="middle" font-size="10" fill="#9c8f7c">
          {{ monthLabel(d.period) }}
        </text>
        <rect v-for="(d, i) in data" :key="'h' + i"
              :x="geom.x(i) - plotW / (2 * Math.max(1, geom.n))" :y="padT"
              :width="plotW / Math.max(1, geom.n)" :height="plotH" fill="transparent"
              class="cursor-pointer" @mouseenter="hover = i" @click="emit('select', d.period)" />
      </g>

      <!-- тултип: что произошло в этом месяце (получение пранамы / инициации) -->
      <g v-if="tip" style="pointer-events: none">
        <rect :x="tip.x" :y="tip.y" :width="TIPW" :height="tip.h" rx="8" fill="#ffffff" stroke="#e7dcc9" stroke-width="1" />
        <text :x="tip.x + 10" :y="tip.y + 18" font-size="11.5" font-weight="700" fill="#2b2320">{{ tip.month }}</text>
        <template v-for="(r, ri) in tip.rows" :key="ri">
          <circle :cx="tip.x + 14" :cy="tip.y + 35 + ri * 16" r="3.5" :fill="r.color" />
          <text :x="tip.x + 24" :y="tip.y + 39 + ri * 16" font-size="11" fill="#5a4a3a">{{ r.text }}</text>
        </template>
        <text v-if="!tip.rows.length" :x="tip.x + 10" :y="tip.y + 35" font-size="11" fill="#9c8f7c">нет событий</text>
      </g>
    </svg>
    <p v-else class="py-8 text-center text-sm text-ink-700/50">Нет данных по датам</p>
  </div>
</template>
