<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import client from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { formatDate, formatPhone } from '../lib/format'
import { confirmDialog } from '../composables/confirm'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Заявки на регистрацию')

const router = useRouter()

function openCard(d) {
  router.push({ name: 'disciple', params: { id: d.id } })
}

const items = ref([])
const threadMap = ref({})
const loading = ref(true)
const approving = ref(null)

function nameOf(d) {
  return d.spiritual_name || d.material_name || 'Без имени'
}

async function load() {
  loading.value = true
  try {
    const [disciplesRes, threadsRes] = await Promise.all([
      client.get('/disciples', { params: { pending: true, limit: 200 } }),
      client.get('/threads', { params: { kind: 'approval' } }),
    ])
    items.value = disciplesRes.data.items || []
    const map = {}
    for (const t of (Array.isArray(threadsRes.data) ? threadsRes.data : [])) {
      map[t.disciple_id] = t
    }
    threadMap.value = map
  } finally {
    loading.value = false
  }
}

function openChat(d) {
  const t = threadMap.value[d.id]
  if (t) router.push({ name: 'thread', params: { id: t.id } })
}

async function approve(d) {
  const ok = await confirmDialog({
    message: `Сделать «${nameOf(d)}» кандидатом? Откроется доступ к кабинету.`,
    confirmText: 'В кандидаты',
    danger: false,
  })
  if (!ok) return
  approving.value = d.id
  try {
    await client.post(`/disciples/${d.id}/approve`)
    items.value = items.value.filter((x) => x.id !== d.id)
  } finally {
    approving.value = null
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="mb-6">
      <p class="text-ink-700/60">Самостоятельно зарегистрированные ученики, ожидающие подтверждения</p>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="card divide-y divide-parchment-100">
      <div v-for="i in 5" :key="i" class="flex items-center justify-between gap-4 p-4">
        <div class="space-y-2"><AppSkeleton w="w-48" /><AppSkeleton w="w-32" h="h-3" /></div>
        <div class="flex gap-2"><AppSkeleton w="w-24" h="h-9" /><AppSkeleton w="w-24" h="h-9" /></div>
      </div>
    </div>

    <!-- Empty -->
    <div v-else-if="!items.length" class="card p-10 text-center text-ink-700/50">
      Нет заявок на рассмотрении
    </div>

    <!-- List -->
    <div v-else class="card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead class="border-b border-parchment-200 bg-parchment-50 text-left text-xs uppercase tracking-wide text-ink-700/60">
            <tr>
              <th class="px-4 py-3">Имя</th>
              <th class="px-4 py-3">Телефон</th>
              <th class="px-4 py-3">Дата регистрации</th>
              <th class="px-4 py-3 text-right">Действия</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-parchment-100">
            <tr v-for="d in items" :key="d.id" class="hover:bg-parchment-50">
              <td class="px-4 py-3">
                <button class="font-medium text-ink-900 hover:text-saffron-700 hover:underline" @click="openCard(d)">{{ nameOf(d) }}</button>
              </td>
              <td class="px-4 py-3 text-ink-700">{{ d.phone ? formatPhone(d.phone) : '—' }}</td>
              <td class="px-4 py-3 text-ink-700">{{ formatDate(d.created_at) }}</td>
              <td class="px-4 py-3">
                <div class="flex items-center justify-end gap-2">
                  <button class="btn-outline" @click="openCard(d)">
                    <AppIcon name="disciples" :size="16" /> Анкета
                  </button>
                  <button
                    v-if="threadMap[d.id]"
                    class="btn-outline relative"
                    @click="openChat(d)"
                  >
                    <AppIcon name="chat" :size="16" /> Открыть чат
                    <span
                      v-if="threadMap[d.id].unread"
                      class="absolute -right-1 -top-1 h-2.5 w-2.5 rounded-full bg-saffron-500"
                      title="Новое сообщение"
                    ></span>
                  </button>
                  <button class="btn-primary whitespace-nowrap" :disabled="approving === d.id" @click="approve(d)">
                    {{ approving === d.id ? '…' : 'В кандидаты' }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
