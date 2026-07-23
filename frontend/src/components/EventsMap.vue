<script setup>
import { ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { geocode } from '../lib/geo'

const props = defineProps({
  events: { type: Array, default: () => [] },
})
const emit = defineEmits(['open'])

const mapEl = ref(null)
const rootEl = ref(null)
let map = null
let layer = null
// высота = ровно до низа экрана (без переполнения/скролла)
function fitHeight() {
  if (!rootEl.value) return
  const top = rootEl.value.getBoundingClientRect().top
  rootEl.value.style.height = Math.max(320, window.innerHeight - top) + 'px'
  if (map) map.invalidateSize()
}

const MON = ['янв', 'фев', 'мар', 'апр', 'май', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек']
function fmt(iso) { if (!iso) return ''; const [y, m, d] = iso.split('-'); return `${+d} ${MON[+m - 1]} ${y}` }
function range(e) { const s = fmt(e.starts_on); return e.ends_on && e.ends_on !== e.starts_on ? `${s} — ${fmt(e.ends_on)}` : s }

// точки маршрута: события с координатами, по дате; исключаем рутинную «Воскресную программу»
function stops() {
  return props.events
    .filter((e) => e.title !== 'Воскресная программа')
    .map((e) => ({ e, c: geocode(e.location) }))
    .filter((x) => x.c)
    .sort((a, b) => (a.e.starts_on || '').localeCompare(b.e.starts_on || ''))
}

function numberIcon(label, color) {
  const s = String(label)
  const w = Math.max(26, 12 + s.length * 8) // расширяем «пилюлю», если в городе несколько событий (напр. «2, 7»)
  return L.divIcon({
    className: '',
    html: `<div style="width:${w}px;height:26px;border-radius:13px;background:${color};color:#fff;display:flex;align-items:center;justify-content:center;font:600 12px/1 system-ui;box-shadow:0 1px 4px rgba(0,0,0,.35);border:2px solid #fff;white-space:nowrap;padding:0 5px">${s}</div>`,
    iconSize: [w, 26], iconAnchor: [w / 2, 13],
  })
}
// маркер «гуру сейчас здесь» — пульсирующее кольцо, ставится позади номера города
function guruIcon() {
  return L.divIcon({
    className: '',
    html: `<div style="position:relative;width:52px;height:52px">
      <div style="position:absolute;inset:0;border-radius:50%;border:2.5px solid #16a34a;animation:guruPulse 2.2s ease-out infinite"></div>
      <div style="position:absolute;inset:15px;border-radius:50%;background:rgba(22,163,74,.25)"></div>
    </div>`,
    iconSize: [52, 52], iconAnchor: [26, 26],
  })
}
// приглушённая палитра (НЕ радуга) — соседние перегоны разного цвета, чтобы линии не сливались
const ROUTE_COLORS = ['#c8742a', '#3f7d6e', '#9c4a3c', '#4a6d9c', '#8a6d2b', '#6b5b8a', '#5a7a3f']
function arrowIcon(deg, color) {
  return L.divIcon({
    className: '',
    html: `<div style="color:${color};font-size:16px;line-height:1;transform:rotate(${deg}deg);text-shadow:0 0 3px #fff,0 0 3px #fff">▶</div>`,
    iconSize: [16, 16], iconAnchor: [8, 8],
  })
}

// плавная дуга между двумя точками (квадратичный Безье). Контрольная точка поднята НА СЕВЕР
// (восходящая дуга) — так параллельные восток-западные перегоны расходятся по высоте и меньше
// сливаются; величина подъёма растёт с длиной перегона (длинные выгибаются сильнее).
function curve(a, b, lift = 1, seg = 26) {
  const mLat = (a[0] + b[0]) / 2, mLng = (a[1] + b[1]) / 2
  const dLat = b[0] - a[0], dLng = b[1] - a[1]
  const d = Math.hypot(dLat, dLng) || 1
  const bend = Math.min(0.22 * d + 0.4, 5) * lift
  // выгиб ПЕРПЕНДИКУЛЯРНО ходу, «влево по движению»: восток→запад и обратно выгибаются в РАЗНЫЕ
  // стороны (не сливаются); при движении на восток дуга уходит вверх (как и просили)
  const cLat = mLat + bend * (dLng / d)
  const cLng = mLng - bend * (dLat / d)
  const out = []
  for (let i = 0; i <= seg; i++) {
    const t = i / seg, u = 1 - t
    out.push([u * u * a[0] + 2 * u * t * cLat + t * t * b[0], u * u * a[1] + 2 * u * t * cLng + t * t * b[1]])
  }
  return out
}

// обрезаем концы дуги ровно на R пикселей от каждого узла (интерполяцией, а не выбрасывая целые точки —
// иначе на длинных перегонах шаг между точками огромный и линия «обрывается» далеко от точки)
function trimEnds(path, R) {
  if (!map || path.length < 3) return path
  const px = path.map((p) => map.latLngToLayerPoint(L.latLng(p[0], p[1])))
  const n = px.length
  const lerp = (a, b, t) => [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t]
  let sIdx = -1, sPt = null
  for (let i = 1; i < n; i++) {
    const d = px[0].distanceTo(px[i])
    if (d >= R) { const dp = px[0].distanceTo(px[i - 1]); sPt = lerp(path[i - 1], path[i], (R - dp) / (d - dp || 1)); sIdx = i; break }
  }
  let eIdx = -1, ePt = null
  for (let i = n - 2; i >= 0; i--) {
    const d = px[n - 1].distanceTo(px[i])
    if (d >= R) { const dp = px[n - 1].distanceTo(px[i + 1]); ePt = lerp(path[i + 1], path[i], (R - dp) / (d - dp || 1)); eIdx = i; break }
  }
  if (sPt == null || ePt == null || sIdx > eIdx) return null
  return [sPt, ...path.slice(sIdx, eIdx + 1), ePt]
}

function render(fit = true) {
  if (!map) return
  if (layer) { layer.remove(); layer = null }
  layer = L.layerGroup().addTo(map)

  const pts = stops()
  if (!pts.length) return

  const latlngs = pts.map((p) => p.c)
  const keyOf = (c) => c[0].toFixed(2) + ',' + c[1].toFixed(2)
  // маршрут — дуги, выгнутые перпендикулярно ходу. Повторные перегоны между теми же городами
  // (в т.ч. «туда-обратно») выгибаются шире по мере повторения — не ложатся друг на друга.
  const pairSeen = new Map()
  for (let i = 0; i < latlngs.length - 1; i++) {
    const a = latlngs[i], b = latlngs[i + 1]
    if (a[0] === b[0] && a[1] === b[1]) continue // подряд один город — нулевой сегмент не рисуем
    const pk = keyOf(a) + '>' + keyOf(b)
    const occ = pairSeen.get(pk) || 0; pairSeen.set(pk, occ + 1)
    const lift = 1 + 0.6 * occ + 0.12 * (i % 3) // повтор того же направления — шире; лёгкий джиттер по индексу
    const path = trimEnds(curve(a, b, lift), 18) // 18px зазор — линия не заходит на кружки
    if (!path) continue
    const color = ROUTE_COLORS[i % ROUTE_COLORS.length]
    L.polyline(path, { color, weight: 3.5, opacity: 0.8, dashArray: '10 9', lineCap: 'round', lineJoin: 'round' }).addTo(layer)
    const m = Math.floor(path.length / 2)
    const p0 = path[Math.max(0, m - 1)], p1 = path[Math.min(path.length - 1, m + 1)]
    const deg = Math.atan2(-(p1[0] - p0[0]), p1[1] - p0[1]) * 180 / Math.PI
    L.marker(path[m], { icon: arrowIcon(deg, color), interactive: false }).addTo(layer)
  }

  // группируем события по городу: один маркер, но в нём номера ВСЕХ событий там (напр. «2, 7»)
  const groups = new Map()
  pts.forEach((p, i) => {
    const k = keyOf(p.c)
    if (!groups.has(k)) groups.set(k, { c: p.c, nums: [], events: [] })
    const g = groups.get(k); g.nums.push(i + 1); g.events.push(p.e)
  })

  // «гуру сейчас»: последнее событие, чья дата уже наступила (пока не наступило — точка предыдущего;
  // если ещё ни одно не прошло — первая точка маршрута)
  const today = todayStr()
  let curIdx = 0
  for (let i = 0; i < pts.length; i++) if ((pts[i].e.starts_on || '') <= today) curIdx = i
  const guruKey = keyOf(pts[curIdx].c)

  for (const g of groups.values()) {
    if (keyOf(g.c) === guruKey) {
      L.marker(g.c, { icon: guruIcon(), interactive: false, zIndexOffset: -500 })
        .addTo(layer)
        .bindTooltip('Гуру сейчас здесь', { permanent: true, direction: 'top', offset: [0, -18], className: 'guru-tip' })
    }
    const gColor = ROUTE_COLORS[(g.nums[0] - 1) % ROUTE_COLORS.length] // цвет = цвет исходящей линии этой точки
    const marker = L.marker(g.c, { icon: numberIcon(g.nums.join(', '), gColor), zIndexOffset: 200 })
      .addTo(layer)
      .bindPopup(popupHtml(g))
    marker.on('popupopen', (ev) => {
      const root = ev.popup.getElement()
      g.events.forEach((e, k) => {
        const btn = root?.querySelector(`.evmap-open[data-k="${k}"]`)
        if (btn) btn.onclick = () => emit('open', e)
      })
    })
  }
  if (fit) map.fitBounds(L.latLngBounds(latlngs).pad(0.25))
}

function todayStr() { const d = new Date(); return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}` }
function popupHtml(g) {
  let h = ''
  if (g.events.length > 1) h += `<div style="font:600 12px system-ui;color:#c8742a;margin-bottom:6px">Событий в этом городе: ${g.events.length}</div>`
  g.events.forEach((e, k) => {
    h += `<div style="margin-top:${k ? 10 : 0}px">`
      + `<div style="font:600 13px/1.3 system-ui;color:#2b2320">${e.title}</div>`
      + `<div style="font:12px/1.4 system-ui;color:#8a7a6a;margin-top:2px">${range(e)}${e.location ? ' · ' + e.location : ''}</div>`
      + `<button class="evmap-open" data-k="${k}" style="margin-top:6px;color:#c8742a;font:600 12px system-ui;cursor:pointer;background:none;border:0;padding:0">Открыть событие →</button>`
      + '</div>'
  })
  return h
}

onMounted(async () => {
  await nextTick()
  map = L.map(mapEl.value, { scrollWheelZoom: true }).setView([50, 60], 3)
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '© OpenStreetMap', maxZoom: 18,
  }).addTo(map)
  fitHeight()
  render()
  map.on('zoomend', () => render(false)) // зазор у точек задан в пикселях — пересчитываем при зуме
  window.addEventListener('resize', fitHeight)
})
onBeforeUnmount(() => { window.removeEventListener('resize', fitHeight); if (map) { map.remove(); map = null } })
watch(() => props.events, () => render(), { deep: true })
</script>

<template>
  <div ref="rootEl" class="flex flex-col">
    <div ref="mapEl" class="min-h-0 w-full flex-1 overflow-hidden"></div>
    <p v-if="!stops().length" class="py-3 text-center text-sm text-ink-700/50">
      В выбранном периоде нет событий с известным местом на карте.
    </p>
  </div>
</template>

<style>
@keyframes guruPulse { 0% { transform: scale(.5); opacity: .9 } 100% { transform: scale(1.5); opacity: 0 } }
.guru-tip { background: #16a34a; color: #fff; border: 0; font: 600 11px/1 system-ui; padding: 4px 8px; border-radius: 9px; box-shadow: 0 1px 4px rgba(0,0,0,.3); }
.guru-tip::before { border-top-color: #16a34a; }
.guru-tip.leaflet-tooltip { white-space: nowrap; }
</style>
