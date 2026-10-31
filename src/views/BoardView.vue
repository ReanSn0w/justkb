<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useBoardStore } from '../stores/board'
import CardTile from '../components/board/CardTile.vue'
import StatusState from '../components/ui/StatusState.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import { useToastStore } from '../stores/toasts'
import type { Card } from '../api'
const board = useBoardStore(),
  toasts = useToastStore(),
  dragged = ref<Card | null>(null),
  draft = ref<Record<string, string>>({})
onMounted(() => board.initialize())
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
  <section class="board-page">
    <header class="board-heading">
      <div>
        <h1>Доска</h1>
        <p class="muted">Одна доска, единый процесс.</p>
      </div>
      <BaseButton tone="quiet" @click="board.loadCards">Обновить</BaseButton>
    </header>
    <StatusState v-if="board.loading" title="Загружаем доску…" tone="loading" /><StatusState
      v-else-if="board.error"
      :title="board.error"
      tone="error"
      ><BaseButton @click="board.initialize">Повторить</BaseButton></StatusState
    ><StatusState
      v-else-if="!board.columns.length"
      title="Нет колонок"
      detail="Администратор должен создать хотя бы одну колонку."
    />
    <div v-else class="board" aria-label="Kanban-доска">
      <section
        v-for="column in board.columns"
        :key="column.id"
        class="board-column"
        @dragover.prevent
        @drop="drop(column.id)"
      >
        <header>
          <h2>{{ column.name }}</h2>
          <span
            >{{ board.byColumn[column.id]?.length || 0
            }}<template v-if="column.wip_limit"> / {{ column.wip_limit }}</template></span
          >
        </header>
        <div class="card-list">
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
        <form class="quick-add" @submit.prevent="add(column.id)">
          <input
            v-model="draft[column.id]"
            :aria-label="`Новая карточка в ${column.name}`"
            placeholder="Новая карточка…"
          /><button aria-label="Создать карточку">+</button>
        </form>
      </section>
    </div>
  </section>
</template>
