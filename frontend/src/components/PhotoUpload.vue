<script setup>
import { ref, watch } from 'vue'
import client from '../api/client'
import AppIcon from './AppIcon.vue'

// modelValue — the chosen avatar URL (stored on the disciple).
const props = defineProps({ modelValue: { type: String, default: '' } })
const emit = defineEmits(['update:modelValue'])

const uploaded = ref(props.modelValue ? [props.modelValue] : [])
const uploading = ref(false)
const error = ref('')
const input = ref(null)

watch(() => props.modelValue, (v) => {
  if (v && !uploaded.value.includes(v)) uploaded.value.unshift(v)
})

async function onFiles(e) {
  const files = Array.from(e.target.files || [])
  if (!files.length) return
  error.value = ''
  uploading.value = true
  try {
    const fd = new FormData()
    files.forEach((f) => fd.append('files', f))
    const { data } = await client.post('/uploads', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    uploaded.value.push(...data.urls)
    if (!props.modelValue && data.urls[0]) emit('update:modelValue', data.urls[0])
  } catch (err) {
    error.value = err.response?.data?.detail || 'Не удалось загрузить'
  } finally {
    uploading.value = false
    if (input.value) input.value.value = ''
  }
}

function setMain(url) { emit('update:modelValue', url) }
function removeOne(url) {
  uploaded.value = uploaded.value.filter((u) => u !== url)
  if (props.modelValue === url) emit('update:modelValue', uploaded.value[0] || '')
}
</script>

<template>
  <div>
    <div class="flex flex-wrap items-start gap-3">
      <!-- thumbnails -->
      <div v-for="url in uploaded" :key="url" class="group relative">
        <img :src="url" class="photo-bw h-20 w-20 rounded-lg border-2 object-cover"
             :class="url === modelValue ? 'border-saffron-500' : 'border-parchment-300'" @click="setMain(url)" />
        <span v-if="url === modelValue" class="absolute -left-1 -top-1 rounded-full bg-saffron-500 p-0.5 text-white">
          <AppIcon name="check" :size="12" />
        </span>
        <button type="button" class="absolute -right-1 -top-1 rounded-full bg-ink-900/80 p-0.5 text-white opacity-0 transition group-hover:opacity-100"
                @click="removeOne(url)"><AppIcon name="close" :size="12" /></button>
      </div>

      <!-- upload button -->
      <button type="button" :disabled="uploading" @click="input.click()"
        class="flex h-20 w-20 flex-col items-center justify-center gap-1 rounded-lg border-2 border-dashed border-parchment-300 text-ink-700/50 transition hover:border-saffron-400 hover:text-saffron-600">
        <AppIcon :name="uploading ? 'lotus' : 'download'" :size="20" :class="uploading && 'animate-pulse rotate-180'" />
        <span class="text-[11px]">{{ uploading ? 'Загрузка…' : 'Загрузить' }}</span>
      </button>
      <input ref="input" type="file" accept="image/*" multiple class="hidden" @change="onFiles" />
    </div>
    <p v-if="uploaded.length > 1" class="mt-2 text-xs text-ink-700/50">Нажмите на фото, чтобы сделать его основным</p>
    <p v-if="error" class="mt-2 text-sm text-red-600">{{ error }}</p>
  </div>
</template>
