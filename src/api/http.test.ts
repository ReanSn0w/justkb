import { beforeEach, describe, expect, it, vi } from 'vitest'
import { request, setCsrfToken } from './http'
import { cardsApi, epicsApi } from '.'

describe('HTTP API client', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    setCsrfToken('csrf')
  })
  it('sends cookies and CSRF for mutations', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      }),
    )
    await request('/test', { method: 'POST', body: JSON.stringify({}) })
    const [, init] = fetchMock.mock.calls[0]!
    expect(init?.credentials).toBe('include')
    expect(new Headers(init?.headers).get('x-csrf-token')).toBe('csrf')
  })
  it('maps the stable error envelope', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(
        JSON.stringify({
          error: {
            code: 'card_version_conflict',
            message: 'Changed',
            details: { version: 2 },
            request_id: 'r1',
          },
        }),
        { status: 409 },
      ),
    )
    await expect(request('/test')).rejects.toMatchObject({
      status: 409,
      code: 'card_version_conflict',
      requestId: 'r1',
    })
  })
  it('serializes repeated tag filters with AND semantics', async () => {
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(
        new Response(JSON.stringify({ items: [], next_cursor: null }), { status: 200 }),
      )
    await cardsApi.list({ epic_id: 'none', tag_ids: ['a', 'b'], archived: false })
    expect(fetchMock.mock.calls[0]![0]).toBe('/api/v1/cards?epic_id=none&tag_id=a&tag_id=b')
  })
  it('loads a complete project resource', async () => {
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(new Response(JSON.stringify({ id: 'e1' }), { status: 200 }))
    await epicsApi.get('e1')
    expect(fetchMock.mock.calls[0]![0]).toBe('/api/v1/epics/e1')
  })
})
