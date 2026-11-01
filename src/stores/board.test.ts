import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cardsApi, directoryApi, epicsApi, type Card } from '../api'
import { useBoardStore } from './board'
vi.mock('../api', async () => {
  const actual = await vi.importActual<typeof import('../api')>('../api')
  return {
    ...actual,
    cardsApi: { ...actual.cardsApi, list: vi.fn(), move: vi.fn(), create: vi.fn() },
    epicsApi: { ...actual.epicsApi, list: vi.fn() },
    directoryApi: {
      columns: vi.fn(),
      groups: vi.fn(),
      users: vi.fn(),
      tags: vi.fn(),
    },
  }
})
const empty = { items: [], next_cursor: null }
const card = {
  id: '1',
  number: 1,
  title: 'A',
  description: '',
  column_id: 'a',
  position: 'a',
  version: 1,
  is_archived: false,
  tags: [],
  created_at: '',
  updated_at: '',
} as Card
describe('board store', () => {
  afterEach(() => vi.useRealTimers())
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.mocked(directoryApi.columns).mockResolvedValue({
      items: [{ id: 'a', name: 'Todo', position: 1, created_at: '', updated_at: '' }],
      next_cursor: null,
    })
    vi.mocked(epicsApi.list).mockResolvedValue(empty)
    vi.mocked(directoryApi.groups).mockResolvedValue(empty)
    vi.mocked(directoryApi.users).mockResolvedValue(empty)
    vi.mocked(directoryApi.tags).mockResolvedValue(empty)
    vi.mocked(cardsApi.list).mockResolvedValue({
      items: [structuredClone(card)],
      next_cursor: null,
    })
  })
  it('groups cards by ordered column', async () => {
    const store = useBoardStore()
    await store.initialize()
    expect(store.byColumn.a?.[0]?.title).toBe('A')
  })
  it('sends version when moving', async () => {
    vi.mocked(cardsApi.move).mockResolvedValue({ ...card, column_id: 'b', version: 2 })
    const store = useBoardStore()
    store.cards = [structuredClone(card)]
    await store.move(store.cards[0]!, 'b')
    expect(cardsApi.move).toHaveBeenCalledWith('1', {
      expected_version: 1,
      target_column_id: 'b',
      before_card_id: undefined,
    })
    expect(store.cards[0]?.version).toBe(2)
  })
  it('assigns quick-created cards to the filtered project', async () => {
    vi.mocked(cardsApi.create).mockResolvedValue({ ...card, epic_id: 'project-1' })
    const store = useBoardStore()
    store.filters = { epic_id: 'project-1' }
    await store.create('a', 'Project task')
    expect(cardsApi.create).toHaveBeenCalledWith({
      title: 'Project task',
      column_id: 'a',
      epic_id: 'project-1',
    })
  })
  it('keeps the current board visible during a readable refresh state', async () => {
    vi.useFakeTimers()
    const store = useBoardStore()
    store.cards = [structuredClone(card)]
    vi.mocked(cardsApi.list).mockResolvedValue({
      items: [{ ...card, id: '2', title: 'Filtered result' }],
      next_cursor: null,
    })

    const refresh = store.loadCards()
    await vi.advanceTimersByTimeAsync(399)
    expect(store.refreshing).toBe(true)
    expect(store.cards[0]?.title).toBe('A')
    await vi.advanceTimersByTimeAsync(1)
    await expect(refresh).resolves.toBe(true)
    expect(store.refreshing).toBe(false)
    expect(store.cards[0]?.title).toBe('Filtered result')
  })
  it('preserves visible cards when a refresh fails', async () => {
    const store = useBoardStore()
    store.cards = [structuredClone(card)]
    vi.mocked(cardsApi.list).mockRejectedValue(new Error('network'))

    await expect(store.loadCards()).resolves.toBe(false)
    expect(store.cards[0]?.title).toBe('A')
    expect(store.refreshing).toBe(false)
  })
})
