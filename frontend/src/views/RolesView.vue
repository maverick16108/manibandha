<script setup>
import { ref, computed, onMounted } from 'vue'
import client from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { ROLE_LABELS } from '../lib/format'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Роли и доступ')

const sections = ref([]) // [[key, label], ...]
const matrix = ref({}) // { role: { section: bool } }
const loading = ref(true)
const saving = ref(false)
const saved = ref(false)

// порядок колонок-ролей
const ROLE_ORDER = ['guru', 'secretary', 'curator', 'student']
const roleCols = computed(() => ROLE_ORDER.filter((r) => r in matrix.value))

async function load() {
  loading.value = true
  try {
    const { data } = await client.get('/permissions')
    sections.value = data.sections
    matrix.value = data.matrix
  } finally {
    loading.value = false
  }
}

function toggle(role, key) {
  if (role === 'guru') return // гуру всегда полный доступ
  matrix.value[role][key] = !matrix.value[role][key]
  saved.value = false
}

async function save() {
  saving.value = true
  try {
    const { data } = await client.put('/permissions', { matrix: matrix.value })
    matrix.value = data
    saved.value = true
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <div class="mb-6 flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="text-ink-700/60">Какие разделы видят и используют предопределённые роли</p>
      </div>
      <button class="btn-primary" :disabled="saving || loading" @click="save">
        <AppIcon v-if="saved" name="check" :size="16" /> {{ saved ? 'Сохранено' : (saving ? 'Сохранение…' : 'Сохранить') }}
      </button>
    </div>

    <div class="card overflow-hidden">
      <div v-if="loading" class="space-y-3 p-6">
        <AppSkeleton v-for="i in 6" :key="i" h="h-6" />
      </div>
      <div v-else class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead class="border-b border-parchment-200 bg-parchment-50 text-left text-xs uppercase tracking-wide text-ink-700/60">
            <tr>
              <th class="px-5 py-3">Раздел</th>
              <th v-for="r in roleCols" :key="r" class="px-4 py-3 text-center">
                {{ ROLE_LABELS[r] || r }}
                <span v-if="r === 'guru'" class="block text-[10px] font-normal normal-case text-ink-700/40">полный доступ</span>
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-parchment-100">
            <tr v-for="[key, label] in sections" :key="key" class="hover:bg-parchment-50">
              <td class="px-5 py-3 font-medium text-ink-800">{{ label }}</td>
              <td v-for="r in roleCols" :key="r" class="px-4 py-3 text-center">
                <button
                  type="button"
                  :disabled="r === 'guru'"
                  class="inline-flex h-6 w-11 items-center rounded-full transition"
                  :class="[
                    matrix[r][key] ? 'bg-saffron-500' : 'bg-parchment-300',
                    r === 'guru' ? 'cursor-not-allowed opacity-60' : 'hover:opacity-90',
                  ]"
                  @click="toggle(r, key)"
                >
                  <span
                    class="h-5 w-5 transform rounded-full bg-white shadow transition"
                    :class="matrix[r][key] ? 'translate-x-5' : 'translate-x-0.5'"
                  ></span>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <p class="mt-4 text-sm text-ink-700/50">
      Гуру всегда имеет полный доступ ко всем разделам и не редактируется.
      Изменения вступают в силу после сохранения и следующего входа пользователя.
    </p>
  </div>
</template>
