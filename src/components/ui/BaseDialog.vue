<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
const props = defineProps<{ open: boolean; title: string }>()
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLDialogElement>()
watch(
  () => props.open,
  async (open) => {
    await nextTick()
    if (open && !dialog.value?.open) dialog.value?.showModal()
    if (!open && dialog.value?.open) dialog.value.close()
  },
  { immediate: true },
)
</script>
<template>
  <dialog ref="dialog" class="dialog" @cancel.prevent="emit('close')">
    <header>
      <h2>{{ title }}</h2>
      <button class="icon-button" aria-label="Закрыть" @click="emit('close')">×</button>
    </header>
    <slot />
    <footer v-if="$slots.actions"><slot name="actions" /></footer>
  </dialog>
</template>
