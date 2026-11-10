<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { archiveApi, cardsApi, epicsApi, type ArchiveEvent, type Epic } from '../api'
import { archiveError, archiveFilters } from '../features/archive'
import { useSessionStore } from '../stores/session'
import { useBoardStore } from '../stores/board'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
import ArchiveEntry from '../components/archive/ArchiveEntry.vue'
import RestoreProjectDialog from '../components/archive/RestoreProjectDialog.vue'
const route = useRoute(),
  router = useRouter(),
  session = useSessionStore(),
  board = useBoardStore()
const items = ref<ArchiveEvent[]>([]),
  projects = ref<Epic[]>([]),
  cursor = ref<string | null>(null)
const loading = ref(false),
  busy = ref(false),
  error = ref(''),
  notice = ref('')
const selected = ref<Epic | null>(null)
const form = ref(archiveFilters(route.query))
let sequence = 0
const groups = computed(() => {
  const grouped = new Map<string, ArchiveEvent[]>()
  for (const item of items.value) {
    const day = item.occurred_at
      ? new Date(item.occurred_at).toLocaleDateString(undefined, {
          year: 'numeric',
          month: 'long',
          day: 'numeric',
        })
      : 'Дата архивации неизвестна'
    const list = grouped.get(day) || []
    list.push(item)
    grouped.set(day, list)
  }
  return [...grouped].map(([day, entries]) => ({ day, entries }))
})
async function load(more = false) {
  const request = ++sequence
  loading.value = true
  error.value = ''
  if (!more) {
    items.value = []
    cursor.value = null
  }
  try {
    const page = await archiveApi.list({
      ...archiveFilters(route.query),
      cursor: more ? cursor.value || undefined : undefined,
    })
    if (request !== sequence) return
    items.value = more ? [...items.value, ...page.items] : page.items
    cursor.value = page.next_cursor || null
  } catch {
    if (request === sequence) error.value = 'Не удалось загрузить архив'
  } finally {
    if (request === sequence) loading.value = false
  }
}
watch(
  () => route.fullPath,
  () => {
    form.value = archiveFilters(route.query)
    void load()
  },
  { immediate: true },
)
onMounted(async () => {
  try {
    projects.value = (await epicsApi.list(true)).items
  } catch {
    error.value = 'Не удалось загрузить список проектов. Поиск по архиву доступен.'
  }
})
async function apply() {
  const query: Record<string, string> = {}
  for (const [key, value] of Object.entries(form.value))
    if (value !== undefined && value !== '' && value !== false) query[key] = String(value)
  await router.replace({ path: '/archive', query })
}
async function restore(entry: ArchiveEvent) {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    if (entry.entity_type === 'project') {
      selected.value = await epicsApi.get(entry.entity_id)
      return
    }
    await cardsApi.restore(entry.entity_id, entry.version)
    await restored()
  } catch (cause) {
    error.value = archiveError(cause)
  } finally {
    busy.value = false
  }
}
async function restored() {
  selected.value = null
  try {
    projects.value = (await epicsApi.list(true)).items
  } catch {
    /* Main archive reload remains available. */
  }
  await board.initialize()
  await load()
  notice.value = 'Восстановлено. Срок автоархивации начинается заново.'
}
</script>
<template>
  <section class="section">
    <div class="container">
      <h1 class="title">Архив</h1>
      <p class="subtitle">История карточек и проектов. Всё нужное можно вернуть на доску.</p>
      <form class="box" @submit.prevent="apply">
        <div class="columns is-multiline">
          <div class="column is-3">
            <BaseField label="Тип" for-id="archive-type"
              ><div class="select is-fullwidth">
                <select id="archive-type" v-model="form.type">
                  <option value="">Все</option>
                  <option value="card">Карточки</option>
                  <option value="project">Проекты</option>
                </select>
              </div></BaseField
            >
          </div>
          <div class="column">
            <BaseField label="Поиск" for-id="archive-query"
              ><input
                id="archive-query"
                v-model="form.q"
                class="input"
                type="search"
                placeholder="Название или номер карточки"
            /></BaseField>
          </div>
          <div class="column is-4">
            <BaseField label="Проект" for-id="archive-project"
              ><div class="select is-fullwidth">
                <select id="archive-project" v-model="form.project_id">
                  <option value="">Все проекты</option>
                  <option v-for="project in projects" :key="project.id" :value="project.id">
                    {{ project.name }}{{ project.is_archived ? ' (в архиве)' : '' }}
                  </option>
                </select>
              </div></BaseField
            >
          </div>
        </div>
        <div class="archive-filter-actions">
          <label class="checkbox"
            ><input v-model="form.history" type="checkbox" /> Вся история, включая
            восстановления</label
          ><BaseButton type="submit">Применить</BaseButton>
        </div>
      </form>
      <p v-if="notice" class="notification is-success" role="status">{{ notice }}</p>
      <div v-if="error" class="notification is-danger" role="alert">
        {{ error }}
        <BaseButton tone="quiet" :disabled="loading" @click="load()">Обновить данные</BaseButton>
      </div>
      <p v-if="loading" role="status">Загружаем архив…</p>
      <p v-else-if="!items.length && !error" class="box">
        В архиве пока нет записей по этим условиям.
      </p>
      <div :aria-busy="loading" class="archive-timeline">
        <section v-for="group in groups" :key="group.day" class="mb-5">
          <h2 class="title is-5">{{ group.day }}</h2>
          <ArchiveEntry
            v-for="entry in group.entries"
            :key="entry.id"
            :entry="entry"
            :return-to="route.fullPath"
            :admin="session.user?.role === 'admin'"
            :busy="busy"
            @restore="restore"
          />
        </section>
      </div>
      <BaseButton v-if="cursor" tone="quiet" :disabled="loading" @click="load(true)"
        >Показать ещё</BaseButton
      >
      <RestoreProjectDialog :project="selected" @close="selected = null" @restored="restored" />
    </div>
  </section>
</template>
<style scoped>
.archive-filter-actions {
  display: flex;
  gap: 1rem;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
}
.archive-timeline {
  border-inline-start: 2px solid var(--bulma-border);
  padding-inline-start: 1rem;
}
</style>
