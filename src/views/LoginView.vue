<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSessionStore } from '../stores/session'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
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
  <main class="centered">
    <form class="auth-card" @submit.prevent="submit">
      <h1>Вход в JKB</h1>
      <p class="muted">Лёгкая доска без лишнего церемониала.</p>
      <BaseField label="Имя пользователя" for-id="username"
        ><input id="username" v-model.trim="username" autocomplete="username" required /></BaseField
      ><BaseField label="Пароль" for-id="password"
        ><input
          id="password"
          v-model="password"
          type="password"
          autocomplete="current-password"
          required
      /></BaseField>
      <p v-if="error" class="field__error" role="alert">{{ error }}</p>
      <BaseButton type="submit" :disabled="busy">{{ busy ? 'Входим…' : 'Войти' }}</BaseButton>
    </form>
  </main>
</template>
