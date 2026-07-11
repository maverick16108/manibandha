<script setup>
import { ref, onBeforeUnmount } from 'vue'
import { useRouter, useRoute, RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'

import AppIcon from '../components/AppIcon.vue'
import PhoneInput from '../components/PhoneInput.vue'
import OtpInput from '../components/OtpInput.vue'
import { formatPhone } from '../lib/format'

const portrait = '/guru/1.jpg'
const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

// 'login' | 'register' — обе вкладки работают по SMS.
// Админский вход (email+пароль) скрыт — доступен только по адресу /login?admin
const mode = ref('login')
const adminMode = ref(route.query.admin !== undefined)

// --- Phone auth ---
const phone = ref('')
const code = ref('')
const step = ref(1) // 1: ввод телефона, 2: ввод кода
const phoneError = ref('')
const phoneLoading = ref(false)
const resent = ref(false)

// --- Resend cooldown ---
const RESEND_COOLDOWN = 60
const secondsLeft = ref(0)
let cooldownTimer = null

function stopCooldown() {
  if (cooldownTimer) {
    clearInterval(cooldownTimer)
    cooldownTimer = null
  }
}

function startCooldown() {
  stopCooldown()
  secondsLeft.value = RESEND_COOLDOWN
  cooldownTimer = setInterval(() => {
    secondsLeft.value -= 1
    if (secondsLeft.value <= 0) {
      secondsLeft.value = 0
      stopCooldown()
    }
  }, 1000)
}

onBeforeUnmount(stopCooldown)

function switchMode(m) {
  if (m === mode.value) return
  mode.value = m
  step.value = 1
  code.value = ''
  phoneError.value = ''
  resent.value = false
  stopCooldown()
  secondsLeft.value = 0
}

async function requestCode() {
  phoneError.value = ''
  phoneLoading.value = true
  try {
    const r = await auth.requestPhoneCode(phone.value)
    if (mode.value === 'login' && r.exists === false) {
      phoneError.value = 'Этот номер не зарегистрирован — перейдите на «Регистрация».'
      return
    }
    if (mode.value === 'register' && r.exists === true) {
      phoneError.value = 'Этот номер уже зарегистрирован — перейдите на «Вход».'
      return
    }
    step.value = 2
    startCooldown()
  } catch (e) {
    phoneError.value = e.response?.data?.detail || 'Не удалось отправить код. Проверьте номер.'
  } finally {
    phoneLoading.value = false
  }
}

async function resendCode() {
  if (secondsLeft.value > 0 || phoneLoading.value) return
  phoneError.value = ''
  resent.value = false
  phoneLoading.value = true
  try {
    await auth.requestPhoneCode(phone.value)
    resent.value = true
    startCooldown()
  } catch (e) {
    phoneError.value = e.response?.data?.detail || 'Не удалось отправить код повторно.'
  } finally {
    phoneLoading.value = false
  }
}

async function verifyCode() {
  if (code.value.length < 4 || phoneLoading.value) return
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
  resent.value = false
  stopCooldown()
  secondsLeft.value = 0
}

// --- Admin email auth (скрытый вход) ---
const email = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(email.value, password.value)
    router.push(route.query.redirect || '/app')
  } catch (e) {
    error.value = e.response?.data?.detail || 'Не удалось войти. Проверьте email и пароль.'
  } finally {
    loading.value = false
  }
}

function closeAdmin() {
  adminMode.value = false
  error.value = ''
  router.replace({ path: '/login' })
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
    <div class="relative flex w-full items-center justify-center bg-parchment-100 px-6 lg:w-1/2">
      <RouterLink to="/" class="absolute left-5 top-5 inline-flex items-center gap-1 text-sm text-ink-700/70 transition hover:text-saffron-700">
        <AppIcon name="chevron" :size="16" class="rotate-90" /> На главную
      </RouterLink>
      <div class="w-full max-w-sm">
        <div class="mb-8 text-center">
          <img src="/lotus-mark.png" alt="" class="mx-auto mb-3 h-11 w-auto" />
          <h1 class="font-display text-3xl font-semibold text-ink-900">Вход в кабинет</h1>
          <p class="mt-2 text-sm text-ink-700/70">для учеников Манибандхи Прабху</p>
        </div>

        <!-- Admin email form (hidden entrance) -->
        <template v-if="adminMode">
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

          <button type="button" class="mt-6 block w-full text-center text-sm text-saffron-600 hover:text-saffron-700" @click="closeAdmin">
            ← Назад ко входу по телефону
          </button>
        </template>

        <!-- Phone auth (login / register tabs) -->
        <template v-else>
          <!-- Tabs -->
          <div class="mb-6 flex rounded-lg border border-parchment-300 bg-parchment-50 p-1">
            <button
              type="button"
              class="flex-1 rounded-md px-3 py-2 text-sm font-medium transition"
              :class="mode === 'login' ? 'bg-saffron-500 text-white shadow-sm' : 'text-ink-700/70 hover:text-ink-900'"
              @click="switchMode('login')"
            >
              Вход
            </button>
            <button
              type="button"
              class="flex-1 rounded-md px-3 py-2 text-sm font-medium transition"
              :class="mode === 'register' ? 'bg-saffron-500 text-white shadow-sm' : 'text-ink-700/70 hover:text-ink-900'"
              @click="switchMode('register')"
            >
              Регистрация
            </button>
          </div>

          <!-- Step 1: enter phone -->
          <form v-if="step === 1" class="space-y-4" @submit.prevent="requestCode">
            <div>
              <label class="label">Телефон</label>
              <PhoneInput v-model="phone" />
              <p class="mt-1.5 text-xs text-ink-700/60">Отправим SMS с кодом.</p>
            </div>

            <p v-if="phoneError" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ phoneError }}</p>

            <button type="submit" class="btn-primary w-full" :disabled="phoneLoading || !phone">
              <template v-if="phoneLoading">Отправляем…</template>
              <template v-else>{{ mode === 'login' ? 'Войти' : 'Зарегистрироваться' }}</template>
            </button>

            <p v-if="mode === 'register'" class="rounded-md bg-parchment-50 px-3 py-2 text-center text-xs text-ink-700/70">
              Аккаунт создастся автоматически. После входа заполните анкету.
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
              <OtpInput v-model="code" :length="4" @complete="verifyCode" />
            </div>

            <p v-if="resent" class="rounded-md bg-sage-500/10 px-3 py-2 text-sm text-sage-600">Код отправлен</p>
            <p v-if="phoneError" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ phoneError }}</p>

            <button type="submit" class="btn-primary w-full" :disabled="phoneLoading || code.length < 4">
              {{ phoneLoading ? 'Входим…' : 'Войти' }}
            </button>

            <button type="button" class="btn-ghost w-full text-sm" :disabled="phoneLoading || secondsLeft > 0" @click="resendCode">
              <template v-if="secondsLeft > 0">Отправить код повторно через {{ secondsLeft }} с</template>
              <template v-else>Отправить код повторно</template>
            </button>
          </form>
        </template>

      </div>
    </div>
  </div>
</template>
