<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
defineOptions({ name: 'ProfileView' })
import { useAuthStore } from '../stores/auth'
import { usePageTitle } from '../composables/pageTitle'
import { ROLE_LABELS, formatPhone, MARITAL_LABELS, thumbUrl, imgFull } from '../lib/format'
import client from '../api/client'
import PhotoUpload from '../components/PhotoUpload.vue'
import AppSelect from '../components/AppSelect.vue'
import PhoneInput from '../components/PhoneInput.vue'
import AppIcon from '../components/AppIcon.vue'

usePageTitle('Профиль')

const auth = useAuthStore()
const form = reactive({
  full_name: auth.user?.full_name || '',
  avatar_url: auth.user?.avatar_url || '',
})
// поля из анкеты ученика (семейное положение + контакты)
const disc = reactive({ marital_status: '', phone: '', email: '', messenger: '' })
const hasDisciple = ref(false)
const saving = ref(false)
const saved = ref(false)

const maritalOptions = [{ value: '', label: '—' }, ...Object.entries(MARITAL_LABELS).map(([value, label]) => ({ value, label }))]

// контакт: для телефонных аккаунтов — телефон, синтетический email @phone.local не показываем
const contact = computed(() => {
  const u = auth.user
  if (!u) return ''
  if (u.email && u.email.endsWith('@phone.local')) return u.phone ? formatPhone(u.phone) : ''
  return u.email || (u.phone ? formatPhone(u.phone) : '')
})

onMounted(async () => {
  const did = auth.user?.disciple_id
  if (!did) return
  try {
    const { data } = await client.get(`/disciples/${did}`)
    disc.marital_status = data.marital_status || ''
    disc.phone = data.phone || ''
    disc.email = data.email || ''
    disc.messenger = data.messenger || ''
    hasDisciple.value = true
  } catch { /* нет доступа к анкете — просто не показываем блок */ }
})

async function save() {
  saving.value = true
  saved.value = false
  try {
    await auth.updateProfile({ full_name: form.full_name, avatar_url: form.avatar_url })
    if (hasDisciple.value && auth.user?.disciple_id) {
      await client.patch(`/disciples/${auth.user.disciple_id}`, {
        marital_status: disc.marital_status || null,
        phone: disc.phone || null,
        email: disc.email || null,
        messenger: disc.messenger || null,
      })
    }
    saved.value = true
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-2xl">
    <div class="card p-6">
      <div class="flex items-center gap-4">
        <img v-if="form.avatar_url" :src="thumbUrl(form.avatar_url)" @error="imgFull($event, form.avatar_url)" class="photo-bw h-20 w-20 rounded-full object-cover" />
        <span v-else class="flex h-20 w-20 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-2xl font-semibold text-white">
          {{ (auth.user?.full_name || '?').trim()[0] }}
        </span>
        <div>
          <div class="font-display text-2xl font-semibold text-ink-900">{{ auth.user?.full_name }}</div>
          <div class="text-sm text-ink-700/60">{{ ROLE_LABELS[auth.user?.role] || auth.user?.role }}<span v-if="contact"> · {{ contact }}</span></div>
        </div>
      </div>

      <div class="mt-6 space-y-5">
        <div>
          <label class="label">Аватар / фото</label>
          <PhotoUpload v-model="form.avatar_url" />
        </div>
        <div>
          <label class="label">Имя</label>
          <input v-model="form.full_name" class="input" placeholder="Ваше имя" />
        </div>

        <template v-if="hasDisciple">
          <div>
            <label class="label">Семейное положение</label>
            <AppSelect v-model="disc.marital_status" :options="maritalOptions" placeholder="—" />
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="label">Телефон</label>
              <PhoneInput v-model="disc.phone" />
            </div>
            <div>
              <label class="label">Email</label>
              <input v-model="disc.email" class="input" placeholder="you@example.com" />
            </div>
          </div>
          <div>
            <label class="label">Мессенджер</label>
            <input v-model="disc.messenger" class="input" placeholder="Telegram / WhatsApp" />
          </div>
        </template>
      </div>

      <div class="mt-6 flex items-center gap-3">
        <button class="btn-primary" :disabled="saving" @click="save">
          <AppIcon v-if="saved" name="check" :size="16" /> {{ saved ? 'Сохранено' : (saving ? 'Сохранение…' : 'Сохранить') }}
        </button>
      </div>
    </div>
  </div>
</template>
