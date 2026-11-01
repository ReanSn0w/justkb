<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ApiError, cardsApi, type Activity, type Card } from '../api'
import { useBoardStore } from '../stores/board'
import { useToastStore } from '../stores/toasts'
import { renderMarkdown } from '../features/markdown'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
import StatusState from '../components/ui/StatusState.vue'
import AttachmentPanel from '../components/card/AttachmentPanel.vue'
import CommentPanel from '../components/comments/CommentPanel.vue'
const route = useRoute(),
  router = useRouter(),
  board = useBoardStore(),
  toasts = useToastStore(),
  card = ref<Card | null>(null),
  activity = ref<Activity[]>([]),
  loading = ref(true),
  error = ref(''),
  preview = ref(false)
const html = computed(() => renderMarkdown(card.value?.description || ''))
const editing = computed(() => route.name === 'card-edit')
const columnName = computed(
  () => board.columns.find((column) => column.id === card.value?.column_id)?.name || '—',
)
onMounted(async () => {
  try {
    if (!board.columns.length) await board.initialize()
    card.value = await cardsApi.get(String(route.params.id))
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
      assignee_id: card.value.assignee_id,
      tag_ids: card.value.tags.map((t) => t.id),
      due_at: card.value.due_at,
    })
    toasts.show('Карточка сохранена')
    await router.push(`/cards/${card.value.id}`)
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
  error.value = ''
  await router.push(`/cards/${card.value.id}`)
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
            <div>
              <button
                class="button is-light mb-3"
                @click="editing ? cancelEdit() : router.push('/')"
              >
                {{ editing ? '← Карточка' : '← Доска' }}
              </button>
              <p class="has-text-grey is-size-7">JKB-{{ card.number }}</p>
            </div>
          </div>
          <div class="level-right">
            <div v-if="editing" class="buttons">
              <BaseButton tone="quiet" @click="toggleArchive">{{
                card.is_archived ? 'Восстановить' : 'В архив'
              }}</BaseButton>
              <BaseButton @click="save">Сохранить</BaseButton>
            </div>
            <BaseButton v-else @click="router.push(`/cards/${card.id}/edit`)">
              Редактировать
            </BaseButton>
          </div>
        </header>
        <div v-if="error" class="notification is-danger is-light" role="alert">
          {{ error }}
          <button class="button is-danger is-light is-small" @click="router.go(0)">
            Перезагрузить
          </button>
        </div>
        <div class="columns is-variable is-5">
          <div class="column is-two-thirds">
            <form v-if="editing" class="box" @submit.prevent="save">
              <BaseField label="Заголовок">
                <input v-model.trim="card.title" class="input" required />
              </BaseField>
              <BaseField label="Описание">
                <div class="tabs is-toggle is-small mb-3">
                  <ul>
                    <li :class="{ 'is-active': !preview }">
                      <a @click="preview = false">Markdown</a>
                    </li>
                    <li :class="{ 'is-active': preview }">
                      <a @click="preview = true">Preview</a>
                    </li>
                  </ul>
                </div>
                <div v-if="preview" class="content box" v-html="html"></div>
                <textarea v-else v-model="card.description" class="textarea" rows="12" />
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
                  <BaseField label="Группа">
                    <div class="select is-fullwidth">
                      <select v-model="card.group_id">
                        <option :value="null">Без группы</option>
                        <option v-for="group in board.groups" :key="group.id" :value="group.id">
                          {{ group.name }}
                        </option>
                      </select>
                    </div>
                  </BaseField>
                </div>
                <div class="column is-half">
                  <BaseField label="Исполнитель">
                    <div class="select is-fullwidth">
                      <select v-model="card.assignee_id">
                        <option :value="null">Не назначен</option>
                        <option v-for="user in board.users" :key="user.id" :value="user.id">
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
              <h1 class="title">{{ card.title }}</h1>
              <div class="tags">
                <span class="tag is-light">{{ columnName }}</span>
                <span v-if="card.epic" class="tag is-link is-light">{{ card.epic.name }}</span>
                <span v-if="card.group" class="tag is-primary is-light">{{ card.group.name }}</span>
                <span v-if="card.assignee" class="tag is-info is-light">
                  {{ card.assignee.display_name }}
                </span>
                <span v-for="tag in card.tags" :key="tag.id" class="tag is-info">
                  {{ tag.name }}
                </span>
                <span v-if="card.due_at" class="tag is-warning is-light">
                  Срок: {{ new Date(card.due_at).toLocaleString() }}
                </span>
              </div>
              <div v-if="card.description" class="content mt-5" v-html="html"></div>
              <StatusState v-else title="Описание пока не добавлено" />
            </article>
            <CommentPanel :entity-id="card.id" entity-type="card" />
          </div>
          <div class="column">
            <aside class="box">
              <AttachmentPanel
                :entity-id="card.id"
                entity-type="card"
                :attachments="card.attachments || []"
                :readonly="!editing"
                @change="card.attachments = $event"
              />
              <hr />
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
