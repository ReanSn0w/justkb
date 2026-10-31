import { describe, expect, it } from 'vitest'
import { canAccessAdmin } from './authz'
describe('admin guard', () => {
  it('allows only admins', () => {
    expect(canAccessAdmin({ role: 'admin' })).toBe(true)
    expect(canAccessAdmin({ role: 'member' })).toBe(false)
    expect(canAccessAdmin(null)).toBe(false)
  })
})
