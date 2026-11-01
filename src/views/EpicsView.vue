<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { epicsApi, type Epic } from '../api'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
import StatusState from '../components/ui/StatusState.vue'
import { useSessionStore } from '../stores/session'
import { useToastStore } from '../stores/toasts'

const router = useRouter()
const session = useSessionStore()
const toasts = useToastStore()
const items = ref<Epic[]>([])
const loading = ref(true)
const error = ref('')
const name = ref('')
const description = ref('')

function completionPercent(epic: Epic) {
  if (!epic.card_count) return 0
  return Math.min(100, Math.round((epic.completed_card_count / epic.card_count) * 100))
}

function taskLabel(count: number) {
  const mod100 = count % 100
  const mod10 = count % 10
  if (mod100 >= 11 && mod100 <= 14) return `${count} задач`
  if (mod10 === 1) return `${count} задача`
  if (mod10 >= 2 && mod10 <= 4) return `${count} задачи`
  return `${count} задач`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    items.value = (await epicsApi.list(session.user?.role === 'admin')).items
  } catch {
    error.value = 'Не удалось загрузить проекты'
  } finally {
    loading.value = false
  }
}

async function create() {
  if (!name.value.trim()) return
  try {
    const epic = await epicsApi.create({
      name: name.value.trim(),
      description: description.value.trim() || null,
      color: '#00d1b2',
      is_archived: false,
    })
    toasts.show('Проект создан')
    await router.push(`/projects/${epic.id}`)
  } catch {
    error.value = 'Не удалось создать проект'
  }
}

onMounted(load)
</script>

<template>
  <section class="section">
    <div class="container">
      <div v-if="error" class="notification is-danger" role="alert">{{ error }}</div>
      <form v-if="session.user?.role === 'admin'" class="box" @submit.prevent="create">
        <h2 class="title is-5">Новый проект</h2>
        <div class="columns is-align-items-flex-end">
          <div class="column">
            <BaseField label="Название">
              <input v-model.trim="name" class="input" required />
            </BaseField>
          </div>
          <div class="column is-half">
            <BaseField label="Описание">
              <textarea v-model.trim="description" class="textarea" rows="3" />
            </BaseField>
          </div>
          <div class="column is-narrow">
            <div class="field">
              <div class="control"><BaseButton type="submit">Создать</BaseButton></div>
            </div>
          </div>
        </div>
      </form>

      <StatusState v-if="loading" title="Загружаем проекты…" tone="loading" />
      <StatusState v-else-if="!items.length" title="Проектов пока нет" />
      <nav v-else class="panel" aria-label="Проекты">
        <RouterLink
          v-for="epic in items"
          :key="epic.id"
          :to="`/projects/${epic.id}`"
          class="panel-block is-block py-5"
        >
          <div class="level mb-3">
            <div class="level-left">
              <h2 class="title is-5 mb-0">{{ epic.name }}</h2>
            </div>
            <div class="level-right">
              <div class="tags">
                <span v-if="epic.is_archived" class="tag">В архиве</span>
                <span class="tag is-primary">{{ taskLabel(epic.card_count) }}</span>
              </div>
            </div>
          </div>
          <div class="level is-mobile mb-2">
            <div class="level-left">
              <span class="is-size-7 has-text-grey">
                {{ epic.completed_card_count }} из {{ epic.card_count }} в последней колонке
              </span>
            </div>
            <div class="level-right">
              <strong class="is-size-7">{{ completionPercent(epic) }}%</strong>
            </div>
          </div>
          <progress class="progress is-primary is-small" :value="completionPercent(epic)" max="100">
            {{ completionPercent(epic) }}%
          </progress>
        </RouterLink>
      </nav>
    </div>
  </section>
</template>
