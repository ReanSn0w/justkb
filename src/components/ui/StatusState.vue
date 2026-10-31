<script setup lang="ts">
defineProps<{ title: string; detail?: string; tone?: 'empty' | 'error' | 'loading' }>()
</script>
<template>
  <div :class="['status-state', `status-state--${tone || 'empty'}`]" role="status">
    <strong>{{ title }}</strong
    ><span v-if="detail">{{ detail }}</span
    ><slot />
  </div>
</template>
