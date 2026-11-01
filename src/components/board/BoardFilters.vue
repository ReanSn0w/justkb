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
  <aside class="box p-4" aria-label="Фильтры доски">
    <header class="level is-mobile mb-4">
      <div class="level-left">
        <h2 class="title is-5 mb-0">Фильтры</h2>
      </div>
      <div class="level-right">
        <span
          :class="['tag', 'is-primary', { 'is-invisible': !board.refreshing }]"
          aria-live="polite"
        >
          Обновляем…
        </span>
      </div>
    </header>
    <div class="field is-grouped is-grouped-multiline">
      <div class="control is-expanded">
        <input
          v-model.trim="board.filters.q"
          class="input"
          type="search"
          placeholder="Поиск задач…"
          aria-label="Поиск задач"
        />
      </div>
      <div class="control is-expanded">
        <div class="select is-fullwidth">
          <select v-model="board.filters.epic_id" aria-label="Проект">
            <option :value="undefined" disabled>Выберите проект</option>
            <option value="">Все проекты</option>
            <option value="none">Без проекта</option>
            <option v-for="epic in board.epics" :key="epic.id" :value="epic.id">
              {{ epic.name }}
            </option>
          </select>
        </div>
      </div>
      <div class="control is-expanded">
        <div class="select is-fullwidth">
          <select v-model="board.filters.group_id" aria-label="Группа">
            <option :value="undefined" disabled>Выберите группу</option>
            <option value="">Все группы</option>
            <option value="none">Без группы</option>
            <option v-for="group in board.groups" :key="group.id" :value="group.id">
              {{ group.name }}
            </option>
          </select>
        </div>
      </div>
      <div class="control is-expanded">
        <div class="select is-fullwidth">
          <select v-model="board.filters.assignee_id" aria-label="Исполнитель">
            <option :value="undefined" disabled>Выберите исполнителя</option>
            <option value="">Все исполнители</option>
            <option value="me">Мои задачи</option>
            <option value="none">Без исполнителя</option>
            <option v-for="user in board.users" :key="user.id" :value="user.id">
              {{ user.display_name }}
            </option>
          </select>
        </div>
      </div>
      <div v-if="active" class="control">
        <button class="button" @click="board.filters = {}">Сбросить</button>
      </div>
    </div>
    <div v-if="board.tags.length" class="tags">
      <button
        v-for="tag in board.tags"
        :key="tag.id"
        type="button"
        :class="[
          'tag',
          'is-medium',
          board.filters.tag_ids?.includes(tag.id) ? 'is-info' : undefined,
        ]"
        @click="toggleTag(tag.id)"
      >
        {{ tag.name }}
      </button>
    </div>
  </aside>
</template>
