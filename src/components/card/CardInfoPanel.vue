<script setup lang="ts">
import type { Card } from '../../api'
import UserAvatar from '../user/UserAvatar.vue'

defineProps<{ card: Card; columnName: string }>()
</script>

<template>
  <aside class="box" aria-labelledby="card-information-title">
    <h2 id="card-information-title" class="title is-5">Информация</h2>
    <dl>
      <div class="mb-4">
        <dt class="has-text-grey is-size-7 mb-1">Столбец</dt>
        <dd>
          <span class="tag">{{ columnName }}</span>
        </dd>
      </div>
      <div class="mb-4">
        <dt class="has-text-grey is-size-7 mb-1">Проект</dt>
        <dd>
          <RouterLink v-if="card.epic" class="tag is-link" :to="`/projects/${card.epic.id}`">
            {{ card.epic.name }}
          </RouterLink>
          <span v-else>Не указан</span>
        </dd>
      </div>
      <div class="mb-4">
        <dt class="has-text-grey is-size-7 mb-1">Группа</dt>
        <dd>
          <span v-if="card.group" class="tag is-primary">{{ card.group.name }}</span>
          <span v-else>Не указана</span>
        </dd>
      </div>
      <div class="mb-4">
        <dt class="has-text-grey is-size-7 mb-1">Исполнитель</dt>
        <dd>
          <span v-if="card.assignee" class="tag is-info">
            <UserAvatar :user="card.assignee" :size="24" />
            <span class="ml-1">{{ card.assignee.display_name }}</span>
          </span>
          <span v-else>Не назначен</span>
        </dd>
      </div>
      <div class="mb-4">
        <dt class="has-text-grey is-size-7 mb-1">Срок выполнения</dt>
        <dd>
          <time v-if="card.due_at" class="tag is-warning" :datetime="card.due_at">
            {{ new Date(card.due_at).toLocaleString() }}
          </time>
          <span v-else>Не указан</span>
        </dd>
      </div>
      <div>
        <dt class="has-text-grey is-size-7 mb-2">Теги</dt>
        <dd>
          <div v-if="card.tags.length" class="tags mb-0">
            <span v-for="tag in card.tags" :key="tag.id" class="tag is-info">
              {{ tag.name }}
            </span>
          </div>
          <span v-else>Нет тегов</span>
        </dd>
      </div>
    </dl>
  </aside>
</template>
