<script setup>
import { ref } from 'vue'
import { useRouter, useRoute, RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const portrait = '/guru/1.jpg'
const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

const email = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(email.value, password.value)
    router.push(route.query.redirect || { name: 'dashboard' })
  } catch (e) {
    error.value = e.response?.data?.detail || 'Не удалось войти. Проверьте email и пароль.'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen">
    <!-- Left: B&W portrait -->
    <div class="relative hidden w-1/2 lg:block">
      <img :src="portrait" alt="Манибандха Прабху" class="photo-bw h-full w-full object-cover object-center" />
      <div class="absolute inset-0 bg-gradient-to-t from-ink-900/80 via-ink-900/30 to-transparent"></div>
      <div class="absolute bottom-0 p-12 text-white">
        <h2 class="font-display text-4xl font-semibold">Манибандха Прабху</h2>
        <p class="mt-2 font-serif text-lg italic text-parchment-100/90">Кабинет учеников и служения</p>
      </div>
    </div>

    <!-- Right: login form -->
    <div class="flex w-full items-center justify-center bg-parchment-100 px-6 lg:w-1/2">
      <div class="w-full max-w-sm">
        <div class="mb-8 text-center">
          <div class="mb-3 text-4xl">🪷</div>
          <h1 class="font-display text-3xl font-semibold text-ink-900">Вход в кабинет</h1>
          <p class="mt-2 text-sm text-ink-700/70">Учёт учеников · Манибандха Прабху</p>
        </div>

        <form class="space-y-4" @submit.prevent="submit">
          <div>
            <label class="label">Email</label>
            <input v-model="email" type="email" autocomplete="username" class="input" placeholder="you@example.com" required />
          </div>
          <div>
            <label class="label">Пароль</label>
            <input v-model="password" type="password" autocomplete="current-password" class="input" placeholder="••••••••" required />
          </div>

          <p v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>

          <button type="submit" class="btn-primary w-full" :disabled="loading">
            {{ loading ? 'Вход…' : 'Войти' }}
          </button>
        </form>

        <RouterLink to="/" class="mt-6 block text-center text-sm text-saffron-600 hover:text-saffron-700">
          ← На главную
        </RouterLink>
      </div>
    </div>
  </div>
</template>
