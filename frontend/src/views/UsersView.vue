<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import client from '../api/client'
import AppSelect from '../components/AppSelect.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import AppIcon from '../components/AppIcon.vue'
import PhoneInput from '../components/PhoneInput.vue'
import { confirmDialog } from '../composables/confirm'
import { ROLE_LABELS } from '../lib/format'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Пользователи')

const roleOptions = Object.entries(ROLE_LABELS).map(([value, label]) => ({ value, label }))
const loading = ref(true)
const nameInput = ref(null)
function focusName() { nextTick(() => nameInput.value?.focus()) }

const users = ref([])
const roles = ref([]) // [{id, name, ...}] — динамические роли для назначения
const search = ref('')
const roleFilter = ref('')
const roleFilterOptions = computed(() => [{ value: '', label: 'Все роли' }, ...roleOptions])
// legacy-роль (enum) выводим из выбранных ролей — для доступа к анкетам/скоупу на бэке
const primaryRole = computed(() => {
  const keys = roles.value.filter((r) => form.role_ids.includes(r.id)).map((r) => r.key)
  if (keys.includes('superadmin') || keys.includes('guru')) return 'guru'
  if (keys.includes('secretary')) return 'secretary'
  if (keys.includes('curator')) return 'curator'
  return 'student'
})
const filteredUsers = computed(() => {
  const q = search.value.trim().toLowerCase()
  return users.value.filter((u) => {
    if (roleFilter.value && u.role !== roleFilter.value) return false
    if (!q) return true
    return [u.full_name, u.email, u.phone].some((f) => (f || '').toString().toLowerCase().includes(q))
  })
})
const showForm = ref(false)
const editing = ref(null)
const error = ref('')
const form = reactive({ email: '', full_name: '', phone: '', role: 'secretary', password: '', is_active: true, disciple_id: '', role_ids: [] })
const disciples = ref([])
const discipleOptions = computed(() => [{ value: '', label: '— не привязан —' }, ...disciples.value.map((d) => ({ value: d.id, label: d.spiritual_name || d.material_name }))])

async function load() {
  loading.value = true
  try {
    const [u, d, r] = await Promise.all([
      client.get('/users'),
      client.get('/disciples', { params: { named: true, limit: 500 } }),
      client.get('/roles'),
    ])
    users.value = u.data
    disciples.value = d.data.items
    roles.value = r.data
  } finally {
    loading.value = false
  }
}

function toggleRole(id) {
  const i = form.role_ids.indexOf(id)
  if (i === -1) form.role_ids.push(id)
  else form.role_ids.splice(i, 1)
}

function startNew() {
  editing.value = null
  Object.assign(form, { email: '', full_name: '', phone: '', role: 'secretary', password: '', is_active: true, disciple_id: '', role_ids: [] })
  error.value = ''
  showForm.value = true
  focusName()
}

async function startEdit(u) {
  editing.value = u.id
  Object.assign(form, { email: u.email, full_name: u.full_name, phone: u.phone ?? '', role: u.role, password: '', is_active: u.is_active, disciple_id: u.disciple_id ?? '', role_ids: [] })
  error.value = ''
  showForm.value = true
  focusName()
  try {
    const { data } = await client.get(`/users/${u.id}/roles`)
    form.role_ids = data.role_ids || []
  } catch {
    form.role_ids = []
  }
  // старый пользователь без назначенных ролей — подставим чип по его legacy-роли, чтобы не сбросить
  if (!form.role_ids.length && u.role) {
    const m = roles.value.find((r) => r.key === u.role)
    if (m) form.role_ids = [m.id]
  }
}

const searchInput = ref(null)
function onKey(e) {
  if (e.key === 'Escape' && showForm.value) { showForm.value = false; return }
  if (showForm.value || e.ctrlKey || e.metaKey || e.altKey) return
  const t = e.target
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
  if (e.key === 'Escape') { search.value = ''; return }
  if (e.key === 'Backspace') { if (search.value) { search.value = search.value.slice(0, -1); e.preventDefault() }; return }
  if (e.key.length === 1) { search.value += e.key; e.preventDefault(); nextTick(() => searchInput.value?.focus()) }
}
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))

async function save() {
  error.value = ''
  try {
    const discipleId = form.disciple_id || null
    const role = primaryRole.value
    let userId
    if (editing.value) {
      const payload = { full_name: form.full_name, phone: form.phone || null, role, is_active: form.is_active, disciple_id: discipleId }
      if (form.password) payload.password = form.password
      await client.patch(`/users/${editing.value}`, payload)
      userId = editing.value
    } else {
      const { role_ids, ...userPayload } = form
      const { data } = await client.post('/users', { ...userPayload, role, disciple_id: discipleId })
      userId = data.id
    }
    await client.put(`/users/${userId}/roles`, { role_ids: form.role_ids })
    showForm.value = false
    await load()
  } catch (e) {
    error.value = e.response?.data?.detail || 'Ошибка сохранения'
  }
}

async function remove(u) {
  if (!(await confirmDialog({ message: `Удалить пользователя ${u.full_name}?` }))) return
  await client.delete(`/users/${u.id}`)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <div class="mb-4 flex flex-wrap items-center gap-3">
      <button class="btn-primary shrink-0" @click="startNew">+ Добавить</button>
      <div class="relative min-w-0 flex-1 sm:max-w-xs">
        <AppIcon name="disciples" :size="16" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-700/40" />
        <input ref="searchInput" v-model="search" class="input pl-9" placeholder="Поиск по имени, email, телефону" />
      </div>
      <div class="w-44 shrink-0"><AppSelect v-model="roleFilter" :options="roleFilterOptions" /></div>
    </div>

    <div class="card divide-y divide-parchment-100">
      <template v-if="loading">
        <div v-for="i in 4" :key="'s' + i" class="flex items-center justify-between p-4">
          <div class="space-y-2"><AppSkeleton w="w-40" /><AppSkeleton w="w-56" h="h-3" /></div>
          <AppSkeleton w="w-20" h="h-8" />
        </div>
      </template>
      <div v-if="!loading && !filteredUsers.length" class="p-8 text-center text-ink-700/50">Никого не найдено</div>
      <div v-for="u in filteredUsers" :key="u.id" v-show="!loading" class="flex items-center justify-between p-4">
        <div>
          <div class="font-medium text-ink-900">{{ u.full_name }} <span v-if="!u.is_active" class="badge bg-red-100 text-red-700">отключён</span></div>
          <div class="text-sm text-ink-700/60">{{ u.email }} · {{ ROLE_LABELS[u.role] }}</div>
        </div>
        <div class="flex gap-2">
          <button class="btn-ghost" @click="startEdit(u)">Изменить</button>
          <button class="text-ink-700/40 hover:text-red-600" @click="remove(u)">✕</button>
        </div>
      </div>
    </div>

    <div v-if="showForm" class="fixed inset-0 z-40 flex items-center justify-center bg-ink-900/40 p-4" @click.self="showForm = false">
      <div class="card w-full max-w-lg p-6">
        <h3 class="mb-4 font-display text-2xl text-ink-900">{{ editing ? 'Изменить пользователя' : 'Новый пользователь' }}</h3>
        <form class="space-y-3" @submit.prevent="save">
          <div><label class="label">Имя *</label><input ref="nameInput" v-model="form.full_name" class="input" required /></div>
          <div v-if="!editing"><label class="label">Email *</label><input v-model="form.email" type="email" class="input" required /></div>
          <div><label class="label">Телефон (для входа по SMS)</label><PhoneInput v-model="form.phone" /></div>
          <div><label class="label">{{ editing ? 'Новый пароль' : 'Пароль *' }}</label>
            <input v-model="form.password" type="password" class="input" :required="!editing" :placeholder="editing ? 'оставьте пустым' : ''" />
          </div>
          <div v-if="primaryRole === 'student'">
            <label class="label">Анкета ученика</label>
            <AppSelect v-model="form.disciple_id" :options="discipleOptions" placeholder="— не привязан —" />
          </div>
          <div v-if="roles.length">
            <label class="label">Роли</label>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="r in roles"
                :key="r.id"
                type="button"
                class="rounded-full border px-3 py-1.5 text-sm transition-colors"
                :class="form.role_ids.includes(r.id)
                  ? 'border-saffron-400 bg-saffron-500/15 text-saffron-700'
                  : 'border-parchment-300 text-ink-700 hover:bg-parchment-100'"
                @click="toggleRole(r.id)"
              >
                <AppIcon v-if="form.role_ids.includes(r.id)" name="check" :size="14" class="mr-1 inline align-[-2px]" />
                {{ r.name }}
              </button>
            </div>
          </div>
          <label class="flex items-center gap-2 text-sm text-ink-700"><input type="checkbox" v-model="form.is_active" /> Активен</label>
          <p v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>
          <div class="flex gap-2 pt-2">
            <button class="btn-primary">Сохранить</button>
            <button type="button" class="btn-ghost" @click="showForm = false">Отмена</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
