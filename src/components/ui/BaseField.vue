<script setup lang="ts">
defineProps<{ label: string; hint?: string; error?: string; forId?: string }>()
</script>
<template>
  <div class="field">
    <label v-if="forId" class="label" :for="forId">{{ label }}</label>
    <p v-else class="label">{{ label }}</p>
    <div class="control"><slot /></div>
    <p v-if="error" class="help is-danger">{{ error }}</p>
    <p v-else-if="hint" class="help">{{ hint }}</p>
  </div>
</template>
