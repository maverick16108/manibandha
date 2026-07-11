<script setup>
import { ref } from 'vue'
import { useRouter, useRoute, RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'

import AppIcon from '../components/AppIcon.vue'
import PhoneInput from '../components/PhoneInput.vue'
import { formatPhone } from '../lib/format'

const portrait = '/guru/1.jpg'
const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

// 'phone' — основной способ (SMS), 'staff' — вход по email для персонала
const mode = ref('phone')

// --- Phone auth ---
const phone = ref('')
const code = ref('')
const step = ref(1) // 1: ввод телефона, 2: ввод кода
const phoneError = ref('')
const phoneLoading = ref(false)

async function requestCode() {
  phoneError.value = ''
  phoneLoading.value = true
  try {
    await auth.requestPhoneCode(phone.value)
    step.value = 2
  } catch (e) {
    phoneError.value = e.response?.data?.detail || 'Не удалось отправить код. Проверьте номер.'
  } finally {
    phoneLoading.value = false
  }
}

async function resendCode() {
  phoneError.value = ''
  phoneLoading.value = true
  try {
    await auth.requestPhoneCode(phone.value)
  } catch (e) {
    phoneError.value = e.response?.data?.detail || 'Не удалось отправить код повторно.'
  } finally {
    phoneLoading.value = false
  }
}

async function verifyCode() {
  phoneError.value = ''
  phoneLoading.value = true
  try {
    await auth.loginByPhone(phone.value, code.value)
    router.push('/app')
  } catch (e) {
    phoneError.value = e.response?.data?.detail || 'Неверный или устаревший код.'
  } finally {
    phoneLoading.value = false
  }
}

function editPhone() {
  step.value = 1
  code.value = ''
  phoneError.value = ''
}

// --- Staff email auth (existing) ---
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
          <AppIcon name="lotus" :size="44" class="mx-auto mb-3 text-saffron-500" />
          <h1 class="font-display text-3xl font-semibold text-ink-900">Вход в кабинет</h1>
          <p class="mt-2 text-sm text-ink-700/70">для учеников Манибандхи Прабху</p>
        </div>

        <!-- Mode toggle -->
        <div class="mb-6 flex rounded-lg border border-parchment-300 bg-parchment-50 p-1">
          <button
            type="button"
            class="flex-1 rounded-md px-3 py-2 text-sm font-medium transition"
            :class="mode === 'phone' ? 'bg-saffron-500 text-white shadow-sm' : 'text-ink-700/70 hover:text-ink-900'"
            @click="mode = 'phone'"
          >
            По телефону
          </button>
          <button
            type="button"
            class="flex-1 rounded-md px-3 py-2 text-sm font-medium transition"
            :class="mode === 'staff' ? 'bg-saffron-500 text-white shadow-sm' : 'text-ink-700/70 hover:text-ink-900'"
            @click="mode = 'staff'"
          >
            Для персонала
          </button>
        </div>

        <!-- Phone mode -->
        <div v-if="mode === 'phone'">
          <!-- Step 1: enter phone -->
          <form v-if="step === 1" class="space-y-4" @submit.prevent="requestCode">
            <div>
              <label class="label">Телефон</label>
              <PhoneInput v-model="phone" />
              <p class="mt-1.5 text-xs text-ink-700/60">Отправим SMS с кодом.</p>
            </div>

            <p v-if="phoneError" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ phoneError }}</p>

            <button type="submit" class="btn-primary w-full" :disabled="phoneLoading || !phone">
              {{ phoneLoading ? 'Отправляем…' : 'Получить код' }}
            </button>

            <p class="rounded-md bg-parchment-50 px-3 py-2 text-center text-xs text-ink-700/70">
              Впервые? Просто введите телефон — аккаунт создастся автоматически.
            </p>
          </form>

          <!-- Step 2: enter code -->
          <form v-else class="space-y-4" @submit.prevent="verifyCode">
            <div class="flex items-center justify-between text-sm">
              <span class="text-ink-700">Код отправлен на <span class="font-medium text-ink-900">{{ formatPhone(phone) }}</span></span>
              <button type="button" class="text-saffron-600 hover:text-saffron-700" @click="editPhone">Изменить</button>
            </div>

            <div>
              <label class="label">Код из SMS</label>
              <input
                v-model="code"
                inputmode="numeric"
                autocomplete="one-time-code"
                maxlength="4"
                class="input text-center text-2xl tracking-[0.5em]"
                placeholder="0000"
                required
              />
            </div>

            <p v-if="phoneError" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ phoneError }}</p>

            <button type="submit" class="btn-primary w-full" :disabled="phoneLoading || code.length < 4">
              {{ phoneLoading ? 'Входим…' : 'Войти' }}
            </button>

            <button type="button" class="btn-ghost w-full text-sm" :disabled="phoneLoading" @click="resendCode">
              Отправить код повторно
            </button>
          </form>
        </div>

        <!-- Staff email mode (existing) -->
        <form v-else class="space-y-4" @submit.prevent="submit">
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
