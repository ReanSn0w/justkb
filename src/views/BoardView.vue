<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import Sortable, { type SortableEvent } from 'sortablejs'
import { useBoardStore } from '../stores/board'
import CardTile from '../components/board/CardTile.vue'
import BoardFilters from '../components/board/BoardFilters.vue'
import StatusState from '../components/ui/StatusState.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseDialog from '../components/ui/BaseDialog.vue'
import BaseField from '../components/ui/BaseField.vue'
import { useToastStore } from '../stores/toasts'
import { ApiError, type Card } from '../api'
import { filtersFromQuery, queryFromFilters, mergeFilterQuery } from '../features/filters'
import { cardMoveReference } from '../features/card-order'
const board = useBoardStore(),
  toasts = useToastStore(),
  initialized = ref(false),
  draft = ref<Record<string, string>>({}),
  route = useRoute(),
  router = useRouter()
let sortables: Sortable[] = []
const columnRow = ref<HTMLElement | null>(null)
let timer: ReturnType<typeof setTimeout> | undefined
let alive = true
let syncing = false
const signature = () => JSON.stringify(queryFromFilters(board.filters))
const descriptionEditor = ref<Pick<Card, 'id' | 'version' | 'description'> | null>(null)
const descriptionInput = ref<HTMLTextAreaElement | null>(null)
const savingDescription = ref(false)
const descriptionConflict = ref(false)
const descriptionError = ref('')
let descriptionTrigger: HTMLButtonElement | null = null

async function editDescription(card: Card, trigger: HTMLButtonElement) {
  descriptionTrigger = trigger
  descriptionEditor.value = { id: card.id, version: card.version, description: card.description }
  descriptionConflict.value = false
  descriptionError.value = ''
  await nextTick()
  descriptionInput.value?.focus()
}

async function closeDescription() {
  if (savingDescription.value) return
  descriptionEditor.value = null
  await nextTick()
  descriptionTrigger?.focus()
  descriptionTrigger = null
}

function keepDescriptionFocus(event: KeyboardEvent) {
  const fields = (event.currentTarget as HTMLElement).querySelectorAll<HTMLElement>(
    'button:enabled, textarea',
  )
  const first = fields[0]
  const last = fields[fields.length - 1]
  if (event.shiftKey ? document.activeElement === first : document.activeElement === last) {
    event.preventDefault()
    const next = event.shiftKey ? last : first
    next?.focus()
  }
}

async function saveDescription() {
  const editor = descriptionEditor.value
  if (!editor || savingDescription.value || descriptionConflict.value) return
  descriptionError.value = ''
  descriptionInput.value?.focus()
  savingDescription.value = true
  try {
    await board.update(editor.id, editor.version, { description: editor.description })
  } catch (cause) {
    if (cause instanceof ApiError && cause.code === 'card_version_conflict') {
      descriptionConflict.value = true
      const refreshed = await board.loadCards()
      descriptionError.value = refreshed
        ? 'Карточка изменилась. Скопируйте черновик и откройте редактор заново.'
        : 'Карточка изменилась. Скопируйте черновик и перезагрузите доску.'
    } else {
      descriptionError.value = 'Не удалось сохранить описание. Попробуйте ещё раз.'
    }
    await nextTick()
    descriptionInput.value?.focus()
    return
  } finally {
    savingDescription.value = false
  }
  await closeDescription()
}

function scheduleLoad() {
  clearTimeout(timer)
  timer = setTimeout(async () => {
    if (!alive) return
    if (!(await board.loadCards())) toasts.show('Не удалось обновить задачи', 'error')
  }, 250)
}

onMounted(async () => {
  board.filters = filtersFromQuery(route.query)
  if (board.columns.length) {
    initialized.value = true
    await nextTick()
    initializeSorting()
  }
  await board.initialize()
  if (!alive) return
  initialized.value = true
  await nextTick()
  initializeSorting()
})
onUnmounted(() => {
  alive = false
  clearTimeout(timer)
  sortables.forEach((sortable) => sortable.destroy())
  sortables = []
})
watch(
  () => board.filters,
  () => {
    if (!initialized.value || syncing || !alive) return
    void router.replace({ query: mergeFilterQuery(route.query, board.filters) })
    scheduleLoad()
  },
  { deep: true, flush: 'sync' },
)
watch(
  () => route.query,
  (query) => {
    const filters = filtersFromQuery(query)
    if (JSON.stringify(queryFromFilters(filters)) === signature()) return
    syncing = true
    board.filters = filters
    syncing = false
    if (initialized.value) scheduleLoad()
  },
)
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
async function reordered(event: SortableEvent & { originalEvent?: MouseEvent | TouchEvent }) {
  const { item, from, to, oldIndex, newIndex, originalEvent } = event
  const pointer =
    originalEvent && 'changedTouches' in originalEvent
      ? originalEvent.changedTouches[0]
      : originalEvent
  const hit =
    pointer && !originalEvent?.type.endsWith('cancel')
      ? document.elementFromPoint(pointer.clientX, pointer.clientY)
      : null
  const destination = originalEvent ? hit?.closest<HTMLElement>('[data-card-list]') : to
  const cardID = item.dataset.cardId
  const columnID = destination?.dataset.columnId
  const sameColumn = destination === from
  const onControls = hit?.closest('header, form')
  const placedAmongCards =
    destination === to &&
    (sameColumn ? !onControls : !originalEvent || Boolean(hit?.closest('[data-card-id]')))
  const reference = placedAmongCards ? cardMoveReference(item) : {}
  const changedOrder = from !== to || oldIndex !== newIndex

  // Sortable moved the DOM directly. Restore it before Vue renders the saved order.
  if (changedOrder && oldIndex !== undefined) {
    const siblings = Array.from(from.children).filter((child) => child !== item)
    from.insertBefore(item, siblings[oldIndex] ?? null)
  }
  columnRow.value?.style.removeProperty('min-height')
  if (!cardID || !columnID || (sameColumn && (!placedAmongCards || !changedOrder))) return
  const card = board.cards.find((item) => item.id === cardID)
  if (!card) return
  try {
    await board.move(card, columnID, sameColumn || reference.before_card_id ? reference : {})
  } catch {
    board.cards = []
    await nextTick()
    await board.loadCards()
    toasts.show('Карточка изменилась. Доска обновлена.', 'error')
  }
}
function initializeSorting() {
  sortables.forEach((sortable) => sortable.destroy())
  sortables = Array.from(document.querySelectorAll<HTMLElement>('[data-card-list]')).map(
    (element) =>
      new Sortable(element, {
        group: 'board-cards',
        animation: 150,
        draggable: '[data-card-id]',
        filter: '[data-card-assignee] button, [data-edit-description]',
        preventOnFilter: false,
        fallbackOnBody: true,
        swapThreshold: 0.65,
        onStart: () => {
          const row = columnRow.value
          if (row) row.style.minHeight = `${row.offsetHeight}px`
        },
        onEnd: (event) => void reordered(event),
      }),
  )
}
async function keyboard(card: Card, direction: -1 | 1) {
  const index = board.columns.findIndex((c) => c.id === card.column_id),
    target = board.columns[index + direction]
  if (target) await board.move(card, target.id)
}
</script>
<template>
  <section class="section pt-4 pb-4">
    <BoardFilters />
  </section>
  <section v-if="board.loading" class="section pt-0">
    <StatusState title="Загружаем доску…" tone="loading" />
  </section>
  <section v-else-if="board.error" class="section pt-0">
    <StatusState :title="board.error" tone="error">
      <BaseButton @click="board.initialize">Повторить</BaseButton>
    </StatusState>
  </section>
  <section v-else-if="!board.columns.length" class="section pt-0">
    <StatusState title="Нет колонок" detail="Администратор должен создать хотя бы одну колонку." />
  </section>
  <div v-else data-board-scroll class="is-overflow-x-auto" aria-label="Kanban-доска">
    <section class="section pt-0">
      <div
        ref="columnRow"
        class="columns is-mobile is-variable is-4 is-flex-wrap-nowrap is-align-items-stretch"
      >
        <section
          v-for="column in board.columns"
          :key="column.id"
          class="column is-11-mobile is-6-tablet is-4-desktop is-3-widescreen is-flex-shrink-0 box mb-0 board-column"
          data-card-list
          :data-column-id="column.id"
        >
          <header class="level is-mobile mb-4">
            <div class="level-left">
              <h2 class="title is-5 mb-0">{{ column.name }}</h2>
            </div>
            <div class="level-right">
              <span class="tag"
                >{{ board.byColumn[column.id]?.length || 0
                }}<template v-if="column.wip_limit"> / {{ column.wip_limit }}</template></span
              >
            </div>
          </header>
          <form class="field has-addons mb-4" @submit.prevent="add(column.id)">
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
          <div v-for="card in board.byColumn[column.id]" :key="card.id" :data-card-id="card.id">
            <CardTile
              :card="card"
              @move="keyboard(card, $event)"
              @edit-description="editDescription(card, $event)"
            />
          </div>
          <StatusState v-if="!board.byColumn[column.id]?.length" title="Пока пусто" />
        </section>
        <div class="section py-0 pl-0 ml-4 is-flex-shrink-0" aria-hidden="true"></div>
      </div>
    </section>
  </div>
  <BaseDialog
    :open="Boolean(descriptionEditor)"
    title="Описание карточки"
    aria-label="Описание карточки"
    @close="closeDescription"
    @keydown.esc.prevent.stop="closeDescription"
    @keydown.tab="keepDescriptionFocus"
  >
    <form v-if="descriptionEditor" id="board-description-form" @submit.prevent="saveDescription">
      <BaseField label="Описание" for-id="board-description">
        <textarea
          id="board-description"
          ref="descriptionInput"
          v-model="descriptionEditor.description"
          class="textarea"
          rows="8"
          :readonly="savingDescription"
        />
      </BaseField>
      <p v-if="descriptionError" class="help is-danger" role="alert">{{ descriptionError }}</p>
    </form>
    <template #actions>
      <BaseButton
        type="submit"
        form="board-description-form"
        :disabled="savingDescription || descriptionConflict"
      >
        Сохранить
      </BaseButton>
      <BaseButton tone="quiet" :disabled="savingDescription" @click="closeDescription">
        Отмена
      </BaseButton>
    </template>
  </BaseDialog>
</template>

<style scoped>
.board-column {
  position: relative;
  isolation: isolate;
  padding: calc(var(--bulma-column-gap) + 1rem);
  background: none;
  box-shadow: none;
}

/* Keep the visual gap inside the full-height drop surface. */
.board-column::before {
  position: absolute;
  inset: var(--bulma-column-gap);
  z-index: -1;
  border-radius: var(--bulma-box-radius);
  background: var(--bulma-box-background-color);
  box-shadow: var(--bulma-box-shadow);
  pointer-events: none;
  content: '';
}
</style>
