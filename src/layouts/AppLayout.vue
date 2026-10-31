<script setup lang="ts">
import { useSessionStore } from '../stores/session'
import { useRouter } from 'vue-router'
const session = useSessionStore()
const router = useRouter()
async function logout() {
  await session.logout()
  await router.push('/login')
}
</script>
<template>
  <div class="app-shell">
    <header class="app-header">
      <RouterLink class="brand" to="/">JKB</RouterLink>
      <nav aria-label="Основная навигация">
        <RouterLink to="/">Доска</RouterLink><RouterLink to="/tokens">API-токены</RouterLink
        ><RouterLink v-if="session.user?.role === 'admin'" to="/settings">Настройки</RouterLink>
      </nav>
      <div class="profile">
        <RouterLink to="/profile">{{ session.user?.display_name }}</RouterLink
        ><button class="button button--quiet" @click="logout">Выйти</button>
      </div>
    </header>
    <main><RouterView /></main>
  </div>
</template>
