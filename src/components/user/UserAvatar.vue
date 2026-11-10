<script setup lang="ts">
import { computed } from 'vue'
import type { User } from '../../api'

const props = withDefaults(
  defineProps<{
    user?: Pick<User, 'display_name' | 'avatar_url'> | null
    size?: 24 | 32 | 48 | 64 | 96 | 128
    as?: 'figure' | 'span'
  }>(),
  { user: null, size: 32, as: 'figure' },
)

const initials = computed(() => {
  const words = props.user?.display_name.trim().split(/\s+/).filter(Boolean) || []
  return (
    words
      .slice(0, 2)
      .map((word) => word[0])
      .join('')
      .toLocaleUpperCase()
      .replace(/[^\p{L}\p{N}]/gu, '') || '?'
  )
})
const fallback = computed(() => {
  const fontSize = Math.round(props.size * 0.38)
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${props.size}" height="${props.size}" viewBox="0 0 ${props.size} ${props.size}"><circle cx="50%" cy="50%" r="50%" fill="#effaf5"/><text x="50%" y="50%" dominant-baseline="central" text-anchor="middle" fill="#257953" font-family="sans-serif" font-size="${fontSize}" font-weight="600">${initials.value}</text></svg>`
  return `data:image/svg+xml,${encodeURIComponent(svg)}`
})
</script>

<template>
  <component
    :is="as"
    :class="['image', `is-${size}x${size}`]"
    :title="user?.display_name || 'Пользователь'"
  >
    <img
      class="is-rounded"
      :src="user?.avatar_url || fallback"
      :alt="`Аватар: ${user?.display_name || 'Пользователь'}`"
      :width="size"
      :height="size"
    />
  </component>
</template>
