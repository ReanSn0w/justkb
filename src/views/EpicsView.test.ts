import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import { epicsApi } from '../api'
import { useSessionStore } from '../stores/session'
import EpicsView from './EpicsView.vue'

vi.mock('../api', async () => {
  const actual = await vi.importActual<typeof import('../api')>('../api')
  return {
    ...actual,
    epicsApi: { ...actual.epicsApi, list: vi.fn(), create: vi.fn() },
  }
})

describe('project directory', () => {
  it('shows project descriptions and task counts', async () => {
    vi.mocked(epicsApi.list).mockResolvedValue({
      items: [
        {
          id: 'e1',
          name: 'Platform',
          description: 'Shared services',
          color: '#00d1b2',
          is_archived: false,
          card_count: 3,
          created_at: '',
          updated_at: '',
        },
      ],
      next_cursor: null,
    })
    const pinia = createPinia()
    setActivePinia(pinia)
    useSessionStore().user = {
      id: 'u1',
      username: 'admin',
      display_name: 'Admin',
      role: 'admin',
      is_active: true,
      created_at: '',
      updated_at: '',
    }
    const wrapper = mount(EpicsView, {
      global: {
        plugins: [pinia],
        mocks: { $router: { push: vi.fn() } },
        stubs: { RouterLink: { template: '<a><slot /></a>' } },
      },
    })
    await flushPromises()
    expect(epicsApi.list).toHaveBeenCalledWith(true)
    expect(wrapper.text()).toContain('Platform')
    expect(wrapper.text()).toContain('Shared services')
    expect(wrapper.text()).toContain('3 задач')
  })
})
