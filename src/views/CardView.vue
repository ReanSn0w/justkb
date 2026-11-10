<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ApiError, cardsApi, type Activity, type Card } from '../api'
import { useBoardStore } from '../stores/board'
import { useToastStore } from '../stores/toasts'
import { prepared } from '../features/view-transitions'
import { boardReturnURL } from '../features/filters'
import { renderMarkdown } from '../features/markdown'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
import StatusState from '../components/ui/StatusState.vue'
import AttachmentPanel from '../components/card/AttachmentPanel.vue'
import CardInfoPanel from '../components/card/CardInfoPanel.vue'
import CommentPanel from '../components/comments/CommentPanel.vue'
import UserAvatar from '../components/user/UserAvatar.vue'
const route = useRoute(),
  router = useRouter(),
  board = useBoardStore(),
  toasts = useToastStore(),
  card = ref<Card | null>(
    prepared.card?.id === String(route.params.id) ? structuredClone(prepared.card) : null,
  ),
  activity = ref<Activity[]>([]),
  loading = ref(!card.value),
  error = ref('')
const returnTo = computed(() => boardReturnURL(route.query.return_to))
const cardLocation = (id: string, edit = false) => ({
  path: `/cards/${id}${edit ? '/edit' : ''}`,
  query: { return_to: returnTo.value },
})
const html = computed(() => renderMarkdown(card.value?.description || ''))
const editing = computed(() => route.name === 'card-edit')
const columnName = computed(
  () => board.columns.find((column) => column.id === card.value?.column_id)?.name || '—',
)
const editedAssigneeIDs = ref<string[]>()
const assigneeSelect = ref<HTMLSelectElement | null>(null)
const assignedIDs = computed(
  () =>
    editedAssigneeIDs.value ??
    card.value?.assignee_ids ??
    (card.value?.assignee_id ? [card.value.assignee_id] : []),
)
const selectedAssignees = computed(() => {
  const assigned = card.value?.assignees ?? (card.value?.assignee ? [card.value.assignee] : [])
  const usersByID = new Map([...assigned, ...board.users].map((user) => [user.id, user]))
  return assignedIDs.value.map((id) => usersByID.get(id)).filter((user) => user !== undefined)
})
const availableAssignees = computed(() =>
  board.users.filter(({ id }) => !assignedIDs.value.includes(id)),
)

function addAssignee(event: Event) {
  const select = event.currentTarget as HTMLSelectElement
  const id = select.value
  if (id && !assignedIDs.value.includes(id)) editedAssigneeIDs.value = [...assignedIDs.value, id]
  select.value = ''
}

async function removeAssignee(id: string) {
  editedAssigneeIDs.value = assignedIDs.value.filter((assigneeID) => assigneeID !== id)
  await nextTick()
  assigneeSelect.value?.focus()
}

onMounted(async () => {
  try {
    if (!board.columns.length) await board.initialize()
    card.value =
      prepared.card?.id === String(route.params.id)
        ? structuredClone(prepared.card)
        : await cardsApi.get(String(route.params.id))
    activity.value = (await cardsApi.activity(card.value.id)).items
  } catch {
    error.value = 'Карточка не найдена или недоступна'
  } finally {
    loading.value = false
  }
})
async function save() {
  if (!card.value) return
  try {
    card.value = await cardsApi.update(card.value.id, card.value.version, {
      title: card.value.title,
      description: card.value.description,
      epic_id: card.value.epic_id,
      group_id: card.value.group_id,
      assignee_ids: editedAssigneeIDs.value,
      tag_ids: card.value.tags.map((t) => t.id),
      due_at: card.value.due_at,
    })
    editedAssigneeIDs.value = undefined
    toasts.show('Карточка сохранена')
    await router.push(cardLocation(card.value.id))
  } catch (cause) {
    if (cause instanceof ApiError && cause.code === 'card_version_conflict') {
      error.value = 'Карточка уже изменена другим участником. Перезагрузите данные.'
    } else error.value = 'Не удалось сохранить карточку'
  }
}
async function toggleArchive() {
  if (
    !card.value ||
    !confirm(card.value.is_archived ? 'Восстановить карточку?' : 'Архивировать карточку?')
  )
    return
  card.value = card.value.is_archived
    ? await cardsApi.restore(card.value.id, card.value.version)
    : await cardsApi.archive(card.value.id, card.value.version)
}
async function cancelEdit() {
  if (!card.value) return
  card.value = await cardsApi.get(card.value.id)
  editedAssigneeIDs.value = undefined
  error.value = ''
  await router.push(cardLocation(card.value.id))
}
function toggleTag(id: string) {
  if (!card.value) return
  const tag = board.tags.find((t) => t.id === id)
  if (!tag) return
  card.value.tags = card.value.tags.some((t) => t.id === id)
    ? card.value.tags.filter((t) => t.id !== id)
    : [...card.value.tags, tag]
}
</script>
<template>
  <section class="section">
    <div class="container">
      <StatusState v-if="loading" title="Загружаем карточку…" tone="loading" />
      <StatusState v-else-if="error && !card" :title="error" tone="error" />
      <template v-else-if="card">
        <header class="level">
          <div class="level-left">
            <button class="button" @click="editing ? cancelEdit() : router.push(returnTo)">
              {{ editing ? '← Карточка' : '← Доска' }}
            </button>
          </div>
          <div class="level-item">
            <p class="card-number has-text-grey is-size-7">JKB-{{ card.number }}</p>
          </div>
          <div class="level-right">
            <div v-if="editing" class="buttons">
              <BaseButton tone="quiet" @click="toggleArchive">{{
                card.is_archived ? 'Восстановить' : 'В архив'
              }}</BaseButton>
              <BaseButton @click="save">Сохранить</BaseButton>
            </div>
            <BaseButton v-else @click="router.push(cardLocation(card.id, true))">
              Редактировать
            </BaseButton>
          </div>
        </header>
        <div v-if="error" class="notification is-danger" role="alert">
          {{ error }}
          <button class="button is-danger is-small" @click="router.go(0)">Перезагрузить</button>
        </div>
        <div class="columns is-variable is-5">
          <div class="column is-two-thirds">
            <form v-if="editing" class="box" @submit.prevent="save">
              <BaseField label="Заголовок">
                <input
                  :data-transition-key="`card-${card.id}`"
                  v-model.trim="card.title"
                  class="input"
                  required
                />
              </BaseField>
              <BaseField label="Описание">
                <textarea v-model="card.description" class="textarea" rows="12" />
              </BaseField>
              <div class="columns is-multiline">
                <div class="column is-half">
                  <BaseField label="Проект">
                    <div class="select is-fullwidth">
                      <select v-model="card.epic_id">
                        <option :value="null">Без проекта</option>
                        <option v-for="epic in board.epics" :key="epic.id" :value="epic.id">
                          {{ epic.name }}
                        </option>
                      </select>
                    </div>
                  </BaseField>
                </div>
                <div class="column is-half">
                  <BaseField label="Команда">
                    <div class="select is-fullwidth">
                      <select v-model="card.group_id">
                        <option :value="null">Без команды</option>
                        <option v-for="group in board.groups" :key="group.id" :value="group.id">
                          {{ group.name }}
                        </option>
                      </select>
                    </div>
                  </BaseField>
                </div>
                <div class="column is-half">
                  <BaseField label="Исполнители" for-id="card-assignee-picker">
                    <div v-if="selectedAssignees.length" class="tags mb-2">
                      <div
                        v-for="user in selectedAssignees"
                        :key="user.id"
                        class="tag is-info card-assignee-badge"
                      >
                        <UserAvatar
                          :user="user"
                          :size="24"
                          class="is-flex-shrink-0"
                          aria-hidden="true"
                        />
                        <span class="card-assignee-name ml-1">{{ user.display_name }}</span>
                        <button
                          type="button"
                          class="card-assignee-remove"
                          :aria-label="`Снять исполнителя: ${user.display_name}`"
                          @click="removeAssignee(user.id)"
                        >
                          <span aria-hidden="true">×</span>
                        </button>
                      </div>
                    </div>
                    <div class="select is-fullwidth">
                      <select
                        id="card-assignee-picker"
                        ref="assigneeSelect"
                        :disabled="!availableAssignees.length"
                        @change="addAssignee"
                      >
                        <option value="" disabled selected>
                          {{
                            availableAssignees.length
                              ? 'Выберите исполнителя'
                              : 'Нет доступных исполнителей'
                          }}
                        </option>
                        <option v-for="user in availableAssignees" :key="user.id" :value="user.id">
                          {{ user.display_name }}
                        </option>
                      </select>
                    </div>
                  </BaseField>
                </div>
                <div class="column is-half">
                  <BaseField label="Срок">
                    <input v-model="card.due_at" class="input" type="datetime-local" />
                  </BaseField>
                </div>
              </div>
              <fieldset>
                <legend class="label">Теги</legend>
                <label v-for="tag in board.tags" :key="tag.id" class="checkbox mr-4 mb-2">
                  <input
                    type="checkbox"
                    :checked="card.tags.some((t) => t.id === tag.id)"
                    @change="toggleTag(tag.id)"
                  />
                  {{ tag.name }}
                </label>
              </fieldset>
            </form>
            <article v-else class="box">
              <h1 :data-transition-key="`card-${card.id}`" class="title">{{ card.title }}</h1>
              <div v-if="card.description" class="content" v-html="html"></div>
              <StatusState v-else title="Описание пока не добавлено" />
              <hr />
              <AttachmentPanel
                :entity-id="card.id"
                entity-type="card"
                :attachments="card.attachments || []"
                readonly
              />
            </article>
            <CardInfoPanel
              v-if="!editing"
              class="is-hidden-tablet"
              :card="card"
              :column-name="columnName"
            />
            <CommentPanel v-if="!editing" :entity-id="card.id" entity-type="card" />
          </div>
          <div class="column">
            <CardInfoPanel
              v-if="!editing"
              class="is-hidden-mobile"
              :card="card"
              :column-name="columnName"
            />
            <aside v-if="editing" class="box">
              <AttachmentPanel
                :entity-id="card.id"
                entity-type="card"
                :attachments="card.attachments || []"
                @change="card.attachments = $event"
              />
            </aside>
            <aside class="box">
              <h2 class="title is-5">История</h2>
              <div class="content">
                <ol>
                  <li v-for="item in activity" :key="item.id" class="mb-4">
                    <strong>{{ item.actor?.display_name || 'Пользователь' }}</strong>
                    {{ item.action }}
                    <small class="is-block has-text-grey">
                      {{
                        item.actor_token_id
                          ? `через MCP${item.token_name ? ' · ' + item.token_name : ''}`
                          : 'через веб'
                      }}
                      · {{ new Date(item.created_at).toLocaleString() }}
                    </small>
                  </li>
                </ol>
              </div>
              <StatusState v-if="!activity.length" title="История пока пуста" />
            </aside>
          </div>
        </div>
      </template>
    </div>
  </section>
</template>

<style scoped>
.card-assignee-badge {
  max-width: 100%;
}

.card-assignee-name {
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-assignee-remove {
  flex-shrink: 0;
  width: 1.5rem;
  height: 100%;
  padding: 0;
  border: 0;
  background: none;
  color: var(--bulma-danger-25);
  font-size: 1rem;
  font-weight: 700;
  cursor: pointer;
}
</style>
