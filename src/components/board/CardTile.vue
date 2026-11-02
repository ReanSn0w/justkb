<script setup lang="ts">
import type { Card } from '../../api'
import UserAvatar from '../user/UserAvatar.vue'
import ColorTag from '../ui/ColorTag.vue'
defineProps<{ card: Card }>()
defineEmits<{ drag: [card: Card]; move: [direction: -1 | 1] }>()
</script>
<template>
  <article
    class="card mb-4"
    draggable="true"
    tabindex="0"
    :aria-label="`JKB-${card.number}: ${card.title}`"
    @dragstart="$emit('drag', card)"
  >
    <div class="card-content p-4">
      <RouterLink :to="`/cards/${card.id}`">
        <p class="is-size-7 has-text-grey mb-2">JKB-{{ card.number }}</p>
        <p class="has-text-weight-semibold">{{ card.title }}</p>
      </RouterLink>
      <div class="tags mt-4 mb-2">
        <span v-if="card.epic" class="tag is-link">{{ card.epic.name }}</span>
        <ColorTag v-if="card.group" :color="card.group.color">{{ card.group.name }}</ColorTag>
        <span v-if="card.assignee" class="tag is-info">
          <UserAvatar :user="card.assignee" :size="24" />
          <span class="ml-1">{{ card.assignee.display_name }}</span>
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
