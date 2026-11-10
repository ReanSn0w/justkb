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

import { boardReturnURL, mergeFilterQuery } from './filters'
it('normalizes tags and preserves unrelated query on reset', () => {
  expect(filtersFromQuery({ tag_id: [' a ', 'a', '', null], q: null }).tag_ids).toEqual(['a'])
  expect(mergeFilterQuery({ q: 'old', tab: 'keep' }, {})).toEqual({ tab: 'keep' })
})
it('only accepts local board return URLs', () => {
  for (const value of ['//evil.test', '/cards/1', 'https://evil.test', '/\\evil', ['/?q=x']])
    expect(boardReturnURL(value)).toBe('/')
  expect(boardReturnURL('/?q=hello&tag_id=a')).toBe('/?q=hello&tag_id=a')
})
it('accepts archive return filters but rejects archive-like external paths', () => {
  expect(boardReturnURL('/archive?q=hello&history=true')).toBe('/archive?q=hello&history=true')
  for (const value of [
    '/archive//evil',
    '/archive.evil',
    '/archive/../login',
    '//archive',
    '/archive\\evil',
  ])
    expect(boardReturnURL(value)).toBe('/')
})
