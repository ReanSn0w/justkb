import { flushPromises, mount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import axe from 'axe-core'
import { archiveApi, type Epic } from '../../api'
import RestoreProjectDialog from './RestoreProjectDialog.vue'
vi.mock('../../api', async (original) => ({
  ...(await original<typeof import('../../api')>()),
  archiveApi: { restoreProject: vi.fn() },
}))
const project: Epic = {
  id: 'p',
  name: 'Project',
  color: '#112233',
  is_archived: true,
  version: 4,
  card_count: 5,
  completed_card_count: 5,
  restorable_cards: 2,
  created_at: '',
  updated_at: '',
}
it('offers both modes with a current count and restores the selected mode', async () => {
  const wrapper = mount(RestoreProjectDialog, { props: { project }, attachTo: document.body })
  await flushPromises()
  expect(wrapper.text()).toContain('вместе с ним (2)')
  expect(
    (await axe.run(wrapper.element, { rules: { 'color-contrast': { enabled: false } } }))
      .violations,
  ).toEqual([])
  await wrapper.get('input[value="project_only"]').setValue()
  vi.mocked(archiveApi.restoreProject).mockResolvedValue({
    ...project,
    is_archived: false,
    version: 5,
  })
  await wrapper
    .findAll('button')
    .find((b) => b.text() === 'Восстановить')!
    .trigger('click')
  await flushPromises()
  expect(archiveApi.restoreProject).toHaveBeenCalledWith('p', 4, 'project_only')
  expect(wrapper.emitted('restored')).toHaveLength(1)
  wrapper.unmount()
})
