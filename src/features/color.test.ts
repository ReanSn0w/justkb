import { describe, expect, it } from 'vitest'
import { contrastRatio, contrastTextColor } from './color'

describe('color contrast', () => {
  it.each([
    ['#FFD54F', '#000000'],
    ['#00D1B2', '#000000'],
    ['#1565C0', '#FFFFFF'],
    ['#111827', '#FFFFFF'],
  ])('chooses readable text for %s', (background, expected) => {
    const foreground = contrastTextColor(background)
    expect(foreground).toBe(expected)
    expect(contrastRatio(background, foreground)).toBeGreaterThanOrEqual(4.5)
  })
})
