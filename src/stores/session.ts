import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface SessionUser {
  id: string
  username: string
  display_name: string
  role: 'admin' | 'member'
  is_active: boolean
}

export const useSessionStore = defineStore('session', () => {
  const user = ref<SessionUser | null>(null)
  const csrf = ref('')
  const ready = ref(false)
  async function restore() {
    ready.value = true
  }
  async function login(_username: string, _password: string) {
    void _password
    user.value = {
      id: 'local',
      username: _username,
      display_name: _username,
      role: 'admin',
      is_active: true,
    }
    csrf.value = 'development'
  }
  async function logout() {
    user.value = null
    csrf.value = ''
  }
  return { user, csrf, ready, restore, login, logout }
})
