// Глобальный просмотр фото с навигацией по всем фото чата.
import { reactive, computed } from 'vue'

export const lb = reactive({ list: [], index: -1 })
export const lightboxSrc = computed(() => (lb.index >= 0 && lb.index < lb.list.length ? lb.list[lb.index] : null))
export const lbHasList = computed(() => lb.list.length > 1)

// openLightbox(src) — одиночное; openLightbox(src, list, index) — с навигацией по списку
// index задаём явно (в списке бывают дубликаты одного URL — indexOf дал бы не то фото)
export function openLightbox(src, list, index) {
  if (Array.isArray(list) && list.length) {
    lb.list = list.slice()
    lb.index = (typeof index === 'number' && index >= 0 && index < lb.list.length) ? index : Math.max(0, lb.list.indexOf(src))
  } else {
    lb.list = [src]
    lb.index = 0
  }
}
export function closeLightbox() { lb.index = -1; lb.list = [] }
export function lbNext() { if (lb.index < lb.list.length - 1) lb.index++ }
export function lbPrev() { if (lb.index > 0) lb.index-- }
