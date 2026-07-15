import { reactive } from 'vue'

// Небольшое всплывающее уведомление (тост) внизу экрана.
export const toastState = reactive({ msg: '', show: false })
let timer = null

export function showToast(msg, ms = 2500) {
  toastState.msg = msg
  toastState.show = true
  clearTimeout(timer)
  timer = setTimeout(() => { toastState.show = false }, ms)
}
