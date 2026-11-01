import { describe, expect, it, vi } from 'vitest'
import { systemApi } from './api'
import { loadSystemName, systemName } from './system'

vi.mock('./api', () => ({
  systemApi: { get: vi.fn() },
}))

describe('system branding', () => {
  it('loads the runtime name and updates the document title', async () => {
    vi.mocked(systemApi.get).mockResolvedValue({ name: 'GostAssistent' })

    await loadSystemName()

    expect(systemName.value).toBe('GostAssistent')
    expect(document.title).toBe('GostAssistent')
  })
})
