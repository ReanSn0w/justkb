<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { useRoute, useRouter } from 'vue-router'
import { useBoardStore } from '../../stores/board'
import { useToastStore } from '../../stores/toasts'
import { mergeFilterQuery } from '../../features/filters'
import { ApiError, type Card } from '../../api'
import UserAvatar from '../user/UserAvatar.vue'
import ColorTag from '../ui/ColorTag.vue'
const props = defineProps<{ card: Card }>()
const route = useRoute()
const router = useRouter()
const board = useBoardStore()
const toasts = useToastStore()
const tile = ref<HTMLElement | null>(null)
const assigneeButton = ref<HTMLButtonElement | null>(null)
const assigneeOptions = ref<HTMLElement | null>(null)
const choosingAssignee = ref(false)
const savingAssignee = ref(false)

onClickOutside(tile, () => (choosingAssignee.value = false))

function dismissAssigneeOnTileClick({ target }: MouseEvent) {
  if (!(target instanceof Node)) return
  if (assigneeButton.value?.contains(target) || assigneeOptions.value?.contains(target)) return
  choosingAssignee.value = false
}

function closeAssigneePicker() {
  if (!choosingAssignee.value) return
  choosingAssignee.value = false
  assigneeButton.value?.focus()
}

async function toggleAssigneePicker() {
  if (savingAssignee.value) return
  if (choosingAssignee.value) {
    closeAssigneePicker()
    return
  }
  choosingAssignee.value = true
  await nextTick()
  assigneeOptions.value?.querySelector<HTMLButtonElement>('[data-assignee-option]')?.focus()
}

async function assign(assigneeID: string | null) {
  if (savingAssignee.value) return
  choosingAssignee.value = false
  assigneeButton.value?.focus()
  if (assigneeID === (props.card.assignee_id ?? null)) return
  savingAssignee.value = true
  try {
    await board.update(props.card.id, props.card.version, { assignee_id: assigneeID })
  } catch (cause) {
    if (cause instanceof ApiError && cause.code === 'card_version_conflict') {
      const refreshed = await board.loadCards()
      toasts.show(
        refreshed
          ? 'Карточка изменилась. Доска обновлена.'
          : 'Карточка изменилась. Не удалось обновить доску.',
        'error',
      )
    } else {
      toasts.show('Не удалось изменить исполнителя', 'error')
    }
  } finally {
    savingAssignee.value = false
  }
}

function cardURL(id: string) {
  const returnTo = router.resolve({
    path: '/',
    query: mergeFilterQuery(route.query, board.filters),
  }).fullPath
  return { path: `/cards/${id}`, query: { return_to: returnTo } }
}
defineEmits<{ move: [direction: -1 | 1]; editDescription: [trigger: HTMLButtonElement] }>()
</script>
<template>
  <article
    ref="tile"
    class="card mb-4"
    tabindex="0"
    :aria-label="`JKB-${card.number}: ${card.title}`"
    @click="dismissAssigneeOnTileClick"
  >
    <div class="card-content p-4">
      <div class="is-flex is-justify-content-space-between is-align-items-flex-start">
        <RouterLink class="card-title-link" :to="cardURL(card.id)">
          <p class="is-size-7 has-text-grey mb-2">JKB-{{ card.number }}</p>
          <p :data-transition-key="`card-${card.id}`" class="has-text-weight-semibold">
            {{ card.title }}
          </p>
        </RouterLink>
        <button
          type="button"
          class="button is-small is-flex-shrink-0 ml-2"
          data-edit-description
          aria-label="Редактировать описание"
          aria-haspopup="dialog"
          @click="$emit('editDescription', $event.currentTarget as HTMLButtonElement)"
        >
          <svg
            viewBox="0 0 24 24"
            width="16"
            height="16"
            aria-hidden="true"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="m14 6 4 4M4 20l4.5-1 11-11a2.8 2.8 0 0 0-4-4l-11 11z" />
          </svg>
        </button>
      </div>
      <div class="tags mt-4 mb-2" data-card-assignee @keydown.esc.stop="closeAssigneePicker">
        <span v-if="card.epic" class="tag is-link">{{ card.epic.name }}</span>
        <ColorTag v-if="card.group" :color="card.group.color">{{ card.group.name }}</ColorTag>
        <span :class="{ 'tag is-info assignee-badge': card.assignee }">
          <UserAvatar
            v-if="card.assignee"
            as="span"
            :user="card.assignee"
            :size="24"
            class="is-flex-shrink-0"
            aria-hidden="true"
          />
          <button
            ref="assigneeButton"
            type="button"
            :class="card.assignee ? 'assignee-action assignee-name ml-1' : 'button is-small'"
            :aria-label="
              card.assignee
                ? `Сменить исполнителя: ${card.assignee.display_name}`
                : 'Назначить исполнителя'
            "
            :aria-expanded="choosingAssignee"
            :aria-controls="choosingAssignee ? `assignees-${card.id}` : undefined"
            :aria-disabled="savingAssignee"
            @click="toggleAssigneePicker"
          >
            <template v-if="card.assignee">{{ card.assignee.display_name }}</template>
            <span v-else aria-hidden="true">+</span>
          </button>
          <button
            v-if="card.assignee"
            type="button"
            class="assignee-action assignee-remove"
            aria-label="Снять исполнителя"
            :aria-disabled="savingAssignee"
            @click="assign(null)"
          >
            <span aria-hidden="true">×</span>
          </button>
        </span>
      </div>
      <div
        v-if="choosingAssignee"
        :id="`assignees-${card.id}`"
        ref="assigneeOptions"
        data-card-assignee
        class="assignee-options box p-1 mb-2"
        @keydown.esc.stop="closeAssigneePicker"
      >
        <p v-if="!board.users.length" class="is-size-7 p-2">Нет доступных исполнителей</p>
        <button
          v-for="user in board.users"
          :key="user.id"
          type="button"
          data-assignee-option
          class="button is-small is-fullwidth is-justify-content-flex-start"
          @click="assign(user.id)"
        >
          <span class="assignee-name">{{ user.display_name }}</span>
        </button>
      </div>
      <div v-if="card.tags.length" class="tags mb-3">
        <ColorTag v-for="tag in card.tags" :key="tag.id" :color="tag.color">
          {{ tag.name }}
        </ColorTag>
      </div>
      <div class="buttons are-small is-right mb-0">
        <button
          class="button"
          aria-label="Переместить в предыдущую колонку"
          @click="$emit('move', -1)"
        >
          ←
        </button>
        <button
          class="button"
          aria-label="Переместить в следующую колонку"
          @click="$emit('move', 1)"
        >
          →
        </button>
      </div>
    </div>
  </article>
</template>

<style scoped>
.card-title-link {
  min-width: 0;
  overflow-wrap: anywhere;
}

.assignee-badge {
  max-width: 100%;
}

.assignee-action {
  appearance: none;
  background: none;
  border: 0;
  color: inherit;
  cursor: pointer;
  font: inherit;
  padding: 0;
}

.assignee-remove {
  color: var(--bulma-danger-25);
  flex-shrink: 0;
  font-size: 1rem;
  font-weight: 700;
  width: 1.5rem;
  height: 100%;
}

.assignee-name {
  overflow: hidden;
  text-overflow: ellipsis;
}

.assignee-options {
  max-height: 12rem;
  overflow-y: auto;
}
</style>
