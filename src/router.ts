import { createRouter, createWebHistory } from 'vue-router'
import { useSessionStore } from './stores/session'

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('./views/LoginView.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    component: () => import('./layouts/AppLayout.vue'),
    children: [
      { path: '', name: 'board', component: () => import('./views/BoardView.vue') },
      { path: 'cards/:id', name: 'card', component: () => import('./views/CardView.vue') },
      {
        path: 'settings',
        name: 'settings',
        component: () => import('./views/SettingsView.vue'),
        meta: { admin: true },
      },
      { path: 'tokens', name: 'tokens', component: () => import('./views/TokensView.vue') },
      { path: 'profile', name: 'profile', component: () => import('./views/ProfileView.vue') },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

export const router = createRouter({ history: createWebHistory(), routes })
router.beforeEach(async (to) => {
  const session = useSessionStore()
  if (!session.ready) await session.restore()
  if (!to.meta.public && !session.user) return { name: 'login', query: { next: to.fullPath } }
  if (to.meta.admin && session.user?.role !== 'admin') return { name: 'board' }
  if (to.name === 'login' && session.user) return { name: 'board' }
})
