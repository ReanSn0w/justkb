<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { ApiError, directoryApi, type BoardColumn, type Group, type Tag, type User } from '../api'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
import UserAvatar from '../components/user/UserAvatar.vue'
import BackupSettings from '../components/admin/BackupSettings.vue'
import ColorTag from '../components/ui/ColorTag.vue'
import { useToastStore } from '../stores/toasts'
type DirectoryTab = 'users' | 'groups' | 'columns' | 'tags'
type Tab = DirectoryTab | 'backup'
type RenameKind = Exclude<DirectoryTab, 'users'>
type RenameItem = Group | BoardColumn | Tag
const labels: Record<Tab, string> = {
  users: 'Пользователи',
  groups: 'Команды',
  columns: 'Колонки',
  tags: 'Теги',
  backup: 'Резервные копии',
}
const users = ref<User[]>([]),
  groups = ref<Group[]>([]),
  columns = ref<BoardColumn[]>([]),
  tags = ref<Tag[]>([]),
  tab = ref<Tab>('users'),
  name = ref(''),
  color = ref('#00d1b2'),
  error = ref(''),
  renaming = ref<{ kind: RenameKind; item: RenameItem } | null>(null),
  renameName = ref(''),
  renameColor = ref('#00d1b2'),
  renameError = ref(''),
  renameBusy = ref(false),
  renameInput = ref<HTMLInputElement | null>(null),
  toasts = useToastStore()
async function load() {
  try {
    const [u, g, c, t] = await Promise.all([
      directoryApi.users(),
      directoryApi.groups(),
      directoryApi.columns(),
      directoryApi.tags(),
    ])
    users.value = u.items
    groups.value = g.items
    columns.value = c.items.sort((a, b) => a.position - b.position)
    tags.value = t.items
  } catch (cause) {
    error.value =
      cause instanceof ApiError && cause.status === 403
        ? 'Недостаточно прав администратора'
        : 'Не удалось загрузить настройки'
  }
}
onMounted(load)
async function create() {
  if (!name.value.trim()) return
  const data = { name: name.value.trim(), color: color.value }
  try {
    if (tab.value === 'users')
      await directoryApi.createUser({
        username: name.value.trim(),
        display_name: name.value.trim(),
        role: 'member',
        password: crypto.randomUUID(),
      })
    else if (tab.value === 'groups') await directoryApi.createGroup(data)
    else if (tab.value === 'columns') await directoryApi.createColumn({ name: data.name })
    else await directoryApi.createTag(data)
    name.value = ''
    await load()
    toasts.show('Справочник обновлён')
  } catch {
    error.value = 'Не удалось создать запись'
  }
}
async function patchUser(user: User, patch: unknown) {
  await directoryApi.updateUser(user.id, {
    display_name: user.display_name,
    email: user.email || '',
    role: user.role,
    is_active: user.is_active,
    ...(patch as Record<string, unknown>),
  })
  await load()
}
async function password(user: User) {
  const value = prompt(`Новый пароль для ${user.display_name}`)
  if (value) await directoryApi.setUserPassword(user.id, value)
}
async function membership(user: User) {
  const value = prompt('UUID команд через запятую', (user.group_ids || []).join(','))
  if (value !== null)
    await directoryApi.setUserGroups(
      user.id,
      value
        .split(',')
        .map((x) => x.trim())
        .filter(Boolean),
    )
}
async function moveColumn(index: number, delta: number) {
  const next = [...columns.value],
    target = index + delta
  if (target < 0 || target >= next.length) return
  ;[next[index], next[target]] = [next[target]!, next[index]!]
  await directoryApi.reorderColumns(next.map((x) => x.id))
  await load()
}
function startRename(kind: RenameKind, item: RenameItem) {
  renaming.value = { kind, item }
  renameName.value = item.name
  if ('color' in item) renameColor.value = item.color
  renameError.value = ''
  void nextTick(() => renameInput.value?.focus())
}
function cancelRename() {
  if (renameBusy.value) return
  renaming.value = null
}
async function saveRename() {
  const current = renaming.value,
    value = renameName.value.trim()
  if (!current || !value) return
  renameBusy.value = true
  renameError.value = ''
  try {
    if (current.kind === 'groups') {
      const group = current.item as Group
      await directoryApi.updateGroup(group.id, {
        name: value,
        color: renameColor.value,
        description: group.description,
      })
    } else if (current.kind === 'columns') {
      const column = current.item as BoardColumn
      await directoryApi.updateColumn(column.id, { name: value, wip_limit: column.wip_limit })
    } else {
      const tag = current.item as Tag
      await directoryApi.updateTag(tag.id, { name: value, color: renameColor.value })
    }
    await load()
    renaming.value = null
    toasts.show('Запись обновлена')
  } catch {
    renameError.value = 'Не удалось изменить запись'
  } finally {
    renameBusy.value = false
  }
}
</script>
<template>
  <section class="section">
    <div class="container">
      <div v-if="error" class="notification is-danger" role="alert">{{ error }}</div>
      <nav class="tabs is-boxed" aria-label="Разделы настроек">
        <ul>
          <li v-for="(_, key) in labels" :key="key" :class="{ 'is-active': tab === key }">
            <a @click="tab = key">{{ labels[key] }}</a>
          </li>
        </ul>
      </nav>
      <form v-if="tab !== 'backup'" class="box" @submit.prevent="create">
        <div class="columns is-align-items-flex-end">
          <div class="column">
            <BaseField :label="tab === 'users' ? 'Имя/логин' : 'Название'">
              <input v-model.trim="name" class="input" required />
            </BaseField>
          </div>
          <div v-if="['groups', 'tags'].includes(tab)" class="column is-one-quarter">
            <BaseField label="Цвет">
              <input v-model="color" class="input" type="color" />
            </BaseField>
          </div>
          <div class="column is-narrow">
            <div class="field">
              <div class="control"><BaseButton type="submit">Добавить</BaseButton></div>
            </div>
          </div>
        </div>
      </form>
      <div v-if="tab !== 'backup'" class="box">
        <template v-if="tab === 'users'">
          <article v-for="user in users" :key="user.id" class="media">
            <div class="media-left"><UserAvatar :user="user" :size="48" /></div>
            <div class="media-content">
              <div class="content">
                <p>
                  <strong>{{ user.display_name }}</strong>
                  <br />
                  <small>
                    @{{ user.username }} · {{ user.role }} ·
                    {{ user.is_active ? 'активен' : 'отключён' }}
                  </small>
                </p>
              </div>
            </div>
            <div class="media-right">
              <div class="buttons are-small">
                <button
                  class="button"
                  @click="patchUser(user, { role: user.role === 'admin' ? 'member' : 'admin' })"
                >
                  Сменить роль
                </button>
                <button class="button" @click="patchUser(user, { is_active: !user.is_active })">
                  {{ user.is_active ? 'Отключить' : 'Включить' }}
                </button>
                <button class="button" @click="membership(user)">Команды</button>
                <button class="button" @click="password(user)">Пароль</button>
              </div>
            </div>
          </article>
        </template>
        <template v-else-if="tab === 'groups'">
          <article v-for="item in groups" :key="item.id" class="media">
            <div class="media-content">
              <ColorTag :color="item.color">{{ item.name }}</ColorTag>
            </div>
            <div class="media-right">
              <button class="button is-small" type="button" @click="startRename('groups', item)">
                Редактировать
              </button>
            </div>
          </article>
        </template>
        <template v-else-if="tab === 'columns'">
          <article v-for="(item, index) in columns" :key="item.id" class="media">
            <div class="media-content">
              <strong>{{ item.name }}</strong>
            </div>
            <div class="media-right">
              <div class="buttons are-small">
                <button class="button" @click="moveColumn(index, -1)">↑</button>
                <button class="button" @click="moveColumn(index, 1)">↓</button>
                <button class="button" type="button" @click="startRename('columns', item)">
                  Редактировать
                </button>
                <button
                  class="button is-danger"
                  @click="directoryApi.deleteColumn(item.id).then(load)"
                >
                  Удалить
                </button>
              </div>
            </div>
          </article>
        </template>
        <template v-else-if="tab === 'tags'">
          <article v-for="item in tags" :key="item.id" class="media">
            <div class="media-content">
              <ColorTag :color="item.color">{{ item.name }}</ColorTag>
            </div>
            <div class="media-right">
              <button class="button is-small" type="button" @click="startRename('tags', item)">
                Редактировать
              </button>
            </div>
          </article>
        </template>
        <p
          v-if="
            (tab === 'users' && !users.length) ||
            (tab === 'groups' && !groups.length) ||
            (tab === 'columns' && !columns.length) ||
            (tab === 'tags' && !tags.length)
          "
          class="has-text-grey"
        >
          Записей пока нет.
        </p>
      </div>
      <BackupSettings v-else />
      <div v-if="renaming" class="modal is-active" role="dialog" aria-modal="true">
        <div class="modal-background" @click="cancelRename" />
        <div class="modal-card">
          <header class="modal-card-head">
            <h2 class="modal-card-title">Редактировать</h2>
            <button class="delete" type="button" aria-label="Закрыть" @click="cancelRename" />
          </header>
          <form @submit.prevent="saveRename">
            <section class="modal-card-body">
              <div v-if="renameError" class="notification is-danger" role="alert">
                {{ renameError }}
              </div>
              <BaseField label="Новое название">
                <input
                  ref="renameInput"
                  v-model="renameName"
                  class="input"
                  required
                  :disabled="renameBusy"
                />
              </BaseField>
              <BaseField v-if="renaming.kind !== 'columns'" label="Цвет">
                <input v-model="renameColor" class="input" type="color" :disabled="renameBusy" />
              </BaseField>
            </section>
            <footer class="modal-card-foot">
              <button class="button is-primary" type="submit" :class="{ 'is-loading': renameBusy }">
                Сохранить
              </button>
              <button class="button" type="button" :disabled="renameBusy" @click="cancelRename">
                Отмена
              </button>
            </footer>
          </form>
        </div>
      </div>
    </div>
  </section>
</template>
