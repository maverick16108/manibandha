<script setup>
import { ref, computed, onMounted, onBeforeUnmount, onActivated, onDeactivated, watch, nextTick } from 'vue'
defineOptions({ name: 'ThreadsView' })
import { RouterLink, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import AppSelect from '../components/AppSelect.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import AppIcon from '../components/AppIcon.vue'
import { formatDate } from '../lib/format'
import { usePageTitle } from '../composables/pageTitle'
import { cachedGet, TTL } from '../composables/apiCache'
import client from '../api/client'

const route = useRoute()
const auth = useAuthStore()
const kind = computed(() => route.meta.kind) // 'question' | 'report'
const isReport = computed(() => kind.value === 'report')

usePageTitle(() => (isReport.value ? 'Отчёты о служении' : 'Вопросы гуру'))

const threads = ref([])
const loading = ref(true)
const flashReady = ref(false) // анимация «вспышки» — только для НОВЫХ элементов (после первой загрузки), не на всю ленту при входе
const disciples = ref([])
const mentors = ref([])
const filterDisciple = ref('')
const filterMentor = ref('')
const filterPeriod = ref('')  // для отчётов — фильтр по месяцу
const search = ref('')
const MONTHS = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь']

// guru/staff can filter by disciple; a student only ever sees their own
const showFilter = computed(() => !auth.user?.disciple_id)
const discipleOptions = computed(() => [{ value: '', label: 'Все ученики' }, ...disciples.value.map((d) => ({ value: d.id, label: d.spiritual_name || d.material_name }))])
const mentorOptions = computed(() => [{ value: '', label: 'Все наставники' }, ...mentors.value.map((d) => ({ value: d.id, label: d.spiritual_name || d.material_name }))])
// доступные месяцы (для отчётов) — из самих отчётов, свежие сверху
const periodOptions = computed(() => {
  const set = [...new Set(threads.value.map((t) => t.period).filter(Boolean))].sort().reverse()
  return [{ value: '', label: 'Все периоды' }, ...set.map((p) => ({ value: p, label: periodLabel(p) }))]
})
// поиск + фильтр по месяцу поверх загруженного списка
const filtered = computed(() => {
  let list = threads.value
  if (isReport.value && filterPeriod.value) list = list.filter((t) => t.period === filterPeriod.value)
  const q = search.value.trim().toLowerCase()
  if (q) list = list.filter((t) => [t.subject, t.disciple_name, t.last_preview].some((s) => (s || '').toLowerCase().includes(q)))
  return list
})

function threadParams() {
  const params = { kind: kind.value }
  if (filterDisciple.value) params.disciple_id = filterDisciple.value
  if (filterMentor.value) params.mentor_id = filterMentor.value
  return params
}
async function load(silent = false, force = false) {
  const p = threadParams()
  // НЕ показываем устаревший снимок кеша (peek) — иначе при заходе видно «лишние» удалённые записи,
  // которые потом исчезают. Скелетон только если данных ещё нет (первый заход); при возврате раздел
  // держится живым (keep-alive) — мгновенно, без скелетона. Явный заход всегда тянет СВЕЖЕЕ.
  if (!silent && !threads.value.length) loading.value = true
  try {
    const data = await cachedGet('/threads', { params: p, ttl: TTL.list, force: force || !silent })
    // переприсваиваем список ТОЛЬКО если он реально изменился — иначе тихий рефреш/поллинг вызывает
    // лишнюю перерисовку и раздел «моргает» при каждом заходе
    const cur = threads.value
    const same = Array.isArray(data) && cur.length === data.length &&
      cur.every((t, i) => t.id === data[i].id && t.updated_at === data[i].updated_at && t.unread === data[i].unread && t.messages_count === data[i].messages_count)
    if (!same) threads.value = data
  } finally {
    loading.value = false
  }
}
// ВАЖНО (keep-alive): kind в watch НЕ включаем. У конкретного инстанса kind фиксирован (Вопросы — всегда
// question); «меняется» он только потому, что деактивированный инстанс читает ОБЩИЙ route (когда открыт
// раздел Отчёты) → иначе watch грузил бы отчёты в список Вопросов, и при возврате мелькали чужие записи.
// Плюс isActive-гейт — на всякий случай не грузим неактивный раздел.
let isActive = true
watch([filterDisciple, filterMentor], () => { if (isActive) load() })

// живое обновление списка (новые вопросы/отчёты появляются сразу)
let poll = null
function startPoll() { clearInterval(poll); poll = setInterval(() => load(true, true), 15000) } // force — держим кэш свежим (живые обновления)
function onVisible() { if (isActive && document.visibilityState === 'visible') load(true, true) }
onMounted(() => {
  startPoll()
  document.addEventListener('visibilitychange', onVisible)
})
onBeforeUnmount(() => { clearInterval(poll); document.removeEventListener('visibilitychange', onVisible) })
// keep-alive: первую активацию (сразу после mount) пропускаем — иначе двойная загрузка/двойной скелетон;
// при последующих возвратах — тихий рефреш без скелетона, поллинг возобновляем
let firstActivate = true
// при возврате — ФОРСИРОВАННЫЙ тихий рефреш: keep-alive показывает актуальный список мгновенно, а
// force не даёт подставить устаревший снимок кеша (иначе мелькали удалённые записи 7→4)
onActivated(() => { isActive = true; startPoll(); if (firstActivate) { firstActivate = false; return } load(true, true) })
onDeactivated(() => { isActive = false; clearInterval(poll) })

function periodLabel(p) {
  if (!p) return ''
  const [y, m] = p.split('-')
  return `${MONTHS[+m - 1]} ${y}`
}

onMounted(async () => {
  // справочники учеников/наставников — из общего кеша (переживают перезаход), в фоне
  if (showFilter.value) {
    try {
      const [ds, ms] = await Promise.all([
        cachedGet('/disciples', { params: { named: true, limit: 500 }, ttl: TTL.ref }),
        cachedGet('/disciples', { params: { is_mentor: true, named: true, limit: 500 }, ttl: TTL.ref }),
      ])
      disciples.value = ds.items
      mentors.value = ms.items
    } catch { /* ignore */ }
  }
  await load()
  await nextTick(); flashReady.value = true // с этого момента подсвечиваем только реально новые элементы
})

// ── соглашение в разделе «Вопросы»: показ при первом входе + управление ──
const agr = ref(null) // { enabled, text, acknowledged, can_manage, version }
const agrChecked = ref(false)
const agrManage = ref(null) // модалка редактирования { enabled, text }
const agrSaving = ref(false)
const showAgreement = computed(() => !isReport.value && agr.value && agr.value.enabled && !agr.value.acknowledged)
async function loadAgreement() {
  if (isReport.value) return
  try { const { data } = await client.get('/questions/agreement'); agr.value = data } catch { /* ignore */ }
}
async function ackAgreement() {
  try { await client.post('/questions/agreement/ack'); if (agr.value) agr.value.acknowledged = true } catch { /* ignore */ }
}
function openAgreementManage() { agrManage.value = { enabled: agr.value.enabled, text: agr.value.text } }
async function saveAgreement() {
  agrSaving.value = true
  try {
    const { data } = await client.put('/questions/agreement', { enabled: agrManage.value.enabled, text: agrManage.value.text })
    agr.value = { ...agr.value, ...data, acknowledged: true }
    agrManage.value = null
  } catch { /* ignore */ } finally { agrSaving.value = false }
}
onMounted(loadAgreement)
onActivated(loadAgreement)
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <div class="mb-6 flex items-center justify-between">
      <div>
        <p class="text-ink-700/60">{{ isReport ? 'Ежемесячные отчёты учеников · доступ: ученик, куратор, гуру' : 'Личные вопросы · видит только гуру и сам ученик' }}</p>
      </div>
      <div class="flex items-center gap-2">
        <button v-if="!isReport && agr && agr.can_manage" class="btn-ghost p-2" title="Соглашение при входе" @click="openAgreementManage"><AppIcon name="settings" :size="18" /></button>
        <RouterLink v-if="auth.user?.disciple_id" :to="{ name: isReport ? 'report-new' : 'question-new' }" class="btn-primary">
          {{ isReport ? '+ Новый отчёт' : '+ Новый вопрос' }}
        </RouterLink>
      </div>
    </div>

    <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
      <div class="flex items-center gap-2 rounded-md border border-parchment-300 bg-white px-3 py-2 sm:w-72">
        <AppIcon name="search" :size="16" class="shrink-0 text-ink-700/40" />
        <input v-model="search" class="w-full bg-transparent text-sm text-ink-800 outline-none placeholder:text-ink-700/40" :placeholder="isReport ? 'Поиск по отчётам…' : 'Поиск по вопросам…'" />
      </div>
      <div v-if="showFilter" class="sm:w-56"><AppSelect v-model="filterDisciple" :options="discipleOptions" placeholder="Все ученики" /></div>
      <div v-if="showFilter" class="sm:w-56"><AppSelect v-model="filterMentor" :options="mentorOptions" placeholder="Все наставники" /></div>
      <div v-if="isReport" class="sm:w-52"><AppSelect v-model="filterPeriod" :options="periodOptions" placeholder="Все периоды" /></div>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 4" :key="i" class="card space-y-2 p-4"><AppSkeleton w="w-48" /><AppSkeleton w="w-full" h="h-3" /></div>
    </div>

    <TransitionGroup v-else tag="div" :name="flashReady ? 'flash' : ''" class="space-y-3">
      <RouterLink v-for="t in filtered" :key="t.id" :to="{ name: 'thread', params: { id: t.id } }"
                  class="card block p-4 transition hover:border-saffron-400/50 hover:shadow">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <span v-if="t.unread" class="h-2.5 w-2.5 shrink-0 rounded-full bg-saffron-500" title="Новое"></span>
              <span class="truncate font-medium text-ink-900">{{ (!isReport && t.subject) ? t.subject : t.disciple_name }}</span>
              <span v-if="t.period" class="badge shrink-0 bg-saffron-500/15 text-saffron-700">{{ periodLabel(t.period) }}</span>
              <span v-if="t.unread" class="badge shrink-0 bg-saffron-500/15 text-saffron-700">Новое</span>
            </div>
            <div v-if="!isReport && t.subject" class="text-sm text-ink-700/60">{{ t.disciple_name }}</div>
            <div class="mt-1 truncate text-sm text-ink-700/60">{{ t.last_preview }}</div>
          </div>
          <div class="shrink-0 text-right text-xs text-ink-700/50">
            <div>{{ formatDate(t.updated_at) }}</div>
            <div class="mt-1">💬 {{ t.messages_count }}</div>
          </div>
        </div>
      </RouterLink>
    </TransitionGroup>
    <div v-if="!loading && !filtered.length" class="card p-8 text-center text-ink-700/50">
      {{ (search || filterPeriod) ? 'Ничего не найдено' : (isReport ? 'Отчётов пока нет' : 'Вопросов пока нет') }}
    </div>

    <!-- соглашение при первом входе в раздел «Вопросы» -->
    <div v-if="showAgreement" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/55 p-4">
      <div class="flex max-h-[88vh] w-full max-w-xl flex-col overflow-hidden rounded-2xl bg-white shadow-2xl">
        <div class="flex-1 overflow-y-auto px-6 py-6">
          <div class="whitespace-pre-line text-[15px] leading-relaxed text-ink-800">{{ agr.text }}</div>
        </div>
        <div class="border-t border-parchment-200 p-5">
          <label class="mb-4 flex cursor-pointer items-start gap-2.5 text-sm text-ink-800">
            <input type="checkbox" v-model="agrChecked" class="mt-0.5" /> <span>Я прочитал(а) и понимаю</span>
          </label>
          <button class="btn-primary w-full" :disabled="!agrChecked" @click="ackAgreement">Продолжить</button>
        </div>
      </div>
    </div>

    <!-- управление соглашением (для тех, у кого есть доступ) -->
    <div v-if="agrManage" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-4" @click.self="agrManage = null">
      <div class="w-full max-w-xl overflow-hidden rounded-2xl bg-white shadow-xl">
        <header class="flex items-center justify-between border-b border-parchment-200 px-5 py-3.5">
          <h3 class="font-medium text-ink-900">Соглашение при входе в «Вопросы»</h3>
          <button class="rounded-lg p-1.5 text-ink-700/60 hover:bg-parchment-100" @click="agrManage = null"><AppIcon name="close" :size="22" /></button>
        </header>
        <div class="space-y-4 p-5">
          <label class="flex items-center gap-2 text-sm text-ink-800">
            <input type="checkbox" v-model="agrManage.enabled" /> Показывать соглашение при входе в раздел
          </label>
          <div>
            <label class="label">Текст соглашения</label>
            <textarea v-model="agrManage.text" rows="12" class="input resize-y text-sm leading-relaxed"></textarea>
            <p class="mt-1 text-xs text-ink-700/50">При изменении текста соглашение снова покажется всем пользователям.</p>
          </div>
        </div>
        <div class="flex justify-end gap-2 border-t border-parchment-200 px-5 py-3.5">
          <button class="btn-ghost" @click="agrManage = null">Отмена</button>
          <button class="btn-primary" :disabled="agrSaving || !agrManage.text.trim()" @click="saveAgreement">{{ agrSaving ? '…' : 'Сохранить' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* новый вопрос/отчёт появляется мягким проявлением (без рамки-обводки, чтобы не мигало при входе) */
.flash-enter-active { transition: opacity 0.4s ease, transform 0.4s ease; }
.flash-enter-from { opacity: 0; transform: translateY(-8px); }
.flash-move { transition: transform 0.4s ease; }
</style>
