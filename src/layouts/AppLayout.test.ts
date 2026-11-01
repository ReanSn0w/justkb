import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import { useSessionStore } from '../stores/session'
import AppLayout from './AppLayout.vue'

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return { ...actual, useRouter: () => ({ push: vi.fn() }) }
})

describe('application navigation', () => {
  it('groups profile, admin settings and logout in the user menu', async () => {
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
    const wrapper = mount(AppLayout, {
      global: {
        plugins: [pinia],
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          RouterView: true,
        },
      },
    })

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
})
