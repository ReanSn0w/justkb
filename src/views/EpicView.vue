<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { cardsApi, epicsApi, type Card, type Epic } from '../api'
import AttachmentPanel from '../components/card/AttachmentPanel.vue'
import CommentPanel from '../components/comments/CommentPanel.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
import StatusState from '../components/ui/StatusState.vue'
import { renderMarkdown } from '../features/markdown'
import { useSessionStore } from '../stores/session'
import { useToastStore } from '../stores/toasts'

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const toasts = useToastStore()
const epic = ref<Epic | null>(null)
const cards = ref<Card[]>([])
const loading = ref(true)
const error = ref('')
const preview = ref(false)
const isAdmin = computed(() => session.user?.role === 'admin')
const editing = computed(() => route.name === 'project-edit')
const html = computed(() => renderMarkdown(epic.value?.description || ''))

onMounted(async () => {
  try {
    const id = String(route.params.id)
    const [loadedEpic, loadedCards] = await Promise.all([
      epicsApi.get(id),
      cardsApi.list({ epic_id: id, limit: 100 }),
    ])
    epic.value = loadedEpic
    cards.value = loadedCards.items
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
  epic.value.is_archived = !epic.value.is_archived
  await save()
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
            <button
              class="button is-light"
              @click="editing ? cancelEdit() : router.push('/projects')"
            >
              {{ editing ? '← Проект' : '← Проекты' }}
            </button>
          </div>
          <div class="level-right">
            <div v-if="editing" class="buttons">
              <BaseButton tone="quiet" @click="toggleArchive">
                {{ epic.is_archived ? 'Восстановить' : 'В архив' }}
              </BaseButton>
              <BaseButton @click="save">Сохранить</BaseButton>
            </div>
            <BaseButton v-else-if="isAdmin" @click="router.push(`/projects/${epic.id}/edit`)">
              Редактировать
            </BaseButton>
          </div>
        </header>
        <div v-if="error" class="notification is-danger is-light" role="alert">{{ error }}</div>

        <div class="columns is-variable is-5">
          <div class="column is-two-thirds">
            <form v-if="editing" class="box" @submit.prevent="save">
              <BaseField label="Название">
                <input v-model.trim="epic.name" class="input" required />
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
                <textarea v-else v-model="epic.description" class="textarea" rows="14" />
              </BaseField>
            </form>
            <div v-else class="box">
              <h1 class="title">{{ epic.name }}</h1>
              <div v-if="epic.description" class="content" v-html="html"></div>
              <StatusState v-else title="Описание пока не добавлено" />
            </div>

            <section class="box">
              <div class="level is-mobile">
                <div class="level-left"><h2 class="title is-5 mb-0">Задачи</h2></div>
                <div class="level-right">
                  <span class="tag is-light">{{ cards.length }}</span>
                </div>
              </div>
              <article v-for="card in cards" :key="card.id" class="media">
                <div class="media-content">
                  <RouterLink :to="`/cards/${card.id}`">
                    <strong>JKB-{{ card.number }}</strong> · {{ card.title }}
                  </RouterLink>
                </div>
              </article>
              <StatusState v-if="!cards.length" title="В этом проекте пока нет задач" />
            </section>
            <CommentPanel :entity-id="epic.id" entity-type="project" />
          </div>

          <div class="column">
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
    </div>
  </section>
</template>
