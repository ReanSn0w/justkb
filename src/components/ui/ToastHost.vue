<script setup lang="ts">
import { useToastStore } from '../../stores/toasts'
const toasts = useToastStore()
</script>
<template>
  <div v-if="toasts.items.length" class="section pt-0" aria-live="polite">
    <div
      v-for="item in toasts.items"
      :key="item.id"
      :class="['notification', item.tone === 'error' ? 'is-danger' : 'is-success']"
    >
      <button class="delete" aria-label="Закрыть" @click="toasts.remove(item.id)"></button>
      {{ item.message }}
    </div>
  </div>
</template>
