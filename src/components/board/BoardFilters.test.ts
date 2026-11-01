import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { describe, expect, it } from 'vitest'
import { useBoardStore } from '../../stores/board'
import BoardFilters from './BoardFilters.vue'

describe('board filters', () => {
  it('shows a stable refresh indicator without replacing the filters', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useBoardStore().refreshing = true
    const wrapper = mount(BoardFilters, { global: { plugins: [pinia] } })

    expect(wrapper.get('[aria-live="polite"]').text()).toBe('Обновляем…')
    expect(wrapper.find('input[type="search"]').exists()).toBe(true)
  })
})
