<script setup>
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import client from '../api/client'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Присоединение к группе')
const route = useRoute(); const router = useRouter()
const error = ref('')

onMounted(async () => {
  try {
    const { data } = await client.post(`/chats/join/${route.params.token}`)
    router.replace({ name: 'chat', params: { id: String(data.id) } })
  } catch (e) {
    error.value = e.response?.data?.detail || 'Ссылка недействительна или устарела'
  }
})
</script>

<template>
  <div class="flex min-h-[50vh] flex-col items-center justify-center gap-4 text-center">
    <template v-if="error">
      <p class="text-lg text-ink-900">{{ error }}</p>
      <RouterLink :to="{ name: 'chat-home' }" class="btn-primary">К чатам</RouterLink>
    </template>
    <template v-else>
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-saffron-500/30 border-t-saffron-500"></div>
      <p class="text-ink-700/60">Присоединяемся к группе…</p>
    </template>
  </div>
</template>
