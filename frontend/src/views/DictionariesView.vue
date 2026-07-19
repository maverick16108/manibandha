<script setup>
import { ref } from 'vue'
defineOptions({ name: 'DictionariesView' })
import DictionaryPanel from '../components/DictionaryPanel.vue'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Справочники')

const tabs = [
  { key: 'cities', title: 'Города', endpoint: '/cities', withCountry: true },
  { key: 'regions', title: 'Области', endpoint: '/regions', withCountry: false },
  { key: 'countries', title: 'Страны', endpoint: '/countries', withCountry: false },
]
const active = ref('cities')
</script>

<template>
  <div>
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
