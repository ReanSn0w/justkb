import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import { commentsApi } from '../../api'
import CommentPanel from './CommentPanel.vue'

vi.mock('../../api', async () => {
  const actual = await vi.importActual<typeof import('../../api')>('../../api')
  return {
    ...actual,
    commentsApi: { list: vi.fn(), create: vi.fn() },
  }
})

describe('comments', () => {
  it('loads and adds Markdown comments', async () => {
    vi.mocked(commentsApi.list).mockResolvedValue({ items: [], next_cursor: null })
    vi.mocked(commentsApi.create).mockResolvedValue({
      id: 'comment-1',
      card_id: 'card-1',
      author_user_id: 'user-1',
      body: '**Готово**',
      created_at: '2026-11-01T10:00:00Z',
      updated_at: '2026-11-01T10:00:00Z',
      author: { id: 'user-1', display_name: 'Administrator' },
    })
    const wrapper = mount(CommentPanel, {
      props: { entityId: 'card-1', entityType: 'card' },
      global: { plugins: [createPinia()] },
    })
    await flushPromises()
    await wrapper.find('textarea').setValue('**Готово**')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(commentsApi.create).toHaveBeenCalledWith('card', 'card-1', '**Готово**')
    expect(wrapper.find('strong').text()).toBe('Administrator')
    expect(wrapper.html()).toContain('<strong>Готово</strong>')
  })
})
