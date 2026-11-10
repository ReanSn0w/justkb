import { flushPromises, mount } from '@vue/test-utils'
import axe from 'axe-core'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, cardsApi, type Card } from '../api'
import { useBoardStore } from '../stores/board'
import CardView from './CardView.vue'

const mockedRoute = vi.hoisted(() => ({
  name: 'card',
  params: { id: 'card-1' },
  query: {} as Record<string, string>,
}))

const push = vi.hoisted(() => vi.fn())

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => mockedRoute,
    useRouter: () => ({ push, go: vi.fn() }),
  }
})

vi.mock('../api', async () => {
  const actual = await vi.importActual<typeof import('../api')>('../api')
  return {
    ...actual,
    cardsApi: { ...actual.cardsApi, get: vi.fn(), activity: vi.fn(), update: vi.fn() },
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
    mockedRoute.query = {}
    vi.resetAllMocks()
    vi.mocked(cardsApi.get).mockResolvedValue(structuredClone(card))
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

  it('adds an assignee while keeping the inactive legacy assignee', async () => {
    mockedRoute.name = 'card-edit'
    const legacy = { id: 'legacy', display_name: 'Прежний исполнитель' }
    const selected = { id: 'new', display_name: 'Новый исполнитель' }
    useBoardStore().users = [
      {
        ...selected,
        username: 'new',
        role: 'member',
        is_active: true,
        created_at: '',
        updated_at: '',
      },
    ]
    vi.mocked(cardsApi.get).mockResolvedValue({ ...card, assignee_id: legacy.id, assignee: legacy })
    vi.mocked(cardsApi.update).mockResolvedValue({
      ...card,
      assignee_ids: [legacy.id, selected.id],
    })
    const wrapper = mountView()
    await flushPromises()
    const select = wrapper.get<HTMLSelectElement>('#card-assignee-picker')
    const remove = (name: string) => `[aria-label="Снять исполнителя: ${name}"]`
    expect(wrapper.find(remove(legacy.display_name)).exists()).toBe(true)
    expect(select.find(`option[value="${legacy.id}"]`).exists()).toBe(false)
    document.body.append(wrapper.element)
    const result = await axe.run(select.element.closest('.field')!, {
      rules: { 'color-contrast': { enabled: false } },
    })
    expect(result.violations).toEqual([])
    await select.setValue(selected.id)
    expect(select.element.value).toBe('')
    expect(wrapper.findAll('.card-assignee-badge')).toHaveLength(2)
    await wrapper.get(remove(selected.display_name)).trigger('click')
    expect(wrapper.findAll('.card-assignee-badge')).toHaveLength(1)
    expect(document.activeElement).toBe(select.element)
    expect(cardsApi.update).not.toHaveBeenCalled()
    await select.setValue(selected.id)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(cardsApi.update).toHaveBeenCalledWith(
      card.id,
      card.version,
      expect.objectContaining({ assignee_ids: [legacy.id, selected.id] }),
    )
    wrapper.unmount()
  })
})

describe('return from card editing', () => {
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    useBoardStore().columns = [
      { id: 'backlog', name: 'Backlog', position: 100, created_at: '', updated_at: '' },
    ]
    mockedRoute.name = 'card-edit'
    mockedRoute.query = {
      return_to: '/?assignee_id=me&tag_id=a&tag_id=b&q=test',
      edit_return: 'board',
    }
    vi.resetAllMocks()
    vi.mocked(cardsApi.get).mockResolvedValue(structuredClone(card))
    vi.mocked(cardsApi.activity).mockResolvedValue({ items: [], next_cursor: null })
    vi.mocked(cardsApi.update).mockResolvedValue({ ...card, version: 2 })
  })

  it('returns directly to the filtered board after saving', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(push).toHaveBeenCalledExactlyOnceWith(mockedRoute.query.return_to)
    wrapper.unmount()
  })

  it('cancels without saving or requiring another card fetch', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('textarea').setValue('Unsaved draft')
    vi.mocked(cardsApi.get).mockRejectedValueOnce(new Error('offline'))
    await wrapper.get('header .level-left button').trigger('click')
    await flushPromises()
    expect(push).toHaveBeenCalledExactlyOnceWith(mockedRoute.query.return_to)
    expect(cardsApi.update).not.toHaveBeenCalled()
    expect(cardsApi.get).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it.each(['save', 'cancel'])(
    'keeps the card details return for ordinary editing: %s',
    async (action) => {
      delete mockedRoute.query.edit_return
      const wrapper = mountView()
      await flushPromises()
      if (action === 'save') await wrapper.get('form').trigger('submit')
      else await wrapper.get('header .level-left button').trigger('click')
      await flushPromises()
      expect(push).toHaveBeenCalledExactlyOnceWith({
        path: '/cards/card-1',
        query: { return_to: mockedRoute.query.return_to },
      })
      wrapper.unmount()
    },
  )

  it.each([new ApiError(409, 'card_version_conflict', 'Conflict', {}), new Error('offline')])(
    'keeps the draft and editor open when saving fails',
    async (error) => {
      vi.mocked(cardsApi.update).mockRejectedValueOnce(error)
      const wrapper = mountView()
      await flushPromises()
      await wrapper.get('textarea').setValue('Keep this draft')
      await wrapper.get('form').trigger('submit')
      await flushPromises()
      expect(push).not.toHaveBeenCalled()
      expect(wrapper.get<HTMLTextAreaElement>('textarea').element.value).toBe('Keep this draft')
      expect(wrapper.get('[role="alert"]').text()).toContain(
        error instanceof ApiError ? 'Карточка уже изменена' : 'Не удалось сохранить',
      )
      wrapper.unmount()
    },
  )

  it('rejects an external return address', async () => {
    mockedRoute.query.return_to = '//example.com'
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(push).toHaveBeenCalledExactlyOnceWith('/')
    wrapper.unmount()
  })
})
