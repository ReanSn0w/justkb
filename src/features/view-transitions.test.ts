import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { installViewTransitions } from './view-transitions'
const reduced = { matches: false, addEventListener: vi.fn() }
afterEach(() => {
  vi.unstubAllGlobals()
  reduced.matches = false
})
async function setup() {
  vi.stubGlobal('matchMedia', () => reduced)
  vi.stubGlobal('scrollTo', vi.fn())
  const router = createRouter({
    history: createMemoryHistory(),
    routes: ['a', 'b', 'c'].map((name) => ({
      path: `/${name}`,
      name,
      component: { template: '<p />' },
    })),
  })
  installViewTransitions(router)
  await router.push('/a')
  return router
}
function nativeAPI() {
  const skip = vi.fn()
  const start = vi.fn((update: () => Promise<void>) => {
    const done = Promise.resolve().then(update)
    return { ready: done, finished: done, updateCallbackDone: done, skipTransition: skip }
  })
  Object.defineProperty(document, 'startViewTransition', { configurable: true, value: start })
  return { start, skip }
}
describe('native router transitions', () => {
  it('commits navigation inside the native update and excludes query-only changes', async () => {
    const router = await setup()
    const { start } = nativeAPI()
    await router.push('/b')
    expect(router.currentRoute.value.name).toBe('b')
    expect(start).toHaveBeenCalledTimes(1)
    await router.replace('/b?q=x')
    expect(start).toHaveBeenCalledTimes(1)
  })
  it('preserves navigation without API and with reduced motion', async () => {
    const router = await setup()
    Object.defineProperty(document, 'startViewTransition', { configurable: true, value: undefined })
    await router.push('/b')
    const { start } = nativeAPI()
    reduced.matches = true
    await router.push('/c')
    expect(start).not.toHaveBeenCalled()
    expect(router.currentRoute.value.name).toBe('c')
  })
  it('settles an aborted guard and survives API failure', async () => {
    const router = await setup()
    const { start } = nativeAPI()
    const remove = router.beforeResolve((to) => (to.name === 'b' ? false : undefined))
    // Registered guards after the integration can still cancel a resolved navigation.
    await router.push('/b')
    expect(router.currentRoute.value.name).toBe('a')
    remove()
    start.mockImplementation(() => {
      throw new Error('snapshot failed')
    })
    await router.push('/c')
    expect(router.currentRoute.value.name).toBe('c')
  })
})

it('only measures the selected entity on a board with 500 cards', async () => {
  const { cardsApi } = await import('../api')
  const get = vi.spyOn(cardsApi, 'get').mockResolvedValue({ id: '500' } as never)
  const router = await setup()
  router.addRoute({ path: '/cards/:id', name: 'card', component: { template: '<p />' } })
  let measured = 0
  for (let i = 1; i <= 500; i++) {
    const node = document.createElement('span')
    node.dataset.transitionKey = `card-${i}`
    node.getBoundingClientRect = () => {
      measured++
      return {
        x: 0,
        y: 0,
        top: 0,
        left: 0,
        right: 100,
        bottom: 20,
        width: 100,
        height: 20,
        toJSON() {},
      }
    }
    document.body.append(node)
  }
  nativeAPI()
  await router.push('/cards/500')
  expect(measured).toBeLessThanOrEqual(2)
  expect(document.querySelectorAll('[style*="selected-entity"]').length).toBeLessThanOrEqual(1)
  document.body.replaceChildren()
  get.mockRestore()
})
