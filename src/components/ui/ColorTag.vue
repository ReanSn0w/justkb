<script setup lang="ts">
import { computed } from 'vue'
import { contrastTextColor } from '../../features/color'

const props = withDefaults(
  defineProps<{ color: string; as?: 'span' | 'button'; selected?: boolean }>(),
  { as: 'span', selected: false },
)
const style = computed(() => ({
  backgroundColor: props.color,
  color: contrastTextColor(props.color),
}))
</script>

<template>
  <component
    :is="as"
    :type="as === 'button' ? 'button' : undefined"
    class="tag color-tag"
    :class="{ 'color-tag--selected': selected }"
    :style="style"
  >
    <slot />
  </component>
</template>

<style scoped>
.color-tag--selected {
  outline: 2px solid var(--bulma-primary);
  outline-offset: 2px;
}
</style>
