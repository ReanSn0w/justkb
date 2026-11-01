import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import { tokensApi } from '../../api'
import TokenSettings from './TokenSettings.vue'

vi.mock('../../api', async () => {
  const actual = await vi.importActual<typeof import('../../api')>('../../api')
  return { ...actual, tokensApi: { list: vi.fn(), create: vi.fn(), revoke: vi.fn() } }
})

describe('token settings', () => {
  it('shows a newly issued secret only until dialog closes', async () => {
    vi.mocked(tokensApi.list).mockResolvedValue({ items: [], next_cursor: null })
    vi.mocked(tokensApi.create).mockResolvedValue({
      id: 't',
      name: 'Agent',
      token_prefix: 'jkb',
      token: 'jkb_pat_secret',
      scopes: ['cards:read'],
      created_at: '',
      updated_at: '',
    })
    const wrapper = mount(TokenSettings, {
      global: {
        plugins: [createPinia()],
        stubs: {
          BaseDialog: {
            props: ['open'],
            template: '<div v-if="open"><slot/><slot name="actions"/></div>',
          },
        },
      },
    })
    await flushPromises()
    await wrapper.find('input[placeholder="Codex"]').setValue('Agent')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('jkb_pat_secret')
    await wrapper
      .findAll('button')
      .find((button) => button.text() === 'Готово')!
      .trigger('click')
    expect(wrapper.text()).not.toContain('jkb_pat_secret')
  })
})
