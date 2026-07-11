<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { RouterLink } from 'vue-router'
import AppIcon from '../components/AppIcon.vue'

// Guru photos live in /public/guru/. Bound via :src so Vite serves them from /public.
const hero = '/guru/hero.jpg' // splash — atmospheric profile
const portrait = '/guru/1.jpg' // clear portrait — About & login
const gallery = Array.from({ length: 12 }, (_, i) => `/guru/gallery/${String(i + 1).padStart(2, '0')}.jpg`)

// Filmstrip scrolling
const strip = ref(null)
function scrollStrip(dir) {
  strip.value?.scrollBy({ left: dir * strip.value.clientWidth * 0.85, behavior: 'smooth' })
}

// Lightbox
const lightbox = ref(-1)
function openLb(i) { lightbox.value = i; document.body.style.overflow = 'hidden' }
function closeLb() { lightbox.value = -1; document.body.style.overflow = '' }
function lbPrev() { lightbox.value = (lightbox.value - 1 + gallery.length) % gallery.length }
function lbNext() { lightbox.value = (lightbox.value + 1) % gallery.length }
function onKey(e) {
  if (lightbox.value < 0) return
  if (e.key === 'Escape') closeLb()
  else if (e.key === 'ArrowLeft') lbPrev()
  else if (e.key === 'ArrowRight') lbNext()
}
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => { document.removeEventListener('keydown', onKey); document.body.style.overflow = '' })

const holyPlaces = ['Вриндаван', 'Маяпур', 'Джаганнатха Пури', 'Говардхан', 'Курукшетра', 'Ахобилам', 'Калькутта']

const service = [
  { icon: 'book', title: 'Лекции', text: 'Ежедневные беседы по священным писаниям — «Бхагавад-гите» и «Шримад-Бхагаватам».' },
  { icon: 'lotus', title: 'Медитация', text: 'Утренняя джапа-медитация и совместное воспевание святых имён.' },
  { icon: 'temple', title: 'Паломничества', text: 'Многонедельные путешествия по святым местам Индии в кругу преданных.' },
]
</script>

<template>
  <div class="min-h-screen bg-parchment-100 text-ink-800">
    <!-- Top bar -->
    <header class="absolute inset-x-0 top-0 z-20">
      <div class="mx-auto flex max-w-6xl items-center justify-between gap-3 px-4 py-4 sm:px-6">
        <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-saffron-400 to-saffron-600 text-white shadow-lg ring-1 ring-white/30">
          <AppIcon name="lotus-solid" :size="32" />
        </span>
        <RouterLink to="/login" class="btn whitespace-nowrap bg-white/90 text-ink-800 hover:bg-white">
          <span class="sm:hidden">Войти</span><span class="hidden sm:inline">Войти в кабинет</span>
        </RouterLink>
      </div>
    </header>

    <!-- Hero -->
    <section class="relative min-h-[100svh] overflow-hidden">
      <img :src="hero" alt="Манибандха Прабху" class="photo-bw absolute inset-0 h-full w-full object-cover object-[52%_20%]" />
      <!-- bottom gradient (mobile legibility) + left gradient (desktop, text sits right) -->
      <div class="absolute inset-0 bg-gradient-to-t from-ink-900 via-ink-900/65 to-ink-900/40 lg:via-ink-900/40 lg:to-ink-900/30"></div>
      <div class="absolute inset-0 hidden lg:block bg-gradient-to-l from-ink-900/85 via-ink-900/25 to-transparent"></div>

      <div class="relative z-10 mx-auto flex min-h-[100svh] max-w-6xl flex-col justify-end px-5 pb-14 pt-24 text-white sm:px-6 lg:justify-center lg:pb-0">
        <div class="max-w-xl text-center lg:ml-auto lg:text-right">
          <p class="mb-3 text-xs uppercase tracking-[0.3em] text-parchment-200/90 sm:mb-4 sm:text-sm">Его Милость</p>
          <h1 class="font-display text-4xl font-semibold leading-tight sm:text-6xl lg:text-7xl">Манибандха Прабху</h1>
          <p class="mt-4 font-serif text-base italic text-parchment-100/90 sm:mt-6 sm:text-lg lg:text-xl">
            Инициирующий духовный учитель Международного общества сознания Кришны (ИСККОН),
            наставник и проводник по пути преданного служения.
          </p>
          <div class="mt-10 flex flex-wrap justify-center gap-3 lg:justify-end">
            <a href="#about" class="btn-primary">О духовном учителе</a>
            <RouterLink to="/login" class="btn border border-white/50 text-white hover:bg-white/10">Кабинет учеников</RouterLink>
          </div>
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
          <div v-for="s in service" :key="s.title" class="text-center">
            <div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full border border-saffron-400/40 bg-saffron-400/10 text-saffron-400">
              <AppIcon :name="s.icon" :size="28" :stroke="1.4" />
            </div>
            <h3 class="font-display text-2xl text-white">{{ s.title }}</h3>
            <p class="mt-2 font-serif text-parchment-200/80">{{ s.text }}</p>
          </div>
        </div>
        <div class="mt-12 flex flex-wrap justify-center gap-2">
          <span v-for="p in holyPlaces" :key="p" class="badge border border-parchment-200/30 text-parchment-200/90">{{ p }}</span>
        </div>
      </div>
    </section>

    <!-- Gallery — swipeable filmstrip + lightbox -->
    <section class="py-20">
      <div class="mx-auto mb-8 flex max-w-6xl items-center justify-between px-6">
        <h2 class="font-display text-4xl font-semibold text-ink-900">Галерея</h2>
        <div class="hidden gap-2 sm:flex">
          <button class="flex h-10 w-10 items-center justify-center rounded-full border border-parchment-300 text-ink-700 transition hover:bg-parchment-200" @click="scrollStrip(-1)">
            <AppIcon name="chevron" :size="20" class="rotate-90" />
          </button>
          <button class="flex h-10 w-10 items-center justify-center rounded-full border border-parchment-300 text-ink-700 transition hover:bg-parchment-200" @click="scrollStrip(1)">
            <AppIcon name="chevron" :size="20" class="-rotate-90" />
          </button>
        </div>
      </div>
      <div ref="strip" class="flex snap-x snap-mandatory gap-4 overflow-x-auto scroll-smooth px-6 pb-4 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
        <button
          v-for="(src, i) in gallery" :key="i"
          class="group relative aspect-[3/4] w-56 shrink-0 snap-start overflow-hidden rounded-xl border border-parchment-300 shadow-sm sm:w-64"
          @click="openLb(i)"
        >
          <img :src="src" alt="Манибандха Прабху" loading="lazy" class="photo-bw h-full w-full object-cover transition duration-500 group-hover:scale-105" />
          <span class="absolute inset-0 flex items-center justify-center bg-ink-900/0 transition group-hover:bg-ink-900/25">
            <AppIcon name="expand" :size="26" class="text-white opacity-0 transition group-hover:opacity-100" />
          </span>
        </button>
      </div>
      <p class="mt-2 text-center text-sm text-ink-700/50 sm:hidden">Листайте вбок · нажмите, чтобы увеличить</p>
    </section>

    <!-- Lightbox -->
    <teleport to="body">
      <transition enter-active-class="transition duration-150" enter-from-class="opacity-0" leave-active-class="transition duration-150" leave-to-class="opacity-0">
        <div v-if="lightbox >= 0" class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/95 p-4" @click.self="closeLb">
          <button class="absolute right-5 top-5 flex h-11 w-11 items-center justify-center rounded-full text-white/80 hover:bg-white/10 hover:text-white" @click="closeLb">
            <AppIcon name="close" :size="24" />
          </button>
          <button class="absolute left-3 flex h-12 w-12 items-center justify-center rounded-full text-white/80 hover:bg-white/10 hover:text-white sm:left-6" @click="lbPrev">
            <AppIcon name="chevron" :size="28" class="rotate-90" />
          </button>
          <img :src="gallery[lightbox]" alt="Манибандха Прабху" class="photo-bw max-h-[88vh] max-w-[92vw] rounded-lg object-contain shadow-2xl" />
          <button class="absolute right-3 flex h-12 w-12 items-center justify-center rounded-full text-white/80 hover:bg-white/10 hover:text-white sm:right-6" @click="lbNext">
            <AppIcon name="chevron" :size="28" class="-rotate-90" />
          </button>
          <div class="absolute bottom-5 text-sm text-white/60">{{ lightbox + 1 }} / {{ gallery.length }}</div>
        </div>
      </transition>
    </teleport>

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
        <span class="flex items-center gap-2.5 text-white">
          <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-saffron-400 to-saffron-600 text-white shadow">
            <AppIcon name="lotus-solid" :size="26" />
          </span>
          <span class="font-script text-2xl font-bold">Манибандха</span>
        </span>
        <span class="text-sm">© {{ new Date().getFullYear() }} · Служение и ученическая преемственность</span>
        <RouterLink to="/login" class="text-sm text-saffron-400 hover:text-saffron-500">Кабинет учеников →</RouterLink>
      </div>
    </footer>
  </div>
</template>
