import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import axe from 'axe-core'
import { archiveApi, directoryApi, ApiError } from '../../api'
import ArchiveSettings from './ArchiveSettings.vue'
vi.mock('../../api', async (original) => ({
  ...(await original<typeof import('../../api')>()),
  archiveApi: { policy: vi.fn(), preview: vi.fn(), savePolicy: vi.fn() },
  directoryApi: { columns: vi.fn() },
}))
beforeEach(() => {
  vi.mocked(archiveApi.policy).mockResolvedValue({
    version: 1,
    done_column_id: 'done',
    cards_enabled: false,
    cards_days: 3,
    projects_enabled: false,
    projects_days: 3,
  })
  vi.mocked(directoryApi.columns).mockResolvedValue({
    items: [{ id: 'done', name: 'Готово', position: 100, created_at: '', updated_at: '' }],
    next_cursor: null,
  })
  vi.mocked(archiveApi.preview).mockResolvedValue({ cards: 4, projects: 1 })
})
it('requires preview and preserves draft on conflict', async () => {
  const wrapper = mount(ArchiveSettings)
  await flushPromises()
  expect(wrapper.get('button').text()).toBe('Проверить правила')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(wrapper.text()).toContain('карточек — 4')
  vi.mocked(archiveApi.savePolicy).mockRejectedValueOnce(
    new ApiError(409, 'archive_version_conflict', 'Conflict'),
  )
  await wrapper
    .findAll('button')
    .find((b) => b.text() === 'Сохранить правила')!
    .trigger('click')
  await flushPromises()
  expect(wrapper.text()).toContain('другим администратором')
  expect((wrapper.get('#archive-card-days').element as HTMLInputElement).value).toBe('3')
  await wrapper.get('#archive-card-days').setValue('7')
  await flushPromises()
  expect(wrapper.text()).not.toContain('карточек — 4')
})
it('has accessible controls', async () => {
  const wrapper = mount(ArchiveSettings, { attachTo: document.body })
  await flushPromises()
  expect(
    (await axe.run(wrapper.element, { rules: { 'color-contrast': { enabled: false } } }))
      .violations,
  ).toEqual([])
  wrapper.unmount()
})
