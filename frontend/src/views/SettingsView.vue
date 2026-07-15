<script setup>
import { ref, onMounted } from 'vue'
import client from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Настройки')

const loading = ref(true)
const saving = ref(false)
const saved = ref(false)
const form = ref({ forum_edit_window_minutes: 60, auth_expire_days: 30 })

async function load() {
  loading.value = true
  try {
    const { data } = await client.get('/settings')
    form.value = {
      forum_edit_window_minutes: data.forum_edit_window_minutes ?? 60,
      auth_expire_days: data.auth_expire_days ?? 30,
    }
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function save() {
  saving.value = true
  saved.value = false
  try {
    await client.put('/settings', {
      forum_edit_window_minutes: Number(form.value.forum_edit_window_minutes),
      auth_expire_days: Number(form.value.auth_expire_days),
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
          <input v-model.number="form.forum_edit_window_minutes" type="number" min="0" class="input w-40" />
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
          <input v-model.number="form.auth_expire_days" type="number" min="1" class="input w-40" />
          <span class="text-sm text-ink-700/60">дней</span>
        </div>
        <p class="mt-1.5 text-xs text-ink-700/50">Скользящее окно: при каждом заходе на сайт срок продлевается заново. Если пользователь не заходил столько дней — потребуется войти снова.</p>
      </div>

      <div class="flex items-center gap-3">
        <button class="btn-primary" :disabled="saving" @click="save">{{ saving ? 'Сохранение…' : 'Сохранить' }}</button>
        <span v-if="saved" class="flex items-center gap-1 text-sm text-green-700"><AppIcon name="check" :size="15" /> Сохранено</span>
      </div>
    </template>
  </div>
</template>
