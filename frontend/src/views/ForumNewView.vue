<script setup>
import { ref, onMounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import client from '../api/client'
import AppSelect from '../components/AppSelect.vue'
import MarkdownEditor from '../components/MarkdownEditor.vue'
import { usePageTitle } from '../composables/pageTitle'
import { backTarget } from '../composables/backTarget'

usePageTitle('Новая тема')
const route = useRoute()
const router = useRouter()
backTarget.value = { name: 'forum' }

const sections = ref([])
const sectionId = ref(route.query.section ? Number(route.query.section) : '')
const title = ref('')
const body = ref('')
const error = ref('')
const saving = ref(false)

const sectionOptions = ref([])

onMounted(async () => {
  try {
    const { data } = await client.get('/forum/sections')
    sections.value = data
    sectionOptions.value = data.map((s) => ({ value: s.id, label: s.title }))
    if (!sectionId.value && data.length) sectionId.value = data[0].id
  } catch { /* ignore */ }
})

async function submit() {
  error.value = ''
  if (!sectionId.value) { error.value = 'Выберите раздел'; return }
  if (!title.value.trim()) { error.value = 'Введите заголовок темы'; return }
  if (!body.value.trim()) { error.value = 'Напишите первое сообщение'; return }
  saving.value = true
  try {
    const { data } = await client.post('/forum/topics', { section_id: sectionId.value, title: title.value.trim(), body: body.value })
    try { await client.delete('/drafts/forum:new') } catch { /* игнор */ }
    router.push({ name: 'forum-topic', params: { id: data.id } })
  } catch (e) {
    error.value = e.response?.data?.detail || 'Не удалось создать тему'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <div v-if="!sections.length" class="card p-8 text-center text-ink-700/60">
      Сначала создайте раздел на форуме.
      <RouterLink :to="{ name: 'forum' }" class="text-saffron-700 hover:underline">К форуму</RouterLink>
    </div>
    <form v-else class="card space-y-4 p-6" @submit.prevent="submit">
      <div>
        <label class="label">Раздел *</label>
        <AppSelect v-model="sectionId" :options="sectionOptions" placeholder="Выберите раздел" />
      </div>
      <div>
        <label class="label">Заголовок темы *</label>
        <input v-model="title" class="input" required placeholder="О чём тема" />
      </div>
      <div>
        <label class="label">Первое сообщение *</label>
        <MarkdownEditor v-model="body" :rows="6" height-class="min-h-[35vh]" type-anywhere draft-scope="forum:new" placeholder="Текст… (можно вставлять фото)" />
      </div>
      <p v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>
      <div class="flex gap-2">
        <button class="btn-primary" :disabled="saving">{{ saving ? 'Создание…' : 'Создать тему' }}</button>
        <RouterLink :to="{ name: 'forum' }" class="btn-ghost">Отмена</RouterLink>
      </div>
    </form>
  </div>
</template>
