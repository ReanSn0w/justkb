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
            <BaseField label="Краткое описание">
              <input v-model.trim="description" class="input" />
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
      <div v-else class="columns is-multiline">
        <div
          v-for="epic in items"
          :key="epic.id"
          class="column is-half-tablet is-one-third-desktop"
        >
          <RouterLink :to="`/projects/${epic.id}`" class="card is-block">
            <div class="card-content">
              <div class="level is-mobile mb-4">
                <div class="level-left">
                  <h2 class="title is-5 mb-0">{{ epic.name }}</h2>
                </div>
                <div class="level-right">
                  <span class="tag is-link">{{ epic.card_count }} задач</span>
                </div>
              </div>
              <p class="has-text-grey">
                {{ epic.description || 'Описание пока не добавлено.' }}
              </p>
              <span v-if="epic.is_archived" class="tag mt-4">В архиве</span>
            </div>
          </RouterLink>
        </div>
      </div>
    </div>
  </section>
</template>
