import { defineStore } from 'pinia'
import { ref } from 'vue'
import { authApi, setCsrfToken, type User } from '../api'

export const useSessionStore = defineStore('session', () => {
  const user = ref<User | null>(null)
  const csrf = ref('')
  const ready = ref(false)
  async function restore() {
    try {
      const result = await authApi.me()
      user.value = result.user
      csrf.value = result.csrf_token
      setCsrfToken(csrf.value)
    } catch {
      user.value = null
    } finally {
      ready.value = true
    }
  }
  async function login(_username: string, _password: string) {
    const result = await authApi.login(_username, _password)
    user.value = result.user
    csrf.value = result.csrf_token
    setCsrfToken(csrf.value)
    ready.value = true
  }
  async function logout() {
    try {
      await authApi.logout()
    } finally {
      user.value = null
      csrf.value = ''
      setCsrfToken('')
    }
  }
  function setUser(value: User) {
    user.value = value
  }
  return { user, csrf, ready, restore, login, logout, setUser }
})
