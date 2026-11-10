<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { cardsApi, directoryApi, epicsApi, type BoardColumn, type Card, type Epic } from '../api'
import AttachmentPanel from '../components/card/AttachmentPanel.vue'
import CommentPanel from '../components/comments/CommentPanel.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
import StatusState from '../components/ui/StatusState.vue'
import { prepared } from '../features/view-transitions'
import { renderMarkdown } from '../features/markdown'
import { useSessionStore } from '../stores/session'
import { boardReturnURL } from '../features/filters'
import { archiveError } from '../features/archive'
import RestoreProjectDialog from '../components/archive/RestoreProjectDialog.vue'
import { useBoardStore } from '../stores/board'
import { useToastStore } from '../stores/toasts'

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const toasts = useToastStore()
const epic = ref<Epic | null>(
  prepared.project?.id === String(route.params.id) ? structuredClone(prepared.project) : null,
)
const cards = ref<Card[]>([])
const columns = ref<BoardColumn[]>([])
const loading = ref(!epic.value)
const error = ref('')
const isAdmin = computed(() => session.user?.role === 'admin')
const editing = computed(() => route.name === 'project-edit' && !epic.value?.is_archived)
const returnTo = computed(() =>
  typeof route.query.return_to === 'string' &&
  boardReturnURL(route.query.return_to).startsWith('/archive')
    ? boardReturnURL(route.query.return_to)
    : '/projects',
)
const restoring = ref<Epic | null>(null)
const board = useBoardStore()
const html = computed(() => renderMarkdown(epic.value?.description || ''))
const countByColumn = computed(() =>
  Object.fromEntries(
    columns.value.map((column) => [
      column.id,
      cards.value.filter((card) => card.column_id === column.id).length,
    ]),
  ),
)

async function loadProjectCards(id: string) {
  const first = await cardsApi.list({ epic_id: id, limit: 100 })
  const items = [...first.items]
  let cursor = first.next_cursor
  while (cursor) {
    const page = await cardsApi.list({ epic_id: id, limit: 100, cursor })
    items.push(...page.items)
    cursor = page.next_cursor
  }
  return items
}

onMounted(async () => {
  try {
    const id = String(route.params.id)
    const [loadedEpic, loadedCards, loadedColumns] = await Promise.all([
      prepared.project?.id === id
        ? Promise.resolve(structuredClone(prepared.project))
        : epicsApi.get(id),
      loadProjectCards(id),
      directoryApi.columns(),
    ])
    epic.value = loadedEpic
    cards.value = loadedCards
    columns.value = loadedColumns.items.sort((a, b) => a.position - b.position)
  } catch {
    error.value = 'Проект не найден или недоступен'
  } finally {
    loading.value = false
  }
})

async function save() {
  if (!epic.value || !isAdmin.value) return
  try {
    const current = epic.value
    const updated = await epicsApi.update(current.id, {
      expected_version: current.version,
      restore_mode: 'project_only',
      name: current.name,
      description: current.description || null,
      color: current.color,
      is_archived: current.is_archived,
    })
    epic.value = {
      ...updated,
      card_count: current.card_count,
      attachments: current.attachments,
    }
    toasts.show('Проект сохранён')
    await router.push(`/projects/${current.id}`)
  } catch {
    error.value = 'Не удалось сохранить проект'
  }
}

async function toggleArchive() {
  if (!epic.value || !isAdmin.value) return
  try {
    if (epic.value.is_archived) {
      restoring.value = await epicsApi.get(epic.value.id)
      return
    }
    if (!confirm('Архивировать проект вместе с его активными карточками?')) return
    epic.value = await epicsApi.update(epic.value.id, {
      name: epic.value.name,
      color: epic.value.color,
      description: epic.value.description,
      expected_version: epic.value.version,
      is_archived: true,
    })
    await board.initialize()
    await router.replace({ path: `/projects/${epic.value.id}`, query: { return_to: '/archive' } })
  } catch (cause) {
    error.value = archiveError(cause)
  }
}
async function restored(project: Epic) {
  epic.value = project
  restoring.value = null
  await board.initialize()
}

async function cancelEdit() {
  if (!epic.value) return
  epic.value = await epicsApi.get(epic.value.id)
  error.value = ''
  await router.push(`/projects/${epic.value.id}`)
}
</script>

<template>
  <section class="section">
    <div class="container">
      <StatusState v-if="loading" title="Загружаем проект…" tone="loading" />
      <StatusState v-else-if="!epic" :title="error" tone="error" />
      <template v-else>
        <header class="level">
          <div class="level-left">
            <button class="button" @click="editing ? cancelEdit() : router.push(returnTo)">
              {{ editing ? '← Проект' : returnTo.startsWith('/archive') ? '← Архив' : '← Проекты' }}
            </button>
          </div>
          <div class="level-right">
            <BaseButton v-if="epic.is_archived && isAdmin" @click="toggleArchive"
              >Восстановить</BaseButton
            >
            <div v-else-if="editing" class="buttons">
              <BaseButton tone="quiet" @click="toggleArchive">
                {{ epic.is_archived ? 'Восстановить' : 'В архив' }}
              </BaseButton>
              <BaseButton @click="save">Сохранить</BaseButton>
            </div>
            <BaseButton
              v-else-if="isAdmin && !epic.is_archived"
              @click="router.push(`/projects/${epic.id}/edit`)"
            >
              Редактировать
            </BaseButton>
          </div>
        </header>
        <div v-if="error" class="notification is-danger" role="alert">{{ error }}</div>

        <div class="columns is-variable is-5">
          <div class="column is-two-thirds">
            <form v-if="editing" class="box" @submit.prevent="save">
              <BaseField label="Название">
                <input
                  :data-transition-key="`project-${epic.id}`"
                  v-model.trim="epic.name"
                  class="input"
                  required
                />
              </BaseField>
              <BaseField label="Описание">
                <textarea v-model="epic.description" class="textarea" rows="14" />
              </BaseField>
            </form>
            <div v-else class="box">
              <h1 :data-transition-key="`project-${epic.id}`" class="title">{{ epic.name }}</h1>
              <div v-if="epic.description" class="content" v-html="html"></div>
              <StatusState v-else title="Описание пока не добавлено" />
            </div>

            <CommentPanel
              v-if="!editing"
              :entity-id="epic.id"
              entity-type="project"
              :readonly="epic.is_archived"
            />
          </div>

          <div class="column">
            <aside class="box">
              <div class="level is-mobile">
                <div class="level-left"><h2 class="title is-5 mb-0">Задачи</h2></div>
                <div class="level-right">
                  <span class="tag">{{ epic.card_count }}</span>
                </div>
              </div>
              <RouterLink
                v-if="!epic.is_archived"
                class="button is-primary is-fullwidth mb-5"
                :to="{ name: 'board', query: { epic_id: epic.id } }"
              >
                Открыть на доске
              </RouterLink>
              <template v-if="cards.length">
                <RouterLink
                  v-for="column in columns"
                  :key="column.id"
                  class="block"
                  :to="{
                    name: 'board',
                    query: { epic_id: epic.id, column_id: column.id },
                  }"
                >
                  <span class="level is-mobile mb-1">
                    <span class="level-left">{{ column.name }}</span>
                    <span class="level-right tag">{{ countByColumn[column.id] || 0 }}</span>
                  </span>
                  <progress
                    class="progress is-primary is-small"
                    :value="countByColumn[column.id] || 0"
                    :max="cards.length"
                  >
                    {{ countByColumn[column.id] || 0 }} из {{ cards.length }}
                  </progress>
                </RouterLink>
              </template>
              <RouterLink
                v-if="epic.is_archived"
                :to="{ path: '/archive', query: { project_id: epic.id, type: 'card' } }"
                >Посмотреть карточки в архиве</RouterLink
              >
              <StatusState v-else-if="!cards.length" title="Активных задач на доске нет" />
            </aside>
            <aside class="box">
              <AttachmentPanel
                :entity-id="epic.id"
                entity-type="epic"
                :attachments="epic.attachments || []"
                :readonly="!editing"
                @change="epic.attachments = $event"
              />
            </aside>
          </div>
        </div>
      </template>
      <RestoreProjectDialog :project="restoring" @close="restoring = null" @restored="restored" />
    </div>
  </section>
</template>
