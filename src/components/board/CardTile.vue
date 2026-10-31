<script setup lang="ts">
import type { Card } from '../../api'
defineProps<{ card: Card }>()
defineEmits<{ drag: [card: Card]; move: [direction: -1 | 1] }>()
</script>
<template>
  <article
    class="card-tile"
    draggable="true"
    tabindex="0"
    :aria-label="`JKB-${card.number}: ${card.title}`"
    @dragstart="$emit('drag', card)"
  >
    <RouterLink :to="`/cards/${card.id}`"
      ><span class="card-number">JKB-{{ card.number }}</span
      ><strong>{{ card.title }}</strong></RouterLink
    >
    <div class="card-meta">
      <span v-if="card.epic" class="chip" :style="{ borderColor: card.epic.color }">{{
        card.epic.name
      }}</span
      ><span v-if="card.group" class="chip">{{ card.group.name }}</span
      ><span v-if="card.assignee" class="avatar" :title="card.assignee.display_name">{{
        card.assignee.display_name.slice(0, 2).toUpperCase()
      }}</span>
    </div>
    <div class="tags">
      <span
        v-for="tag in card.tags"
        :key="tag.id"
        class="tag"
        :style="{ backgroundColor: tag.color }"
        >{{ tag.name }}</span
      >
    </div>
    <div class="keyboard-move">
      <button aria-label="Переместить в предыдущую колонку" @click="$emit('move', -1)">←</button
      ><button aria-label="Переместить в следующую колонку" @click="$emit('move', 1)">→</button>
    </div>
  </article>
</template>
