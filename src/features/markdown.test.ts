import { describe, expect, it } from 'vitest'
import { renderMarkdown } from './markdown'
describe('markdown preview', () => {
  it('renders markdown', () =>
    expect(renderMarkdown('**safe**')).toContain('<strong>safe</strong>'))
  it('removes scripts and event handlers', () => {
    const html = renderMarkdown('<img src=x onerror=alert(1)><script>alert(1)</script>')
    expect(html).not.toContain('onerror')
    expect(html).not.toContain('<script')
  })
})
