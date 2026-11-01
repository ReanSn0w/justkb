<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  title: string
  detail?: string
  tone?: 'empty' | 'error' | 'loading'
}>()
const toneClass = computed(() =>
  props.tone === 'error' ? 'is-danger' : props.tone === 'loading' ? 'is-info' : undefined,
)
</script>
<template>
  <div :class="['notification', 'has-text-centered', toneClass]" role="status">
    <p class="has-text-weight-semibold">{{ title }}</p>
    <p v-if="detail">{{ detail }}</p>
    <div v-if="$slots.default" class="mt-3"><slot /></div>
  </div>
</template>
