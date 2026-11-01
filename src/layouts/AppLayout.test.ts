import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import { useSessionStore } from '../stores/session'
import AppLayout from './AppLayout.vue'

const mockedRoute = vi.hoisted(() => ({ name: 'board' }))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => mockedRoute,
    useRouter: () => ({ push: vi.fn() }),
  }
})

function mountLayout(pinia: ReturnType<typeof createPinia>) {
  return mount(AppLayout, {
    global: {
      plugins: [pinia],
      stubs: {
        RouterLink: { template: '<a><slot /></a>' },
        RouterView: true,
      },
    },
  })
}

describe('application navigation', () => {
  it('groups profile, admin settings and logout in the user menu', async () => {
    mockedRoute.name = 'board'
    const pinia = createPinia()
    setActivePinia(pinia)
    useSessionStore().user = {
      id: 'u1',
      username: 'admin',
      display_name: 'Ada Admin',
      role: 'admin',
      is_active: true,
      created_at: '',
      updated_at: '',
    }
    const wrapper = mountLayout(pinia)

    expect(wrapper.get('.navbar').classes()).not.toContain('is-light')
    expect(
      wrapper.findAll('.navbar-start .navbar-item').every((item) => item.classes('is-tab')),
    ).toBe(true)
    expect(wrapper.get('.navbar-start').text()).toContain('Доска')
    expect(wrapper.get('.navbar-start').text()).toContain('Проекты')
    expect(wrapper.get('.navbar-start').text()).not.toContain('Настройки')
    const dropdown = wrapper.get('.has-dropdown')
    expect(dropdown.classes()).not.toContain('is-active')
    await wrapper.get('[aria-haspopup="true"]').trigger('click')
    expect(dropdown.classes()).toContain('is-active')
    expect(dropdown.text()).toContain('Настройки профиля')
    expect(dropdown.text()).toContain('Настройки системы')
    expect(dropdown.text()).toContain('Выйти')
    expect(dropdown.get('button').classes()).not.toContain('is-white')
    await dropdown.trigger('keydown.esc')
    expect(dropdown.classes()).not.toContain('is-active')
  })

  it('marks only the navigation section that owns the current route as active', () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    mockedRoute.name = 'project'
    const projectLayout = mountLayout(pinia)
    const projectLinks = projectLayout.findAll('.navbar-start .navbar-item')
    expect(projectLinks).toHaveLength(2)
    expect(projectLinks[0]!.classes()).not.toContain('is-active')
    expect(projectLinks[1]!.classes()).toContain('is-active')
    expect(projectLinks[1]!.attributes('aria-current')).toBe('page')
    projectLayout.unmount()

    mockedRoute.name = 'settings'
    const settingsLayout = mountLayout(pinia)
    expect(
      settingsLayout
        .findAll('.navbar-start .navbar-item')
        .every((item) => !item.classes('is-active')),
    ).toBe(true)
  })
})
