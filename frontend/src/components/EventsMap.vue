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
let map = null
let layer = null

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

function numberIcon(n) {
  return L.divIcon({
    className: '',
    html: `<div style="width:26px;height:26px;border-radius:50%;background:#c8742a;color:#fff;display:flex;align-items:center;justify-content:center;font:600 13px/1 system-ui;box-shadow:0 1px 4px rgba(0,0,0,.35);border:2px solid #fff">${n}</div>`,
    iconSize: [26, 26], iconAnchor: [13, 13],
  })
}
function arrowIcon(deg) {
  return L.divIcon({
    className: '',
    html: `<div style="color:#c8742a;font-size:16px;line-height:1;transform:rotate(${deg}deg);text-shadow:0 0 3px #fff,0 0 3px #fff">▶</div>`,
    iconSize: [16, 16], iconAnchor: [8, 8],
  })
}

// плавная дуга между двумя точками (квадратичный Безье со смещением контрольной точки
// перпендикулярно сегменту) — маршрут выглядит мягкими кривыми, а не ломаной
function curve(a, b, seg = 22, bend = 0.13) {
  const mx = (a[0] + b[0]) / 2, my = (a[1] + b[1]) / 2
  const dx = b[0] - a[0], dy = b[1] - a[1]
  const cx = mx - dy * bend, cy = my + dx * bend
  const out = []
  for (let i = 0; i <= seg; i++) {
    const t = i / seg, u = 1 - t
    out.push([u * u * a[0] + 2 * u * t * cx + t * t * b[0], u * u * a[1] + 2 * u * t * cy + t * t * b[1]])
  }
  return out
}

function render() {
  if (!map) return
  if (layer) { layer.remove(); layer = null }
  layer = L.layerGroup().addTo(map)

  const pts = stops()
  if (!pts.length) return

  const latlngs = pts.map((p) => p.c)
  // маршрут — плавные жирные кривые (пунктир, скруглённые концы)
  if (latlngs.length > 1) {
    for (let i = 0; i < latlngs.length - 1; i++) {
      const path = curve(latlngs[i], latlngs[i + 1])
      L.polyline(path, { color: '#c8742a', weight: 3.5, opacity: 0.7, dashArray: '10 9', lineCap: 'round', lineJoin: 'round' }).addTo(layer)
      // стрелка направления на середине дуги (по касательной)
      const m = Math.floor(path.length / 2)
      const p0 = path[Math.max(0, m - 1)], p1 = path[Math.min(path.length - 1, m + 1)]
      const deg = Math.atan2(-(p1[0] - p0[0]), p1[1] - p0[1]) * 180 / Math.PI
      L.marker(path[m], { icon: arrowIcon(deg), interactive: false }).addTo(layer)
    }
  }
  // точки
  pts.forEach((p, i) => {
    const marker = L.marker(p.c, { icon: numberIcon(i + 1) })
      .addTo(layer)
      .bindPopup(
        `<div style="font:600 13px/1.3 system-ui;color:#2b2320">${p.e.title}</div>` +
        `<div style="font:12px/1.4 system-ui;color:#8a7a6a;margin-top:2px">${range(p.e)}${p.e.location ? ' · ' + p.e.location : ''}</div>` +
        `<button class="evmap-open" style="margin-top:8px;color:#c8742a;font:600 12px system-ui;cursor:pointer;background:none;border:0;padding:0">Открыть событие →</button>`,
      )
    marker.on('popupopen', (ev) => {
      const btn = ev.popup.getElement()?.querySelector('.evmap-open')
      if (btn) btn.onclick = () => emit('open', p.e)
    })
  })
  map.fitBounds(L.latLngBounds(latlngs).pad(0.25))
}

onMounted(async () => {
  await nextTick()
  map = L.map(mapEl.value, { scrollWheelZoom: true }).setView([50, 60], 3)
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '© OpenStreetMap', maxZoom: 18,
  }).addTo(map)
  render()
})
onBeforeUnmount(() => { if (map) { map.remove(); map = null } })
watch(() => props.events, () => render(), { deep: true })
</script>

<template>
  <div>
    <div ref="mapEl" class="h-[68vh] w-full overflow-hidden rounded-xl border border-parchment-200"></div>
    <p v-if="!stops().length" class="mt-3 text-center text-sm text-ink-700/50">
      В выбранном периоде нет событий с известным местом на карте.
    </p>
  </div>
</template>
