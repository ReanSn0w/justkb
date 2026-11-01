import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import AttachmentPanel from './AttachmentPanel.vue'
vi.mock('../../api', async () => {
  const a = await vi.importActual<typeof import('../../api')>('../../api')
  return { ...a, cardsApi: { ...a.cardsApi, upload: vi.fn(), deleteAttachment: vi.fn() } }
})
describe('AttachmentPanel', () => {
  it('rejects an obviously oversized file before upload', async () => {
    const wrapper = mount(AttachmentPanel, {
      global: { plugins: [createPinia()] },
      props: { entityId: 'c', entityType: 'card', attachments: [] },
    })
    const file = new File(['x'], 'big.bin')
    Object.defineProperty(file, 'size', { value: 30 * 1024 * 1024 })
    Object.defineProperty(wrapper.find('input').element as HTMLInputElement, 'files', {
      value: [file],
    })
    await wrapper.find('input').trigger('change')
    expect(wrapper.text()).toContain('Файл больше')
  })
})
