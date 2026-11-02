import type { MoveCardInput } from '../api'

export type CardMoveReference = Pick<MoveCardInput, 'before_card_id' | 'after_card_id'>

export function compareCardPositions(first: string, second: string) {
  return Number(first) - Number(second)
}

function adjacentCard(element: Element | null, direction: 'next' | 'previous') {
  let current = element
  while (current) {
    if (current instanceof HTMLElement && current.dataset.cardId) return current.dataset.cardId
    current = direction === 'next' ? current.nextElementSibling : current.previousElementSibling
  }
  return undefined
}

export function cardMoveReference(item: HTMLElement): CardMoveReference {
  const before = adjacentCard(item.nextElementSibling, 'next')
  if (before) return { before_card_id: before }
  const after = adjacentCard(item.previousElementSibling, 'previous')
  return after ? { after_card_id: after } : {}
}
