<script setup>
import { ref, computed, onMounted } from 'vue'
defineOptions({ name: 'SpacesView' })
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { usePageTitle } from '../composables/pageTitle'
import { useSpacesStore } from '../stores/spaces'
import { useAutoRefresh } from '../composables/useAutoRefresh'

usePageTitle('Пространства')
const spaces = useSpacesStore()

const TYPES = [
  { key: 'guru', label: 'Гуру' },
  { key: 'obuchenie', label: 'Обучение' },
]
const typeLabel = (t) => TYPES.find((x) => x.key === t)?.label || t

async function load(silent = false) {
  if (!silent && spaces.loaded) return
  await spaces.load(silent)
}
onMounted(() => load())
useAutoRefresh(() => load(true))

// ── создание ──────────────────────────────────────────────────────────────
const showCreate = ref(false)
const creating = ref(false)
const createErr = ref('')
const form = ref({ name: '', slug: '', type: 'guru' })
const slugEdited = ref(false)

const slugPreview = computed(() => (form.value.slug || autoSlug(form.value.name)))
function autoSlug(name) {
  const tr = { а:'a',б:'b',в:'v',г:'g',д:'d',е:'e',ё:'e',ж:'zh',з:'z',и:'i',й:'y',к:'k',л:'l',м:'m',н:'n',о:'o',п:'p',р:'r',с:'s',т:'t',у:'u',ф:'f',х:'h',ц:'ts',ч:'ch',ш:'sh',щ:'sch',ъ:'',ы:'y',ь:'',э:'e',ю:'yu',я:'ya' }
  return (name || '').toLowerCase().split('').map((c) => tr[c] ?? c).join('')
    .replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
}
function onName() { if (!slugEdited.value) form.value.slug = autoSlug(form.value.name) }
function onSlug() { slugEdited.value = true; form.value.slug = autoSlug(form.value.slug) }

function openCreate() {
  form.value = { name: '', slug: '', type: 'guru' }
  slugEdited.value = false; createErr.value = ''
  showCreate.value = true
}
async function submitCreate() {
  if (!form.value.name.trim()) { createErr.value = 'Укажите название'; return }
  creating.value = true; createErr.value = ''
  try {
    await spaces.create({ name: form.value.name.trim(), slug: form.value.slug.trim(), type: form.value.type })
    showCreate.value = false
  } catch (e) {
    createErr.value = e?.response?.data?.detail || 'Не удалось создать пространство'
  } finally {
    creating.value = false
  }
}

const busy = ref(new Set())
async function join(sp) {
  if (busy.value.has(sp.id)) return
  busy.value = new Set(busy.value).add(sp.id)
  try { await spaces.join(sp.id) } finally { const s = new Set(busy.value); s.delete(sp.id); busy.value = s }
}
async function leave(sp) {
  if (busy.value.has(sp.id)) return
  busy.value = new Set(busy.value).add(sp.id)
  try { await spaces.leave(sp.id) } finally { const s = new Set(busy.value); s.delete(sp.id); busy.value = s }
}
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <div class="mb-6 flex items-center justify-between gap-3">
      <p class="text-ink-700/60">Сообщества на платформе — присоединяйтесь или создайте своё</p>
      <button class="btn-primary shrink-0" @click="openCreate"><AppIcon name="plus" :size="16" /> Создать</button>
    </div>

    <div v-if="spaces.loading && !spaces.loaded" class="grid gap-4 sm:grid-cols-2">
      <div v-for="i in 4" :key="i" class="card p-5"><AppSkeleton w="w-40" /><AppSkeleton w="w-24" h="h-4" class="mt-3" /></div>
    </div>

    <div v-else class="grid gap-4 sm:grid-cols-2">
      <div v-for="sp in spaces.list" :key="sp.id" class="card flex flex-col p-5">
        <div class="mb-2 flex items-start justify-between gap-2">
          <h3 class="font-display text-lg font-semibold text-ink-900">{{ sp.name }}</h3>
          <span class="shrink-0 rounded-full bg-saffron-50 px-2 py-0.5 text-xs font-medium text-saffron-700">{{ typeLabel(sp.type) }}</span>
        </div>
        <div class="mb-4 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-700/50">
          <span class="inline-flex items-center gap-1"><AppIcon name="users" :size="13" /> {{ sp.member_count }}</span>
          <span>/s/{{ sp.slug }}</span>
          <span v-if="sp.custom_domain" class="text-saffron-600">{{ sp.custom_domain }}</span>
          <span v-if="sp.is_owner" class="inline-flex items-center gap-1 font-medium text-ink-700/70"><AppIcon name="key" :size="12" /> Модератор</span>
        </div>
        <div class="mt-auto">
          <div v-if="spaces.activeId === sp.id" class="flex items-center gap-2">
            <span class="inline-flex items-center gap-1 text-sm font-semibold text-saffron-700"><AppIcon name="check" :size="15" /> Вы здесь</span>
          </div>
          <div v-else-if="sp.my_status === 'active'" class="flex items-center gap-2">
            <button class="btn-outline flex-1" @click="spaces.enter(sp.id)">Войти</button>
            <button v-if="!sp.is_owner" class="text-sm text-ink-700/50 hover:text-red-600" :disabled="busy.has(sp.id)" @click="leave(sp)">Выйти</button>
          </div>
          <div v-else-if="sp.my_status === 'pending'" class="text-sm text-ink-700/60">Заявка отправлена — ожидает подтверждения</div>
          <button v-else class="btn-outline w-full" :disabled="busy.has(sp.id)" @click="join(sp)">
            {{ busy.has(sp.id) ? '…' : (sp.join_mode === 'request' ? 'Подать заявку' : 'Вступить') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Создание пространства -->
    <div v-if="showCreate" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-4" @click.self="showCreate = false">
      <div class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
        <h2 class="mb-4 font-display text-xl font-semibold text-ink-900">Новое пространство</h2>
        <label class="label">Название</label>
        <input v-model="form.name" class="input mb-3" placeholder="Например: Школа бхакти" @input="onName" />
        <label class="label">Адрес</label>
        <div class="mb-1 flex items-center rounded-lg border border-parchment-300 bg-parchment-50 px-3">
          <span class="text-sm text-ink-700/40">svistok.io/s/</span>
          <input v-model="form.slug" class="flex-1 bg-transparent py-2 text-sm outline-none" :placeholder="slugPreview || 'адрес'" @input="onSlug" />
        </div>
        <p class="mb-3 text-xs text-ink-700/40">Латиница, цифры и дефис. Если занят — добавится номер.</p>
        <label class="label">Тип</label>
        <div class="mb-4 flex gap-2">
          <button v-for="t in TYPES" :key="t.key" type="button"
                  class="flex-1 rounded-lg border px-3 py-2 text-sm transition"
                  :class="form.type === t.key ? 'border-saffron-500 bg-saffron-50 font-semibold text-saffron-700' : 'border-parchment-300 text-ink-700 hover:bg-parchment-100'"
                  @click="form.type = t.key">{{ t.label }}</button>
        </div>
        <p v-if="createErr" class="mb-3 text-sm text-red-600">{{ createErr }}</p>
        <div class="flex justify-end gap-2">
          <button class="btn-ghost" @click="showCreate = false">Отмена</button>
          <button class="btn-primary" :disabled="creating" @click="submitCreate">{{ creating ? 'Создание…' : 'Создать' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
