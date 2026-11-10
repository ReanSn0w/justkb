import { flushPromises, mount } from '@vue/test-utils'
import axe from 'axe-core'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { expect, it } from 'vitest'
import type { Card } from '../../api'
import { useBoardStore } from '../../stores/board'
import CardTile from './CardTile.vue'

it('opens editing directly with current filters and an accessible keyboard link', async () => {
  const pinia = createPinia()
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/cards/:id', component: { template: '<div />' } },
      { path: '/cards/:id/edit', component: { template: '<div />' } },
    ],
  })
  await router.push('/?q=old')
  useBoardStore(pinia).filters = { q: 'new', assignee_id: 'me', tag_ids: ['a', 'b'] }
  const card: Card = {
    id: 'card-1',
    number: 42,
    title: 'Shared task',
    description: '',
    column_id: 'backlog',
    position: '1',
    version: 1,
    is_archived: false,
    tags: [],
    created_at: '',
    updated_at: '',
    assignees: [
      { id: 'a', display_name: 'Alice' },
      { id: 'b', display_name: 'Bob' },
    ],
  }
  const wrapper = mount(CardTile, {
    props: { card },
    attachTo: document.body,
    global: { plugins: [pinia, router], stubs: { RouterLink: false } },
  })
  const link = wrapper.get<HTMLAnchorElement>('[data-edit-card]')
  expect(link.attributes('aria-label')).toBe('Редактировать карточку JKB-42')
  expect(link.element.tagName).toBe('A')
  expect(wrapper.text()).toContain('Alice')
  expect(wrapper.text()).toContain('Bob')
  const result = await axe.run(link.element, { rules: { 'color-contrast': { enabled: false } } })
  expect(result.violations).toEqual([])
  expect(wrapper.get('.card-title-link').attributes('href')).toMatch(/^\/cards\/card-1\?/)
  await link.trigger('click')
  await flushPromises()
  expect(router.currentRoute.value.path).toBe('/cards/card-1/edit')
  expect(router.currentRoute.value.query.edit_return).toBe('board')
  const back = router.resolve(String(router.currentRoute.value.query.return_to))
  expect(back.path).toBe('/')
  expect(back.query).toEqual({ q: 'new', assignee_id: 'me', tag_id: ['a', 'b'] })
  wrapper.unmount()
})
