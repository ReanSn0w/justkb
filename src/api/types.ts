export type UUID = string
export type Role = 'admin' | 'member'
export interface Timestamps {
  created_at: string
  updated_at: string
}
export interface User extends Timestamps {
  id: UUID
  username: string
  display_name: string
  email?: string | null
  avatar_url?: string | null
  role: Role
  is_active: boolean
  group_ids?: UUID[]
}
export interface Group extends Timestamps {
  id: UUID
  name: string
  color: string
  description?: string | null
}
export interface BoardColumn extends Timestamps {
  id: UUID
  name: string
  position: number
  wip_limit?: number | null
}
export interface Epic extends Timestamps {
  restorable_cards: number
  version: number
  id: UUID
  name: string
  color: string
  description?: string | null
  is_archived: boolean
  card_count: number
  completed_card_count: number
  attachments?: Attachment[]
}
export interface Tag extends Timestamps {
  id: UUID
  name: string
  color: string
}
export interface Attachment {
  id: UUID
  card_id?: UUID
  epic_id?: UUID
  original_name: string
  media_type: string
  size: number
  created_by: UUID
  created_at: string
  author?: Pick<User, 'id' | 'display_name'>
}
export interface Activity {
  id: UUID
  card_id: UUID
  actor_user_id: UUID | null
  actor_token_id?: UUID | null
  action: string
  changes: Record<string, unknown>
  created_at: string
  actor?: Pick<User, 'id' | 'display_name'>
  token_name?: string | null
}
export interface Comment {
  id: UUID
  card_id?: UUID
  epic_id?: UUID
  author_user_id: UUID
  author_token_id?: UUID | null
  body: string
  created_at: string
  updated_at: string
  author: Pick<User, 'id' | 'display_name' | 'avatar_url'>
}
export type CardAssignee = Pick<User, 'id' | 'display_name' | 'avatar_url'>

export interface Card extends Timestamps {
  id: UUID
  number: number
  title: string
  description: string
  column_id: UUID
  position: string
  version: number
  is_archived: boolean
  epic_id?: UUID | null
  group_id?: UUID | null
  assignee_id?: UUID | null
  assignee_ids?: UUID[]
  due_at?: string | null
  tags: Tag[]
  attachments?: Attachment[]
  epic?: Epic | null
  group?: Group | null
  assignee?: CardAssignee | null
  assignees?: CardAssignee[]
}
export interface ListResult<T> {
  items: T[]
  next_cursor: string | null
}
export interface ErrorEnvelope {
  error: { code: string; message: string; details?: Record<string, unknown>; request_id?: string }
}
export interface CardFilters {
  epic_id?: UUID | 'none'
  group_id?: UUID | 'none'
  assignee_id?: UUID | 'me' | 'none'
  tag_ids?: UUID[]
  column_id?: UUID
  q?: string
  archived?: boolean
  limit?: number
  cursor?: string
}
export interface CardInput {
  title: string
  description?: string
  column_id: UUID
  epic_id?: UUID | null
  group_id?: UUID | null
  assignee_id?: UUID | null
  assignee_ids?: UUID[]
  tag_ids?: UUID[]
  due_at?: string | null
}
export interface CardPatch {
  title?: string
  description?: string
  epic_id?: UUID | null
  group_id?: UUID | null
  assignee_id?: UUID | null
  assignee_ids?: UUID[]
  tag_ids?: UUID[]
  due_at?: string | null
}
export interface MoveCardInput {
  expected_version: number
  target_column_id: UUID
  before_card_id?: UUID
  after_card_id?: UUID
}
export interface ApiToken extends Timestamps {
  id: UUID
  name: string
  token_prefix: string
  scopes: ('cards:read' | 'cards:write')[]
  expires_at?: string | null
  last_used_at?: string | null
  revoked_at?: string | null
}
export interface IssuedToken extends ApiToken {
  token: string
}

export interface ArchivePolicy {
  version: number
  done_column_id: UUID | null
  cards_enabled: boolean
  cards_days: number
  projects_enabled: boolean
  projects_days: number
}
export interface ArchivePreview {
  cards: number
  projects: number
}
export interface ArchiveEvent {
  id: string
  entity_type: 'card' | 'project'
  entity_id: UUID
  project_id: UUID | null
  action: 'archived' | 'restored'
  occurred_at: string | null
  reason: 'manual' | 'automatic' | 'legacy'
  actor_type: 'user' | 'system' | 'unknown'
  actor_name: string | null
  batch_id: string | null
  title: string
  number: number | null
  rule_days: number | null
  is_archived: boolean
  version: number
  child_count: number
  restorable_cards: number
}
export interface ArchiveFilters {
  type?: 'card' | 'project' | ''
  project_id?: UUID
  q?: string
  from?: string
  to?: string
  state?: 'archived' | 'restored' | 'all'
  history?: boolean
  cursor?: string
  limit?: number
  batch_id?: string
}
