<script setup>
import { RouterLink } from 'vue-router'
import { usePageTitle } from '../composables/pageTitle'

usePageTitle('Свисток — пространства сообществ')

const features = [
  { icon: '💬', title: 'Общие чаты', text: 'Личные и групповые переписки — единые для всех пространств, всегда под рукой.' },
  { icon: '🏛️', title: 'Пространства', text: 'Присоединяйтесь к сообществам или создайте своё за минуту и настройте под себя.' },
  { icon: '🧩', title: 'Модули', text: 'Включайте нужное: события, ученики, вопросы, форум, конференции, галерея.' },
  { icon: '🛡️', title: 'Роли и права', text: 'Гибкое управление доступом участников — каждый видит только своё.' },
]
</script>

<template>
  <div class="min-h-screen bg-parchment-50 text-ink-900">
    <!-- шапка -->
    <header class="mx-auto flex max-w-5xl items-center justify-between px-5 py-5">
      <div class="flex items-center gap-2 text-xl font-semibold tracking-tight">
        <span class="text-2xl">🐦</span> свисток
      </div>
      <RouterLink to="/login" class="rounded-lg bg-ink-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-ink-700">Войти</RouterLink>
    </header>

    <!-- герой -->
    <section class="mx-auto max-w-3xl px-5 pb-8 pt-10 text-center sm:pt-20">
      <h1 class="font-display text-4xl font-bold leading-tight sm:text-6xl">Пространства сообществ<br />с общими чатами</h1>
      <p class="mx-auto mt-6 max-w-xl text-lg text-ink-700/70">
        Свисток — платформа, где сообщества живут как отдельные пространства, а общение остаётся единым.
        Зарегистрируйтесь, войдите в чаты и присоединяйтесь к пространствам — или создайте своё.
      </p>
      <div class="mt-8 flex flex-wrap justify-center gap-3">
        <RouterLink to="/login" class="rounded-xl bg-saffron-500 px-6 py-3 font-semibold text-white shadow-sm transition hover:bg-saffron-600">Создать аккаунт</RouterLink>
        <RouterLink to="/login" class="rounded-xl border border-parchment-300 bg-white px-6 py-3 font-semibold text-ink-800 transition hover:bg-parchment-100">Войти</RouterLink>
      </div>
    </section>

    <!-- возможности -->
    <section class="mx-auto max-w-5xl px-5 py-12">
      <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div v-for="f in features" :key="f.title" class="rounded-2xl bg-white p-6 ring-1 ring-parchment-200">
          <div class="text-3xl">{{ f.icon }}</div>
          <h3 class="mt-3 font-display text-lg font-semibold">{{ f.title }}</h3>
          <p class="mt-1.5 text-sm leading-relaxed text-ink-700/60">{{ f.text }}</p>
        </div>
      </div>
    </section>

    <footer class="mx-auto max-w-5xl px-5 py-10 text-center text-sm text-ink-700/40">
      © svistok.io — платформа пространств
    </footer>
  </div>
</template>
