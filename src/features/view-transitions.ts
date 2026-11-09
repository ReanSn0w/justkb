import { nextTick } from 'vue'
import type { Router, RouteLocationNormalized } from 'vue-router'
import { cardsApi, epicsApi, type Card, type Epic } from '../api'

// Only the destination record is retained, never a second copy of the board.
export const prepared = {
  card: null as Card | null,
  project: null as Epic | null,
  projects: null as Epic[] | null,
}
export const routeKey = (route: RouteLocationNormalized) =>
  `${String(route.name)}:${String(route.params.id || '')}`
function entityKey(route: RouteLocationNormalized) {
  const name = String(route.name)
  return name.startsWith('card')
    ? `card-${route.params.id}`
    : name === 'project' || name === 'project-edit'
      ? `project-${route.params.id}`
      : ''
}
function mark(key: string) {
  if (!key) return
  for (const node of document.querySelectorAll<HTMLElement>('[data-transition-key]')) {
    if (node.dataset.transitionKey !== key) continue
    const rect = node.getBoundingClientRect()
    if (
      rect.width &&
      rect.height &&
      rect.bottom > 0 &&
      rect.top < innerHeight &&
      rect.right > 0 &&
      rect.left < innerWidth
    ) {
      node.style.viewTransitionName = 'selected-entity'
      return
    }
  }
}
function clearMarks() {
  for (const node of document.querySelectorAll<HTMLElement>('[data-transition-key]'))
    node.style.removeProperty('view-transition-name')
}
export function installViewTransitions(router: Router) {
  let active:
    { transition: ViewTransition; finish: () => void; target: RouteLocationNormalized } | undefined
  let serial = 0
  const positions = new Map<string, { x: number; y: number; board: number }>()
  const stop = () => {
    active?.finish()
    active?.transition.skipTransition()
    active = undefined
    clearMarks()
  }
  const motion = window.matchMedia('(prefers-reduced-motion: reduce)')
  motion.addEventListener('change', () => {
    if (motion.matches) stop()
  })
  router.beforeEach(() => {
    serial++
    stop()
  })
  router.beforeResolve(async (to, from) => {
    if (routeKey(to) === routeKey(from)) return
    const request = serial
    prepared.card = null
    prepared.project = null
    let card: Card | null = null
    let project: Epic | null = null
    let projects: Epic[] | null = null
    try {
      if (String(to.name).startsWith('card')) card = await cardsApi.get(String(to.params.id))
      if (to.name === 'projects') projects = (await epicsApi.list()).items
      if (to.name === 'project' || to.name === 'project-edit')
        project = await epicsApi.get(String(to.params.id))
    } catch {
      /* The view renders the ordinary loading/error state. */
    }
    if (request !== serial) return false
    prepared.card = card
    prepared.project = project
    prepared.projects = projects
    positions.set(from.fullPath, {
      x: window.scrollX,
      y: window.scrollY,
      board: document.querySelector('[data-board-scroll]')?.scrollLeft || 0,
    })
    if (positions.size > 100) positions.delete(positions.keys().next().value!)
    if (!from.name || motion.matches || !document.startViewTransition) return
    const key = entityKey(to) || entityKey(from)
    mark(key)
    await new Promise<void>((release) => {
      let finish!: () => void
      const updated = new Promise<void>((resolve) => {
        finish = resolve
      })
      try {
        const transition = document.startViewTransition(async () => {
          release() // Let the already resolved router navigation commit.
          await updated
        })
        active = { transition, finish, target: to }
        void transition.ready.catch(() => {})
        void transition.finished
          .catch(() => {})
          .finally(() => {
            if (active?.transition === transition) {
              active = undefined
              clearMarks()
            }
          })
      } catch {
        clearMarks()
        release()
      }
    })
  })
  router.afterEach(async (to, from, failure) => {
    if (failure) {
      if (active?.target === to) stop()
      return
    }
    if (routeKey(to) === routeKey(from)) return
    await nextTick()
    const position = positions.get(to.fullPath)
    window.scrollTo(position?.x || 0, position?.y || 0)
    const board = document.querySelector('[data-board-scroll]')
    if (board) board.scrollLeft = position?.board || 0
    if (active?.target === to) mark(entityKey(to) || entityKey(from))
    if (active?.target === to) active.finish()
    document.querySelector<HTMLElement>('#main-content')?.focus({ preventScroll: true })
  })
  router.onError(stop)
}
