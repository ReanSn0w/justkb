import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { Card } from '../../api'
import CardInfoPanel from './CardInfoPanel.vue'

const timestamps = { created_at: '', updated_at: '' }
const card: Card = {
  ...timestamps,
  id: 'card-1',
  number: 7,
  title: 'Проверить сводку',
  description: '',
  column_id: 'column-1',
  position: '1',
  version: 1,
  is_archived: false,
  due_at: '2026-11-03T12:00:00Z',
  epic: {
    ...timestamps,
    id: 'project-1',
    name: 'Запуск',
    color: '#00d1b2',
    is_archived: false,
    card_count: 1,
    completed_card_count: 0,
  },
  group: { ...timestamps, id: 'group-1', name: 'Backend', color: '#00d1b2' },
  assignee: {
    ...timestamps,
    id: 'user-1',
    username: 'ada',
    display_name: 'Ada',
    role: 'member',
    is_active: true,
  },
  tags: [{ ...timestamps, id: 'tag-1', name: 'Важно', color: '#00d1b2' }],
}

describe('card information panel', () => {
  it('keeps task metadata together in the sidebar', () => {
    const wrapper = mount(CardInfoPanel, {
      props: { card, columnName: 'В работе' },
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })

    expect(wrapper.text()).toContain('Столбец')
    expect(wrapper.text()).toContain('В работе')
    expect(wrapper.text()).toContain('Запуск')
    expect(wrapper.text()).toContain('Команда')
    expect(wrapper.text()).not.toContain('Группа')
    expect(wrapper.text()).toContain('Backend')
    expect(wrapper.text()).toContain('Ada')
    expect(wrapper.text()).toContain('Важно')
    expect(wrapper.get('time').attributes('datetime')).toBe(card.due_at)
  })

  it('shows explicit empty values instead of hiding metadata', () => {
    const wrapper = mount(CardInfoPanel, {
      props: {
        card: { ...card, epic: null, group: null, assignee: null, due_at: null, tags: [] },
        columnName: 'Backlog',
      },
      global: { stubs: { RouterLink: true } },
    })

    expect(wrapper.text()).toContain('Не указан')
    expect(wrapper.text()).toContain('Не указана')
    expect(wrapper.text()).toContain('Не назначен')
    expect(wrapper.text()).toContain('Нет тегов')
  })
})
