<script setup>
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import AppIcon from './AppIcon.vue'
import { lb, lightboxSrc, lbHasList, openLightbox, closeLightbox, lbNext, lbPrev } from '../composables/lightbox'

const rot = ref(0) // угол поворота (кратно 90), анимируется плавно

// клик по любой картинке внутри текста (markdown) — открыть на весь экран
function onDocClick(e) {
  const t = e.target
  if (t && t.tagName === 'IMG' && t.closest('.markdown-body')) {
    e.preventDefault()
    openLightbox(t.currentSrc || t.src)
  }
}
function onKey(e) {
  if (!lightboxSrc.value) return
  if (e.key === 'Escape') { closeLightbox(); return }
  if (e.key === 'ArrowRight') { e.preventDefault(); lbNext() }
  else if (e.key === 'ArrowLeft') { e.preventDefault(); lbPrev() }
}
function rotate() { rot.value += 90 }
function download() {
  const url = lightboxSrc.value; if (!url) return
  const a = document.createElement('a')
  a.href = url; a.download = (url.split('/').pop() || 'photo').split('?')[0]
  a.target = '_blank'; a.rel = 'noopener'
  document.body.appendChild(a); a.click(); a.remove()
}
// сброс поворота при смене изображения
watch(lightboxSrc, () => { rot.value = 0 })

// свайп мышью/пальцем по картинке
let sx = 0, moved = false
function down(e) { sx = (e.touches ? e.touches[0].clientX : e.clientX); moved = false }
function up(e) {
  if (!lbHasList.value) return
  const x = (e.changedTouches ? e.changedTouches[0].clientX : e.clientX)
  const dx = x - sx
  if (Math.abs(dx) > 50) { moved = true; if (dx < 0) lbNext(); else lbPrev() }
}

onMounted(() => {
  document.addEventListener('click', onDocClick, true)
  document.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', onDocClick, true)
  document.removeEventListener('keydown', onKey)
})
</script>

<template>
  <transition name="lb">
    <div v-if="lightboxSrc" class="fixed inset-0 z-[70] flex items-center justify-center bg-black/85 p-4" @click="closeLightbox">
      <img :src="lightboxSrc" :style="{ transform: `rotate(${rot}deg)` }"
           class="lb-img max-h-full max-w-full select-none rounded-lg object-contain shadow-2xl"
           @click.stop @mousedown="down" @mouseup="up" @touchstart.passive="down" @touchend="up" />

      <!-- счётчик -->
      <div v-if="lbHasList" class="absolute left-1/2 top-4 -translate-x-1/2 rounded-full bg-white/10 px-3 py-1 text-sm text-white">
        {{ lb.index + 1 }} из {{ lb.list.length }}
      </div>

      <!-- стрелки -->
      <button v-if="lbHasList && lb.index > 0" class="absolute left-3 top-1/2 -translate-y-1/2 rounded-full bg-white/10 p-2.5 text-white transition hover:bg-white/20" title="Назад (←)" @click.stop="lbPrev">
        <AppIcon name="chevron" :size="28" class="rotate-90" />
      </button>
      <button v-if="lbHasList && lb.index < lb.list.length - 1" class="absolute right-3 top-1/2 -translate-y-1/2 rounded-full bg-white/10 p-2.5 text-white transition hover:bg-white/20" title="Вперёд (→)" @click.stop="lbNext">
        <AppIcon name="chevron" :size="28" class="-rotate-90" />
      </button>

      <!-- панель действий -->
      <div class="absolute right-3 top-3 flex items-center gap-2">
        <button class="rounded-full bg-white/10 p-2 text-white transition hover:bg-white/20" title="Повернуть" @click.stop="rotate">
          <AppIcon name="rotate" :size="22" />
        </button>
        <button class="rounded-full bg-white/10 p-2 text-white transition hover:bg-white/20" title="Скачать" @click.stop="download">
          <AppIcon name="download" :size="22" />
        </button>
        <button class="rounded-full bg-white/10 p-2 text-white transition hover:bg-white/20" title="Закрыть (Esc)" @click.stop="closeLightbox">
          <AppIcon name="close" :size="24" />
        </button>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.lb-enter-active, .lb-leave-active { transition: opacity .18s ease; }
.lb-enter-from, .lb-leave-to { opacity: 0; }
.lb-img { transition: transform .3s ease; } /* плавный поворот */
</style>
