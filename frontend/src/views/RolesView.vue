<script setup>
import { ref, computed, onMounted } from 'vue'
defineOptions({ name: 'RolesView' })
import client from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { confirmDialog } from '../composables/confirm'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Роли и доступ')

const roles = ref([]) // [{id, key, name, is_system, is_superadmin, is_default, capabilities: []}]
const catalog = ref([]) // [{group, items: [{key, label}]}]
const loading = ref(true)
const saving = ref(false)
const saved = ref(false)
const error = ref('')

// Локальная копия выбранной роли для редактирования.
// { id|null, name, key, is_default, is_system, is_superadmin, caps: {key: bool} }
const draft = ref(null)

const allCaps = computed(() => catalog.value.flatMap((g) => g.items.map((i) => i.key)))

function displayCount(role) {
  return role.is_superadmin ? allCaps.value.length : role.capabilities.length
}

const draftCount = computed(() => {
  if (!draft.value) return 0
  if (draft.value.is_superadmin) return allCaps.value.length
  return Object.values(draft.value.caps).filter(Boolean).length
})

function makeDraft(role) {
  const caps = {}
  for (const k of allCaps.value) caps[k] = role ? role.capabilities.includes(k) : false
  return {
    id: role?.id ?? null,
    name: role?.name ?? '',
    key: role?.key ?? '',
    is_default: role?.is_default ?? false,
    is_system: role?.is_system ?? false,
    is_superadmin: role?.is_superadmin ?? false,
    caps,
  }
}

async function load() {
  loading.value = true
  try {
    const [r, c] = await Promise.all([client.get('/roles'), client.get('/capabilities')])
    roles.value = r.data
    catalog.value = c.data
    // Если что-то было выбрано — переинициализируем из свежих данных.
    if (draft.value?.id) {
      const fresh = roles.value.find((x) => x.id === draft.value.id)
      draft.value = fresh ? makeDraft(fresh) : null
    }
  } finally {
    loading.value = false
  }
}

function selectRole(role) {
  draft.value = makeDraft(role)
  saved.value = false
  error.value = ''
}

function startNew() {
  draft.value = makeDraft(null)
  saved.value = false
  error.value = ''
}

const isSelected = (role) => draft.value?.id === role.id

function toggleCap(key) {
  if (draft.value.is_superadmin) return
  draft.value.caps[key] = !draft.value.caps[key]
  saved.value = false
}

function toggleDefault() {
  if (draft.value.is_superadmin) return
  draft.value.is_default = !draft.value.is_default
  saved.value = false
}

async function save() {
  if (!draft.value || draft.value.is_superadmin) return
  if (!draft.value.name.trim()) {
    error.value = 'Укажите название роли'
    return
  }
  saving.value = true
  error.value = ''
  try {
    const capabilities = allCaps.value.filter((k) => draft.value.caps[k])
    const body = {
      name: draft.value.name.trim(),
      capabilities,
      is_default: draft.value.is_default,
    }
    let res
    if (draft.value.id) {
      res = await client.put(`/roles/${draft.value.id}`, body)
    } else {
      res = await client.post('/roles', body)
    }
    await load()
    const savedRole = roles.value.find((x) => x.id === res.data.id)
    if (savedRole) draft.value = makeDraft(savedRole)
    saved.value = true
  } catch (e) {
    error.value = e.response?.data?.detail || 'Ошибка сохранения'
  } finally {
    saving.value = false
  }
}

async function remove() {
  if (!draft.value?.id || draft.value.is_system) return
  if (!(await confirmDialog({ message: `Удалить роль «${draft.value.name}»?` }))) return
  try {
    await client.delete(`/roles/${draft.value.id}`)
    draft.value = null
    await load()
  } catch (e) {
    error.value = e.response?.data?.detail || 'Ошибка удаления'
  }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <div class="mb-6 flex flex-wrap items-center justify-between gap-3">
      <p class="text-ink-700/60">Роли и наборы прав, которые они дают</p>
      <button class="btn-primary" :disabled="loading" @click="startNew">+ Новая роль</button>
    </div>

    <div v-if="loading" class="grid gap-6 lg:grid-cols-[300px_1fr]">
      <div class="card space-y-3 p-4">
        <AppSkeleton v-for="i in 4" :key="i" h="h-12" />
      </div>
      <div class="card space-y-3 p-6">
        <AppSkeleton v-for="i in 8" :key="i" h="h-6" />
      </div>
    </div>

    <div v-else class="grid gap-6 lg:grid-cols-[300px_1fr]">
      <!-- Список ролей -->
      <div class="card divide-y divide-parchment-100 self-start overflow-hidden">
        <button
          v-for="role in roles"
          :key="role.id"
          type="button"
          class="flex w-full items-center justify-between gap-2 px-4 py-3 text-left transition-colors"
          :class="isSelected(role) ? 'bg-saffron-500/10' : 'hover:bg-parchment-50'"
          @click="selectRole(role)"
        >
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <span class="truncate font-medium" :class="isSelected(role) ? 'text-saffron-700' : 'text-ink-800'">{{ role.name }}</span>
            </div>
            <div class="mt-1 flex flex-wrap items-center gap-1.5">
              <span v-if="role.is_system" class="badge bg-parchment-200 text-ink-700">системная</span>
              <span v-if="role.is_default" class="badge bg-saffron-500/15 text-saffron-700">по умолчанию</span>
              <span class="text-xs text-ink-700/50">{{ displayCount(role) }} прав</span>
            </div>
          </div>
          <AppIcon name="chevron" :size="16" class="shrink-0 -rotate-90 text-ink-700/30" />
        </button>
        <div v-if="!roles.length" class="px-4 py-6 text-center text-sm text-ink-700/50">Ролей пока нет</div>
      </div>

      <!-- Редактор -->
      <div v-if="!draft" class="card flex min-h-[200px] items-center justify-center p-6 text-center text-ink-700/50">
        Выберите роль слева или создайте новую
      </div>

      <div v-else class="card p-6">
        <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
          <div class="flex-1 min-w-[200px]">
            <label class="label">Название роли</label>
            <input
              v-model="draft.name"
              class="input"
              :disabled="draft.is_superadmin"
              placeholder="Например, Куратор региона"
              @input="saved = false"
            />
          </div>
          <div class="flex items-center gap-2 pt-6">
            <button
              v-if="!draft.is_superadmin"
              class="btn-primary"
              :disabled="saving"
              @click="save"
            >
              <AppIcon v-if="saved" name="check" :size="16" />
              {{ saved ? 'Сохранено' : (saving ? 'Сохранение…' : 'Сохранить') }}
            </button>
            <button
              v-if="draft.id && !draft.is_system"
              class="btn-outline text-red-600 hover:bg-red-50"
              @click="remove"
            >
              Удалить
            </button>
          </div>
        </div>

        <p v-if="draft.is_superadmin" class="mb-4 rounded-md bg-parchment-100 px-3 py-2 text-sm text-ink-700/70">
          Роль «{{ draft.name }}» имеет полный доступ ко всем возможностям и не редактируется.
        </p>
        <p v-if="error" class="mb-4 rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>

        <!-- Роль по умолчанию -->
        <label
          class="mb-6 flex items-center justify-between gap-3 rounded-lg border border-parchment-200 bg-parchment-50 px-4 py-3"
          :class="draft.is_superadmin ? 'opacity-60' : 'cursor-pointer'"
        >
          <div>
            <div class="font-medium text-ink-800">Роль по умолчанию</div>
            <div class="text-xs text-ink-700/50">Выдаётся при регистрации</div>
          </div>
          <button
            type="button"
            :disabled="draft.is_superadmin"
            class="inline-flex h-6 w-11 shrink-0 items-center rounded-full transition"
            :class="[
              draft.is_default ? 'bg-saffron-500' : 'bg-parchment-300',
              draft.is_superadmin ? 'cursor-not-allowed' : 'hover:opacity-90',
            ]"
            @click="toggleDefault"
          >
            <span
              class="h-5 w-5 transform rounded-full bg-white shadow transition"
              :class="draft.is_default ? 'translate-x-5' : 'translate-x-0.5'"
            ></span>
          </button>
        </label>

        <!-- Каталог возможностей -->
        <div class="mb-3 flex items-center justify-between">
          <h3 class="font-display text-lg text-ink-900">Возможности</h3>
          <span class="text-sm text-ink-700/50">{{ draftCount }} из {{ allCaps.length }}</span>
        </div>

        <div class="space-y-6">
          <div v-for="grp in catalog" :key="grp.group">
            <div class="mb-2 text-xs font-medium uppercase tracking-wide text-ink-700/50">{{ grp.group }}</div>
            <div class="divide-y divide-parchment-100 overflow-hidden rounded-lg border border-parchment-200">
              <div
                v-for="item in grp.items"
                :key="item.key"
                class="flex items-center justify-between gap-3 px-4 py-2.5"
              >
                <span class="text-sm text-ink-800">{{ item.label }}</span>
                <button
                  type="button"
                  :disabled="draft.is_superadmin"
                  class="inline-flex h-6 w-11 shrink-0 items-center rounded-full transition"
                  :class="[
                    (draft.is_superadmin || draft.caps[item.key]) ? 'bg-saffron-500' : 'bg-parchment-300',
                    draft.is_superadmin ? 'cursor-not-allowed opacity-60' : 'hover:opacity-90',
                  ]"
                  @click="toggleCap(item.key)"
                >
                  <span
                    class="h-5 w-5 transform rounded-full bg-white shadow transition"
                    :class="(draft.is_superadmin || draft.caps[item.key]) ? 'translate-x-5' : 'translate-x-0.5'"
                  ></span>
                </button>
              </div>
            </div>
          </div>
          <div v-if="!catalog.length" class="text-sm text-ink-700/50">Каталог возможностей пуст</div>
        </div>
      </div>
    </div>
  </div>
</template>
