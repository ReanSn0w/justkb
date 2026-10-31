import { describe, expect, it } from 'vitest'
import { filtersFromQuery, queryFromFilters } from './filters'
describe('board filter URL', () => {
  it('round trips combined filters', () => {
    const filters = filtersFromQuery({
      epic_id: 'none',
      group_id: 'g',
      assignee_id: 'me',
      tag_id: ['a', 'b'],
      q: 'auth',
    })
    expect(filters).toMatchObject({
      epic_id: 'none',
      group_id: 'g',
      assignee_id: 'me',
      tag_ids: ['a', 'b'],
      q: 'auth',
    })
    expect(queryFromFilters(filters)).toEqual({
      epic_id: 'none',
      group_id: 'g',
      assignee_id: 'me',
      tag_id: ['a', 'b'],
      q: 'auth',
    })
  })
  it('omits empty values', () =>
    expect(queryFromFilters({ tag_ids: [], archived: false })).toEqual({}))
})
