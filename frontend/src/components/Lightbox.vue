<script setup>
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import AppIcon from './AppIcon.vue'
import { thumbUrl } from '../lib/format'
import { lb, lightboxSrc, lightboxMid, lbHasList, openLightbox, closeLightbox, lbNext, lbPrev, lbGoto, lightboxAction } from '../composables/lightbox'

const rot = ref(0)
const menu = ref(null)      // { x, y } контекстного меню или null
const grid = ref(false)     // показать сетку «Все фотографии»

// blur-up: миниатюра (из кэша) как размытый плейсхолдер, пока грузится полное фото
const displaySrc = ref(null)
const loading = ref(false)
watch(lightboxSrc, (u) => {
  if (!u) { displaySrc.value = null; loading.value = false; return }
  loading.value = true
  displaySrc.value = thumbUrl(u)
  const full = new Image()
  full.onload = () => { if (lightboxSrc.value === u) { displaySrc.value = u; loading.value = false } }
  full.onerror = () => { if (lightboxSrc.value === u) { loading.value = false } }
  full.src = u
}, { immediate: true })
watch(lightboxSrc, () => { rot.value = 0; menu.value = null })
watch(() => lb.index, () => {
  menu.value = null
  // предзагружаем пару фото слева и справа — при листании нет лоадера
  for (const d of [1, -1, 2, -2]) {
    const it = lb.items[lb.index + d]
    if (it && it.url) { const im = new Image(); im.decoding = 'async'; im.src = it.url }
  }
}, { immediate: true })

function onDocClick(e) {
  const t = e.target
  if (t && t.tagName === 'IMG' && t.closest('.markdown-body')) { e.preventDefault(); openLightbox(t.currentSrc || t.src) }
}
function onKey(e) {
  if (!lightboxSrc.value) return
  if (e.key === 'Escape') { if (grid.value) { grid.value = false } else if (menu.value) { menu.value = null } else closeLightbox(); return }
  if (e.key === 'ArrowRight') { e.preventDefault(); lbNext() }
  else if (e.key === 'ArrowLeft') { e.preventDefault(); lbPrev() }
}
function rotate() { rot.value += 90 }
function download() {
  const url = lightboxSrc.value; if (!url) return
  const a = document.createElement('a')
  a.href = url; a.download = (url.split('/').pop() || 'photo').split('?')[0]; a.target = '_blank'; a.rel = 'noopener'
  document.body.appendChild(a); a.click(); a.remove()
}
async function copyImage() {
  const url = lightboxSrc.value; if (!url) return
  try {
    const blob = await (await fetch(url)).blob()
    await navigator.clipboard.write([new window.ClipboardItem({ [blob.type]: blob })])
  } catch { /* не поддерживается — тихо */ }
}

// свайп
let sx = 0
function down(e) { sx = (e.touches ? e.touches[0].clientX : e.clientX) }
function up(e) {
  if (!lbHasList.value) return
  const x = (e.changedTouches ? e.changedTouches[0].clientX : e.clientX)
  const dx = x - sx
  if (Math.abs(dx) > 50) { if (dx < 0) lbNext(); else lbPrev() }
}

function openMenu(e) { menu.value = { x: e.clientX, y: e.clientY } }
function run(name) { menu.value = null; lightboxAction(name) }
function pickFromGrid(i) { lbGoto(i); grid.value = false }

onMounted(() => { document.addEventListener('click', onDocClick, true); document.addEventListener('keydown', onKey) })
onBeforeUnmount(() => { document.removeEventListener('click', onDocClick, true); document.removeEventListener('keydown', onKey) })
</script>

<template>
  <transition name="lb">
    <div v-if="lightboxSrc" class="fixed inset-0 z-[70] flex flex-col bg-black/90" @click="closeLightbox">
      <!-- верхняя панель -->
      <div class="flex shrink-0 items-center justify-between px-4 py-3 text-white" @click.stop>
        <div class="text-sm tabular-nums text-white/90">{{ lbHasList ? `${lb.index + 1} из ${lb.items.length}` : '' }}</div>
        <div class="flex items-center gap-2">
          <button v-if="lbHasList" class="rounded-full bg-white/10 p-2 text-white transition hover:bg-white/20" title="Все фотографии" @click="grid = true"><AppIcon name="grid" :size="22" /></button>
          <button class="rounded-full bg-white/10 p-2 text-white transition hover:bg-white/20" title="Повернуть" @click="rotate"><AppIcon name="rotate" :size="22" /></button>
          <button class="rounded-full bg-white/10 p-2 text-white transition hover:bg-white/20" title="Скачать" @click="download"><AppIcon name="download" :size="22" /></button>
          <button class="rounded-full bg-white/10 p-2 text-white transition hover:bg-white/20" title="Закрыть (Esc)" @click="closeLightbox"><AppIcon name="close" :size="24" /></button>
        </div>
      </div>

      <!-- область фото -->
      <div class="relative flex min-h-0 flex-1 items-center justify-center px-16 pb-4" @click="closeLightbox">
        <img :src="displaySrc" :style="{ transform: `rotate(${rot}deg)` }"
             class="lb-img max-h-full max-w-full select-none rounded-lg object-contain shadow-2xl transition-[filter] duration-300"
             :class="loading && 'blur-lg'"
             @click.stop @contextmenu.prevent.stop="openMenu" @mousedown="down" @mouseup="up" @touchstart.passive="down" @touchend="up" />
        <span v-if="loading" class="pointer-events-none absolute h-11 w-11 animate-spin rounded-full border-[3px] border-white/30 border-t-white"></span>

        <button v-if="lbHasList && lb.index > 0" class="absolute left-3 top-1/2 -translate-y-1/2 rounded-full bg-white/10 p-2.5 text-white transition hover:bg-white/20" title="Назад (←)" @click.stop="lbPrev"><AppIcon name="chevron" :size="28" class="rotate-90" /></button>
        <button v-if="lbHasList && lb.index < lb.items.length - 1" class="absolute right-3 top-1/2 -translate-y-1/2 rounded-full bg-white/10 p-2.5 text-white transition hover:bg-white/20" title="Вперёд (→)" @click.stop="lbNext"><AppIcon name="chevron" :size="28" class="-rotate-90" /></button>
      </div>

      <!-- контекстное меню (ПКМ по фото) -->
      <template v-if="menu">
        <div class="fixed inset-0 z-[80]" @click.stop="menu = null" @contextmenu.prevent.stop="menu = null"></div>
        <div class="fixed z-[81] w-60 overflow-hidden rounded-xl bg-ink-900 py-1 text-white shadow-2xl ring-1 ring-white/10"
             :style="{ left: Math.min(menu.x, (typeof window !== 'undefined' ? window.innerWidth : 9999) - 250) + 'px', top: menu.y + 'px' }" @click.stop>
          <button v-if="lightboxMid" class="flex w-full items-center gap-3 px-4 py-2.5 text-left text-[15px] hover:bg-white/10" @click="run('goto')"><AppIcon name="eye" :size="19" /> Перейти к сообщению</button>
          <button class="flex w-full items-center gap-3 px-4 py-2.5 text-left text-[15px] hover:bg-white/10" @click="menu = null; copyImage()"><AppIcon name="copy" :size="19" /> Копировать</button>
          <button v-if="lightboxMid" class="flex w-full items-center gap-3 px-4 py-2.5 text-left text-[15px] hover:bg-white/10" @click="run('forward')"><AppIcon name="reply" :size="19" class="-scale-x-100" /> Переслать</button>
          <button v-if="lightboxMid" class="flex w-full items-center gap-3 px-4 py-2.5 text-left text-[15px] text-red-400 hover:bg-white/10" @click="run('delete')"><AppIcon name="trash" :size="19" /> Удалить</button>
          <button class="flex w-full items-center gap-3 px-4 py-2.5 text-left text-[15px] hover:bg-white/10" @click="menu = null; download()"><AppIcon name="download" :size="19" /> Сохранить как…</button>
          <button v-if="lbHasList" class="flex w-full items-center gap-3 border-t border-white/10 px-4 py-2.5 text-left text-[15px] hover:bg-white/10" @click="menu = null; grid = true"><AppIcon name="grid" :size="19" /> Все фотографии</button>
        </div>
      </template>

      <!-- сетка «Все фотографии» -->
      <div v-if="grid" class="absolute inset-0 z-[82] overflow-y-auto bg-black/95 p-4" @click.stop>
        <div class="mb-3 flex items-center justify-between text-white">
          <div class="text-sm">Все фотографии · {{ lb.items.length }}</div>
          <button class="rounded-full bg-white/10 p-2 hover:bg-white/20" @click="grid = false"><AppIcon name="close" :size="22" /></button>
        </div>
        <div class="grid grid-cols-3 gap-1 sm:grid-cols-4 md:grid-cols-6">
          <button v-for="(it, i) in lb.items" :key="i" class="aspect-square overflow-hidden rounded" :class="i === lb.index && 'ring-2 ring-saffron-400'" @click="pickFromGrid(i)">
            <img :src="thumbUrl(it.url)" class="h-full w-full object-cover" />
          </button>
        </div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.lb-enter-active, .lb-leave-active { transition: opacity .18s ease; }
.lb-enter-from, .lb-leave-to { opacity: 0; }
.lb-img { transition: transform .3s ease; }
</style>
