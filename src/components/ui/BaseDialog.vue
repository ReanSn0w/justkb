<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
const props = defineProps<{ open: boolean; title: string }>()
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLElement | null>(null)
let previous: HTMLElement | null = null
const focusable = () =>
  Array.from(
    dialog.value?.querySelectorAll<HTMLElement>(
      'button:not(:disabled),a[href],input:not(:disabled),select:not(:disabled),textarea:not(:disabled),[tabindex="0"]',
    ) || [],
  )
watch(
  () => props.open,
  async (open) => {
    if (open) {
      previous = document.activeElement as HTMLElement
      await nextTick()
      if (props.open && !dialog.value?.contains(document.activeElement))
        (focusable()[0] || dialog.value)?.focus()
    } else {
      previous?.focus()
      previous = null
    }
  },
  { immediate: true },
)
onBeforeUnmount(() => previous?.focus())
function keydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    emit('close')
  }
  if (event.key !== 'Tab') return
  const nodes = focusable()
  const first = nodes[0],
    last = nodes.at(-1)
  if (!first) {
    event.preventDefault()
    dialog.value?.focus()
    return
  }
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}
</script>
<template>
  <div
    v-if="props.open"
    ref="dialog"
    class="modal is-active"
    role="dialog"
    aria-modal="true"
    :aria-label="title"
    tabindex="-1"
    @keydown="keydown"
  >
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
