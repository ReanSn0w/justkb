<script setup lang="ts">
withDefaults(
  defineProps<{
    type?: 'button' | 'submit'
    tone?: 'primary' | 'quiet' | 'danger'
    disabled?: boolean
  }>(),
  { type: 'button', tone: 'primary' },
)
</script>
<template>
  <button :type="type" :class="['button', `button--${tone}`]" :disabled="disabled"><slot /></button>
</template>
