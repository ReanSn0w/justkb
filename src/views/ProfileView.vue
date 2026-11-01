<script setup lang="ts">
import { ref } from 'vue'
import { authApi } from '../api'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
import { useToastStore } from '../stores/toasts'
const current = ref('')
const next = ref('')
const confirm = ref('')
const error = ref('')
const toasts = useToastStore()
async function save() {
  if (next.value.length < 10) {
    error.value = 'Новый пароль должен содержать не менее 10 символов'
    return
  }
  if (next.value !== confirm.value) {
    error.value = 'Пароли не совпадают'
    return
  }
  try {
    await authApi.password(current.value, next.value)
    current.value = next.value = confirm.value = ''
    error.value = ''
    toasts.show('Пароль изменён')
  } catch {
    error.value = 'Не удалось изменить пароль'
  }
}
</script>
<template>
  <section class="section">
    <div class="container">
      <div class="columns is-centered">
        <div class="column is-two-thirds-tablet is-half-desktop">
          <h1 class="title">Профиль</h1>
          <form class="box" @submit.prevent="save">
            <BaseField label="Текущий пароль"
              ><input
                v-model="current"
                class="input"
                type="password"
                autocomplete="current-password" /></BaseField
            ><BaseField label="Новый пароль"
              ><input
                v-model="next"
                class="input"
                type="password"
                autocomplete="new-password" /></BaseField
            ><BaseField label="Повторите пароль"
              ><input v-model="confirm" class="input" type="password" autocomplete="new-password"
            /></BaseField>
            <div v-if="error" class="notification is-danger is-light" role="alert">
              {{ error }}
            </div>
            <BaseButton type="submit">Изменить пароль</BaseButton>
          </form>
        </div>
      </div>
    </div>
  </section>
</template>
