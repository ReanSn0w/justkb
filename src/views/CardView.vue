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
  <section class="page card-page">
    <StatusState v-if="loading" title="Загружаем карточку…" tone="loading" /><StatusState
      v-else-if="error && !card"
      :title="error"
      tone="error"
    /><template v-else-if="card"
      ><header class="card-page__header">
        <div>
          <button class="button button--quiet" @click="router.push('/')">← Доска</button>
          <p class="card-number">JKB-{{ card.number }}</p>
        </div>
        <div>
          <BaseButton tone="quiet" @click="toggleArchive">{{
            card.is_archived ? 'Восстановить' : 'В архив'
          }}</BaseButton>
          <BaseButton @click="save">Сохранить</BaseButton>
        </div>
      </header>
      <p v-if="error" class="field__error" role="alert">
        {{ error }} <button @click="router.go(0)">Перезагрузить</button>
      </p>
      <div class="card-grid">
        <form class="panel" @submit.prevent="save">
          <BaseField label="Заголовок"><input v-model.trim="card.title" required /></BaseField
          ><BaseField label="Описание"
            ><div class="editor-tabs">
              <button type="button" @click="preview = false">Markdown</button
              ><button type="button" @click="preview = true">Preview</button>
            </div>
            <div v-if="preview" class="markdown" v-html="html"></div>
            <textarea v-else v-model="card.description" rows="12" />
          </BaseField>
          <div class="form-grid">
            <BaseField label="EPIC"
              ><select v-model="card.epic_id">
                <option :value="null">Без EPIC</option>
                <option v-for="epic in board.epics" :key="epic.id" :value="epic.id">
                  {{ epic.name }}
                </option>
              </select></BaseField
            ><BaseField label="Группа"
              ><select v-model="card.group_id">
                <option :value="null">Без группы</option>
                <option v-for="group in board.groups" :key="group.id" :value="group.id">
                  {{ group.name }}
                </option>
              </select></BaseField
            ><BaseField label="Исполнитель"
              ><select v-model="card.assignee_id">
                <option :value="null">Не назначен</option>
                <option v-for="user in board.users" :key="user.id" :value="user.id">
                  {{ user.display_name }}
                </option>
              </select></BaseField
            ><BaseField label="Срок"
              ><input v-model="card.due_at" type="datetime-local"
            /></BaseField>
          </div>
          <fieldset>
            <legend>Теги</legend>
            <label v-for="tag in board.tags" :key="tag.id" class="check"
              ><input
                type="checkbox"
                :checked="card.tags.some((t) => t.id === tag.id)"
                @change="toggleTag(tag.id)"
              />{{ tag.name }}</label
            >
          </fieldset>
        </form>
        <aside class="panel">
          <AttachmentPanel
            :card-id="card.id"
            :attachments="card.attachments || []"
            @change="card.attachments = $event"
          />
          <h2>История</h2>
          <ol class="activity">
            <li v-for="item in activity" :key="item.id">
              <strong>{{ item.actor?.display_name || 'Пользователь' }}</strong> {{ item.action
              }}<small
                >{{
                  item.actor_token_id
                    ? `через MCP${item.token_name ? ' · ' + item.token_name : ''}`
                    : 'через веб'
                }}
                · {{ new Date(item.created_at).toLocaleString() }}</small
              >
            </li>
          </ol>
          <StatusState v-if="!activity.length" title="История пока пуста" />
        </aside></div
    ></template>
  </section>
</template>
