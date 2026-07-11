<script setup>
import { ref, reactive, onMounted, onBeforeUnmount, computed, nextTick, watch } from 'vue'
import { useRoute, useRouter, RouterLink, onBeforeRouteLeave } from 'vue-router'
import client from '../api/client'
import { useAuthStore } from '../stores/auth'
import { confirmDialog } from '../composables/confirm'
import AppSelect from '../components/AppSelect.vue'
import AppDatePicker from '../components/AppDatePicker.vue'
import PhotoUpload from '../components/PhotoUpload.vue'
import PhoneInput from '../components/PhoneInput.vue'
import { STATUS_LABELS, STATUS_ORDER, MARITAL_LABELS } from '../lib/format'
import { usePageTitle } from '../composables/pageTitle'

const maritalOptions = [{ value: '', label: '—' }, ...Object.entries(MARITAL_LABELS).map(([value, label]) => ({ value, label }))]
const statusOptions = STATUS_ORDER.map((s) => ({ value: s, label: STATUS_LABELS[s] }))

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const id = computed(() => route.params.id)
const isEdit = computed(() => !!id.value)
// самостоятельное заполнение анкеты кандидатом (не персонал, своя анкета) — при регистрации
const selfFill = computed(() => !auth.isStaff && String(auth.user?.disciple_id || '') === String(id.value))
const nameInput = ref(null)
// back to the disciple card when editing, otherwise to the list
const backTo = computed(() => (isEdit.value ? { name: 'disciple', params: { id: id.value } } : { name: 'disciples' }))

usePageTitle(() => (isEdit.value ? 'Редактировать ученика' : 'Новый ученик'))

const mentors = ref([])
const cities = ref([])
const regions = ref([])
const countries = ref([])
// наставники — это ученики с признаком is_mentor (кроме самого себя)
const mentorOptions = computed(() => [
  { value: '', label: '—' },
  ...mentors.value.filter((m) => String(m.id) !== String(id.value)).map((m) => ({ value: m.id, label: m.spiritual_name || m.material_name })),
])
// keep the disciple's existing value even if not yet in the dictionary
function dictOptions(list, current) {
  const names = list.map((x) => x.name)
  if (current && !names.includes(current)) names.unshift(current)
  return [{ value: '', label: '—' }, ...names.map((n) => ({ value: n, label: n }))]
}
const cityOptions = computed(() => dictOptions(cities.value, form.city))
const regionOptions = computed(() => dictOptions(regions.value, form.region))
const countryOptions = computed(() => dictOptions(countries.value, form.country))
const error = ref('')
const saving = ref(false)
const saved = ref(false)
let snapshot = '' // JSON of the form at load; used to detect unsaved changes
const isDirty = () => !saved.value && snapshot !== '' && JSON.stringify(form) !== snapshot

const form = reactive({
  material_name: '', spiritual_name: '', photo_url: '',
  phone: '', email: '', messenger: '',
  country: '', region: '', city: '',
  marital_status: '', date_of_birth: '',
  initiation_status: 'aspirant', pranama_date: '', harinama_date: '', harinama_name: '', brahman_date: '',
  seva: '', current_activity: '',
  mentor_id: '', is_mentor: false, recommended_by: '', application_date: '', ready_for_pranama: false, ready_for_initiation: false,
  notes: '',
})

function clean(obj) {
  const out = {}
  for (const [k, v] of Object.entries(obj)) out[k] = v === '' ? null : v
  return out
}

// обязательные поля (при самостоятельной регистрации — расширенный набор)
const errors = reactive({})
const REQUIRED_MSG = {
  material_name: 'Укажите ФИО',
  country: 'Выберите страну',
  region: 'Выберите область',
  city: 'Выберите город',
  date_of_birth: 'Укажите дату рождения',
  marital_status: 'Выберите семейное положение',
}
const requiredFields = computed(() => (selfFill.value
  ? ['material_name', 'country', 'region', 'city', 'date_of_birth', 'marital_status']
  : ['material_name']))
const req = (f) => requiredFields.value.includes(f)
function validate() {
  Object.keys(errors).forEach((k) => delete errors[k])
  for (const f of requiredFields.value) {
    if (!String(form[f] ?? '').trim()) errors[f] = REQUIRED_MSG[f]
  }
  return Object.keys(errors).length === 0
}
// убирать ошибку поля, как только оно заполнено
watch(form, () => { for (const k of Object.keys(errors)) if (String(form[k] ?? '').trim()) delete errors[k] })

async function save() {
  error.value = ''
  if (!validate()) { error.value = 'Заполните обязательные поля, отмеченные звёздочкой.'; return }
  saving.value = true
  try {
    const payload = clean(form)
    if (isEdit.value) {
      await client.patch(`/disciples/${id.value}`, payload)
      saved.value = true
      router.push({ name: 'disciple', params: { id: id.value } })
    } else {
      const { data } = await client.post('/disciples', payload)
      saved.value = true
      router.push({ name: 'disciple', params: { id: data.id } })
    }
  } catch (e) {
    error.value = e.response?.data?.detail || 'Ошибка сохранения'
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  const [m, c, r, co] = await Promise.all([
    client.get('/disciples', { params: { is_mentor: true, limit: 500 } }),
    client.get('/cities'), client.get('/regions'), client.get('/countries'),
  ])
  mentors.value = m.data.items
  cities.value = c.data
  regions.value = r.data
  countries.value = co.data
  if (isEdit.value) {
    const { data } = await client.get(`/disciples/${id.value}`)
    for (const k of Object.keys(form)) {
      if (k === 'mentor_id') form[k] = data[k] ?? ''
      else form[k] = data[k] ?? (typeof form[k] === 'boolean' ? false : '')
    }
  }
  snapshot = JSON.stringify(form) // baseline for unsaved-changes detection
  // при самостоятельном заполнении — пустое имя в фокусе
  if (selfFill.value && !form.material_name) nextTick(() => nameInput.value?.focus())

  // при выборе города — автоматически проставить область (и страну)
  watch(() => form.city, (name) => {
    const c = cities.value.find((x) => x.name === name)
    if (c?.region) form.region = c.region
    if (c && !form.country) form.country = 'Россия'
  })
})

// Warn on leaving with unsaved changes
onBeforeRouteLeave(async () => {
  if (!isDirty()) return true
  return await confirmDialog({
    title: 'Несохранённые изменения',
    message: 'Вы точно хотите выйти без сохранения? Изменения не будут сохранены.',
    confirmText: 'Выйти без сохранения',
    cancelText: 'Остаться',
    danger: true,
  })
})
function beforeUnload(e) {
  if (isDirty()) { e.preventDefault(); e.returnValue = '' }
}
onMounted(() => window.addEventListener('beforeunload', beforeUnload))
onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
</script>

<template>
  <div class="mx-auto max-w-6xl">
    <form class="space-y-6" novalidate @submit.prevent="save">
      <section class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Основное</h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="label">ФИО <span class="text-red-500">*</span></label>
            <input ref="nameInput" v-model="form.material_name" class="input" :class="errors.material_name && 'border-red-400'" />
            <p v-if="errors.material_name" class="mt-1 text-xs text-red-600">{{ errors.material_name }}</p>
          </div>
          <div v-if="!selfFill"><label class="label">Духовное имя</label><input v-model="form.spiritual_name" class="input" /></div>
          <div class="sm:col-span-2"><label class="label">Фото</label><PhotoUpload v-model="form.photo_url" /></div>
        </div>
      </section>

      <section class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Контакты и место</h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <div><label class="label">Телефон</label><PhoneInput v-model="form.phone" /></div>
          <div><label class="label">Email</label><input v-model="form.email" type="email" class="input" /></div>
          <div><label class="label">Мессенджер</label><input v-model="form.messenger" class="input" /></div>
          <div>
            <label class="label">Семейное положение <span v-if="req('marital_status')" class="text-red-500">*</span></label>
            <AppSelect v-model="form.marital_status" :options="maritalOptions" placeholder="—" />
            <p v-if="errors.marital_status" class="mt-1 text-xs text-red-600">{{ errors.marital_status }}</p>
          </div>
          <div>
            <label class="label">Страна <span v-if="req('country')" class="text-red-500">*</span></label>
            <AppSelect v-model="form.country" :options="countryOptions" placeholder="—" />
            <p v-if="errors.country" class="mt-1 text-xs text-red-600">{{ errors.country }}</p>
          </div>
          <div>
            <label class="label">Область <span v-if="req('region')" class="text-red-500">*</span></label>
            <AppSelect v-model="form.region" :options="regionOptions" placeholder="—" />
            <p v-if="errors.region" class="mt-1 text-xs text-red-600">{{ errors.region }}</p>
          </div>
          <div>
            <label class="label">Город <span v-if="req('city')" class="text-red-500">*</span></label>
            <AppSelect v-model="form.city" :options="cityOptions" placeholder="—" />
            <p v-if="errors.city" class="mt-1 text-xs text-red-600">{{ errors.city }}</p>
          </div>
          <div>
            <label class="label">Дата рождения <span v-if="req('date_of_birth')" class="text-red-500">*</span></label>
            <AppDatePicker v-model="form.date_of_birth" />
            <p v-if="errors.date_of_birth" class="mt-1 text-xs text-red-600">{{ errors.date_of_birth }}</p>
          </div>
        </div>
      </section>

      <section v-if="!selfFill" class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Инициация</h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <div><label class="label">Статус</label>
            <AppSelect v-model="form.initiation_status" :options="statusOptions" />
          </div>
          <div><label class="label">Дата получения пранама-мантры</label><AppDatePicker v-model="form.pranama_date" /></div>
          <div><label class="label">Дата харинамы</label><AppDatePicker v-model="form.harinama_date" /></div>
          <div><label class="label">Духовное имя (харинама)</label><input v-model="form.harinama_name" class="input" /></div>
          <div><label class="label">Дата второй инициации (брахман)</label><AppDatePicker v-model="form.brahman_date" /></div>
        </div>
      </section>

      <!-- при самостоятельной регистрации — только комментарий куратору -->
      <section v-if="selfFill" class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Комментарий для куратора</h3>
        <textarea v-model="form.notes" rows="5" class="input min-h-[7rem] resize-y"
                  placeholder="Напишите пару слов о себе или вопрос куратору (необязательно)"></textarea>
      </section>

      <section v-if="!selfFill" class="card p-6">
        <h3 class="mb-4 font-display text-xl text-ink-900">Путь аспиранта и служение</h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <div><label class="label">Наставник</label>
            <AppSelect v-model="form.mentor_id" :options="mentorOptions" placeholder="—" />
          </div>
          <div><label class="label">Кто рекомендовал</label><input v-model="form.recommended_by" class="input" placeholder="Наставник / президент храма" /></div>
          <div><label class="label">Дата заявки</label><AppDatePicker v-model="form.application_date" /></div>
          <div class="flex flex-col justify-end gap-2">
            <label class="flex items-center gap-2 text-sm text-ink-700">
              <input type="checkbox" v-model="form.is_mentor" /> Является наставником
            </label>
            <label class="flex items-center gap-2 text-sm text-ink-700">
              <input type="checkbox" v-model="form.ready_for_pranama" /> Готов(а) к пранаме
            </label>
            <label class="flex items-center gap-2 text-sm text-ink-700">
              <input type="checkbox" v-model="form.ready_for_initiation" /> Готов(а) к инициации
            </label>
          </div>
          <div class="sm:col-span-2"><label class="label">Севы (служение)</label><textarea v-model="form.seva" rows="4" class="input resize-y min-h-[6rem]"></textarea></div>
          <div class="sm:col-span-2"><label class="label">Текущая деятельность</label><textarea v-model="form.current_activity" rows="4" class="input resize-y min-h-[6rem]"></textarea></div>
          <div class="sm:col-span-2"><label class="label">Примечания</label><textarea v-model="form.notes" rows="6" class="input resize-y min-h-[8rem]"></textarea></div>
        </div>
      </section>

      <p v-if="error" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>

      <div class="flex gap-3">
        <button type="submit" class="btn-primary" :disabled="saving">{{ saving ? 'Сохранение…' : 'Сохранить' }}</button>
        <RouterLink :to="backTo" class="btn-ghost">Отмена</RouterLink>
      </div>
    </form>
  </div>
</template>
