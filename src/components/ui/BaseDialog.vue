<script setup lang="ts">
const props = defineProps<{ open: boolean; title: string }>()
const emit = defineEmits<{ close: [] }>()
</script>
<template>
  <div v-if="props.open" class="modal is-active" role="dialog" aria-modal="true">
    <div class="modal-background" @click="emit('close')"></div>
    <div class="modal-card">
      <header class="modal-card-head">
        <p class="modal-card-title">{{ title }}</p>
        <button class="delete" aria-label="Закрыть" @click="emit('close')"></button>
      </header>
      <section class="modal-card-body"><slot /></section>
      <footer v-if="$slots.actions" class="modal-card-foot">
        <div class="buttons mb-0"><slot name="actions" /></div>
      </footer>
    </div>
  </div>
</template>
