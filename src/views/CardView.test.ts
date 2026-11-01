import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { cardsApi, type Card } from '../api'
import { useBoardStore } from '../stores/board'
import CardView from './CardView.vue'

const mockedRoute = vi.hoisted(() => ({
  name: 'card',
  params: { id: 'card-1' },
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => mockedRoute,
    useRouter: () => ({ push: vi.fn(), go: vi.fn() }),
  }
})

vi.mock('../api', async () => {
  const actual = await vi.importActual<typeof import('../api')>('../api')
  return {
    ...actual,
    cardsApi: { ...actual.cardsApi, get: vi.fn(), activity: vi.fn() },
  }
})

const card: Card = {
  id: 'card-1',
  number: 1,
  title: 'Проверить вложения',
  description: 'Описание',
  column_id: 'backlog',
  position: '1',
  version: 1,
  is_archived: false,
  tags: [],
  attachments: [],
  created_at: '',
  updated_at: '',
}
let pinia: ReturnType<typeof createPinia>

function mountView() {
  return mount(CardView, {
    global: {
      plugins: [pinia],
      stubs: {
        AttachmentPanel: true,
        CardInfoPanel: true,
        CommentPanel: true,
        RouterLink: true,
      },
    },
  })
}

describe('card details layout', () => {
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    useBoardStore().columns = [
      { id: 'backlog', name: 'Backlog', position: 100, created_at: '', updated_at: '' },
    ]
    mockedRoute.name = 'card'
    vi.clearAllMocks()
    vi.mocked(cardsApi.get).mockResolvedValue(card)
    vi.mocked(cardsApi.activity).mockResolvedValue({ items: [], next_cursor: null })
  })

  it('shows readonly attachments with the description and comments in view mode', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('header > .level-item .card-number').text()).toBe('JKB-1')
    expect(wrapper.findAll('attachment-panel-stub')).toHaveLength(1)
    expect(wrapper.find('comment-panel-stub').exists()).toBe(true)
  })

  it('keeps upload controls on the side and hides comments in edit mode', async () => {
    mockedRoute.name = 'card-edit'
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('attachment-panel-stub')).toHaveLength(1)
    expect(wrapper.find('comment-panel-stub').exists()).toBe(false)
    expect(wrapper.text()).toContain('Команда')
    expect(wrapper.text()).toContain('Без команды')
  })
})
