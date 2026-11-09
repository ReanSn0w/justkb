import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cardsApi, directoryApi, epicsApi, type Card } from '../api'
import BoardView from './BoardView.vue'

const sortableState = vi.hoisted(() => ({
  options: [] as Array<{ onEnd: (event: Record<string, unknown>) => void }>,
  destroy: vi.fn(),
}))

vi.mock('sortablejs', () => ({
  default: class {
    constructor(
      _element: HTMLElement,
      options: { onEnd: (event: Record<string, unknown>) => void },
    ) {
      sortableState.options.push(options)
    }
    destroy() {
      sortableState.destroy()
    }
  },
}))
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({
    replace: vi.fn().mockResolvedValue(true),
    resolve: () => ({ fullPath: '/' }),
  }),
}))
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

const card = (id: string, column_id: string, position: string): Card => ({
  id,
  number: Number(id),
  title: `Card ${id}`,
  description: '',
  column_id,
  position,
  version: 1,
  is_archived: false,
  tags: [],
  created_at: '',
  updated_at: '',
})
const initialCards = [card('1', 'a', '1000'), card('2', 'a', '2000'), card('3', 'b', '1000')]
const empty = { items: [], next_cursor: null }

describe('board sorting', () => {
  beforeEach(() => {
    sortableState.options.length = 0
    vi.clearAllMocks()
    vi.mocked(directoryApi.columns).mockResolvedValue({
      items: [
        { id: 'a', name: 'Todo', position: 1, created_at: '', updated_at: '' },
        { id: 'b', name: 'Done', position: 2, created_at: '', updated_at: '' },
      ],
      next_cursor: null,
    })
    vi.mocked(directoryApi.groups).mockResolvedValue(empty)
    vi.mocked(directoryApi.users).mockResolvedValue(empty)
    vi.mocked(directoryApi.tags).mockResolvedValue(empty)
    vi.mocked(epicsApi.list).mockResolvedValue(empty)
    vi.mocked(cardsApi.list).mockResolvedValue({
      items: initialCards.map((item) => ({ ...item })),
      next_cursor: null,
    })
    vi.mocked(cardsApi.move).mockImplementation(async (id, input) => ({
      ...initialCards.find((item) => item.id === id)!,
      column_id: input.target_column_id,
      position: id === '2' ? '0' : '2000',
      version: 2,
    }))
  })

  afterEach(() => document.body.replaceChildren())

  async function mountedBoard() {
    const wrapper = mount(BoardView, {
      attachTo: document.body,
      global: {
        plugins: [createPinia()],
        stubs: { RouterLink: { template: '<a><slot /></a>' } },
      },
    })
    await flushPromises()
    return wrapper
  }

  it('moves a card before its next sibling in the same column', async () => {
    const wrapper = await mountedBoard()
    const list = wrapper.get<HTMLElement>('[data-column-id="a"]').element
    const moved = list.querySelector<HTMLElement>('[data-card-id="2"]')!
    const first = list.querySelector<HTMLElement>('[data-card-id="1"]')!
    list.insertBefore(moved, first)

    sortableState.options[0]!.onEnd({ item: moved, from: list, to: list, oldIndex: 1, newIndex: 0 })
    await flushPromises()

    expect(cardsApi.move).toHaveBeenCalledWith('2', {
      expected_version: 1,
      target_column_id: 'a',
      before_card_id: '1',
    })
    wrapper.unmount()
  })

  it('moves a card after the last card in another column', async () => {
    const wrapper = await mountedBoard()
    const from = wrapper.get<HTMLElement>('[data-column-id="a"]').element
    const to = wrapper.get<HTMLElement>('[data-column-id="b"]').element
    const moved = from.querySelector<HTMLElement>('[data-card-id="1"]')!
    to.append(moved)

    sortableState.options[0]!.onEnd({ item: moved, from, to, oldIndex: 0, newIndex: 1 })
    await flushPromises()

    expect(cardsApi.move).toHaveBeenCalledWith('1', {
      expected_version: 1,
      target_column_id: 'b',
      after_card_id: '3',
    })
    wrapper.unmount()
  })

  it('reloads server order when moving fails', async () => {
    vi.mocked(cardsApi.move).mockRejectedValueOnce(new Error('conflict'))
    const wrapper = await mountedBoard()
    const from = wrapper.get<HTMLElement>('[data-column-id="a"]').element
    const to = wrapper.get<HTMLElement>('[data-column-id="b"]').element
    const moved = from.querySelector<HTMLElement>('[data-card-id="1"]')!
    to.append(moved)

    sortableState.options[0]!.onEnd({ item: moved, from, to, oldIndex: 0, newIndex: 1 })
    await flushPromises()

    expect(cardsApi.list).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-column-id="a"] [data-card-id="1"]').exists()).toBe(true)
    wrapper.unmount()
  })
})
