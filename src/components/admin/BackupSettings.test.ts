import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { adminApi } from '../../api'
import BackupSettings from './BackupSettings.vue'

vi.mock('../../api', async () => {
  const actual = await vi.importActual<typeof import('../../api')>('../../api')
  return {
    ...actual,
    adminApi: { createBackup: vi.fn(), restoreBackup: vi.fn() },
  }
})

describe('backup settings', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('describes the SQLite backup contents', () => {
    const wrapper = mount(BackupSettings, { global: { plugins: [createPinia()] } })

    expect(wrapper.text()).toContain('snapshot SQLite')
    expect(wrapper.text()).not.toContain('PostgreSQL')
  })

  it('requires explicit confirmation before restoring', async () => {
    vi.stubGlobal(
      'confirm',
      vi.fn(() => true),
    )
    vi.mocked(adminApi.restoreBackup).mockResolvedValue({ restored: true })
    const wrapper = mount(BackupSettings, { global: { plugins: [createPinia()] } })
    const input = wrapper.get('input[type="file"]')
    const backup = new File(['backup'], 'jkb-backup.tar.gz', { type: 'application/gzip' })
    Object.defineProperty(input.element, 'files', { value: [backup] })
    await input.trigger('change')
    const restoreButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('Восстановить'))!
    expect(restoreButton.attributes('disabled')).toBeDefined()
    await wrapper.get('input[autocomplete="off"]').setValue('RESTORE')
    expect(restoreButton.attributes('disabled')).toBeUndefined()
    await restoreButton.trigger('click')
    await flushPromises()
    expect(adminApi.restoreBackup).toHaveBeenCalledWith(backup, expect.any(Function))
    expect(wrapper.text()).toContain('Система восстановлена')
  })
})
