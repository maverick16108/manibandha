<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppIcon from '../components/AppIcon.vue'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Заявка на рассмотрении')

const auth = useAuthStore()
const router = useRouter()

const approvalThread = ref(null)
const formFilled = ref(false)
const loaded = ref(false)

onMounted(async () => {
  try {
    const { data } = await client.get('/threads')
    approvalThread.value = (Array.isArray(data) ? data : []).find((t) => t.kind === 'approval') || null
  } catch { /* ignore */ }
  try {
    const { data } = await client.get(`/disciples/${auth.user.disciple_id}`)
    formFilled.value = !!(data.material_name || data.spiritual_name)
  } catch { /* ignore */ } finally { loaded.value = true }
})

function fillForm() {
  router.push({ name: 'disciple-edit', params: { id: auth.user.disciple_id } })
}

function openChat() {
  if (approvalThread.value) router.push({ name: 'thread', params: { id: approvalThread.value.id } })
}

function logout() {
  auth.logout()
  router.push('/')
}
</script>

<template>
  <div class="mx-auto max-w-xl">
    <div class="card p-8 text-center">
      <img src="/lotus-mark.png" alt="" class="mx-auto mb-4 h-12 w-auto" />
      <h1 class="font-display text-2xl font-semibold text-ink-900">Спасибо за регистрацию!</h1>
      <p class="mt-3 font-serif text-ink-700/80">
        Ваш аккаунт создан и ожидает подтверждения куратором.
        Как только заявку одобрят, откроется полный доступ к кабинету.
      </p>

      <p class="mt-4 flex min-h-[5.5rem] items-center gap-2 rounded-lg px-4 py-3 text-sm font-medium ring-1"
         :class="!loaded ? 'animate-pulse bg-parchment-100 ring-parchment-200'
           : (formFilled ? 'bg-emerald-50 text-emerald-700 ring-emerald-200' : 'bg-saffron-500/10 text-saffron-800 ring-saffron-300')">
        <template v-if="loaded">
          <AppIcon :name="formFilled ? 'check' : 'chat'" :size="18" class="shrink-0" />
          <span>{{ formFilled
            ? 'Анкета заполнена. Ожидайте — с Вами свяжется куратор для завершения регистрации.'
            : 'Обязательно заполните анкету — только после этого заявку рассмотрят и откроется доступ. Без анкеты заявку не одобрят.' }}</span>
        </template>
      </p>

      <div class="mt-6 flex flex-col gap-3">
        <button class="btn-primary w-full" :disabled="!loaded" @click="fillForm">
          {{ !loaded ? '…' : (formFilled ? 'Редактировать анкету' : 'Заполнить анкету') }}
        </button>
        <button v-if="approvalThread" class="btn-outline w-full" @click="openChat">
          <AppIcon name="chat" :size="16" /> Чат с куратором
        </button>
      </div>

      <button class="btn-ghost mt-6 text-sm text-ink-700/60" @click="logout">
        <AppIcon name="logout" :size="16" /> Выйти
      </button>
    </div>
  </div>
</template>
