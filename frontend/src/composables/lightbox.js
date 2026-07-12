// Глобальный просмотр фото: клик по картинке в тексте открывает её на весь экран.
import { ref } from 'vue'

export const lightboxSrc = ref(null)
export function openLightbox(src) { lightboxSrc.value = src }
export function closeLightbox() { lightboxSrc.value = null }
