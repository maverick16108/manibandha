<script setup>
import { RouterLink } from 'vue-router'

// Guru photos live in /public/guru/ (1 = main portrait, 2–4 = gallery).
// Bound via :src so Vite serves them from /public at runtime (not build-time imports).
const hero = '/guru/hero.jpg'          // splash — atmospheric kirtan shot
const portrait = '/guru/1.jpg'         // clear portrait — About section & login
const gallery = ['/guru/2.jpg', '/guru/3.jpg', '/guru/4.jpg']

const holyPlaces = ['Вриндаван', 'Маяпур', 'Джаганнатха Пури', 'Говардхан', 'Курукшетра', 'Ахобилам', 'Калькутта']
</script>

<template>
  <div class="min-h-screen bg-parchment-100 text-ink-800">
    <!-- Top bar -->
    <header class="absolute inset-x-0 top-0 z-10">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-6 py-5">
        <span class="font-display text-xl tracking-wide text-white drop-shadow">🪷 Манибандха Прабху</span>
        <RouterLink to="/login" class="btn bg-white/90 text-ink-800 hover:bg-white">Войти в кабинет</RouterLink>
      </div>
    </header>

    <!-- Hero -->
    <section class="relative overflow-hidden">
      <div class="absolute inset-0">
        <img :src="hero" alt="Манибандха Прабху" class="photo-bw h-full w-full object-cover object-[50%_22%]" />
        <div class="absolute inset-0 bg-gradient-to-t from-ink-900/85 via-ink-900/45 to-ink-900/60"></div>
      </div>
      <div class="relative mx-auto flex max-w-6xl flex-col items-center px-6 py-40 text-center text-white">
        <p class="mb-4 text-sm uppercase tracking-[0.3em] text-parchment-200/90">Его Милость</p>
        <h1 class="font-display text-5xl font-semibold leading-tight sm:text-7xl">Манибандха Прабху</h1>
        <p class="mt-6 max-w-2xl font-serif text-lg italic text-parchment-100/90 sm:text-xl">
          Инициирующий духовный учитель Международного общества сознания Кришны (ИСККОН),
          наставник и проводник по пути преданного служения.
        </p>
        <div class="mt-10 flex flex-wrap justify-center gap-3">
          <a href="#about" class="btn-primary">О духовном учителе</a>
          <RouterLink to="/login" class="btn border border-white/50 text-white hover:bg-white/10">Кабинет учеников</RouterLink>
        </div>
      </div>
    </section>

    <!-- About -->
    <section id="about" class="mx-auto max-w-5xl px-6 py-20">
      <div class="grid items-center gap-12 md:grid-cols-2">
        <div class="order-2 md:order-1">
          <p class="mb-3 text-sm uppercase tracking-[0.25em] text-saffron-600">О наставнике</p>
          <h2 class="font-display text-4xl font-semibold text-ink-900">Более 30 лет на пути бхакти</h2>
          <div class="mt-6 space-y-4 font-serif text-lg leading-relaxed text-ink-700">
            <p>
              Манибандха Прабху — специалист в области ведической философии и культуры, посвятивший
              духовной практике более тридцати лет. Четырнадцать из них он провёл в статусе монаха и
              девятнадцать — в семейной жизни, обретя редкое сочетание отречения и практической мудрости.
            </p>
            <p>
              Он известен как опытный лидер и наставник, специалист в вопросах психологии и семейных
              отношений. С недавнего времени Манибандха Прабху принимает служение инициирующего гуру,
              принимая учеников и ведя их по ступеням духовного посвящения.
            </p>
          </div>
        </div>
        <div class="order-1 md:order-2">
          <div class="overflow-hidden rounded-2xl border border-parchment-300 shadow-lg">
            <img :src="portrait" alt="Манибандха Прабху" class="photo-bw aspect-[4/5] w-full object-cover" />
          </div>
        </div>
      </div>
    </section>

    <!-- Service / teaching -->
    <section class="bg-ink-900 text-parchment-100">
      <div class="mx-auto max-w-5xl px-6 py-20">
        <p class="mb-3 text-center text-sm uppercase tracking-[0.25em] text-saffron-400">Служение</p>
        <h2 class="text-center font-display text-4xl font-semibold text-white">Проповедь и паломничества</h2>
        <div class="mt-12 grid gap-8 sm:grid-cols-3">
          <div class="text-center">
            <div class="mx-auto mb-4 text-3xl">📖</div>
            <h3 class="font-display text-2xl text-white">Лекции</h3>
            <p class="mt-2 font-serif text-parchment-200/80">
              Ежедневные беседы по священным писаниям — «Бхагавад-гите» и «Шримад-Бхагаватам».
            </p>
          </div>
          <div class="text-center">
            <div class="mx-auto mb-4 text-3xl">🕉️</div>
            <h3 class="font-display text-2xl text-white">Медитация</h3>
            <p class="mt-2 font-serif text-parchment-200/80">
              Утренняя джапа-медитация и совместное воспевание святых имён.
            </p>
          </div>
          <div class="text-center">
            <div class="mx-auto mb-4 text-3xl">🛕</div>
            <h3 class="font-display text-2xl text-white">Паломничества</h3>
            <p class="mt-2 font-serif text-parchment-200/80">
              Многонедельные путешествия по святым местам Индии в кругу преданных.
            </p>
          </div>
        </div>
        <div class="mt-12 flex flex-wrap justify-center gap-2">
          <span v-for="p in holyPlaces" :key="p" class="badge border border-parchment-200/30 text-parchment-200/90">
            {{ p }}
          </span>
        </div>
      </div>
    </section>

    <!-- Gallery -->
    <section class="mx-auto max-w-6xl px-6 py-20">
      <h2 class="mb-10 text-center font-display text-4xl font-semibold text-ink-900">Галерея</h2>
      <div class="grid gap-6 sm:grid-cols-3">
        <div v-for="(src, i) in gallery" :key="i" class="overflow-hidden rounded-2xl border border-parchment-300 shadow">
          <img :src="src" alt="Манибандха Прабху" class="photo-bw aspect-square w-full object-cover transition duration-500 hover:scale-105" />
        </div>
      </div>
    </section>

    <!-- Quote -->
    <section class="bg-parchment-200">
      <div class="mx-auto max-w-3xl px-6 py-20 text-center">
        <p class="font-serif text-2xl italic leading-relaxed text-ink-700 sm:text-3xl">
          «Знание оживает лишь тогда, когда воплощается в практике и передаётся от сердца к сердцу».
        </p>
        <p class="mt-6 text-sm uppercase tracking-[0.25em] text-saffron-600">Манибандха Прабху</p>
      </div>
    </section>

    <!-- Footer -->
    <footer class="bg-ink-900 text-parchment-200/70">
      <div class="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 px-6 py-8 sm:flex-row">
        <span class="font-display text-lg text-white">🪷 Манибандха Прабху</span>
        <span class="text-sm">© {{ new Date().getFullYear() }} · Служение и ученическая преемственность</span>
        <RouterLink to="/login" class="text-sm text-saffron-400 hover:text-saffron-500">Кабинет учеников →</RouterLink>
      </div>
    </footer>
  </div>
</template>
