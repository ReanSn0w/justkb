<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useDebounceFn } from '@vueuse/core'
import { useBoardStore } from '../stores/board'
import CardTile from '../components/board/CardTile.vue'
import BoardFilters from '../components/board/BoardFilters.vue'
import StatusState from '../components/ui/StatusState.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import { useToastStore } from '../stores/toasts'
import type { Card } from '../api'
import { filtersFromQuery, queryFromFilters } from '../features/filters'
const board = useBoardStore(),
  toasts = useToastStore(),
  dragged = ref<Card | null>(null),
  draft = ref<Record<string, string>>({}),
  route = useRoute(),
  router = useRouter()
onMounted(async () => {
  board.filters = filtersFromQuery(route.query)
  await board.initialize()
})
const applyFilters = useDebounceFn(async () => {
  await router.replace({ query: queryFromFilters(board.filters) })
  await board.loadCards()
}, 250)
watch(() => board.filters, applyFilters, { deep: true })
async function add(column: string) {
  const title = draft.value[column]?.trim()
  if (!title) return
  try {
    await board.create(column, title)
    draft.value[column] = ''
  } catch {
    toasts.show('Не удалось создать карточку', 'error')
  }
}
async function drop(column: string, before?: string) {
  if (!dragged.value) return
  try {
    await board.move(dragged.value, column, before)
  } catch {
    toasts.show('Карточка изменилась. Доска обновлена.', 'error')
  } finally {
    dragged.value = null
  }
}
async function keyboard(card: Card, direction: -1 | 1) {
  const index = board.columns.findIndex((c) => c.id === card.column_id),
    target = board.columns[index + direction]
  if (target) {
    dragged.value = card
    await drop(target.id)
  }
}
</script>
<template>
  <section class="section">
    <div class="container is-fluid">
      <BoardFilters />
      <StatusState v-if="board.loading" title="Загружаем доску…" tone="loading" />
      <StatusState v-else-if="board.error" :title="board.error" tone="error">
        <BaseButton @click="board.initialize">Повторить</BaseButton>
      </StatusState>
      <StatusState
        v-else-if="!board.columns.length"
        title="Нет колонок"
        detail="Администратор должен создать хотя бы одну колонку."
      />
      <div v-else class="columns is-multiline is-variable is-4" aria-label="Kanban-доска">
        <div
          v-for="column in board.columns"
          :key="column.id"
          class="column is-half-tablet is-one-quarter-widescreen"
        >
          <section class="box" @dragover.prevent @drop="drop(column.id)">
            <header class="level is-mobile mb-4">
              <div class="level-left">
                <h2 class="title is-5 mb-0">{{ column.name }}</h2>
              </div>
              <div class="level-right">
                <span class="tag is-light"
                  >{{ board.byColumn[column.id]?.length || 0
                  }}<template v-if="column.wip_limit"> / {{ column.wip_limit }}</template></span
                >
              </div>
            </header>
            <div>
              <div
                v-for="card in board.byColumn[column.id]"
                :key="card.id"
                @dragover.prevent.stop
                @drop.stop="drop(column.id, card.id)"
              >
                <CardTile :card="card" @drag="dragged = $event" @move="keyboard(card, $event)" />
              </div>
              <StatusState v-if="!board.byColumn[column.id]?.length" title="Пока пусто" />
            </div>
            <form class="field has-addons mt-4" @submit.prevent="add(column.id)">
              <div class="control is-expanded">
                <input
                  v-model="draft[column.id]"
                  class="input"
                  :aria-label="`Новая карточка в ${column.name}`"
                  placeholder="Новая карточка…"
                />
              </div>
              <div class="control">
                <button class="button is-primary" aria-label="Создать карточку">+</button>
              </div>
            </form>
          </section>
        </div>
      </div>
    </div>
  </section>
</template>
