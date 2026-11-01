<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    type?: 'button' | 'submit'
    tone?: 'primary' | 'quiet' | 'danger'
    disabled?: boolean
  }>(),
  { type: 'button', tone: 'primary' },
)
const toneClass = computed(
  () => ({ primary: 'is-primary', quiet: undefined, danger: 'is-danger' })[props.tone],
)
</script>
<template>
  <button :type="type" :class="['button', toneClass]" :disabled="disabled"><slot /></button>
</template>
