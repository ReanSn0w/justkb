<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useBoardStore } from '../../stores/board'
import { mergeFilterQuery } from '../../features/filters'
import type { Card } from '../../api'
import UserAvatar from '../user/UserAvatar.vue'
import ColorTag from '../ui/ColorTag.vue'
defineProps<{ card: Card }>()
const route = useRoute()
const router = useRouter()
const board = useBoardStore()
function cardURL(id: string) {
  const returnTo = router.resolve({
    path: '/',
    query: mergeFilterQuery(route.query, board.filters),
  }).fullPath
  return { path: `/cards/${id}`, query: { return_to: returnTo } }
}
defineEmits<{ move: [direction: -1 | 1] }>()
</script>
<template>
  <article class="card mb-4" tabindex="0" :aria-label="`JKB-${card.number}: ${card.title}`">
    <div class="card-content p-4">
      <RouterLink :to="cardURL(card.id)">
        <p class="is-size-7 has-text-grey mb-2">JKB-{{ card.number }}</p>
        <p :data-transition-key="`card-${card.id}`" class="has-text-weight-semibold">
          {{ card.title }}
        </p>
      </RouterLink>
      <div class="tags mt-4 mb-2">
        <span v-if="card.epic" class="tag is-link">{{ card.epic.name }}</span>
        <ColorTag v-if="card.group" :color="card.group.color">{{ card.group.name }}</ColorTag>
        <span
          v-for="assignee in card.assignees ?? (card.assignee ? [card.assignee] : [])"
          :key="assignee.id"
          class="tag is-info"
        >
          <UserAvatar :user="assignee" :size="24" />
          <span class="ml-1">{{ assignee.display_name }}</span>
        </span>
      </div>
      <div v-if="card.tags.length" class="tags mb-3">
        <ColorTag v-for="tag in card.tags" :key="tag.id" :color="tag.color">
          {{ tag.name }}
        </ColorTag>
      </div>
      <div class="buttons are-small is-right mb-0">
        <button
          class="button"
          aria-label="Переместить в предыдущую колонку"
          @click="$emit('move', -1)"
        >
          ←
        </button>
        <button
          class="button"
          aria-label="Переместить в следующую колонку"
          @click="$emit('move', 1)"
        >
          →
        </button>
      </div>
    </div>
  </article>
</template>
