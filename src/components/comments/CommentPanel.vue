<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { commentsApi, type Comment } from '../../api'
import { renderMarkdown } from '../../features/markdown'
import { useToastStore } from '../../stores/toasts'
import BaseButton from '../ui/BaseButton.vue'
import StatusState from '../ui/StatusState.vue'

const props = defineProps<{ entityId: string; entityType: 'card' | 'project' }>()
const items = ref<Comment[]>([])
const body = ref('')
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const toasts = useToastStore()

async function load() {
  loading.value = true
  try {
    items.value = (await commentsApi.list(props.entityType, props.entityId)).items
  } catch {
    error.value = 'Не удалось загрузить комментарии'
  } finally {
    loading.value = false
  }
}

async function submit() {
  const value = body.value.trim()
  if (!value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    const item = await commentsApi.create(props.entityType, props.entityId, value)
    items.value.push(item)
    body.value = ''
    toasts.show('Комментарий добавлен')
  } catch {
    error.value = 'Не удалось добавить комментарий'
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="box">
    <h2 class="title is-5">Комментарии</h2>
    <form @submit.prevent="submit">
      <div class="field">
        <div class="control">
          <textarea
            v-model="body"
            class="textarea"
            rows="3"
            placeholder="Написать комментарий…"
            aria-label="Новый комментарий"
          ></textarea>
        </div>
      </div>
      <div class="field is-grouped is-grouped-right">
        <div class="control">
          <BaseButton type="submit" :disabled="!body.trim() || busy">Отправить</BaseButton>
        </div>
      </div>
    </form>
    <div v-if="error" class="notification is-danger is-light" role="alert">{{ error }}</div>
    <StatusState v-if="loading" title="Загружаем комментарии…" tone="loading" />
    <StatusState v-else-if="!items.length" title="Комментариев пока нет" />
    <template v-else>
      <article v-for="item in items" :key="item.id" class="media">
        <div class="media-content">
          <div class="content">
            <p class="mb-2">
              <strong>{{ item.author.display_name }}</strong>
              <small class="has-text-grey">
                · {{ new Date(item.created_at).toLocaleString() }}</small
              >
            </p>
            <div v-html="renderMarkdown(item.body)"></div>
          </div>
        </div>
      </article>
    </template>
  </section>
</template>
