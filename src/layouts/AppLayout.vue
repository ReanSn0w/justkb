<script setup lang="ts">
import { ref } from 'vue'
import { useSessionStore } from '../stores/session'
import { useRouter } from 'vue-router'
import UserAvatar from '../components/user/UserAvatar.vue'
const session = useSessionStore()
const router = useRouter()
const menuOpen = ref(false)
async function logout() {
  await session.logout()
  await router.push('/login')
}
</script>
<template>
  <a class="is-sr-only" href="#main-content">К основному содержимому</a>
  <nav class="navbar is-light" aria-label="Основная навигация">
    <div class="navbar-brand">
      <RouterLink class="navbar-item title is-4 mb-0 has-text-primary" to="/">JKB</RouterLink>
      <button
        :class="['navbar-burger', { 'is-active': menuOpen }]"
        aria-label="Открыть меню"
        :aria-expanded="menuOpen"
        @click="menuOpen = !menuOpen"
      >
        <span aria-hidden="true"></span>
        <span aria-hidden="true"></span>
        <span aria-hidden="true"></span>
        <span aria-hidden="true"></span>
      </button>
    </div>
    <div :class="['navbar-menu', { 'is-active': menuOpen }]">
      <div class="navbar-start">
        <RouterLink class="navbar-item" active-class="is-active" to="/" @click="menuOpen = false"
          >Доска</RouterLink
        >
        <RouterLink
          class="navbar-item"
          active-class="is-active"
          to="/projects"
          @click="menuOpen = false"
          >Проекты</RouterLink
        >
        <RouterLink
          class="navbar-item"
          active-class="is-active"
          to="/tokens"
          @click="menuOpen = false"
          >API-токены</RouterLink
        >
        <RouterLink
          v-if="session.user?.role === 'admin'"
          class="navbar-item"
          active-class="is-active"
          to="/settings"
          @click="menuOpen = false"
          >Настройки</RouterLink
        >
      </div>
      <div class="navbar-end">
        <RouterLink class="navbar-item" to="/profile" @click="menuOpen = false">
          <UserAvatar :user="session.user" :size="32" />
          <span class="ml-2">{{ session.user?.display_name }}</span>
        </RouterLink>
        <div class="navbar-item">
          <button class="button is-light" @click="logout">Выйти</button>
        </div>
      </div>
    </div>
  </nav>
  <main id="main-content" tabindex="-1"><RouterView /></main>
</template>
