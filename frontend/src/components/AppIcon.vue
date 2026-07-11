<script setup>
// Custom inline SVG icon set (stroke = currentColor), themed for Manibandha.
defineProps({
  name: { type: String, required: true },
  size: { type: [Number, String], default: 20 },
  stroke: { type: [Number, String], default: 1.6 },
})
</script>

<template>
  <svg :width="size" :height="size" viewBox="0 0 24 24" fill="none" stroke="currentColor"
       :stroke-width="stroke" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <!-- Обзор -->
    <template v-if="name === 'overview'">
      <rect x="3.5" y="3.5" width="7" height="7" rx="1.6" />
      <rect x="13.5" y="3.5" width="7" height="7" rx="1.6" />
      <rect x="3.5" y="13.5" width="7" height="7" rx="1.6" />
      <rect x="13.5" y="13.5" width="7" height="7" rx="1.6" />
    </template>

    <!-- Ученики (person) -->
    <template v-else-if="name === 'disciples'">
      <circle cx="12" cy="8" r="3.3" />
      <path d="M5.5 19.5c0-3.6 2.9-6.2 6.5-6.2s6.5 2.6 6.5 6.2" />
    </template>

    <!-- Пользователи (two people) -->
    <template v-else-if="name === 'users'">
      <circle cx="9" cy="8" r="3" />
      <path d="M3.5 19c0-3 2.4-5.2 5.5-5.2S14.5 16 14.5 19" />
      <path d="M15.5 5.6a3 3 0 0 1 0 5.6" />
      <path d="M17 13.9c2 .5 3.5 2.3 3.5 4.8" />
    </template>

    <!-- Храмы / Паломничества (mandir) -->
    <template v-else-if="name === 'temple'">
      <path d="M12 2.5 8.5 7h7L12 2.5Z" />
      <path d="M8.5 7v3h7V7" />
      <path d="M7 21V10h10v11" />
      <path d="M4.5 21h15" />
      <path d="M10.5 21v-3.5h3V21" />
    </template>

    <!-- Отчёты (document) -->
    <template v-else-if="name === 'reports'">
      <path d="M7 3.5h6.5L18 8v11.5a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1v-15a1 1 0 0 1 1-1Z" />
      <path d="M13.5 3.5V8H18" />
      <path d="M9 12.5h6M9 16h4" />
    </template>

    <!-- Лекции (open book) -->
    <template v-else-if="name === 'book'">
      <path d="M12 6.6C10.4 5.4 8.4 4.8 6 4.8c-.8 0-1.5.6-1.5 1.4v10.4c0 .8.7 1.3 1.5 1.3 2.4 0 4.4.6 6 1.8" />
      <path d="M12 6.6c1.6-1.2 3.6-1.8 6-1.8.8 0 1.5.6 1.5 1.4v10.4c0 .8-.7 1.3-1.5 1.3-2.4 0-4.4.6-6 1.8" />
      <path d="M12 6.6V19.7" />
    </template>

    <!-- expand / maximize -->
    <template v-else-if="name === 'expand'">
      <path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3M3 16v3a2 2 0 0 0 2 2h3m8 0h3a2 2 0 0 0 2-2v-3" />
    </template>

    <!-- close -->
    <template v-else-if="name === 'close'">
      <path d="M6 6 18 18M18 6 6 18" />
    </template>

    <!-- city / pin -->
    <template v-else-if="name === 'pin'">
      <path d="M12 21s6-5.3 6-10a6 6 0 1 0-12 0c0 4.7 6 10 6 10Z" />
      <circle cx="12" cy="11" r="2.3" />
    </template>

    <!-- chevron (dropdown) -->
    <template v-else-if="name === 'chevron'">
      <path d="m6 9 6 6 6-6" />
    </template>

    <!-- check -->
    <template v-else-if="name === 'check'">
      <path d="m5 12.5 4.5 4.5L19 7" />
    </template>

    <!-- search -->
    <template v-else-if="name === 'search'">
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-3.2-3.2" />
    </template>

    <!-- download -->
    <template v-else-if="name === 'download'">
      <path d="M12 3.5v10.5m0 0 4-4m-4 4-4-4" />
      <path d="M4.5 20h15" />
    </template>

    <!-- Медитация / логотип (lotus) -->
    <template v-else-if="name === 'lotus'">
      <path d="M12 13.4c-1.7-1.4-2.6-3.4-2.6-5.4 0-1.8 1-3.6 2.6-5 1.6 1.4 2.6 3.2 2.6 5 0 2-.9 4-2.6 5.4Z" />
      <path d="M9.4 8.6C8 7.5 6 7 3.8 7.2c-.2 2.1.5 4.1 2 5.4" />
      <path d="M14.6 8.6c1.4-1.1 3.4-1.6 5.6-1.4.2 2.1-.5 4.1-2 5.4" />
      <path d="M4 12.3c1.5 2.7 4.6 4.4 8 4.4s6.5-1.7 8-4.4" />
    </template>
  </svg>
</template>
