import axe from 'axe-core'
import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { describe, expect, it } from 'vitest'
import LoginView from '../views/LoginView.vue'
describe('automated accessibility', () => {
  it('has no serious login violations', async () => {
    const wrapper = mount(LoginView, {
      attachTo: document.body,
      global: {
        plugins: [createPinia()],
        mocks: { $route: { query: {} }, $router: { replace: () => {} } },
      },
    })
    const result = await axe.run(wrapper.element, {
      rules: { 'color-contrast': { enabled: false } },
    })
    expect(wrapper.get('h1').text()).toBe('JKB')
    expect(wrapper.get('.subtitle').text()).toBe('Канбан доска')
    expect(wrapper.get('form h2').text()).toBe('Вход')
    expect(wrapper.get('.hero').classes()).not.toContain('is-light')
    expect(
      result.violations.filter((v) => ['serious', 'critical'].includes(v.impact || '')),
    ).toEqual([])
    wrapper.unmount()
  })
})
