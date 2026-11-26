import { mount, flushPromises } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import axe from 'axe-core'
import { ApiError, botsApi, directoryApi, type User, type IssuedToken } from '../../api'
import BotSettings from './BotSettings.vue'

const bot: User = {
  id: 'bot-1',
  username: 'bot-uuid',
  display_name: 'Agent One',
  account_type: 'bot',
  role: 'member',
  is_active: true,
  created_at: '',
  updated_at: '',
}
const issued: IssuedToken = {
  id: 'token-1',
  name: 'MCP',
  token: 'synthetic-test-token',
  token_prefix: 'synthetic',
  scopes: ['cards:read', 'cards:write'],
  created_at: '',
  updated_at: '',
}
beforeEach(() => {
  vi.spyOn(directoryApi, 'users').mockResolvedValue({ items: [bot], next_cursor: null })
  vi.spyOn(directoryApi, 'updateUser').mockResolvedValue(bot)
  vi.spyOn(botsApi, 'create').mockResolvedValue({ user_id: bot.id, token: issued })
  vi.spyOn(botsApi, 'token').mockResolvedValue(issued)
  vi.spyOn(botsApi, 'rotate').mockResolvedValue({
    ...issued,
    id: 'token-2',
    token: 'synthetic-rotated-token',
  })
  vi.spyOn(botsApi, 'uploadAvatar').mockResolvedValue(undefined)
  vi.spyOn(botsApi, 'deleteAvatar').mockResolvedValue(undefined)
})
afterEach(() => {
  vi.restoreAllMocks()
  document.body.replaceChildren()
})
async function setup() {
  const wrapper = mount(BotSettings, {
    attachTo: document.body,
    global: { plugins: [createPinia()] },
  })
  await flushPromises()
  return wrapper
}
describe('bot management', () => {
  it('creates without a password and clears the one-time secret on close', async () => {
    const w = await setup()
    await w.get('#bot-name').setValue('New Agent')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(botsApi.create).toHaveBeenCalledWith('New Agent')
    expect(w.get<HTMLTextAreaElement>('#bot-secret').element.value).toBe(issued.token)
    await w
      .findAll('button')
      .find((x) => x.text() === 'Готово')!
      .trigger('click')
    expect(w.find('#bot-secret').exists()).toBe(false)
    expect(w.find('input[type="password"]').exists()).toBe(false)
    w.unmount()
  })
  it('requires an explicit rotation and does not retry a conflict', async () => {
    const w = await setup()
    vi.mocked(botsApi.rotate).mockRejectedValueOnce(
      new ApiError(409, 'bot_token_conflict', 'changed'),
    )
    await w
      .findAll('button')
      .find((x) => x.text() === 'Перевыпустить токен')!
      .trigger('click')
    await flushPromises()
    expect(botsApi.rotate).not.toHaveBeenCalled()
    await w
      .findAll('button')
      .find((x) => x.text() === 'Перевыпустить')!
      .trigger('click')
    await flushPromises()
    expect(botsApi.rotate).toHaveBeenCalledExactlyOnceWith(bot.id, issued.id)
    expect(w.text()).toContain('Токен уже изменился')
    expect(w.find('#bot-secret').exists()).toBe(false)
    w.unmount()
  })
  it('updates bot names and supports keyboard-accessible dialogs', async () => {
    const w = await setup()
    await w
      .findAll('button')
      .find((x) => x.text() === 'Имя и аватар')!
      .trigger('click')
    expect(
      (await axe.run(w.get('.modal').element, { rules: { 'color-contrast': { enabled: false } } }))
        .violations,
    ).toEqual([])
    await w.get('#bot-edit-name').setValue('Renamed')
    await w.get('#bot-edit').trigger('submit')
    await flushPromises()
    expect(directoryApi.updateUser).toHaveBeenCalledWith(
      bot.id,
      expect.objectContaining({ display_name: 'Renamed', role: 'member' }),
    )
    w.unmount()
  })
  it('uploads an avatar and refreshes the displayed user', async () => {
    const w = await setup()
    await w
      .findAll('button')
      .find((x) => x.text() === 'Имя и аватар')!
      .trigger('click')
    const file = new File(['test'], 'avatar.png', { type: 'image/png' })
    Object.defineProperty(w.get('#bot-avatar').element, 'files', { value: [file] })
    await w.get('#bot-avatar').trigger('change')
    await w
      .findAll('button')
      .find((x) => x.text() === 'Загрузить аватар')!
      .trigger('click')
    await flushPromises()
    expect(botsApi.uploadAvatar).toHaveBeenCalledWith(bot.id, file)
    expect(directoryApi.users).toHaveBeenCalledTimes(2)
    w.unmount()
  })
  it('reactivates disabled bots from the inclusive directory', async () => {
    vi.mocked(directoryApi.users).mockResolvedValue({
      items: [{ ...bot, is_active: false }],
      next_cursor: null,
    })
    const w = await setup()
    expect(directoryApi.users).toHaveBeenCalledWith(true)
    await w
      .findAll('button')
      .find((x) => x.text() === 'Включить')!
      .trigger('click')
    await flushPromises()
    expect(directoryApi.updateUser).toHaveBeenCalledWith(
      bot.id,
      expect.objectContaining({ is_active: true }),
    )
    w.unmount()
  })
})
