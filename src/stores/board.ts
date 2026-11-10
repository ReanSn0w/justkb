import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import {
  ApiError,
  cardsApi,
  directoryApi,
  epicsApi,
  type BoardColumn,
  type Card,
  type CardFilters,
  type CardPatch,
  type Epic,
  type Group,
  type MoveCardInput,
  type Tag,
  type User,
} from '../api'
import { compareCardPositions } from '../features/card-order'

const minimumRefreshDuration = 400

export const useBoardStore = defineStore('board', () => {
  const columns = ref<BoardColumn[]>([]),
    cards = ref<Card[]>([]),
    epics = ref<Epic[]>([]),
    groups = ref<Group[]>([]),
    users = ref<User[]>([]),
    tags = ref<Tag[]>([])
  const loading = ref(false),
    refreshing = ref(false),
    error = ref(''),
    filters = ref<CardFilters>({})
  let refreshSequence = 0
  watch(
    filters,
    () => {
      refreshSequence += 1
    },
    { deep: true, flush: 'sync' },
  )
  const byColumn = computed(() =>
    Object.fromEntries(
      columns.value.map((column) => [
        column.id,
        cards.value
          .filter((card) => card.column_id === column.id)
          .sort((a, b) => compareCardPositions(a.position, b.position)),
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
  async function fetchCards(activeFilters: CardFilters) {
    const result = await cardsApi.list({ ...activeFilters, limit: 100 })
    const items = [...result.items]
    let cursor = result.next_cursor
    while (cursor) {
      const page = await cardsApi.list({ ...activeFilters, limit: 100, cursor })
      items.push(...page.items)
      cursor = page.next_cursor
    }
    return items
  }
  async function loadCards(activeFilters: CardFilters = { ...filters.value }) {
    const sequence = ++refreshSequence
    const startedAt = Date.now()
    refreshing.value = true
    try {
      const items = await fetchCards(activeFilters)
      if (sequence !== refreshSequence) return true
      cards.value = items
      const remaining = minimumRefreshDuration - (Date.now() - startedAt)
      if (remaining > 0) await new Promise((resolve) => setTimeout(resolve, remaining))
      return true
    } catch {
      return sequence !== refreshSequence
    } finally {
      if (sequence === refreshSequence) refreshing.value = false
    }
  }
  async function initialize() {
    const sequence = ++refreshSequence
    loading.value = columns.value.length === 0
    error.value = ''
    try {
      const [, items] = await Promise.all([loadDirectories(), fetchCards({ ...filters.value })])
      if (sequence === refreshSequence) cards.value = items
      else await loadCards()
    } catch {
      if (sequence === refreshSequence) error.value = 'Не удалось загрузить доску'
      else await loadCards()
    } finally {
      loading.value = false
    }
  }
  async function create(column_id: string, title: string) {
    const epicID = filters.value.epic_id
    const card = await cardsApi.create({
      title,
      column_id,
      epic_id: epicID && epicID !== 'none' ? epicID : undefined,
    })
    cards.value.push(card)
    return card
  }
  async function update(cardID: string, version: number, patch: CardPatch) {
    const updated = await cardsApi.update(cardID, version, patch)
    const current = cards.value.find((item) => item.id === updated.id)
    if (current && current.version <= updated.version) Object.assign(current, updated)
    if (
      refreshing.value ||
      (patch.assignee_id !== undefined && filters.value.assignee_id) ||
      (patch.description !== undefined && filters.value.q)
    )
      await loadCards()
  }
  async function move(
    card: Card,
    target_column_id: string,
    reference: Pick<MoveCardInput, 'before_card_id' | 'after_card_id'> = {},
  ) {
    const previous = { column_id: card.column_id, position: card.position, version: card.version }
    card.column_id = target_column_id
    try {
      const updated = await cardsApi.move(card.id, {
        expected_version: previous.version,
        target_column_id,
        ...reference,
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
    refreshing,
    error,
    filters,
    byColumn,
    initialize,
    loadCards,
    create,
    update,
    move,
  }
})
