import { json, request, upload } from './http'
import type {
  Activity,
  ApiToken,
  Attachment,
  BoardColumn,
  Card,
  CardFilters,
  CardInput,
  CardPatch,
  Comment,
  Epic,
  Group,
  IssuedToken,
  ListResult,
  MoveCardInput,
  Tag,
  User,
  UUID,
} from './types'

function query(filters: CardFilters = {}) {
  const out = new URLSearchParams()
  for (const [key, value] of Object.entries(filters)) {
    if (value === undefined || value === '' || value === false) continue
    if (key === 'tag_ids') for (const tag of value as string[]) out.append('tag_id', tag)
    else out.set(key, String(value))
  }
  const encoded = out.toString()
  return encoded ? `?${encoded}` : ''
}

export const authApi = {
  login: (username: string, password: string) =>
    request<{ user: User; csrf_token: string }>(
      '/auth/login',
      json('POST', { username, password }),
    ),
  logout: () => request<void>('/auth/logout', json('POST')),
  me: () => request<{ user: User; groups: Group[]; csrf_token: string }>('/auth/me'),
  password: (current_password: string, new_password: string) =>
    request<void>('/auth/password', json('PUT', { current_password, new_password })),
}
export const directoryApi = {
  users: () => request<ListResult<User>>('/users?limit=100'),
  createUser: (data: unknown) => request<User>('/users', json('POST', data)),
  updateUser: (id: UUID, data: unknown) => request<User>(`/users/${id}`, json('PATCH', data)),
  setUserGroups: (id: UUID, group_ids: UUID[]) =>
    request<User>(`/users/${id}/groups`, json('PUT', { group_ids })),
  setUserPassword: (id: UUID, password: string) =>
    request<void>(`/users/${id}/password`, json('PUT', { password })),
  groups: () => request<ListResult<Group>>('/groups?limit=100'),
  createGroup: (data: unknown) => request<Group>('/groups', json('POST', data)),
  updateGroup: (id: UUID, data: unknown) => request<Group>(`/groups/${id}`, json('PATCH', data)),
  columns: () => request<ListResult<BoardColumn>>('/columns?limit=100'),
  createColumn: (data: unknown) => request<BoardColumn>('/columns', json('POST', data)),
  updateColumn: (id: UUID, data: unknown) =>
    request<BoardColumn>(`/columns/${id}`, json('PATCH', data)),
  reorderColumns: (column_ids: UUID[]) =>
    request<BoardColumn[]>('/columns/reorder', json('POST', { column_ids })),
  deleteColumn: (id: UUID) => request<void>(`/columns/${id}`, { method: 'DELETE' }),
  tags: () => request<ListResult<Tag>>('/tags?limit=100'),
  createTag: (data: unknown) => request<Tag>('/tags', json('POST', data)),
  updateTag: (id: UUID, data: unknown) => request<Tag>(`/tags/${id}`, json('PATCH', data)),
}
export const epicsApi = {
  list: (include_archived = false) =>
    request<ListResult<Epic>>(`/epics?limit=100&include_archived=${include_archived}`),
  get: (id: UUID) => request<Epic>(`/epics/${id}`),
  create: (data: unknown) => request<Epic>('/epics', json('POST', data)),
  update: (id: UUID, data: unknown) => request<Epic>(`/epics/${id}`, json('PATCH', data)),
  upload: (id: UUID, file: File, progress?: (loaded: number, total: number) => void) =>
    upload<Attachment>(`/epics/${id}/attachments`, file, progress),
  deleteAttachment: (id: UUID) => request<void>(`/attachments/${id}`, { method: 'DELETE' }),
  attachmentUrl: (id: UUID) => `/api/v1/attachments/${encodeURIComponent(id)}`,
}
export const cardsApi = {
  list: (filters?: CardFilters) => request<ListResult<Card>>(`/cards${query(filters)}`),
  get: (id: UUID) => request<Card>(`/cards/${id}`),
  create: (data: CardInput) => request<Card>('/cards', json('POST', data)),
  update: (id: UUID, expected_version: number, patch: CardPatch) =>
    request<Card>(`/cards/${id}`, json('PATCH', { expected_version, ...patch })),
  move: (id: UUID, data: MoveCardInput) => request<Card>(`/cards/${id}/move`, json('POST', data)),
  archive: (id: UUID, expected_version: number) =>
    request<Card>(`/cards/${id}/archive`, json('POST', { expected_version })),
  restore: (id: UUID, expected_version: number) =>
    request<Card>(`/cards/${id}/restore`, json('POST', { expected_version })),
  activity: (id: UUID, cursor?: string) =>
    request<ListResult<Activity>>(
      `/cards/${id}/activity${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`,
    ),
  upload: (id: UUID, file: File, progress?: (loaded: number, total: number) => void) =>
    upload<Attachment>(`/cards/${id}/attachments`, file, progress),
  deleteAttachment: (id: UUID) => request<void>(`/attachments/${id}`, { method: 'DELETE' }),
  attachmentUrl: (id: UUID) => `/api/v1/attachments/${encodeURIComponent(id)}`,
}
export const commentsApi = {
  list: (type: 'card' | 'project', id: UUID) =>
    request<ListResult<Comment>>(`/${type === 'card' ? 'cards' : 'epics'}/${id}/comments`),
  create: (type: 'card' | 'project', id: UUID, body: string) =>
    request<Comment>(
      `/${type === 'card' ? 'cards' : 'epics'}/${id}/comments`,
      json('POST', { body }),
    ),
}
export const tokensApi = {
  list: () => request<ListResult<ApiToken>>('/tokens'),
  create: (data: { name: string; scopes: string[]; expires_at?: string | null }) =>
    request<IssuedToken>('/tokens', json('POST', data)),
  revoke: (id: UUID) => request<void>(`/tokens/${id}`, { method: 'DELETE' }),
}
export { ApiError, setCsrfToken } from './http'
export type * from './types'
