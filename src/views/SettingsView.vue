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
  <section class="page settings">
    <h1>Настройки</h1>
    <p v-if="error" class="field__error" role="alert">{{ error }}</p>
    <nav class="tabs" aria-label="Разделы настроек">
      <button
        v-for="(_, key) in labels"
        :key="key"
        :class="{ selected: tab === key }"
        @click="tab = key"
      >
        {{ labels[key] }}
      </button>
    </nav>
    <form class="panel inline-form" @submit.prevent="create">
      <BaseField :label="tab === 'users' ? 'Имя/логин' : 'Название'"
        ><input v-model.trim="name" required /></BaseField
      ><BaseField v-if="['groups', 'epics', 'tags'].includes(tab)" label="Цвет"
        ><input v-model="color" type="color" /></BaseField
      ><BaseButton type="submit">Добавить</BaseButton>
    </form>
    <div class="panel admin-list">
      <template v-if="tab === 'users'"
        ><article v-for="user in users" :key="user.id">
          <div>
            <strong>{{ user.display_name }}</strong
            ><small
              >@{{ user.username }} · {{ user.role }} ·
              {{ user.is_active ? 'активен' : 'отключён' }}</small
            >
          </div>
          <div>
            <button @click="patchUser(user, { role: user.role === 'admin' ? 'member' : 'admin' })">
              Сменить роль</button
            ><button @click="patchUser(user, { is_active: !user.is_active })">
              {{ user.is_active ? 'Отключить' : 'Включить' }}</button
            ><button @click="membership(user)">Группы</button
            ><button @click="password(user)">Пароль</button>
          </div>
        </article></template
      ><template v-else-if="tab === 'groups'"
        ><article v-for="item in groups" :key="item.id">
          <span class="color-dot" :style="{ background: item.color }" /><strong>{{
            item.name
          }}</strong
          ><button @click="rename('groups', item)">Переименовать</button>
        </article></template
      ><template v-else-if="tab === 'columns'"
        ><article v-for="(item, index) in columns" :key="item.id">
          <strong>{{ item.name }}</strong>
          <div>
            <button @click="moveColumn(index, -1)">↑</button
            ><button @click="moveColumn(index, 1)">↓</button
            ><button @click="rename('columns', item)">Переименовать</button
            ><button @click="directoryApi.deleteColumn(item.id).then(load)">Удалить</button>
          </div>
        </article></template
      ><template v-else-if="tab === 'epics'"
        ><article v-for="item in epics" :key="item.id">
          <span class="color-dot" :style="{ background: item.color }" /><strong
            >{{ item.name }} {{ item.is_archived ? '(архив)' : '' }}</strong
          >
          <div>
            <button @click="rename('epics', item)">Переименовать</button
            ><button @click="toggleEpic(item)">
              {{ item.is_archived ? 'Восстановить' : 'Архивировать' }}
            </button>
          </div>
        </article></template
      ><template v-else
        ><article v-for="item in tags" :key="item.id">
          <span class="color-dot" :style="{ background: item.color }" /><strong>{{
            item.name
          }}</strong
          ><button @click="rename('tags', item)">Переименовать</button>
        </article></template
      >
    </div>
  </section>
</template>
