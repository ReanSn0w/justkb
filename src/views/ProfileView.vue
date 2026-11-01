<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { authApi } from '../api'
import TokenSettings from '../components/user/TokenSettings.vue'
import UserAvatar from '../components/user/UserAvatar.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
import { useSessionStore } from '../stores/session'
import { useToastStore } from '../stores/toasts'

const session = useSessionStore()
const toasts = useToastStore()
const route = useRoute()
const router = useRouter()
type Tab = 'profile' | 'security' | 'tokens'
const labels: Record<Tab, string> = {
  profile: 'Профиль',
  security: 'Безопасность',
  tokens: 'API-токены',
}
const tab = computed<Tab>(() => {
  const value = Array.isArray(route.query.tab) ? route.query.tab[0] : route.query.tab
  return value === 'security' || value === 'tokens' ? value : 'profile'
})
const displayName = ref(session.user?.display_name || '')
const avatar = ref<File | null>(null)
const profileError = ref('')
const profileBusy = ref(false)
const current = ref('')
const next = ref('')
const confirm = ref('')
const passwordError = ref('')

async function selectTab(value: Tab) {
  await router.replace({
    query: { ...route.query, tab: value === 'profile' ? undefined : value },
  })
}

async function saveProfile() {
  const value = displayName.value.trim()
  if (!value || profileBusy.value) return
  profileBusy.value = true
  profileError.value = ''
  try {
    session.setUser(await authApi.profile(value))
    displayName.value = session.user?.display_name || value
    toasts.show('Профиль сохранён')
  } catch {
    profileError.value = 'Не удалось сохранить профиль'
  } finally {
    profileBusy.value = false
  }
}

function selectAvatar(event: Event) {
  profileError.value = ''
  const file = (event.target as HTMLInputElement).files?.[0] || null
  if (file && !['image/jpeg', 'image/png', 'image/gif', 'image/webp'].includes(file.type)) {
    profileError.value = 'Поддерживаются JPEG, PNG, GIF и WebP'
    avatar.value = null
    return
  }
  if (file && file.size > 5 * 1024 * 1024) {
    profileError.value = 'Размер аватара не должен превышать 5 МБ'
    avatar.value = null
    return
  }
  avatar.value = file
}

async function uploadAvatar() {
  if (!avatar.value || profileBusy.value) return
  profileBusy.value = true
  profileError.value = ''
  try {
    session.setUser(await authApi.uploadAvatar(avatar.value))
    avatar.value = null
    toasts.show('Аватар обновлён')
  } catch {
    profileError.value = 'Не удалось загрузить аватар'
  } finally {
    profileBusy.value = false
  }
}

async function deleteAvatar() {
  if (!session.user?.avatar_url || profileBusy.value) return
  profileBusy.value = true
  profileError.value = ''
  try {
    await authApi.deleteAvatar()
    session.setUser({ ...session.user, avatar_url: null })
    toasts.show('Аватар удалён')
  } catch {
    profileError.value = 'Не удалось удалить аватар'
  } finally {
    profileBusy.value = false
  }
}

async function savePassword() {
  if (next.value.length < 12) {
    passwordError.value = 'Новый пароль должен содержать не менее 12 символов'
    return
  }
  if (next.value !== confirm.value) {
    passwordError.value = 'Пароли не совпадают'
    return
  }
  try {
    await authApi.password(current.value, next.value)
    current.value = next.value = confirm.value = ''
    passwordError.value = ''
    toasts.show('Пароль изменён')
  } catch {
    passwordError.value = 'Не удалось изменить пароль'
  }
}
</script>

<template>
  <section class="section">
    <div class="container">
      <nav class="tabs is-boxed" aria-label="Разделы настроек пользователя">
        <ul>
          <li v-for="(label, key) in labels" :key="key" :class="{ 'is-active': tab === key }">
            <a @click="selectTab(key)">{{ label }}</a>
          </li>
        </ul>
      </nav>

      <div v-if="tab === 'profile'" class="columns is-centered">
        <div class="column is-two-thirds-tablet is-half-desktop">
          <form class="box" @submit.prevent="saveProfile">
            <article class="media">
              <div class="media-left">
                <UserAvatar :user="session.user" :size="96" />
              </div>
              <div class="media-content">
                <div class="file has-name is-fullwidth mb-3">
                  <label class="file-label">
                    <input
                      class="file-input"
                      type="file"
                      accept="image/jpeg,image/png,image/gif,image/webp"
                      @change="selectAvatar"
                    />
                    <span class="file-cta"><span class="file-label">Выбрать аватар</span></span>
                    <span class="file-name">{{ avatar?.name || 'Файл не выбран' }}</span>
                  </label>
                </div>
                <div class="buttons">
                  <BaseButton :disabled="!avatar || profileBusy" @click="uploadAvatar">
                    Загрузить
                  </BaseButton>
                  <BaseButton
                    v-if="session.user?.avatar_url"
                    tone="quiet"
                    :disabled="profileBusy"
                    @click="deleteAvatar"
                  >
                    Удалить
                  </BaseButton>
                </div>
                <p class="help">JPEG, PNG, GIF или WebP, не более 5 МБ.</p>
              </div>
            </article>
            <hr />
            <BaseField label="Отображаемое имя">
              <input v-model="displayName" class="input" maxlength="200" required />
            </BaseField>
            <div v-if="profileError" class="notification is-danger is-light" role="alert">
              {{ profileError }}
            </div>
            <BaseButton type="submit" :disabled="profileBusy">Сохранить профиль</BaseButton>
          </form>
        </div>
      </div>

      <div v-else-if="tab === 'security'" class="columns is-centered">
        <div class="column is-two-thirds-tablet is-half-desktop">
          <form class="box" @submit.prevent="savePassword">
            <BaseField label="Текущий пароль">
              <input
                v-model="current"
                class="input"
                type="password"
                autocomplete="current-password"
                required
              />
            </BaseField>
            <BaseField label="Новый пароль">
              <input
                v-model="next"
                class="input"
                type="password"
                autocomplete="new-password"
                minlength="12"
                required
              />
            </BaseField>
            <BaseField label="Повторите пароль">
              <input
                v-model="confirm"
                class="input"
                type="password"
                autocomplete="new-password"
                minlength="12"
                required
              />
            </BaseField>
            <div v-if="passwordError" class="notification is-danger is-light" role="alert">
              {{ passwordError }}
            </div>
            <BaseButton type="submit">Изменить пароль</BaseButton>
          </form>
        </div>
      </div>

      <TokenSettings v-else />
    </div>
  </section>
</template>
