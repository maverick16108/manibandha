import { ref, watchEffect, onActivated, onDeactivated } from 'vue'

// Заголовок текущей страницы, отображается в верхней панели (AppLayout)
export const pageTitle = ref('')

// Вызывать в <script setup> страницы. Принимает строку или геттер (для динамических заголовков).
// ВАЖНО (keep-alive): при возврате в закешированный раздел watchEffect НЕ перезапускается, если его
// зависимости не изменились (напр. isReport остаётся false при Чат→Вопросы) — тогда заголовок «застревал»
// на предыдущем («Чат»). Поэтому переустанавливаем заголовок в onActivated, а деактивированный раздел
// не даём перетирать общий заголовок (иначе фоновые инстансы дерутся за него).
export function usePageTitle(source) {
  const compute = () => (typeof source === 'function' ? source() : source) || ''
  let active = true
  watchEffect(() => { const t = compute(); if (active) pageTitle.value = t })
  onActivated(() => { active = true; pageTitle.value = compute() })
  onDeactivated(() => { active = false })
}
