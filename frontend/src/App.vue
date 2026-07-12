<script setup>
import ConfirmDialog from './components/ConfirmDialog.vue'
import Lightbox from './components/Lightbox.vue'
</script>

<template>
  <router-view />
  <ConfirmDialog />
  <Lightbox />
</template>
