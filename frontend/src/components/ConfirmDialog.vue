<script setup>
import { onMounted, onBeforeUnmount } from 'vue'
import { confirmState, answerConfirm } from '../composables/confirm'

function onKey(e) {
  if (!confirmState.show) return
  if (e.key === 'Escape') answerConfirm(false)
  else if (e.key === 'Enter') answerConfirm(true)
}
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))
</script>

<template>
  <teleport to="body">
    <transition
      enter-active-class="transition duration-150" enter-from-class="opacity-0"
      leave-active-class="transition duration-150" leave-to-class="opacity-0"
    >
      <div v-if="confirmState.show" class="fixed inset-0 z-[60] flex items-center justify-center bg-ink-900/50 p-4"
           @click.self="answerConfirm(false)">
        <div class="card w-full max-w-sm p-6">
          <h3 class="font-display text-2xl text-ink-900">{{ confirmState.title }}</h3>
          <p v-if="confirmState.message" class="mt-2 text-ink-700">{{ confirmState.message }}</p>
          <div class="mt-6 flex justify-end gap-2">
            <button class="btn-ghost" @click="answerConfirm(false)">{{ confirmState.cancelText }}</button>
            <button
              class="btn text-white"
              :class="confirmState.danger ? 'bg-red-600 hover:bg-red-700' : 'bg-saffron-500 hover:bg-saffron-600'"
              autofocus @click="answerConfirm(true)"
            >{{ confirmState.confirmText }}</button>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>
