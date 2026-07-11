import { ref, watchEffect } from 'vue'

// Заголовок текущей страницы, отображается в верхней панели (AppLayout)
export const pageTitle = ref('')

// Вызывать в <script setup> страницы. Принимает строку или геттер (для динамических заголовков).
export function usePageTitle(source) {
  watchEffect(() => {
    pageTitle.value = (typeof source === 'function' ? source() : source) || ''
  })
}
