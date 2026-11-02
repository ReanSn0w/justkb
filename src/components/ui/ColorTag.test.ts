import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ColorTag from './ColorTag.vue'

describe('ColorTag', () => {
  it('renders the entity color with readable text', () => {
    const wrapper = mount(ColorTag, {
      props: { color: '#1565C0' },
      slots: { default: 'Backend' },
    })

    expect(wrapper.text()).toBe('Backend')
    expect(wrapper.attributes('style')).toContain('background-color: rgb(21, 101, 192)')
    expect(wrapper.attributes('style')).toContain('color: rgb(255, 255, 255)')
  })

  it('supports an accessible selected button state', () => {
    const wrapper = mount(ColorTag, {
      props: { as: 'button', color: '#FFD54F', selected: true },
      attrs: { 'aria-pressed': 'true' },
    })

    expect(wrapper.element.tagName).toBe('BUTTON')
    expect(wrapper.attributes('type')).toBe('button')
    expect(wrapper.attributes('aria-pressed')).toBe('true')
    expect(wrapper.classes()).toContain('color-tag--selected')
  })
})
