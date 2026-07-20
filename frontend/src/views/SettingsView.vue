<script setup>
import { ref, onMounted } from 'vue'
defineOptions({ name: 'SettingsView' })
import client from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Настройки')

const loading = ref(true)
const saving = ref(false)
const saved = ref(false)
const form = ref({ forum_edit_window_minutes: 60, auth_expire_days: 30, recording_enabled: true, recording_height: 720 })

// Локальная (для этого устройства) настройка — фоновая подгрузка разделов в кэш. Применяется сразу,
// не через кнопку «Сохранить» (это не серверная настройка).
const prefetchOn = ref(localStorage.getItem('apiPrefetch') !== '0')
function togglePrefetch() { try { localStorage.setItem('apiPrefetch', prefetchOn.value ? '1' : '0') } catch { /* ignore */ } }

async function load() {
  loading.value = true
  try {
    const { data } = await client.get('/settings')
    form.value = {
      forum_edit_window_minutes: data.forum_edit_window_minutes ?? 60,
      auth_expire_days: data.auth_expire_days ?? 30,
      recording_enabled: data.recording_enabled !== false,
      recording_height: data.recording_height ?? 720,
    }
  } finally {
    loading.value = false
  }
}
onMounted(load)

function step(key, delta, min) {
  const v = Number(form.value[key]) || 0
  form.value[key] = Math.max(min, v + delta)
}

async function save() {
  saving.value = true
  saved.value = false
  try {
    await client.put('/settings', {
      forum_edit_window_minutes: Number(form.value.forum_edit_window_minutes),
      auth_expire_days: Number(form.value.auth_expire_days),
      recording_enabled: !!form.value.recording_enabled,
      recording_height: Number(form.value.recording_height),
    })
    saved.value = true
    setTimeout(() => { saved.value = false }, 2500)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-2xl">
    <p class="mb-6 text-ink-700/60">Общие параметры работы приложения</p>

    <div v-if="loading" class="card space-y-4 p-6">
      <AppSkeleton w="w-56" /><AppSkeleton w="w-full" h="h-9" />
      <AppSkeleton w="w-56" /><AppSkeleton w="w-full" h="h-9" />
    </div>

    <template v-else>
      <!-- Форум -->
      <div class="card mb-4 p-6">
        <div class="mb-4 flex items-center gap-2">
          <AppIcon name="forum" :size="18" class="text-saffron-600" />
          <h2 class="font-display text-lg font-semibold text-ink-900">Форум</h2>
        </div>
        <label class="label">Сколько минут можно изменять/удалять своё сообщение</label>
        <div class="flex items-center gap-2">
          <div class="stepper">
            <button type="button" @click="step('forum_edit_window_minutes', -5, 0)">−</button>
            <input v-model.number="form.forum_edit_window_minutes" type="number" min="0" />
            <button type="button" @click="step('forum_edit_window_minutes', 5, 0)">+</button>
          </div>
          <span class="text-sm text-ink-700/60">минут</span>
        </div>
        <p class="mt-1.5 text-xs text-ink-700/50">По истечении этого времени автор больше не сможет изменить или удалить сообщение (модератор — всегда может). 0 — запретить сразу.</p>
      </div>

      <!-- Авторизация -->
      <div class="card mb-4 p-6">
        <div class="mb-4 flex items-center gap-2">
          <AppIcon name="shield" :size="18" class="text-saffron-600" />
          <h2 class="font-display text-lg font-semibold text-ink-900">Авторизация</h2>
        </div>
        <label class="label">Через сколько дней без входа теряется авторизация</label>
        <div class="flex items-center gap-2">
          <div class="stepper">
            <button type="button" @click="step('auth_expire_days', -1, 1)">−</button>
            <input v-model.number="form.auth_expire_days" type="number" min="1" />
            <button type="button" @click="step('auth_expire_days', 1, 1)">+</button>
          </div>
          <span class="text-sm text-ink-700/60">дней</span>
        </div>
        <p class="mt-1.5 text-xs text-ink-700/50">Скользящее окно: при каждом заходе на сайт срок продлевается заново. Если пользователь не заходил столько дней — потребуется войти снова.</p>
      </div>

      <!-- Запись конференций -->
      <div class="card mb-4 p-6">
        <div class="mb-4 flex items-center gap-2">
          <AppIcon name="video" :size="18" class="text-saffron-600" />
          <h2 class="font-display text-lg font-semibold text-ink-900">Запись конференций</h2>
        </div>
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="form.recording_enabled" /> Разрешить запись конференций</label>
        <p class="mb-4 mt-1.5 text-xs text-ink-700/50">Записи ведёт сервер (LiveKit Egress) и складывает в архив. На слабом сервере запись нагружает процессор — при частом использовании стоит добавить мощности.</p>
        <div :class="!form.recording_enabled && 'pointer-events-none opacity-50'">
          <label class="label">Качество записи</label>
          <div class="flex gap-2">
            <button v-for="q in [480, 720, 1080]" :key="q" type="button"
                    class="rounded-lg border px-3 py-1.5 text-sm transition"
                    :class="form.recording_height === q ? 'border-saffron-500 bg-saffron-50 font-semibold text-saffron-700' : 'border-parchment-300 text-ink-700 hover:bg-parchment-100'"
                    @click="form.recording_height = q">{{ q }}p</button>
          </div>
          <p class="mt-1.5 text-xs text-ink-700/50">Чем выше качество — тем больше нагрузка и размер файла. На этом сервере рекомендуется 480p или 720p.</p>
        </div>
      </div>

      <!-- Устройство (локально, применяется сразу) -->
      <div class="card mb-4 p-6">
        <div class="mb-4 flex items-center gap-2">
          <AppIcon name="settings" :size="18" class="text-saffron-600" />
          <h2 class="font-display text-lg font-semibold text-ink-900">Это устройство</h2>
        </div>
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="prefetchOn" @change="togglePrefetch" /> Фоновая подгрузка разделов</label>
        <p class="mt-1.5 text-xs text-ink-700/50">Заранее и в фоне подгружает данные разделов (ученики, форум, справочники) в кэш — при заходе они открываются мгновенно, без скелетонов. Отключите, чтобы экономить трафик.</p>
      </div>

      <div class="flex items-center gap-3">
        <button class="btn-primary" :disabled="saving" @click="save">{{ saving ? 'Сохранение…' : 'Сохранить' }}</button>
        <span v-if="saved" class="flex items-center gap-1 text-sm text-green-700"><AppIcon name="check" :size="15" /> Сохранено</span>
      </div>
    </template>
  </div>
</template>

<style scoped>
/* кастомный числовой степпер (без нативных стрелок) */
.stepper { display: inline-flex; align-items: stretch; overflow: hidden; border: 1px solid #d8c9b0; border-radius: 0.5rem; background: #fff; }
.stepper button {
  width: 2.5rem; font-size: 1.25rem; line-height: 1; color: #7a6a55;
  transition: background-color 0.12s ease;
}
.stepper button:hover { background: #f3ead9; color: #c8742a; }
.stepper input {
  width: 4.5rem; text-align: center; font-size: 0.95rem; font-weight: 600; color: #2b2320;
  border-left: 1px solid #e7dcc7; border-right: 1px solid #e7dcc7; outline: none; -moz-appearance: textfield;
}
.stepper input::-webkit-outer-spin-button,
.stepper input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
</style>
