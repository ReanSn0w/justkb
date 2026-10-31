import type { LocationQuery, LocationQueryRaw } from 'vue-router'
import type { CardFilters } from '../api'
export function filtersFromQuery(query: LocationQuery): CardFilters {
  const one = (value: unknown) => (Array.isArray(value) ? value[0] : value)
  const tags = query.tag_id
    ? (Array.isArray(query.tag_id) ? query.tag_id : [query.tag_id]).filter(
        (x): x is string => typeof x === 'string',
      )
    : undefined
  return {
    epic_id: one(query.epic_id) as CardFilters['epic_id'],
    group_id: one(query.group_id) as CardFilters['group_id'],
    assignee_id: one(query.assignee_id) as CardFilters['assignee_id'],
    tag_ids: tags,
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
