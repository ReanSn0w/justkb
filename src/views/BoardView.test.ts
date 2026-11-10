import { flushPromises, mount } from '@vue/test-utils'
import axe from 'axe-core'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, cardsApi, directoryApi, epicsApi, type Card, type User } from '../api'
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
    cardsApi: {
      ...actual.cardsApi,
      list: vi.fn(),
      move: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
    },
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
const boardUsers: User[] = ['Анна', 'Борис'].map((display_name, index) => ({
  id: `user-${index}`,
  username: `user-${index}`,
  display_name,
  is_active: true,
  role: 'member',
  created_at: '',
  updated_at: '',
}))

describe('board', () => {
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

  it('assigns, replaces and clears an assignee from the tile using each new version', async () => {
    vi.mocked(directoryApi.users).mockResolvedValue({ ...empty, items: boardUsers })
    vi.mocked(cardsApi.update).mockImplementation(async (_id, version, patch) => ({
      ...initialCards[0]!,
      assignee_id: patch.assignee_id,
      assignee: boardUsers.find((user) => user.id === patch.assignee_id) ?? null,
      version: version + 1,
    }))
    const wrapper = await mountedBoard()
    const tile = wrapper.get('[data-card-id="1"]')
    const trigger = tile.get<HTMLButtonElement>('[aria-label="Назначить исполнителя"]')

    await trigger.trigger('click')
    const firstOption = tile.get('[data-assignee-option]')
    expect(document.activeElement).toBe(firstOption.element)
    const result = await axe.run(tile.element, { rules: { 'color-contrast': { enabled: false } } })
    expect(result.violations).toEqual([])
    await tile.get('.tags[data-card-assignee]').trigger('click')
    expect(tile.find('[data-assignee-option]').exists()).toBe(false)
    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(cardsApi.update).not.toHaveBeenCalled()

    await trigger.trigger('click')
    await tile.get('[data-assignee-option]').trigger('keydown.esc')
    expect(tile.find('[data-assignee-option]').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)

    await trigger.trigger('click')
    await tile.get('[data-assignee-option]').trigger('click')
    await flushPromises()
    expect(cardsApi.update).toHaveBeenNthCalledWith(1, '1', 1, { assignee_id: 'user-0' })
    await tile.get('[aria-label="Сменить исполнителя: Анна"]').trigger('click')
    await tile.findAll('[data-assignee-option]')[1]!.trigger('click')
    await flushPromises()
    expect(cardsApi.update).toHaveBeenNthCalledWith(2, '1', 2, { assignee_id: 'user-1' })
    expect(tile.get('[aria-label="Сменить исполнителя: Борис"]').text()).toBe('Борис')
    await tile.get('[aria-label="Снять исполнителя"]').trigger('click')
    await flushPromises()
    expect(cardsApi.update).toHaveBeenNthCalledWith(3, '1', 3, { assignee_id: null })
    expect(tile.find('[aria-label="Снять исполнителя"]').exists()).toBe(false)
    expect(tile.get('[aria-label="Назначить исполнителя"]').text()).toBe('+')
    wrapper.unmount()
  })

  it('reloads the board after a conflicting assignment', async () => {
    const assignee = boardUsers[1]!
    vi.mocked(directoryApi.users).mockResolvedValue({ ...empty, items: boardUsers })
    vi.mocked(cardsApi.update).mockRejectedValueOnce(
      new ApiError(409, 'card_version_conflict', 'Conflict'),
    )
    const wrapper = await mountedBoard()
    vi.mocked(cardsApi.list).mockResolvedValueOnce({
      ...empty,
      items: [{ ...initialCards[0]!, assignee, assignee_id: assignee.id, version: 2 }],
    })
    const tile = wrapper.get('[data-card-id="1"]')
    await tile.get('[aria-label="Назначить исполнителя"]').trigger('click')
    await tile.get('[data-assignee-option]').trigger('click')
    await flushPromises()

    expect(cardsApi.list).toHaveBeenCalledTimes(2)
    expect(tile.get('[aria-label="Сменить исполнителя: Борис"]').text()).toBe('Борис')
    wrapper.unmount()
  })
})
