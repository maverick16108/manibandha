<script setup>
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppIcon from '../components/AppIcon.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

onMounted(async () => {
  const code = route.params.code
  try {
    const { data } = await client.get(`/conferences/by-code/${code}`)
    if (auth.isAuthenticated) {
      router.replace({ name: 'conference-room', params: { id: data.id } })
    } else if (data.guests_allowed) {
      router.replace({ name: 'conference-guest', params: { room: data.room } })
    } else {
      router.replace({ name: 'login', query: { redirect: `/c/${code}` } })
    }
  } catch {
    router.replace(auth.isAuthenticated ? { name: 'conference' } : { name: 'home' })
  }
})
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-parchment-100 text-ink-700/60">
    <div class="text-center">
      <AppIcon name="video" :size="40" class="mx-auto mb-3 animate-pulse text-saffron-500" />
      Открываем конференцию…
    </div>
  </div>
</template>
