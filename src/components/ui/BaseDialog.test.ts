import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import BaseDialog from './BaseDialog.vue'

describe('BaseDialog', () => {
  it('groups action buttons with consistent spacing', () => {
    const wrapper = mount(BaseDialog, {
      props: { open: true, title: 'Диалог' },
      slots: {
        actions: '<button>Сохранить</button><button>Отмена</button>',
      },
    })

    const actions = wrapper.get('.modal-card-foot > .buttons')
    expect(actions.classes()).toContain('mb-0')
    expect(actions.findAll('button')).toHaveLength(2)
  })
})
