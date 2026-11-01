import { createPinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { directoryApi, type BoardColumn } from '../api'
import SettingsView from './SettingsView.vue'

const column: BoardColumn = {
  id: 'backlog',
  name: 'Backlog',
  position: 100,
  created_at: '',
  updated_at: '',
}

describe('settings directories', () => {
  beforeEach(() => {
    vi.spyOn(directoryApi, 'users').mockResolvedValue({ items: [], next_cursor: null })
    vi.spyOn(directoryApi, 'groups').mockResolvedValue({ items: [], next_cursor: null })
    vi.spyOn(directoryApi, 'columns').mockResolvedValue({ items: [column], next_cursor: null })
    vi.spyOn(directoryApi, 'tags').mockResolvedValue({ items: [], next_cursor: null })
    vi.spyOn(directoryApi, 'updateColumn').mockResolvedValue({ ...column, name: 'Очередь' })
  })

  afterEach(() => vi.restoreAllMocks())

  it('renames a column through the visible form', async () => {
    const wrapper = mount(SettingsView, { global: { plugins: [createPinia()] } })
    await flushPromises()

    const columnsTab = wrapper.findAll('.tabs a').find((item) => item.text() === 'Колонки')
    expect(columnsTab).toBeDefined()
    await columnsTab!.trigger('click')

    const renameButton = wrapper.findAll('button').find((item) => item.text() === 'Переименовать')
    expect(renameButton).toBeDefined()
    await renameButton!.trigger('click')

    const input = wrapper.get<HTMLInputElement>('.modal input')
    expect(input.element.value).toBe('Backlog')
    await input.setValue('Очередь')
    await wrapper.get('.modal form').trigger('submit')
    await flushPromises()

    expect(directoryApi.updateColumn).toHaveBeenCalledWith('backlog', {
      name: 'Очередь',
      wip_limit: undefined,
    })
    expect(wrapper.find('.modal').exists()).toBe(false)
  })
})
