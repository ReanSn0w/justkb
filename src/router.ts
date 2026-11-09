import { installViewTransitions } from './features/view-transitions'
import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useSessionStore } from './stores/session'
import { canAccessAdmin } from './features/authz'

const routes: RouteRecordRaw[] = [
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
      { path: 'projects', name: 'projects', component: () => import('./views/EpicsView.vue') },
      {
        path: 'projects/:id/edit',
        name: 'project-edit',
        component: () => import('./views/EpicView.vue'),
        meta: { admin: true },
      },
      { path: 'projects/:id', name: 'project', component: () => import('./views/EpicView.vue') },
      { path: 'epics', redirect: '/projects' },
      {
        path: 'epics/:id/edit',
        redirect: (to) => `/projects/${String(to.params.id)}/edit`,
      },
      {
        path: 'epics/:id',
        redirect: (to) => `/projects/${String(to.params.id)}`,
      },
      {
        path: 'cards/:id/edit',
        name: 'card-edit',
        component: () => import('./views/CardView.vue'),
      },
      { path: 'cards/:id', name: 'card', component: () => import('./views/CardView.vue') },
      {
        path: 'settings',
        name: 'settings',
        component: () => import('./views/SettingsView.vue'),
        meta: { admin: true },
      },
      { path: 'tokens', redirect: { name: 'profile', query: { tab: 'tokens' } } },
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
  if (to.meta.admin && !canAccessAdmin(session.user)) return { name: 'board' }
  if (to.name === 'login' && session.user) return { name: 'board' }
})

installViewTransitions(router)
