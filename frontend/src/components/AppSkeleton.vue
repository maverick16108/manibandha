<script setup>
// Shimmering skeleton placeholder. Use `w`/`h`/`rounded` or pass classes.
defineProps({
  w: { type: String, default: 'w-full' },
  h: { type: String, default: 'h-4' },
  rounded: { type: String, default: 'rounded-md' },
})
</script>

<template>
  <div class="animate-pulse bg-parchment-200/80" :class="[w, h, rounded]"></div>
</template>
