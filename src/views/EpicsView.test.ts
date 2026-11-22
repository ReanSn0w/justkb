import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
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
  it('shows projects as progress rows without full descriptions', async () => {
    vi.mocked(epicsApi.list).mockResolvedValue({
      items: [
        {
          id: 'e1',
          name: 'Platform',
          description: 'Shared services',
          color: '#00d1b2',
          is_archived: false,
          card_count: 3,
          version: 1,
          restorable_cards: 0,
          completed_card_count: 2,
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
    expect(epicsApi.list).toHaveBeenCalledWith()
    expect(wrapper.text()).toContain('Platform')
    expect(wrapper.text()).not.toContain('Shared services')
    expect(wrapper.text()).toContain('3 задачи')
    expect(wrapper.text()).toContain('2 из 3 в последней колонке')
    expect(wrapper.get('progress').attributes('value')).toBe('67')
  })

  it.each(['admin', 'member'] as const)(
    'creates a project from the directory as %s',
    async (role) => {
      vi.mocked(epicsApi.list).mockResolvedValue({ items: [], next_cursor: null })
      vi.mocked(epicsApi.create).mockReset().mockResolvedValue({
        id: 'created-project',
        name: 'Release',
        description: 'Next milestone',
        color: '#00d1b2',
        is_archived: false,
        card_count: 0,
        completed_card_count: 0,
        restorable_cards: 0,
        version: 1,
        created_at: '',
        updated_at: '',
      })
      const pinia = createPinia()
      setActivePinia(pinia)
      useSessionStore().user = {
        id: 'u1',
        username: role,
        display_name: 'Test user',
        role,
        is_active: true,
        created_at: '',
        updated_at: '',
      }
      const router = createRouter({
        history: createMemoryHistory(),
        routes: [
          { path: '/projects', component: EpicsView },
          { path: '/projects/:id', component: { template: '<div />' } },
        ],
      })
      await router.push('/projects')
      const wrapper = mount(EpicsView, {
        attachTo: document.body,
        global: { plugins: [pinia, router] },
      })
      try {
        await flushPromises()

        expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
        expect(wrapper.find('form').exists()).toBe(false)

        const button = wrapper.get('button')
        expect(button.text()).toBe('Создать проект')
        await button.trigger('click')

        expect(wrapper.get('[role="dialog"]').text()).toContain('Создать проект')
        expect(wrapper.get('#project-name').element).toBe(document.activeElement)
        expect(wrapper.get('#project-description').attributes('rows')).toBe('8')

        await wrapper.get('#project-name').setValue('Release')
        await wrapper.get('#project-description').setValue('Next milestone')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(epicsApi.create).toHaveBeenCalledWith({
          name: 'Release',
          description: 'Next milestone',
          color: '#00d1b2',
          is_archived: false,
        })
        expect(router.currentRoute.value.path).toBe('/projects/created-project')
      } finally {
        wrapper.unmount()
      }
    },
  )
})
