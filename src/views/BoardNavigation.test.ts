import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, expect, it, vi } from 'vitest'
import { cardsApi, directoryApi, epicsApi } from '../api'
import { useBoardStore } from '../stores/board'
import BoardView from './BoardView.vue'
vi.mock('sortablejs', () => ({
  default: class {
    destroy() {}
  },
}))
vi.mock('../api', async () => {
  const actual = await vi.importActual<typeof import('../api')>('../api')
  return {
    ...actual,
    cardsApi: { ...actual.cardsApi, list: vi.fn() },
    epicsApi: { ...actual.epicsApi, list: vi.fn() },
    directoryApi: { columns: vi.fn(), groups: vi.fn(), users: vi.fn(), tags: vi.fn() },
  }
})
afterEach(() => vi.useRealTimers())
it('restores query-only history, preserves unrelated query and cancels delayed work after leaving', async () => {
  const empty = { items: [], next_cursor: null }
  for (const method of [
    directoryApi.groups,
    directoryApi.users,
    directoryApi.tags,
    epicsApi.list,
    cardsApi.list,
  ])
    vi.mocked(method).mockResolvedValue(empty)
  vi.mocked(directoryApi.columns).mockResolvedValue({
    items: [{ id: 'a', name: 'Todo', position: 1, created_at: '', updated_at: '' }],
    next_cursor: null,
  })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: BoardView },
      { path: '/cards/:id', component: { template: '<p>Card</p>' } },
    ],
  })
  await router.push('/?q=first&extra=keep')
  const pinia = createPinia()
  const wrapper = mount(
    { template: '<RouterView />' },
    { global: { plugins: [pinia, router], stubs: { RouterView: false, RouterLink: false } } },
  )
  await flushPromises()
  const board = useBoardStore(pinia)
  expect(board.filters.q).toBe('first')
  await router.push('/?q=second&extra=keep')
  await flushPromises()
  expect(wrapper.get('input[type=search]').element).toHaveProperty('value', 'second')
  router.back()
  await flushPromises()
  expect(board.filters.q).toBe('first')
  vi.useFakeTimers()
  await wrapper.get('input[type=search]').setValue('last')
  await flushPromises()
  expect(router.currentRoute.value.query).toMatchObject({ q: 'last', extra: 'keep' })
  await router.push('/cards/1')
  await vi.advanceTimersByTimeAsync(500)
  expect(router.currentRoute.value.path).toBe('/cards/1')
  wrapper.unmount()
})
