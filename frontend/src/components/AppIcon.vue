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

    <!-- chat (questions) -->
    <template v-else-if="name === 'chat'">
      <path d="M21 11.5a7.5 7.5 0 0 1-10.9 6.7L4 20l1.8-4.1A7.5 7.5 0 1 1 21 11.5Z" />
    </template>

    <!-- chart (statistics) -->
    <template v-else-if="name === 'chart'">
      <path d="M4 20V10M9.5 20V4M15 20v-6M20.5 20V8M3 20h18" />
    </template>

    <!-- menu (burger) -->
    <template v-else-if="name === 'menu'">
      <path d="M4 7h16M4 12h16M4 17h16" />
    </template>

    <!-- calendar -->
    <template v-else-if="name === 'calendar'">
      <rect x="3.5" y="5" width="17" height="15.5" rx="2" />
      <path d="M3.5 9.5h17M8 3.5v3M16 3.5v3" />
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

    <!-- filled lotus (logo badge) -->
    <template v-else-if="name === 'lotus-solid'">
      <g fill="currentColor" stroke="none">
        <path d="M12 2.8c1.5 2.2 2.2 4.3 2.2 6.2 0 2.1-.7 4-2.2 5.6-1.5-1.6-2.2-3.5-2.2-5.6 0-1.9.7-4 2.2-6.2Z" />
        <path d="M11.4 14.6C9.9 12 7.7 10.4 5 10c-.2 2.6 1.1 4.8 3.7 6 1 .5 2 .3 2.7-1.4Z" />
        <path d="M12.6 14.6C14.1 12 16.3 10.4 19 10c.2 2.6-1.1 4.8-3.7 6-1 .5-2 .3-2.7-1.4Z" />
        <path d="M12 16.9c-3 0-5.9-1.2-7.9-3.3.3 2.5 1.8 4.5 4.1 5.3 1.9.7 5.7.7 7.6 0 2.3-.8 3.8-2.8 4.1-5.3-2 2.1-4.9 3.3-7.9 3.3Z" />
      </g>
    </template>

    <!-- image (photo) -->
    <template v-else-if="name === 'image'">
      <rect x="3.5" y="4.5" width="17" height="15" rx="2.2" />
      <circle cx="8.5" cy="9.5" r="1.6" />
      <path d="m4 16.5 4.2-4 3.3 3.2L15 12l5 5" />
    </template>

    <!-- link -->
    <template v-else-if="name === 'link'">
      <path d="M10 13.8a3.6 3.6 0 0 0 5.1 0l2.4-2.4a3.6 3.6 0 1 0-5.1-5.1l-1.3 1.3" />
      <path d="M14 10.2a3.6 3.6 0 0 0-5.1 0L6.5 12.6a3.6 3.6 0 1 0 5.1 5.1l1.3-1.3" />
    </template>

    <!-- mic (голосовое) -->
    <template v-else-if="name === 'mic'">
      <rect x="9" y="3" width="6" height="11" rx="3" />
      <path d="M5.5 11a6.5 6.5 0 0 0 13 0" />
      <path d="M12 17.5V21m-3 0h6" />
    </template>

    <!-- eye (preview) -->
    <template v-else-if="name === 'eye'">
      <path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z" />
      <circle cx="12" cy="12" r="3" />
    </template>

    <!-- logout -->
    <template v-else-if="name === 'logout'">
      <path d="M15 4.5H6a1.5 1.5 0 0 0-1.5 1.5v12A1.5 1.5 0 0 0 6 19.5h9" />
      <path d="M18.5 12H10m8.5 0-3-3m3 3-3 3" />
    </template>

    <!-- shield (roles / доступ) -->
    <template v-else-if="name === 'shield'">
      <path d="M12 3 5 5.5v5.2c0 4.4 3 7.6 7 9.3 4-1.7 7-4.9 7-9.3V5.5L12 3Z" />
      <path d="m9 11.5 2 2 4-4" />
    </template>

    <!-- edit (карандаш) -->
    <template v-else-if="name === 'edit'">
      <path d="M4 20h4L18.5 9.5a2 2 0 0 0-2.8-2.8L5 17.2 4 20Z" />
      <path d="m14 8 2.8 2.8" />
    </template>

    <!-- trash (удалить) -->
    <template v-else-if="name === 'trash'">
      <path d="M4 6.5h16M9 6.5V5a1.5 1.5 0 0 1 1.5-1.5h3A1.5 1.5 0 0 1 15 5v1.5" />
      <path d="M6.5 6.5 7.5 19a1.5 1.5 0 0 0 1.5 1.4h6a1.5 1.5 0 0 0 1.5-1.4l1-12.5" />
      <path d="M10 10v6.5M14 10v6.5" />
    </template>

    <!-- react (смайл — добавить реакцию) -->
    <template v-else-if="name === 'react'">
      <circle cx="12" cy="12" r="8.5" />
      <path d="M8.5 14.5s1.3 1.8 3.5 1.8 3.5-1.8 3.5-1.8" />
      <path d="M9 9.5h.01M15 9.5h.01" />
    </template>

    <!-- play -->
    <template v-else-if="name === 'play'">
      <path d="M7 5.5v13l11-6.5-11-6.5Z" />
    </template>

    <!-- pause -->
    <template v-else-if="name === 'pause'">
      <rect x="6.5" y="5" width="3.5" height="14" rx="1" />
      <rect x="14" y="5" width="3.5" height="14" rx="1" />
    </template>

    <!-- rewind (перемотать назад) -->
    <template v-else-if="name === 'rewind'">
      <path d="M11 6 4.5 12 11 18V6Z" />
      <path d="M19.5 6 13 12l6.5 6V6Z" />
    </template>

    <!-- forward (перемотать вперёд) -->
    <template v-else-if="name === 'forward'">
      <path d="M13 6l6.5 6L13 18V6Z" />
      <path d="M4.5 6 11 12l-6.5 6V6Z" />
    </template>

    <!-- volume -->
    <template v-else-if="name === 'volume'">
      <path d="M4 9.5v5h3.5L12 18V6L7.5 9.5H4Z" />
      <path d="M15.5 9a4 4 0 0 1 0 6" />
    </template>

    <!-- volume-x (без звука) -->
    <template v-else-if="name === 'volume-x'">
      <path d="M4 9.5v5h3.5L12 18V6L7.5 9.5H4Z" />
      <path d="m16 9.5 4 4m0-4-4 4" />
    </template>

    <!-- reply (ответить) -->
    <template v-else-if="name === 'reply'">
      <path d="M9 8 4 12.5 9 17" />
      <path d="M4.5 12.5H14a5 5 0 0 1 5 5V19" />
    </template>

    <!-- close (крестик) -->
    <template v-else-if="name === 'close'">
      <path d="m6 6 12 12M18 6 6 18" />
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
