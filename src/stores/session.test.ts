import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { authApi } from '../api'
import { useSessionStore } from './session'
vi.mock('../api', async () => {
  const actual = await vi.importActual<typeof import('../api')>('../api')
  return { ...actual, authApi: { me: vi.fn(), login: vi.fn(), logout: vi.fn() } }
})
const user = {
  id: 'u',
  username: 'ada',
  display_name: 'Ada',
  role: 'admin' as const,
  is_active: true,
  created_at: '',
  updated_at: '',
}
describe('session store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  it('restores the user once', async () => {
    vi.mocked(authApi.me).mockResolvedValue({ user, groups: [], csrf_token: 'x' })
    const store = useSessionStore()
    await store.restore()
    expect(store.user?.username).toBe('ada')
    expect(store.ready).toBe(true)
  })
  it('clears invalid sessions', async () => {
    vi.mocked(authApi.me).mockRejectedValue(new Error('401'))
    const store = useSessionStore()
    await store.restore()
    expect(store.user).toBeNull()
    expect(store.ready).toBe(true)
  })
  it('logs out even when the server rejects', async () => {
    vi.mocked(authApi.logout).mockRejectedValue(new Error('offline'))
    const store = useSessionStore()
    store.user = user
    await expect(store.logout()).rejects.toThrow()
    expect(store.user).toBeNull()
  })
})
