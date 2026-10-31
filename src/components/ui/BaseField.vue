<script setup lang="ts">
defineProps<{ label: string; hint?: string; error?: string; forId?: string }>()
</script>
<template>
  <label class="field" :for="forId"
    ><span class="field__label">{{ label }}</span
    ><slot /><small v-if="error" class="field__error">{{ error }}</small
    ><small v-else-if="hint" class="muted">{{ hint }}</small></label
  >
</template>
