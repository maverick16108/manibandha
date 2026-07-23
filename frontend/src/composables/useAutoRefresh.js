import { onActivated, onDeactivated, onBeforeUnmount } from 'vue'

// Тихое авто-обновление keep-alive раздела: при ВОЗВРАТЕ в раздел и при возврате фокуса на вкладку
// (чтобы чужие изменения — например добавленное кем-то событие — подтягивались сами, без скелетона).
// load(silent) должна не показывать скелетон при silent=true. Первую активацию пропускаем (уже загружено в onMounted).
export function useAutoRefresh(load) {
  let first = true
  let active = true
  onActivated(() => { active = true; if (first) { first = false; return } load(true) })
  onDeactivated(() => { active = false })
  const onFocus = () => { if (active && document.visibilityState === 'visible') load(true) }
  window.addEventListener('focus', onFocus)
  document.addEventListener('visibilitychange', onFocus)
  onBeforeUnmount(() => {
    window.removeEventListener('focus', onFocus)
    document.removeEventListener('visibilitychange', onFocus)
  })
}
