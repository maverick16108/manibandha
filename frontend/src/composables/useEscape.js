import { onMounted, onBeforeUnmount } from 'vue'

// Вызывает handler при нажатии Escape (пока компонент смонтирован).
export function onEscape(handler) {
  function onKey(e) { if (e.key === 'Escape') handler(e) }
  onMounted(() => document.addEventListener('keydown', onKey))
  onBeforeUnmount(() => document.removeEventListener('keydown', onKey))
}
