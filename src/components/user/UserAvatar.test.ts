import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import UserAvatar from './UserAvatar.vue'

describe('UserAvatar', () => {
  it('shows an uploaded avatar', () => {
    const wrapper = mount(UserAvatar, {
      props: { user: { display_name: 'Ada Lovelace', avatar_url: '/avatar' } },
    })
    expect(wrapper.get('img').attributes('src')).toBe('/avatar')
    expect(wrapper.get('img').attributes('alt')).toContain('Ada Lovelace')
  })

  it('uses initials when no avatar exists', () => {
    const wrapper = mount(UserAvatar, {
      props: { user: { display_name: 'Ada Lovelace', avatar_url: null } },
    })
    expect(wrapper.get('img').attributes('src')).toContain(encodeURIComponent('AL'))
  })
})
