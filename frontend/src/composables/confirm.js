import { reactive } from 'vue'

// Singleton state for the app-wide confirm dialog.
export const confirmState = reactive({
  show: false,
  title: 'Подтвердите действие',
  message: '',
  confirmText: 'Удалить',
  cancelText: 'Отмена',
  danger: true,
  _resolve: null,
})

// Returns a Promise<boolean>.
export function confirmDialog(opts = {}) {
  confirmState.title = opts.title || 'Подтвердите действие'
  confirmState.message = opts.message || ''
  confirmState.confirmText = opts.confirmText || 'Удалить'
  confirmState.cancelText = opts.cancelText || 'Отмена'
  confirmState.danger = opts.danger !== false
  confirmState.show = true
  return new Promise((resolve) => { confirmState._resolve = resolve })
}

export function answerConfirm(value) {
  confirmState.show = false
  const r = confirmState._resolve
  confirmState._resolve = null
  if (r) r(value)
}
