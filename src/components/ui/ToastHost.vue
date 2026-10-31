<script setup lang="ts">
import { useToastStore } from '../../stores/toasts'
const toasts = useToastStore()
</script>
<template>
  <div class="toasts" aria-live="polite">
    <button
      v-for="item in toasts.items"
      :key="item.id"
      :class="['toast', `toast--${item.tone}`]"
      @click="toasts.remove(item.id)"
    >
      {{ item.message }}
    </button>
  </div>
</template>
