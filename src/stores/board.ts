import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  ApiError,
  cardsApi,
  directoryApi,
  epicsApi,
  type BoardColumn,
  type Card,
  type CardFilters,
  type Epic,
  type Group,
  type Tag,
  type User,
} from '../api'

export const useBoardStore = defineStore('board', () => {
  const columns = ref<BoardColumn[]>([]),
    cards = ref<Card[]>([]),
    epics = ref<Epic[]>([]),
    groups = ref<Group[]>([]),
    users = ref<User[]>([]),
    tags = ref<Tag[]>([])
  const loading = ref(false),
    error = ref(''),
    filters = ref<CardFilters>({})
  const byColumn = computed(() =>
    Object.fromEntries(
      columns.value.map((column) => [
        column.id,
        cards.value
          .filter((card) => card.column_id === column.id)
          .sort((a, b) => a.position.localeCompare(b.position)),
      ]),
    ),
  )
  async function loadDirectories() {
    const [c, e, g, u, t] = await Promise.all([
      directoryApi.columns(),
      epicsApi.list(),
      directoryApi.groups(),
      directoryApi.users(),
      directoryApi.tags(),
    ])
    columns.value = c.items.sort((a, b) => a.position - b.position)
    epics.value = e.items
    groups.value = g.items
    users.value = u.items.filter((x) => x.is_active)
    tags.value = t.items
  }
  async function loadCards() {
    loading.value = true
    error.value = ''
    try {
      const result = await cardsApi.list({ ...filters.value, limit: 100 })
      cards.value = result.items
      let cursor = result.next_cursor
      while (cursor) {
        const page = await cardsApi.list({ ...filters.value, limit: 100, cursor })
        cards.value.push(...page.items)
        cursor = page.next_cursor
      }
    } catch {
      error.value = 'Не удалось загрузить доску'
    } finally {
      loading.value = false
    }
  }
  async function initialize() {
    await Promise.all([loadDirectories(), loadCards()])
  }
  async function create(column_id: string, title: string) {
    const card = await cardsApi.create({ title, column_id })
    cards.value.push(card)
    return card
  }
  async function move(card: Card, target_column_id: string, before_card_id?: string) {
    const previous = { column_id: card.column_id, position: card.position, version: card.version }
    card.column_id = target_column_id
    try {
      const updated = await cardsApi.move(card.id, {
        expected_version: previous.version,
        target_column_id,
        before_card_id,
      })
      Object.assign(card, updated)
    } catch (cause) {
      Object.assign(card, previous)
      if (cause instanceof ApiError && cause.code === 'card_version_conflict' && cause.details.card)
        Object.assign(card, cause.details.card)
      throw cause
    }
  }
  return {
    columns,
    cards,
    epics,
    groups,
    users,
    tags,
    loading,
    error,
    filters,
    byColumn,
    initialize,
    loadCards,
    create,
    move,
  }
})
