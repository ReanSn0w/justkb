<script setup lang="ts">
import { ref } from 'vue'
import { archiveApi, type ArchiveEvent } from '../../api'
import BaseButton from '../ui/BaseButton.vue'
const props = defineProps<{
  entry: ArchiveEvent
  returnTo: string
  admin: boolean
  busy?: boolean
}>()
const emit = defineEmits<{ restore: [ArchiveEvent] }>()
const expanded = ref(false),
  loading = ref(false),
  error = ref('')
const children = ref<ArchiveEvent[]>([]),
  cursor = ref<string | null>('')
async function loadMore() {
  if (!props.entry.batch_id) return
  loading.value = true
  error.value = ''
  try {
    const page = await archiveApi.list({
      batch_id: props.entry.batch_id,
      history: true,
      cursor: cursor.value || undefined,
    })
    children.value.push(...page.items)
    cursor.value = page.next_cursor || null
  } catch {
    error.value = 'Не удалось загрузить карточки операции'
  } finally {
    loading.value = false
  }
}
async function toggle() {
  expanded.value = !expanded.value
  if (expanded.value && !children.value.length) await loadMore()
}
</script>
<template>
  <article class="box archive-entry">
    <div class="archive-entry-main">
      <div class="archive-entry-description">
        <p class="is-size-7 mb-1">
          {{ entry.entity_type === 'project' ? 'Проект' : `Карточка JKB-${entry.number}` }} ·
          {{ entry.action === 'archived' ? 'Архивирование' : 'Восстановление' }}
        </p>
        <RouterLink
          class="has-text-weight-semibold"
          :to="{
            path:
              entry.entity_type === 'project'
                ? `/projects/${entry.entity_id}`
                : `/cards/${entry.entity_id}`,
            query: { return_to: returnTo },
          }"
          >{{ entry.title }}</RouterLink
        >
        <p class="is-size-7 mt-2">
          <time v-if="entry.occurred_at" :datetime="entry.occurred_at"
            >{{
              new Date(entry.occurred_at).toLocaleTimeString([], {
                hour: '2-digit',
                minute: '2-digit',
              })
            }}
            ·
          </time>
          {{
            entry.reason === 'automatic'
              ? 'Автоматически'
              : entry.reason === 'manual'
                ? 'Вручную'
                : 'Перенесено из старой истории'
          }}
          <span v-if="entry.actor_name"> · {{ entry.actor_name }}</span>
          <span v-if="entry.reason === 'automatic' && entry.rule_days">
            · срок {{ entry.rule_days }} дн.</span
          >
        </p>
        <p v-if="!entry.is_archived" class="is-size-7 mt-1">Сейчас вне архива</p>
      </div>
      <BaseButton
        v-if="entry.is_archived && (entry.entity_type === 'card' || admin)"
        tone="quiet"
        :disabled="busy"
        @click="emit('restore', entry)"
        >Восстановить</BaseButton
      >
    </div>
    <template v-if="entry.child_count">
      <button class="button is-small mt-3" :aria-expanded="expanded" @click="toggle">
        {{ expanded ? 'Скрыть' : 'Показать' }} карточки ({{ entry.child_count }})
      </button>
      <div v-if="expanded" class="mt-4" :aria-busy="loading">
        <ArchiveEntry
          v-for="child in children"
          :key="child.id"
          :entry="child"
          :return-to="returnTo"
          :admin="admin"
          :busy="busy"
          @restore="emit('restore', $event)"
        />
        <p v-if="error" role="alert">{{ error }}</p>
        <BaseButton v-if="cursor || error" tone="quiet" :disabled="loading" @click="loadMore">{{
          error ? 'Повторить' : 'Ещё карточки'
        }}</BaseButton>
      </div>
    </template>
  </article>
</template>
<style scoped>
.archive-entry-main {
  display: flex;
  gap: 1rem;
  justify-content: space-between;
  align-items: flex-start;
}
.archive-entry-description {
  min-width: 0;
  overflow-wrap: anywhere;
}
@media (max-width: 768px) {
  .archive-entry-main {
    flex-direction: column;
  }
}
</style>
