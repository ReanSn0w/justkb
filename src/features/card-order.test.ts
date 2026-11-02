import { describe, expect, it } from 'vitest'
import { cardMoveReference, compareCardPositions } from './card-order'

function card(id: string) {
  const element = document.createElement('div')
  element.dataset.cardId = id
  return element
}

describe('card ordering', () => {
  it('compares decimal positions numerically', () => {
    const positions = ['10000', '2.5', '-1000', '2000']
    expect(positions.sort(compareCardPositions)).toEqual(['-1000', '2.5', '2000', '10000'])
  })

  it('targets the next card after inserting at the start or middle', () => {
    const list = document.createElement('div')
    const moved = card('moved')
    list.append(moved, card('next'))
    expect(cardMoveReference(moved)).toEqual({ before_card_id: 'next' })
  })

  it('targets the previous card after inserting at the end', () => {
    const list = document.createElement('div')
    const previous = card('previous')
    const moved = card('moved')
    list.append(previous, moved)
    expect(cardMoveReference(moved)).toEqual({ after_card_id: 'previous' })
  })
})
