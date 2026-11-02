import { createPinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { directoryApi, type BoardColumn, type Group, type Tag } from '../api'
import SettingsView from './SettingsView.vue'

const column: BoardColumn = {
  id: 'backlog',
  name: 'Backlog',
  position: 100,
  created_at: '',
  updated_at: '',
}
const group: Group = {
  id: 'backend',
  name: 'Backend',
  color: '#3366FF',
  description: 'API team',
  created_at: '',
  updated_at: '',
}
const tag: Tag = {
  id: 'bug',
  name: 'Bug',
  color: '#CC0000',
  created_at: '',
  updated_at: '',
}

describe('settings directories', () => {
  beforeEach(() => {
    vi.spyOn(directoryApi, 'users').mockResolvedValue({ items: [], next_cursor: null })
    vi.spyOn(directoryApi, 'groups').mockResolvedValue({ items: [group], next_cursor: null })
    vi.spyOn(directoryApi, 'columns').mockResolvedValue({ items: [column], next_cursor: null })
    vi.spyOn(directoryApi, 'tags').mockResolvedValue({ items: [tag], next_cursor: null })
    vi.spyOn(directoryApi, 'updateColumn').mockResolvedValue({ ...column, name: 'Очередь' })
    vi.spyOn(directoryApi, 'createGroup').mockResolvedValue(group)
    vi.spyOn(directoryApi, 'createTag').mockResolvedValue(tag)
    vi.spyOn(directoryApi, 'updateGroup').mockResolvedValue(group)
    vi.spyOn(directoryApi, 'updateTag').mockResolvedValue(tag)
  })

  afterEach(() => vi.restoreAllMocks())

  it('renames a column through the visible form', async () => {
    const wrapper = mount(SettingsView, { global: { plugins: [createPinia()] } })
    await flushPromises()

    const columnsTab = wrapper.findAll('.tabs a').find((item) => item.text() === 'Колонки')
    expect(columnsTab).toBeDefined()
    await columnsTab!.trigger('click')

    const renameButton = wrapper.findAll('button').find((item) => item.text() === 'Редактировать')
    expect(renameButton).toBeDefined()
    await renameButton!.trigger('click')

    const input = wrapper.get<HTMLInputElement>('.modal input')
    expect(input.element.value).toBe('Backlog')
    expect(wrapper.get('.modal-card-foot > .buttons').findAll('button')).toHaveLength(2)
    await input.setValue('Очередь')
    await wrapper.get('.modal form').trigger('submit')
    await flushPromises()

    expect(directoryApi.updateColumn).toHaveBeenCalledWith('backlog', {
      name: 'Очередь',
      wip_limit: undefined,
    })
    expect(wrapper.find('.modal').exists()).toBe(false)
  })

  it.each([
    ['Команды', 'createGroup'],
    ['Теги', 'createTag'],
  ] as const)('передаёт выбранный цвет при создании: %s', async (tabName, methodName) => {
    const wrapper = mount(SettingsView, { global: { plugins: [createPinia()] } })
    await flushPromises()

    await wrapper
      .findAll('.tabs a')
      .find((item) => item.text() === tabName)!
      .trigger('click')
    await wrapper.get('form.box input:not([type="color"])').setValue('Новая запись')
    await wrapper.get('form.box input[type="color"]').setValue('#123456')
    await wrapper.get('form.box').trigger('submit')
    await flushPromises()

    expect(directoryApi[methodName]).toHaveBeenCalledWith({
      name: 'Новая запись',
      color: '#123456',
    })
  })

  it.each([
    ['Команды', 'updateGroup', 'backend'],
    ['Теги', 'updateTag', 'bug'],
  ] as const)('изменяет цвет через форму редактирования: %s', async (tabName, methodName, id) => {
    const wrapper = mount(SettingsView, { global: { plugins: [createPinia()] } })
    await flushPromises()

    await wrapper
      .findAll('.tabs a')
      .find((item) => item.text() === tabName)!
      .trigger('click')
    await wrapper
      .findAll('button')
      .find((item) => item.text() === 'Редактировать')!
      .trigger('click')
    await wrapper.get('.modal input[type="color"]').setValue('#123456')
    await wrapper.get('.modal form').trigger('submit')
    await flushPromises()

    const updateMethod = vi.mocked(directoryApi[methodName])
    expect(updateMethod).toHaveBeenCalled()
    expect(updateMethod.mock.calls[0]?.[0]).toBe(id)
    expect(updateMethod.mock.calls[0]?.[1]).toMatchObject({ color: '#123456' })
  })
})
