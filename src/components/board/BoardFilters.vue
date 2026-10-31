<script setup lang="ts">
import { computed } from 'vue'
import { useBoardStore } from '../../stores/board'
const board = useBoardStore()
const active = computed(() =>
  Object.values(board.filters).some((value) =>
    Array.isArray(value) ? value.length : Boolean(value),
  ),
)
function toggleTag(id: string) {
  const current = board.filters.tag_ids || []
  board.filters.tag_ids = current.includes(id) ? current.filter((x) => x !== id) : [...current, id]
}
</script>
<template>
  <aside class="filters" aria-label="Фильтры доски">
    <input
      v-model.trim="board.filters.q"
      type="search"
      placeholder="Поиск задач…"
      aria-label="Поиск задач"
    /><select v-model="board.filters.epic_id" aria-label="EPIC">
      <option value="">Все EPIC</option>
      <option value="none">Без EPIC</option>
      <option v-for="epic in board.epics" :key="epic.id" :value="epic.id">
        {{ epic.name }}
      </option></select
    ><select v-model="board.filters.group_id" aria-label="Группа">
      <option value="">Все группы</option>
      <option value="none">Без группы</option>
      <option v-for="group in board.groups" :key="group.id" :value="group.id">
        {{ group.name }}
      </option></select
    ><select v-model="board.filters.assignee_id" aria-label="Исполнитель">
      <option value="">Все исполнители</option>
      <option value="me">Мои задачи</option>
      <option value="none">Без исполнителя</option>
      <option v-for="user in board.users" :key="user.id" :value="user.id">
        {{ user.display_name }}
      </option>
    </select>
    <div class="tag-filter">
      <button
        v-for="tag in board.tags"
        :key="tag.id"
        type="button"
        :class="['tag-choice', { selected: board.filters.tag_ids?.includes(tag.id) }]"
        @click="toggleTag(tag.id)"
      >
        {{ tag.name }}
      </button>
    </div>
    <button v-if="active" class="button button--quiet" @click="board.filters = {}">Сбросить</button>
  </aside>
</template>
