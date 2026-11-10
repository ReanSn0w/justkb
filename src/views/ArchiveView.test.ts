import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, expect, it, vi } from 'vitest'
import axe from 'axe-core'
import { archiveApi, cardsApi, epicsApi, type ArchiveEvent } from '../api'
import { useBoardStore } from '../stores/board'
import ArchiveView from './ArchiveView.vue'
vi.mock('../api', async (original) => ({
  ...(await original<typeof import('../api')>()),
  archiveApi: { list: vi.fn() },
  epicsApi: { list: vi.fn(), get: vi.fn() },
  cardsApi: { restore: vi.fn() },
}))
const entry: ArchiveEvent = {
  id: 'e',
  entity_type: 'card',
  entity_id: 'c',
  project_id: null,
  action: 'archived',
  occurred_at: '2026-11-10T12:00:00Z',
  reason: 'automatic',
  actor_type: 'system',
  actor_name: null,
  batch_id: null,
  title: 'Complete work',
  number: 3,
  rule_days: 3,
  is_archived: true,
  version: 2,
  child_count: 0,
  restorable_cards: 0,
}
beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(archiveApi.list).mockResolvedValue({ items: [entry], next_cursor: null })
  vi.mocked(epicsApi.list).mockResolvedValue({ items: [], next_cursor: null })
})
async function setup() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/archive', component: ArchiveView },
      { path: '/cards/:id', component: { template: '<p>Card</p>' } },
    ],
  })
  await router.push('/archive?q=work&history=true')
  await router.isReady()
  const pinia = createPinia()
  const board = useBoardStore(pinia)
  vi.spyOn(board, 'initialize').mockResolvedValue()
  const wrapper = mount(ArchiveView, {
    attachTo: document.body,
    global: { plugins: [pinia, router], stubs: { RouterLink: false } },
  })
  await flushPromises()
  return { wrapper, router, board }
}
it('reads URL filters, preserves return URL and has accessible timeline', async () => {
  const { wrapper, router } = await setup()
  expect(archiveApi.list).toHaveBeenCalledWith(
    expect.objectContaining({ q: 'work', history: true }),
  )
  expect(
    new URL(wrapper.get('a').attributes('href')!, 'http://localhost').searchParams.get('return_to'),
  ).toBe('/archive?q=work&history=true')
  await wrapper.get('#archive-query').setValue('next')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(router.currentRoute.value.query.q).toBe('next')
  expect(
    (await axe.run(wrapper.element, { rules: { 'color-contrast': { enabled: false } } }))
      .violations,
  ).toEqual([])
  wrapper.unmount()
})
it('refreshes the shared board and archive after restoration', async () => {
  const { wrapper, board } = await setup()
  await wrapper
    .findAll('button')
    .find((b) => b.text() === 'Восстановить')!
    .trigger('click')
  await flushPromises()
  expect(cardsApi.restore).toHaveBeenCalledWith('c', 2)
  expect(board.initialize).toHaveBeenCalledOnce()
  expect(archiveApi.list).toHaveBeenCalledTimes(2)
  wrapper.unmount()
})
