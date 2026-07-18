// Глобальный просмотр фото с навигацией по всем фото чата.
// Элементы списка: { url, mid? } — mid нужен для действий меню (перейти/переслать/удалить).
import { reactive, computed } from 'vue'

export const lb = reactive({ items: [], index: -1 })
export const lightboxItem = computed(() => (lb.index >= 0 && lb.index < lb.items.length ? lb.items[lb.index] : null))
export const lightboxSrc = computed(() => lightboxItem.value?.url ?? null)
export const lightboxMid = computed(() => lightboxItem.value?.mid ?? null)
export const lbHasList = computed(() => lb.items.length > 1)

// действия меню регистрирует ChatView (у него есть переход/пересылка/удаление)
let actions = {}
export function setLightboxActions(a) { actions = a || {} }
export function lightboxAction(name) { if (actions[name]) actions[name](lightboxMid.value, lightboxSrc.value) }

function norm(x) { return typeof x === 'string' ? { url: x } : x }
// openLightbox(src) — одиночное; openLightbox(src, list, index) — с навигацией по списку
export function openLightbox(src, list, index) {
  if (Array.isArray(list) && list.length) {
    lb.items = list.map(norm)
    lb.index = (typeof index === 'number' && index >= 0 && index < lb.items.length)
      ? index : Math.max(0, lb.items.findIndex((i) => i.url === src))
  } else {
    lb.items = [{ url: src }]
    lb.index = 0
  }
}
export function closeLightbox() { lb.index = -1; lb.items = [] }
export function lbNext() { if (lb.index < lb.items.length - 1) lb.index++ }
export function lbPrev() { if (lb.index > 0) lb.index-- }
export function lbGoto(i) { if (i >= 0 && i < lb.items.length) lb.index = i }
