<script setup lang="ts">
import { ref } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { useSessionStore } from '../stores/session'
import { useRouter } from 'vue-router'
import UserAvatar from '../components/user/UserAvatar.vue'
import { systemName } from '../system'
const session = useSessionStore()
const router = useRouter()
const menuOpen = ref(false)
const userMenuOpen = ref(false)
const userMenu = ref<HTMLElement | null>(null)
onClickOutside(userMenu, () => (userMenuOpen.value = false))
function closeMenus() {
  menuOpen.value = false
  userMenuOpen.value = false
}
function toggleMobileMenu() {
  menuOpen.value = !menuOpen.value
  userMenuOpen.value = false
}
async function logout() {
  closeMenus()
  await session.logout()
  await router.push('/login')
}
</script>
<template>
  <a class="is-sr-only" href="#main-content">К основному содержимому</a>
  <nav class="navbar is-light" aria-label="Основная навигация">
    <div class="navbar-brand">
      <RouterLink class="navbar-item title is-4 mb-0 has-text-primary" to="/" @click="closeMenus">
        {{ systemName }}
      </RouterLink>
      <button
        :class="['navbar-burger', { 'is-active': menuOpen }]"
        aria-label="Открыть меню"
        :aria-expanded="menuOpen"
        @click="toggleMobileMenu"
      >
        <span aria-hidden="true"></span>
        <span aria-hidden="true"></span>
        <span aria-hidden="true"></span>
        <span aria-hidden="true"></span>
      </button>
    </div>
    <div :class="['navbar-menu', { 'is-active': menuOpen }]">
      <div class="navbar-start">
        <RouterLink class="navbar-item" active-class="is-active" to="/" @click="closeMenus"
          >Доска</RouterLink
        >
        <RouterLink class="navbar-item" active-class="is-active" to="/projects" @click="closeMenus"
          >Проекты</RouterLink
        >
      </div>
      <div class="navbar-end">
        <div
          ref="userMenu"
          :class="['navbar-item', 'has-dropdown', { 'is-active': userMenuOpen }]"
          @keydown.esc="userMenuOpen = false"
        >
          <a
            class="navbar-link"
            role="button"
            tabindex="0"
            aria-haspopup="true"
            :aria-expanded="userMenuOpen"
            @click="userMenuOpen = !userMenuOpen"
            @keydown.enter.prevent="userMenuOpen = !userMenuOpen"
            @keydown.space.prevent="userMenuOpen = !userMenuOpen"
          >
            <UserAvatar :user="session.user" :size="32" />
            <span class="ml-2">{{ session.user?.display_name }}</span>
          </a>
          <div class="navbar-dropdown is-right">
            <RouterLink class="navbar-item" to="/profile" @click="closeMenus">
              Настройки профиля
            </RouterLink>
            <RouterLink
              v-if="session.user?.role === 'admin'"
              class="navbar-item"
              to="/settings"
              @click="closeMenus"
            >
              Настройки системы
            </RouterLink>
            <hr class="navbar-divider" />
            <div class="navbar-item">
              <button class="button is-white is-fullwidth" @click="logout">Выйти</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </nav>
  <main id="main-content" tabindex="-1"><RouterView /></main>
</template>
