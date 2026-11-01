import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { cardsApi, directoryApi, epicsApi, type Card } from '../api'
import EpicView from './EpicView.vue'

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => ({ name: 'project', params: { id: 'project-1' } }),
    useRouter: () => ({ push: vi.fn() }),
  }
})

vi.mock('../api', async () => {
  const actual = await vi.importActual<typeof import('../api')>('../api')
  return {
    ...actual,
    cardsApi: { ...actual.cardsApi, list: vi.fn() },
    epicsApi: { ...actual.epicsApi, get: vi.fn(), update: vi.fn() },
    directoryApi: { ...actual.directoryApi, columns: vi.fn() },
  }
})

const card = (id: string, column_id: string): Card => ({
  id,
  number: Number(id),
  title: `Task ${id}`,
  description: '',
  column_id,
  position: id,
  version: 1,
  is_archived: false,
  tags: [],
  created_at: '',
  updated_at: '',
})

describe('project details', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.mocked(epicsApi.get).mockResolvedValue({
      id: 'project-1',
      name: 'Platform',
      color: '#00d1b2',
      is_archived: false,
      card_count: 3,
      created_at: '',
      updated_at: '',
    })
    vi.mocked(directoryApi.columns).mockResolvedValue({
      items: [
        { id: 'backlog', name: 'Backlog', position: 100, created_at: '', updated_at: '' },
        { id: 'done', name: 'Done', position: 200, created_at: '', updated_at: '' },
      ],
      next_cursor: null,
    })
    vi.mocked(cardsApi.list)
      .mockResolvedValueOnce({
        items: [card('1', 'backlog'), card('2', 'backlog')],
        next_cursor: 'next',
      })
      .mockResolvedValueOnce({ items: [card('3', 'done')], next_cursor: null })
  })

  it('shows a paginated task distribution instead of a duplicate task list', async () => {
    const wrapper = mount(EpicView, {
      global: {
        plugins: [createPinia()],
        stubs: {
          AttachmentPanel: true,
          CommentPanel: true,
          RouterLink: { props: ['to'], template: '<a><slot /></a>' },
        },
      },
    })
    await flushPromises()
    expect(cardsApi.list).toHaveBeenNthCalledWith(2, {
      epic_id: 'project-1',
      limit: 100,
      cursor: 'next',
    })
    expect(wrapper.text()).toContain('Открыть на доске')
    expect(wrapper.text()).toContain('Backlog')
    expect(wrapper.text()).toContain('Done')
    expect(wrapper.findAll('progress').map((item) => item.attributes('value'))).toEqual(['2', '1'])
    expect(wrapper.text()).not.toContain('Task 1')
  })
})
