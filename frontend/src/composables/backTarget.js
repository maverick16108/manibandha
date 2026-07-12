import { ref } from 'vue'

// Явная цель кнопки «Назад» для страниц, где router.back() ненадёжен
// (напр. в ветке — назад в список вопросов/отчётов, а не в форму создания).
export const backTarget = ref(null)
