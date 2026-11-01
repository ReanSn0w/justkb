<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  ApiError,
  directoryApi,
  type BoardColumn,
  type Epic,
  type Group,
  type Tag,
  type User,
} from '../api'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
import { useToastStore } from '../stores/toasts'
type Tab = 'users' | 'groups' | 'columns' | 'epics' | 'tags'
const labels: Record<Tab, string> = {
  users: 'Пользователи',
  groups: 'Группы',
  columns: 'Колонки',
  epics: 'EPIC',
  tags: 'Теги',
}
const users = ref<User[]>([]),
  groups = ref<Group[]>([]),
  columns = ref<BoardColumn[]>([]),
  epics = ref<Epic[]>([]),
  tags = ref<Tag[]>([]),
  tab = ref<Tab>('users'),
  name = ref(''),
  color = ref('#5b6ee1'),
  error = ref(''),
  toasts = useToastStore()
async function load() {
  try {
    const [u, g, c, e, t] = await Promise.all([
      directoryApi.users(),
      directoryApi.groups(),
      directoryApi.columns(),
      directoryApi.epics(true),
      directoryApi.tags(),
    ])
    users.value = u.items
    groups.value = g.items
    columns.value = c.items.sort((a, b) => a.position - b.position)
    epics.value = e.items
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
    else if (tab.value === 'epics') await directoryApi.createEpic(data)
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
  const value = prompt('UUID групп через запятую', (user.group_ids || []).join(','))
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
async function rename(kind: Exclude<Tab, 'users'>, item: Group | BoardColumn | Epic | Tag) {
  const value = prompt('Новое название', item.name)
  if (!value) return
  if (kind === 'groups') {
    const group = item as Group
    await directoryApi.updateGroup(group.id, {
      name: value,
      color: group.color,
      description: group.description,
    })
  } else if (kind === 'columns') {
    const column = item as BoardColumn
    await directoryApi.updateColumn(column.id, { name: value, wip_limit: column.wip_limit })
  } else if (kind === 'epics') {
    const epic = item as Epic
    await directoryApi.updateEpic(epic.id, {
      name: value,
      color: epic.color,
      description: epic.description,
      is_archived: epic.is_archived,
    })
  } else {
    const tag = item as Tag
    await directoryApi.updateTag(tag.id, { name: value, color: tag.color })
  }
  await load()
}
async function toggleEpic(item: Epic) {
  await directoryApi.updateEpic(item.id, {
    name: item.name,
    color: item.color,
    description: item.description,
    is_archived: !item.is_archived,
  })
  await load()
}
</script>
<template>
  <section class="section">
    <div class="container">
      <div v-if="error" class="notification is-danger is-light" role="alert">{{ error }}</div>
      <nav class="tabs is-boxed" aria-label="Разделы настроек">
        <ul>
          <li v-for="(_, key) in labels" :key="key" :class="{ 'is-active': tab === key }">
            <a @click="tab = key">{{ labels[key] }}</a>
          </li>
        </ul>
      </nav>
      <form class="box" @submit.prevent="create">
        <div class="columns is-align-items-flex-end">
          <div class="column">
            <BaseField :label="tab === 'users' ? 'Имя/логин' : 'Название'">
              <input v-model.trim="name" class="input" required />
            </BaseField>
          </div>
          <div v-if="['groups', 'epics', 'tags'].includes(tab)" class="column is-one-quarter">
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
      <div class="box">
        <template v-if="tab === 'users'">
          <article v-for="user in users" :key="user.id" class="media">
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
                  class="button is-light"
                  @click="patchUser(user, { role: user.role === 'admin' ? 'member' : 'admin' })"
                >
                  Сменить роль
                </button>
                <button
                  class="button is-light"
                  @click="patchUser(user, { is_active: !user.is_active })"
                >
                  {{ user.is_active ? 'Отключить' : 'Включить' }}
                </button>
                <button class="button is-light" @click="membership(user)">Группы</button>
                <button class="button is-light" @click="password(user)">Пароль</button>
              </div>
            </div>
          </article>
        </template>
        <template v-else-if="tab === 'groups'">
          <article v-for="item in groups" :key="item.id" class="media">
            <div class="media-content">
              <span class="tag is-primary is-light">{{ item.name }}</span>
            </div>
            <div class="media-right">
              <button class="button is-light is-small" @click="rename('groups', item)">
                Переименовать
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
                <button class="button is-light" @click="moveColumn(index, -1)">↑</button>
                <button class="button is-light" @click="moveColumn(index, 1)">↓</button>
                <button class="button is-light" @click="rename('columns', item)">
                  Переименовать
                </button>
                <button
                  class="button is-danger is-light"
                  @click="directoryApi.deleteColumn(item.id).then(load)"
                >
                  Удалить
                </button>
              </div>
            </div>
          </article>
        </template>
        <template v-else-if="tab === 'epics'">
          <article v-for="item in epics" :key="item.id" class="media">
            <div class="media-content">
              <span class="tag is-link is-light">
                {{ item.name }} {{ item.is_archived ? '(архив)' : '' }}
              </span>
            </div>
            <div class="media-right">
              <div class="buttons are-small">
                <button class="button is-light" @click="rename('epics', item)">
                  Переименовать
                </button>
                <button class="button is-light" @click="toggleEpic(item)">
                  {{ item.is_archived ? 'Восстановить' : 'Архивировать' }}
                </button>
              </div>
            </div>
          </article>
        </template>
        <template v-else>
          <article v-for="item in tags" :key="item.id" class="media">
            <div class="media-content">
              <span class="tag is-info">{{ item.name }}</span>
            </div>
            <div class="media-right">
              <button class="button is-light is-small" @click="rename('tags', item)">
                Переименовать
              </button>
            </div>
          </article>
        </template>
        <p
          v-if="
            (tab === 'users' && !users.length) ||
            (tab === 'groups' && !groups.length) ||
            (tab === 'columns' && !columns.length) ||
            (tab === 'epics' && !epics.length) ||
            (tab === 'tags' && !tags.length)
          "
          class="has-text-grey"
        >
          Записей пока нет.
        </p>
      </div>
    </div>
  </section>
</template>
