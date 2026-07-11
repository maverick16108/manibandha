<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import client from '../api/client'
import { renderMarkdown } from '../lib/markdown'
import PublicShell from '../components/PublicShell.vue'
import AppIcon from '../components/AppIcon.vue'

const route = useRoute()
const router = useRouter()
const ev = ref(null)
const loading = ref(true)
const notFound = ref(false)

const fromCalendar = computed(() => route.query.from === 'calendar')
// клик по крошке: если пришли оттуда — назад (сохранит позицию), иначе прямой переход
function goCrumb(target) {
  const back = window.history.length > 1
  if (target === 'calendar') fromCalendar.value && back ? router.back() : router.push('/calendar')
  else (!fromCalendar.value && back) ? router.back() : router.push('/')
}

const MON = ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря']
function fmt(iso) { if (!iso) return ''; const [y, m, d] = iso.split('-'); return `${+d} ${MON[+m - 1]} ${y}` }
function range(e) { if (!e.starts_on) return ''; const s = fmt(e.starts_on); return e.ends_on && e.ends_on !== e.starts_on ? `${s} — ${fmt(e.ends_on)}` : s }

onMounted(async () => {
  try { const { data } = await client.get(`/events/public/${route.params.id}`); ev.value = data }
  catch { notFound.value = true }
  finally { loading.value = false }
})
</script>

<template>
  <PublicShell>
    <div v-if="loading" class="text-ink-700/50">Загрузка…</div>
    <div v-else-if="notFound" class="text-center text-ink-700/60">
      Событие не найдено. <RouterLink to="/calendar" class="text-saffron-700 hover:underline">К календарю</RouterLink>
    </div>
    <article v-else>
      <nav class="mb-5 flex flex-wrap items-center gap-1.5 text-sm text-ink-700/60">
        <button class="hover:text-saffron-700" @click="goCrumb('home')">Главная</button>
        <span class="text-ink-700/30">/</span>
        <template v-if="fromCalendar">
          <button class="hover:text-saffron-700" @click="goCrumb('calendar')">Календарь</button>
          <span class="text-ink-700/30">/</span>
        </template>
        <span class="truncate text-ink-800">{{ ev.title }}</span>
      </nav>
      <h1 class="font-display text-4xl font-semibold text-ink-900">{{ ev.title }}</h1>
      <div class="mt-4 flex flex-wrap gap-x-6 gap-y-2 text-ink-700">
        <span class="inline-flex items-center gap-1.5"><AppIcon name="calendar" :size="16" class="text-saffron-600" /> {{ range(ev) }}</span>
        <span v-if="ev.location" class="inline-flex items-center gap-1.5"><AppIcon name="pin" :size="16" class="text-saffron-600" /> {{ ev.location }}</span>
      </div>
      <div v-if="ev.description" class="markdown-body mt-8 text-lg leading-relaxed text-ink-700" v-html="renderMarkdown(ev.description)"></div>
      <p v-else class="mt-8 text-ink-700/50">Описание пока не добавлено.</p>
    </article>
  </PublicShell>
</template>

<style scoped>
.markdown-body :deep(img) { max-width: 100%; border-radius: 0.75rem; margin: 0.75rem 0; }
.markdown-body :deep(a) { text-decoration: underline; }
.markdown-body :deep(ul) { list-style: disc; padding-left: 1.25rem; margin: 0.5rem 0; }
.markdown-body :deep(p) { margin: 0.5rem 0; }
</style>
