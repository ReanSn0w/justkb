<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSessionStore } from '../stores/session'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
import { systemName } from '../system'
const username = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)
const session = useSessionStore()
const router = useRouter()
const route = useRoute()
async function submit() {
  if (!username.value || !password.value) {
    error.value = 'Введите имя и пароль'
    return
  }
  busy.value = true
  error.value = ''
  try {
    await session.login(username.value, password.value)
    await router.replace(typeof route.query.next === 'string' ? route.query.next : '/')
  } catch {
    error.value = 'Не удалось войти. Проверьте учётные данные.'
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <main class="hero is-fullheight">
    <div class="hero-body">
      <div class="container">
        <div class="columns is-centered">
          <div class="column is-one-third-desktop is-half-tablet">
            <form class="box" @submit.prevent="submit">
              <h1 class="title">{{ systemName }}</h1>
              <p class="subtitle">Канбан-доска проекта</p>
              <BaseField label="Имя пользователя" for-id="username"
                ><input
                  id="username"
                  v-model.trim="username"
                  class="input"
                  autocomplete="username"
                  required /></BaseField
              ><BaseField label="Пароль" for-id="password"
                ><input
                  id="password"
                  v-model="password"
                  class="input"
                  type="password"
                  autocomplete="current-password"
                  required
              /></BaseField>
              <div v-if="error" class="notification is-danger" role="alert">
                {{ error }}
              </div>
              <BaseButton type="submit" :disabled="busy">{{
                busy ? 'Входим…' : 'Войти'
              }}</BaseButton>
            </form>
          </div>
        </div>
      </div>
    </div>
  </main>
</template>
