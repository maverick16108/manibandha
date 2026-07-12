<script setup>
import { onMounted, onBeforeUnmount } from 'vue'
import AppIcon from './AppIcon.vue'
import { lightboxSrc, openLightbox, closeLightbox } from '../composables/lightbox'

// клик по любой картинке внутри текста (markdown) — открыть на весь экран
function onDocClick(e) {
  const t = e.target
  if (t && t.tagName === 'IMG' && t.closest('.markdown-body')) {
    e.preventDefault()
    openLightbox(t.currentSrc || t.src)
  }
}
function onKey(e) { if (e.key === 'Escape' && lightboxSrc.value) closeLightbox() }
onMounted(() => {
  document.addEventListener('click', onDocClick)
  document.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onKey)
})
</script>

<template>
  <transition name="lb">
    <div v-if="lightboxSrc" class="fixed inset-0 z-[70] flex items-center justify-center bg-black/85 p-4" @click="closeLightbox">
      <img :src="lightboxSrc" class="max-h-full max-w-full rounded-lg object-contain shadow-2xl" @click.stop />
      <button class="absolute right-3 top-3 rounded-full bg-white/10 p-2 text-white transition hover:bg-white/20" title="Закрыть (Esc)" @click.stop="closeLightbox">
        <AppIcon name="close" :size="26" />
      </button>
    </div>
  </transition>
</template>

<style scoped>
.lb-enter-active, .lb-leave-active { transition: opacity .18s ease; }
.lb-enter-from, .lb-leave-to { opacity: 0; }
</style>
