<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { RouterLink } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import PhotoUpload from '../components/PhotoUpload.vue'
import { confirmDialog } from '../composables/confirm'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Форум')
const auth = useAuthStore()

const topics = ref([])
const sections = ref([])
const loading = ref(true)
const tab = ref('latest') // latest | unread | active | sections
const sectionFilter = ref(null)
const sectionMenu = ref(false)

// создание/редактирование раздела
const showSectionForm = ref(false)
const editingSectionId = ref(null)
const secTitle = ref('')
const secDesc = ref('')
const secColor = ref('#c8742a')
const secCover = ref('')
const savingSection = ref(false)
const COLORS = ['#c8742a', '#3b82c4', '#22a06b', '#8b5cf6', '#e0574b', '#d99a00', '#475569', '#0f766e']

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const [tRes, sRes] = await Promise.all([client.get('/forum/topics'), client.get('/forum/sections')])
    topics.value = tRes.data
    sections.value = sRes.data
  } finally {
    loading.value = false
  }
}
let poll = null
onMounted(() => { load(); poll = setInterval(() => load(true), 25000) })
onBeforeUnmount(() => clearInterval(poll))

const unreadCount = computed(() => topics.value.filter((t) => t.unread).length)
const currentSection = computed(() => sections.value.find((s) => s.id === sectionFilter.value) || null)

function byActivity(a, b) { return new Date(b.last_activity) - new Date(a.last_activity) }

const list = computed(() => {
  let arr = topics.value
  if (sectionFilter.value) arr = arr.filter((t) => t.section_id === sectionFilter.value)
  if (tab.value === 'unread') return [...arr].filter((t) => t.unread).sort(byActivity)
  if (tab.value === 'active') return [...arr].sort((a, b) => b.replies - a.replies)
  // latest: закреплённые → непрочитанные → по активности
  return [...arr].sort((a, b) => (Number(b.pinned) - Number(a.pinned)) || (Number(b.unread) - Number(a.unread)) || byActivity(a, b))
})
// индекс первой прочитанной темы (для разделителя в «Последних»)
const firstReadIdx = computed(() => {
  if (tab.value !== 'latest') return -1
  const i = list.value.findIndex((t) => !t.unread && !t.pinned)
  return i > 0 ? i : -1
})

function fmtNum(n) {
  if (n == null) return '0'
  if (n < 1000) return String(n)
  return `${(n / 1000).toFixed(1)} тыс.`
}
function fmtAgo(iso) {
  if (!iso) return ''
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 60) return 'сейчас'
  if (diff < 3600) return `${Math.floor(diff / 60)}м`
  if (diff < 86400) return `${Math.floor(diff / 3600)}ч`
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)}д`
  return `${Math.floor(diff / (86400 * 30))}мес`
}
function initials(name) { return (name || '?').trim()[0]?.toUpperCase() || '?' }

function pickSection(id) { sectionFilter.value = id; sectionMenu.value = false; if (tab.value === 'sections') tab.value = 'latest' }

function openCreateSection() {
  editingSectionId.value = null
  secTitle.value = ''; secDesc.value = ''; secColor.value = '#c8742a'; secCover.value = ''
  showSectionForm.value = true
}
function openEditSection(s) {
  editingSectionId.value = s.id
  secTitle.value = s.title; secDesc.value = s.description || ''; secColor.value = s.color; secCover.value = s.cover_url || ''
  showSectionForm.value = true
}
function closeSectionForm() { showSectionForm.value = false; editingSectionId.value = null }

async function saveSection() {
  if (!secTitle.value.trim()) return
  savingSection.value = true
  const payload = { title: secTitle.value.trim(), description: secDesc.value.trim() || null, color: secColor.value, cover_url: secCover.value || null }
  try {
    if (editingSectionId.value) {
      const { data } = await client.patch(`/forum/sections/${editingSectionId.value}`, payload)
      const i = sections.value.findIndex((x) => x.id === data.id)
      if (i >= 0) sections.value[i] = data
    } else {
      const { data } = await client.post('/forum/sections', payload)
      sections.value.push(data)
    }
    closeSectionForm()
  } finally { savingSection.value = false }
}
async function removeSection(s) {
  if (!(await confirmDialog({ message: `Удалить раздел «${s.title}» со всеми темами?`, confirmText: 'Удалить', danger: true }))) return
  await client.delete(`/forum/sections/${s.id}`)
  sections.value = sections.value.filter((x) => x.id !== s.id)
  topics.value = topics.value.filter((t) => t.section_id !== s.id)
  if (sectionFilter.value === s.id) sectionFilter.value = null
}
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <!-- Панель управления -->
    <div class="mb-4 flex flex-wrap items-center gap-2">
      <!-- выбор раздела -->
      <div class="relative">
        <button class="btn-outline" @click.stop="sectionMenu = !sectionMenu">
          {{ currentSection ? currentSection.title : 'Все разделы' }}
          <AppIcon name="chevron" :size="14" class="text-ink-700/50" />
        </button>
        <div v-if="sectionMenu" class="absolute left-0 top-full z-30 mt-1 max-h-72 w-60 overflow-y-auto rounded-lg border border-parchment-200 bg-white py-1 shadow-lg">
          <button class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-parchment-100" :class="!sectionFilter && 'font-semibold text-saffron-700'" @click="pickSection(null)">Все разделы</button>
          <button v-for="s in sections" :key="s.id" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-parchment-100" :class="sectionFilter === s.id && 'font-semibold text-saffron-700'" @click="pickSection(s.id)">
            <span class="h-2.5 w-2.5 shrink-0 rounded-sm" :style="{ background: s.color }"></span>
            <span class="truncate">{{ s.title }}</span>
          </button>
          <div v-if="!sections.length" class="px-3 py-2 text-sm text-ink-700/50">Разделов пока нет</div>
        </div>
      </div>

      <button class="rounded-lg px-3 py-1.5 text-sm font-medium transition" :class="tab === 'latest' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="tab = 'latest'">Последние</button>
      <button class="rounded-lg px-3 py-1.5 text-sm font-medium transition" :class="tab === 'unread' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="tab = 'unread'">
        Непрочитанные<span v-if="unreadCount"> ({{ unreadCount }})</span>
      </button>
      <button class="rounded-lg px-3 py-1.5 text-sm font-medium transition" :class="tab === 'active' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="tab = 'active'">Обсуждаемые</button>
      <button class="rounded-lg px-3 py-1.5 text-sm font-medium transition" :class="tab === 'sections' ? 'bg-saffron-500 text-white' : 'text-ink-700 hover:bg-parchment-100'" @click="tab = 'sections'">Разделы</button>

      <RouterLink v-if="auth.can('forum.post')" :to="{ name: 'forum-new', query: sectionFilter ? { section: sectionFilter } : {} }" class="btn-primary ml-auto shrink-0">
        <AppIcon name="forum" :size="16" /> Создать тему
      </RouterLink>
    </div>
    <div v-if="sectionMenu" class="fixed inset-0 z-20" @click="sectionMenu = false"></div>

    <div v-if="loading" class="card divide-y divide-parchment-100">
      <div v-for="i in 6" :key="i" class="flex items-center gap-4 p-4"><div class="flex-1 space-y-2"><AppSkeleton w="w-64" /><AppSkeleton w="w-32" h="h-3" /></div><AppSkeleton w="w-24" h="h-6" /></div>
    </div>

    <!-- Разделы -->
    <template v-else-if="tab === 'sections'">
      <div v-if="auth.can('forum.post')" class="mb-4">
        <button v-if="!showSectionForm" class="btn-outline" @click="openCreateSection"><AppIcon name="forum" :size="16" /> Новый раздел</button>
        <div v-else class="card space-y-3 p-4">
          <div class="font-medium text-ink-900">{{ editingSectionId ? 'Редактирование раздела' : 'Новый раздел' }}</div>
          <input v-model="secTitle" class="input" placeholder="Название раздела" />
          <input v-model="secDesc" class="input" placeholder="Описание (необязательно)" />
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-sm text-ink-700/60">Цвет:</span>
            <button v-for="c in COLORS" :key="c" class="h-6 w-6 rounded-md ring-2 transition" :style="{ background: c }" :class="secColor === c ? 'ring-ink-800' : 'ring-transparent'" @click="secColor = c"></button>
          </div>
          <div>
            <div class="label">Фото раздела (необязательно)</div>
            <PhotoUpload v-model="secCover" />
          </div>
          <div class="flex gap-2">
            <button class="btn-primary" :disabled="savingSection || !secTitle.trim()" @click="saveSection">{{ savingSection ? '…' : (editingSectionId ? 'Сохранить' : 'Создать') }}</button>
            <button class="btn-ghost" @click="closeSectionForm">Отмена</button>
          </div>
        </div>
      </div>
      <div v-if="!sections.length" class="card p-10 text-center text-ink-700/50">Разделов пока нет</div>
      <div v-else class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="s in sections" :key="s.id" class="card overflow-hidden">
          <img v-if="s.cover_url" :src="s.cover_url" alt="" class="h-28 w-full object-cover" />
          <div class="p-4">
            <div class="flex items-start justify-between gap-2">
              <button class="flex min-w-0 items-center gap-2 text-left" @click="pickSection(s.id)">
                <span class="h-3 w-3 shrink-0 rounded-sm" :style="{ background: s.color }"></span>
                <span class="truncate font-semibold text-ink-900 hover:text-saffron-700">{{ s.title }}</span>
              </button>
              <div v-if="s.can_edit" class="flex shrink-0 items-center gap-1">
                <button class="text-ink-700/40 hover:text-saffron-700" title="Изменить" @click="openEditSection(s)"><AppIcon name="edit" :size="15" /></button>
                <button class="text-ink-700/30 hover:text-red-600" title="Удалить" @click="removeSection(s)"><AppIcon name="trash" :size="15" /></button>
              </div>
            </div>
            <p v-if="s.description" class="mt-1 text-sm text-ink-700/60">{{ s.description }}</p>
            <div class="mt-2 text-xs text-ink-700/50">Тем: {{ s.topics_count }}</div>
          </div>
        </div>
      </div>
    </template>

    <!-- Список тем -->
    <template v-else>
      <div v-if="!list.length" class="card p-10 text-center text-ink-700/50">
        {{ tab === 'unread' ? 'Непрочитанных тем нет' : 'Тем пока нет' }}
      </div>
      <div v-else class="card overflow-hidden">
        <!-- шапка -->
        <div class="hidden items-center gap-4 border-b border-parchment-200 px-4 py-2.5 text-xs uppercase tracking-wide text-ink-700/50 sm:flex">
          <div class="flex-1">Тема</div>
          <div class="w-40 shrink-0"></div>
          <div class="w-16 shrink-0 text-center">Ответов</div>
          <div class="w-20 shrink-0 text-center">Просм.</div>
          <div class="w-24 shrink-0 text-right normal-case tracking-normal">Активность</div>
        </div>
        <template v-for="(t, i) in list" :key="t.id">
          <div v-if="i === firstReadIdx" class="flex items-center gap-2 bg-parchment-50 px-4 py-1.5 text-xs font-semibold uppercase tracking-wide text-ink-700/40">
            <span class="h-px flex-1 bg-parchment-200"></span> Прочитанное <span class="h-px flex-1 bg-parchment-200"></span>
          </div>
          <RouterLink :to="{ name: 'forum-topic', params: { id: t.id } }"
                      class="flex items-center gap-4 border-b border-parchment-100 px-4 py-3 transition last:border-0 hover:bg-parchment-50">
            <img v-if="t.cover_url" :src="t.cover_url" alt="" class="h-11 w-11 shrink-0 rounded-lg object-cover" />
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span v-if="t.unread" class="h-2 w-2 shrink-0 rounded-full bg-saffron-500" title="Непрочитано"></span>
                <AppIcon v-if="t.pinned" name="pin" :size="14" class="shrink-0 text-saffron-600" />
                <span class="truncate text-[15px]" :class="t.unread ? 'font-semibold text-ink-900' : 'text-ink-800'">{{ t.title }}</span>
              </div>
              <div v-if="t.section_title" class="mt-0.5 flex items-center gap-1.5">
                <span class="h-2.5 w-2.5 rounded-sm" :style="{ background: t.section_color }"></span>
                <span class="text-xs text-ink-700/60">{{ t.section_title }}</span>
              </div>
            </div>
            <!-- участники -->
            <div class="hidden w-40 shrink-0 sm:flex sm:justify-end">
              <div class="flex -space-x-2">
                <template v-for="(p, pi) in t.participants" :key="pi">
                  <img v-if="p.avatar" :src="p.avatar" class="photo-bw h-7 w-7 rounded-full object-cover ring-2 ring-white" :title="p.name" />
                  <span v-else class="flex h-7 w-7 items-center justify-center rounded-full bg-gradient-to-br from-saffron-400 to-saffron-600 text-[11px] font-semibold text-white ring-2 ring-white" :title="p.name">{{ initials(p.name) }}</span>
                </template>
              </div>
            </div>
            <div class="w-12 shrink-0 text-center text-sm font-semibold sm:w-16" :class="t.replies ? 'text-saffron-700' : 'text-ink-700/40'">{{ fmtNum(t.replies) }}</div>
            <div class="hidden w-20 shrink-0 text-center text-sm sm:block" :class="t.views > 1000 ? 'font-medium text-saffron-700' : 'text-ink-700/60'">{{ fmtNum(t.views) }}</div>
            <div class="w-12 shrink-0 text-right text-xs text-ink-700/50 sm:w-24">{{ fmtAgo(t.last_activity) }}</div>
          </RouterLink>
        </template>
      </div>
    </template>
  </div>
</template>
