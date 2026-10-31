import type { User } from '../api'
export function canAccessAdmin(user: Pick<User, 'role'> | null | undefined) {
  return user?.role === 'admin'
}
