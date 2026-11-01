import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import BaseButton from './BaseButton.vue'
import StatusState from './StatusState.vue'

describe('system theme colors', () => {
  it('leaves neutral controls to the active Bulma color scheme', () => {
    const button = mount(BaseButton, { props: { tone: 'quiet' } })
    const status = mount(StatusState, { props: { title: 'Пусто' } })

    expect(button.get('button').classes()).not.toContain('is-light')
    expect(button.get('button').classes()).not.toContain('is-white')
    expect(status.get('.notification').classes()).not.toContain('is-light')
  })
})
