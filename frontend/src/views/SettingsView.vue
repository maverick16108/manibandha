<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
defineOptions({ name: 'SettingsView' })
import client from '../api/client'
import AppIcon from '../components/AppIcon.vue'
import AppSkeleton from '../components/AppSkeleton.vue'
import { usePageTitle } from '../composables/pageTitle'
import { localCacheStats, wipeLocalChatCache } from '../chat/store'
import { confirmDialog } from '../composables/confirm'
import { useAutoRefresh } from '../composables/useAutoRefresh'

usePageTitle('Настройки')

// ── Память устройства / локальный кэш чатов (перенесено из шапки чата) ────────
function fmtSize(bytes) { if (!bytes) return '0 Б'; if (bytes < 1024) return `${bytes} Б`; if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} КБ`; if (bytes < 1073741824) return `${(bytes / 1048576).toFixed(1)} МБ`; return `${(bytes / 1073741824).toFixed(2)} ГБ` }
const storageBusy = ref(false)
const storageWiping = ref(false)
const storageInfo = reactive({ usage: 0, quota: 0, cacheBytes: 0, messages: 0, chats: 0 })
const storagePct = computed(() => storageInfo.quota ? Math.min(100, Math.round(storageInfo.usage / storageInfo.quota * 100)) : 0)
async function computeStorage() {
  storageBusy.value = true
  try {
    let usage = 0, quota = 0
    try { const e = await navigator.storage?.estimate?.(); usage = e?.usage || 0; quota = e?.quota || 0 } catch { /* ignore */ }
    let cacheBytes = 0
    try { for (const k of Object.keys(localStorage)) if (/^chat/i.test(k)) cacheBytes += ((localStorage.getItem(k) || '').length + k.length) * 2 } catch { /* ignore */ }
    const st = await localCacheStats()
    storageInfo.usage = usage; storageInfo.quota = quota; storageInfo.cacheBytes = cacheBytes
    storageInfo.messages = st?.messages || 0; storageInfo.chats = st?.chats || 0
  } catch { /* ignore */ } finally { storageBusy.value = false }
}
function clearPreviewCache() {
  try { for (const k of ['chatLinkPreviews', 'chatImgDims', 'chatImgColors', 'chatImgMicros', 'chatVideoLoaded', 'chatInfoCache']) localStorage.removeItem(k) } catch { /* ignore */ }
  computeStorage()
}
async function wipeAllCache() {
  if (!(await confirmDialog({ message: 'Очистить весь локальный кэш чатов? Сообщения и медиа заново подгрузятся с сервера. Страница перезагрузится.', confirmText: 'Очистить', danger: true }))) return
  storageWiping.value = true
  await wipeLocalChatCache()
  location.reload()
}
onMounted(computeStorage)

const loading = ref(true)
const saving = ref(false)
const saved = ref(false)
const form = ref({ forum_edit_window_minutes: 60, auth_expire_days: 30, recording_enabled: true, recording_height: 720 })

// Локальная (для этого устройства) настройка — фоновая подгрузка разделов в кэш. Применяется сразу,
// не через кнопку «Сохранить» (это не серверная настройка).
const prefetchOn = ref(localStorage.getItem('apiPrefetch') !== '0')
function togglePrefetch() { try { localStorage.setItem('apiPrefetch', prefetchOn.value ? '1' : '0') } catch { /* ignore */ } }

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const { data } = await client.get('/settings')
    form.value = {
      forum_edit_window_minutes: data.forum_edit_window_minutes ?? 60,
      auth_expire_days: data.auth_expire_days ?? 30,
      recording_enabled: data.recording_enabled !== false,
      recording_height: data.recording_height ?? 720,
    }
  } finally {
    loading.value = false
  }
}
onMounted(load)
useAutoRefresh(load)

function step(key, delta, min) {
  const v = Number(form.value[key]) || 0
  form.value[key] = Math.max(min, v + delta)
}

async function save() {
  saving.value = true
  saved.value = false
  try {
    await client.put('/settings', {
      forum_edit_window_minutes: Number(form.value.forum_edit_window_minutes),
      auth_expire_days: Number(form.value.auth_expire_days),
      recording_enabled: !!form.value.recording_enabled,
      recording_height: Number(form.value.recording_height),
    })
    saved.value = true
    setTimeout(() => { saved.value = false }, 2500)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-2xl">
    <p class="mb-6 text-ink-700/60">Общие параметры работы приложения</p>

    <div v-if="loading" class="card space-y-4 p-6">
      <AppSkeleton w="w-56" /><AppSkeleton w="w-full" h="h-9" />
      <AppSkeleton w="w-56" /><AppSkeleton w="w-full" h="h-9" />
    </div>

    <template v-else>
      <!-- Форум -->
      <div class="card mb-4 p-6">
        <div class="mb-4 flex items-center gap-2">
          <AppIcon name="forum" :size="18" class="text-saffron-600" />
          <h2 class="font-display text-lg font-semibold text-ink-900">Форум</h2>
        </div>
        <label class="label">Сколько минут можно изменять/удалять своё сообщение</label>
        <div class="flex items-center gap-2">
          <div class="stepper">
            <button type="button" @click="step('forum_edit_window_minutes', -5, 0)">−</button>
            <input v-model.number="form.forum_edit_window_minutes" type="number" min="0" />
            <button type="button" @click="step('forum_edit_window_minutes', 5, 0)">+</button>
          </div>
          <span class="text-sm text-ink-700/60">минут</span>
        </div>
        <p class="mt-1.5 text-xs text-ink-700/50">По истечении этого времени автор больше не сможет изменить или удалить сообщение (модератор — всегда может). 0 — запретить сразу.</p>
      </div>

      <!-- Авторизация -->
      <div class="card mb-4 p-6">
        <div class="mb-4 flex items-center gap-2">
          <AppIcon name="shield" :size="18" class="text-saffron-600" />
          <h2 class="font-display text-lg font-semibold text-ink-900">Авторизация</h2>
        </div>
        <label class="label">Через сколько дней без входа теряется авторизация</label>
        <div class="flex items-center gap-2">
          <div class="stepper">
            <button type="button" @click="step('auth_expire_days', -1, 1)">−</button>
            <input v-model.number="form.auth_expire_days" type="number" min="1" />
            <button type="button" @click="step('auth_expire_days', 1, 1)">+</button>
          </div>
          <span class="text-sm text-ink-700/60">дней</span>
        </div>
        <p class="mt-1.5 text-xs text-ink-700/50">Скользящее окно: при каждом заходе на сайт срок продлевается заново. Если пользователь не заходил столько дней — потребуется войти снова.</p>
      </div>

      <!-- Запись конференций -->
      <div class="card mb-4 p-6">
        <div class="mb-4 flex items-center gap-2">
          <AppIcon name="video" :size="18" class="text-saffron-600" />
          <h2 class="font-display text-lg font-semibold text-ink-900">Запись конференций</h2>
        </div>
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="form.recording_enabled" /> Разрешить запись конференций</label>
        <p class="mb-4 mt-1.5 text-xs text-ink-700/50">Записи ведёт сервер (LiveKit Egress) и складывает в архив. На слабом сервере запись нагружает процессор — при частом использовании стоит добавить мощности.</p>
        <div :class="!form.recording_enabled && 'pointer-events-none opacity-50'">
          <label class="label">Качество записи</label>
          <div class="flex gap-2">
            <button v-for="q in [480, 720, 1080]" :key="q" type="button"
                    class="rounded-lg border px-3 py-1.5 text-sm transition"
                    :class="form.recording_height === q ? 'border-saffron-500 bg-saffron-50 font-semibold text-saffron-700' : 'border-parchment-300 text-ink-700 hover:bg-parchment-100'"
                    @click="form.recording_height = q">{{ q }}p</button>
          </div>
          <p class="mt-1.5 text-xs text-ink-700/50">Чем выше качество — тем больше нагрузка и размер файла. На этом сервере рекомендуется 480p или 720p.</p>
        </div>
      </div>

      <!-- Устройство (локально, применяется сразу) -->
      <div class="card mb-4 p-6">
        <div class="mb-4 flex items-center gap-2">
          <AppIcon name="settings" :size="18" class="text-saffron-600" />
          <h2 class="font-display text-lg font-semibold text-ink-900">Это устройство</h2>
        </div>
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="prefetchOn" @change="togglePrefetch" /> Фоновая подгрузка разделов</label>
        <p class="mt-1.5 text-xs text-ink-700/50">Заранее и в фоне подгружает данные разделов (ученики, форум, справочники) в кэш — при заходе они открываются мгновенно, без скелетонов. Отключите, чтобы экономить трафик.</p>
      </div>

      <!-- Память устройства / локальный кэш чатов -->
      <div class="card mb-4 p-6">
        <div class="mb-4 flex items-center gap-2">
          <AppIcon name="settings" :size="18" class="text-saffron-600" />
          <h2 class="font-display text-lg font-semibold text-ink-900">Память устройства</h2>
        </div>
        <div class="rounded-xl bg-parchment-50 p-4 ring-1 ring-parchment-200">
          <div class="flex items-baseline justify-between">
            <span class="text-sm text-ink-700/60">Занято приложением</span>
            <span class="text-lg font-semibold text-ink-900">{{ storageBusy ? '…' : fmtSize(storageInfo.usage) }}</span>
          </div>
          <div class="mt-2 h-2 w-full overflow-hidden rounded-full bg-parchment-200">
            <div class="h-full rounded-full bg-saffron-500 transition-all" :style="{ width: storagePct + '%' }"></div>
          </div>
          <div class="mt-1.5 text-xs text-ink-700/40">
            <template v-if="storageInfo.quota">из ~{{ fmtSize(storageInfo.quota) }} доступно на устройстве</template>
            <template v-else>оценка недоступна в этом браузере</template>
          </div>
        </div>
        <div class="mt-4 divide-y divide-parchment-100 overflow-hidden rounded-xl ring-1 ring-parchment-200">
          <div class="flex items-center justify-between gap-3 px-4 py-3">
            <div class="min-w-0">
              <div class="text-[15px] text-ink-900">База чатов</div>
              <div class="text-xs text-ink-700/50">{{ storageInfo.messages }} сообщений · {{ storageInfo.chats }} чатов</div>
            </div>
            <span class="shrink-0 text-xs text-ink-700/40">локально</span>
          </div>
          <div class="flex items-center justify-between gap-3 px-4 py-3">
            <div class="min-w-0">
              <div class="text-[15px] text-ink-900">Превью и метаданные</div>
              <div class="text-xs text-ink-700/50">ссылки, размеры фото, состояния · {{ fmtSize(storageInfo.cacheBytes) }}</div>
            </div>
            <button class="shrink-0 rounded-lg px-3 py-1.5 text-sm font-medium text-saffron-700 hover:bg-saffron-500/10" @click="clearPreviewCache">Очистить</button>
          </div>
        </div>
        <p class="mt-3 text-xs leading-relaxed text-ink-700/50">
          Медиа-файлы (фото и видео) хранятся в кэше браузера и очищаются вместе с ним. Полная очистка удалит
          локальную копию переписки — она заново загрузится с сервера, ничего не потеряется.
        </p>
        <button class="btn-outline mt-3 w-full text-red-600 ring-red-200 hover:bg-red-50" :disabled="storageWiping" @click="wipeAllCache">
          <AppIcon name="trash" :size="16" /> {{ storageWiping ? 'Очистка…' : 'Очистить весь кэш чатов' }}
        </button>
      </div>

      <div class="flex items-center gap-3">
        <button class="btn-primary" :disabled="saving" @click="save">{{ saving ? 'Сохранение…' : 'Сохранить' }}</button>
        <span v-if="saved" class="flex items-center gap-1 text-sm text-green-700"><AppIcon name="check" :size="15" /> Сохранено</span>
      </div>
    </template>
  </div>
</template>

<style scoped>
/* кастомный числовой степпер (без нативных стрелок) */
.stepper { display: inline-flex; align-items: stretch; overflow: hidden; border: 1px solid #d8c9b0; border-radius: 0.5rem; background: #fff; }
.stepper button {
  width: 2.5rem; font-size: 1.25rem; line-height: 1; color: #7a6a55;
  transition: background-color 0.12s ease;
}
.stepper button:hover { background: #f3ead9; color: #c8742a; }
.stepper input {
  width: 4.5rem; text-align: center; font-size: 0.95rem; font-weight: 600; color: #2b2320;
  border-left: 1px solid #e7dcc7; border-right: 1px solid #e7dcc7; outline: none; -moz-appearance: textfield;
}
.stepper input::-webkit-outer-spin-button,
.stepper input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
</style>
