import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import BaseButton from './BaseButton.vue'
import StatusState from './StatusState.vue'
import '../../theme.css'

describe('system theme colors', () => {
  it('leaves neutral controls to the active Bulma color scheme', () => {
    const button = mount(BaseButton, { props: { tone: 'quiet' } })
    const status = mount(StatusState, { props: { title: 'Пусто' } })

    expect(button.get('button').classes()).not.toContain('is-light')
    expect(button.get('button').classes()).not.toContain('is-white')
    expect(status.get('.notification').classes()).not.toContain('is-light')
  })

  it('uses the Bulma primary palette for link accents', () => {
    const rootStyle = getComputedStyle(document.documentElement)

    expect(rootStyle.getPropertyValue('--bulma-link-h').trim()).toBe('var(--bulma-primary-h)')
    expect(rootStyle.getPropertyValue('--bulma-link-s').trim()).toBe('var(--bulma-primary-s)')
    expect(rootStyle.getPropertyValue('--bulma-link-l').trim()).toBe(
      'var(--bulma-primary-on-scheme-l)',
    )
    expect(rootStyle.getPropertyValue('--bulma-link-on-scheme-l').trim()).toBe(
      'var(--bulma-primary-on-scheme-l)',
    )
  })
})
