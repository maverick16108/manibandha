<script setup>
import { ref } from 'vue'
import DictionaryPanel from '../components/DictionaryPanel.vue'

const tabs = [
  { key: 'cities', title: 'Города', endpoint: '/cities', withCountry: true },
  { key: 'regions', title: 'Области', endpoint: '/regions', withCountry: false },
  { key: 'countries', title: 'Страны', endpoint: '/countries', withCountry: false },
  { key: 'mentors', title: 'Наставники', endpoint: '/mentors', withCountry: false },
]
const active = ref('cities')
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <h1 class="mb-1 font-display text-3xl font-semibold text-ink-900">Справочники</h1>
    <p class="mb-6 text-ink-700/60">Города, области и страны для анкет и фильтров</p>

    <div class="mb-5 flex gap-1 border-b border-parchment-200">
      <button
        v-for="t in tabs" :key="t.key"
        class="relative -mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors"
        :class="active === t.key ? 'border-saffron-500 text-saffron-700' : 'border-transparent text-ink-700/60 hover:text-ink-800'"
        @click="active = t.key"
      >
        {{ t.title }}
      </button>
    </div>

    <DictionaryPanel
      v-for="t in tabs" v-show="active === t.key" :key="t.key"
      :endpoint="t.endpoint" :with-country="t.withCountry"
      :empty-text="`Список пуст — добавьте первый элемент`"
    />
  </div>
</template>
