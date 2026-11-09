import type { LocationQuery, LocationQueryRaw } from 'vue-router'
import type { CardFilters } from '../api'
export function filtersFromQuery(query: LocationQuery): CardFilters {
  const one = (value: unknown): string | undefined => {
    const item = Array.isArray(value) ? value[0] : value
    return typeof item === 'string' && item.trim() ? item.trim() : undefined
  }
  const tags = query.tag_id
    ? (Array.isArray(query.tag_id) ? query.tag_id : [query.tag_id])
        .filter((x): x is string => typeof x === 'string' && Boolean(x.trim()))
        .map((x) => x.trim())
    : undefined
  return {
    epic_id: one(query.epic_id) as CardFilters['epic_id'],
    group_id: one(query.group_id) as CardFilters['group_id'],
    assignee_id: one(query.assignee_id) as CardFilters['assignee_id'],
    tag_ids: tags?.length ? [...new Set(tags)] : undefined,
    column_id: one(query.column_id) as string | undefined,
    q: one(query.q) as string | undefined,
    archived: one(query.archived) === 'true',
  }
}
export function queryFromFilters(filters: CardFilters): LocationQueryRaw {
  const query: LocationQueryRaw = {}
  if (filters.epic_id) query.epic_id = filters.epic_id
  if (filters.group_id) query.group_id = filters.group_id
  if (filters.assignee_id) query.assignee_id = filters.assignee_id
  if (filters.tag_ids?.length) query.tag_id = filters.tag_ids
  if (filters.column_id) query.column_id = filters.column_id
  if (filters.q) query.q = filters.q
  if (filters.archived) query.archived = 'true'
  return query
}

const filterKeys = ['epic_id', 'group_id', 'assignee_id', 'tag_id', 'column_id', 'q', 'archived']
export function mergeFilterQuery(query: LocationQuery, filters: CardFilters): LocationQueryRaw {
  const rest = { ...query }
  for (const key of filterKeys) delete rest[key]
  return { ...rest, ...queryFromFilters(filters) }
}
export function boardReturnURL(value: unknown): string {
  if (typeof value !== 'string' || !/^\/(?:[?#]|$)/.test(value) || /[\\\r\n]/.test(value))
    return '/'
  return value
}
