<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { ApiError, botsApi, directoryApi, type User } from '../../api'
import { useBoardStore } from '../../stores/board'
import BaseDialog from '../ui/BaseDialog.vue'
import BaseButton from '../ui/BaseButton.vue'
import UserAvatar from '../user/UserAvatar.vue'

const board = useBoardStore()
const users = ref<User[]>([])
const name = ref('')
const error = ref('')
const busy = ref(false)
const secret = ref('')
const secretName = ref('')
const copied = ref(false)
const editing = ref<User | null>(null)
const editName = ref('')
const avatar = ref<File | null>(null)
const editError = ref('')
const rotation = ref<{ user: User; tokenId: string } | null>(null)
let alive = true
onBeforeUnmount(() => {
  alive = false
  secret.value = ''
})
async function load() {
  const result = await directoryApi.users(true)
  if (!alive) return
  users.value = result.items.filter((user) => user.account_type === 'bot')
  board.users = result.items.filter((user) => user.is_active)
}
onMounted(async () => {
  try {
    await load()
  } catch {
    error.value = 'Не удалось загрузить ботов'
  }
})
function showSecret(value: string, displayName: string) {
  if (!alive) return
  secretName.value = displayName
  secret.value = value
  copied.value = false
}
function closeSecret() {
  secret.value = ''
  secretName.value = ''
  copied.value = false
}
async function copy() {
  try {
    await navigator.clipboard.writeText(secret.value)
    copied.value = true
  } catch {
    error.value = 'Не удалось скопировать. Выделите токен и скопируйте вручную.'
  }
}
async function create() {
  if (busy.value || !name.value.trim()) return
  busy.value = true
  error.value = ''
  try {
    const displayName = name.value.trim()
    const result = await botsApi.create(displayName)
    showSecret(result.token.token, displayName)
    name.value = ''
    await load()
  } catch {
    error.value =
      'Не удалось завершить создание. Обновите список: если бот создан, перевыпустите его токен.'
    await load().catch(() => undefined)
  } finally {
    busy.value = false
  }
}
function edit(user: User) {
  editing.value = user
  editName.value = user.display_name
  avatar.value = null
  editError.value = ''
}
function closeEdit() {
  if (!busy.value) {
    editing.value = null
    avatar.value = null
  }
}
async function save() {
  const user = editing.value
  if (!user || busy.value || !editName.value.trim()) return
  busy.value = true
  editError.value = ''
  try {
    await directoryApi.updateUser(user.id, {
      display_name: editName.value.trim(),
      email: user.email || '',
      role: 'member',
      is_active: user.is_active,
    })
    await load()
    editing.value = null
  } catch {
    editError.value = 'Не удалось сохранить имя'
  } finally {
    busy.value = false
  }
}
function selectAvatar(event: Event) {
  avatar.value = null
  editError.value = ''
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (
    !['image/png', 'image/jpeg', 'image/gif', 'image/webp'].includes(file.type) ||
    file.size > 5 * 1024 * 1024
  ) {
    editError.value = 'Выберите JPEG, PNG, GIF или WebP размером до 5 МБ'
    return
  }
  avatar.value = file
}
async function changeAvatar(remove = false) {
  const user = editing.value
  if (!user || busy.value || (!remove && !avatar.value)) return
  busy.value = true
  editError.value = ''
  try {
    if (remove) await botsApi.deleteAvatar(user.id)
    else await botsApi.uploadAvatar(user.id, avatar.value!)
    await load()
    editing.value = users.value.find((item) => item.id === user.id) || null
    avatar.value = null
  } catch {
    editError.value = 'Не удалось изменить аватар'
  } finally {
    busy.value = false
  }
}
async function toggle(user: User) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await directoryApi.updateUser(user.id, {
      display_name: user.display_name,
      email: user.email || '',
      role: 'member',
      is_active: !user.is_active,
    })
    await load()
  } catch {
    error.value = 'Не удалось изменить активность бота'
  } finally {
    busy.value = false
  }
}
async function prepareRotation(user: User) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    const token = await botsApi.token(user.id)
    rotation.value = { user, tokenId: token.id }
  } catch {
    error.value = 'Не удалось получить состояние токена'
  } finally {
    busy.value = false
  }
}
async function rotate() {
  const current = rotation.value
  if (!current || busy.value) return
  busy.value = true
  error.value = ''
  try {
    const result = await botsApi.rotate(current.user.id, current.tokenId)
    rotation.value = null
    showSecret(result.token, current.user.display_name)
  } catch (cause) {
    rotation.value = null
    error.value =
      cause instanceof ApiError && cause.code === 'bot_token_conflict'
        ? 'Токен уже изменился. Откройте перевыпуск заново, если он ещё нужен.'
        : 'Ответ о перевыпуске не получен. Если токен потерян, выполните новый явный перевыпуск.'
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <div>
    <div v-if="error" class="notification is-danger" role="alert">{{ error }}</div>
    <form class="box" @submit.prevent="create">
      <h2 class="title is-4">Новый бот</h2>
      <p class="mb-4">
        Отдельный исполнитель для агента. Вход только через MCP; токен всегда разрешает чтение и
        изменение карточек.
      </p>
      <label class="label" for="bot-name">Имя бота</label>
      <input
        id="bot-name"
        v-model="name"
        class="input mb-4"
        maxlength="200"
        required
        :disabled="busy"
        placeholder="Например, Агент разработки"
      />
      <BaseButton type="submit" :disabled="busy || !name.trim()">Создать бота</BaseButton>
    </form>
    <section class="box" aria-label="Аккаунты ботов">
      <h2 class="title is-4">Боты</h2>
      <p v-if="!users.length">Ботов пока нет.</p>
      <article v-for="user in users" :key="user.id" class="media">
        <div class="media-left"><UserAvatar :user="user" :size="48" /></div>
        <div class="media-content">
          <strong>{{ user.display_name }}</strong> <span class="tag">Бот</span>
          <p class="help">{{ user.is_active ? 'Активен' : 'Отключён' }} · MCP: чтение и запись</p>
          <div class="buttons are-small mt-3">
            <button class="button" :disabled="busy" @click="edit(user)">Имя и аватар</button>
            <button class="button" :disabled="busy" @click="prepareRotation(user)">
              Перевыпустить токен
            </button>
            <button class="button" :disabled="busy" @click="toggle(user)">
              {{ user.is_active ? 'Отключить' : 'Включить' }}
            </button>
          </div>
        </div>
      </article>
    </section>
    <BaseDialog :open="!!editing" title="Имя и аватар бота" @close="closeEdit">
      <div v-if="editError" class="notification is-danger" role="alert">{{ editError }}</div>
      <form id="bot-edit" @submit.prevent="save">
        <label class="label" for="bot-edit-name">Имя бота</label>
        <input
          id="bot-edit-name"
          v-model="editName"
          class="input mb-4"
          maxlength="200"
          required
          :disabled="busy"
        />
      </form>
      <UserAvatar v-if="editing" :user="editing" :size="64" />
      <label class="label mt-3" for="bot-avatar">Аватар (до 5 МБ)</label>
      <input
        id="bot-avatar"
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp"
        :disabled="busy"
        @change="selectAvatar"
      />
      <div class="buttons mt-3">
        <BaseButton :disabled="busy || !avatar" @click="changeAvatar()"
          >Загрузить аватар</BaseButton
        >
        <BaseButton
          tone="quiet"
          :disabled="busy || !editing?.avatar_url"
          @click="changeAvatar(true)"
          >Удалить аватар</BaseButton
        >
      </div>
      <template #actions>
        <BaseButton type="submit" form="bot-edit" :disabled="busy || !editName.trim()"
          >Сохранить имя</BaseButton
        >
        <BaseButton tone="quiet" :disabled="busy" @click="closeEdit">Закрыть</BaseButton>
      </template>
    </BaseDialog>
    <BaseDialog
      :open="!!rotation"
      title="Перевыпустить токен бота"
      @close="!busy && (rotation = null)"
    >
      <p>
        Предыдущий токен бота «{{ rotation?.user.display_name }}» сразу перестанет работать. Новый
        нужно будет указать в MCP-клиенте агента.
      </p>
      <template #actions>
        <BaseButton :disabled="busy" @click="rotate">Перевыпустить</BaseButton>
        <BaseButton tone="quiet" :disabled="busy" @click="rotation = null">Отмена</BaseButton>
      </template>
    </BaseDialog>
    <BaseDialog :open="!!secret" title="MCP-токен бота" @close="closeSecret">
      <p>
        Бот: {{ secretName }}. Скопируйте токен сейчас: после закрытия он больше не будет показан.
      </p>
      <label class="label mt-3" for="bot-secret">Токен</label>
      <textarea
        id="bot-secret"
        :value="secret"
        class="textarea"
        readonly
        autocomplete="off"
        spellcheck="false"
      />
      <p v-if="copied" role="status">Скопировано</p>
      <template #actions>
        <BaseButton tone="quiet" @click="copy">Копировать</BaseButton>
        <BaseButton @click="closeSecret">Готово</BaseButton>
      </template>
    </BaseDialog>
  </div>
</template>
