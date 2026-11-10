<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { archiveApi, directoryApi, ApiError } from '../../api'
import type { ArchivePolicy, ArchivePreview, BoardColumn } from '../../api/types'
import BaseButton from '../ui/BaseButton.vue'
import BaseField from '../ui/BaseField.vue'

const policy = ref<ArchivePolicy | null>(null)
const columns = ref<BoardColumn[]>([])
const preview = ref<ArchivePreview | null>(null)
const busy = ref(false)
const error = ref('')
const message = ref('')
watch(
  policy,
  () => {
    preview.value = null
    message.value = ''
  },
  { deep: true },
)
async function load() {
  busy.value = true
  error.value = ''
  try {
    const [saved, directory] = await Promise.all([archiveApi.policy(), directoryApi.columns()])
    policy.value = saved
    columns.value = directory.items
  } catch {
    error.value = 'Не удалось загрузить настройки архивации'
  } finally {
    busy.value = false
  }
}
onMounted(load)
async function check() {
  if (!policy.value) return
  busy.value = true
  error.value = ''
  preview.value = null
  try {
    preview.value = await archiveApi.preview(policy.value)
  } catch {
    error.value = 'Не удалось проверить правила. Выберите столбец и срок от 1 до 3650 дней.'
  } finally {
    busy.value = false
  }
}
async function save() {
  if (!policy.value || !preview.value) return
  busy.value = true
  error.value = ''
  try {
    policy.value = await archiveApi.savePolicy(policy.value)
    // The watcher invalidates preview after the saved version changes.
    await Promise.resolve()
    message.value = 'Правила сохранены. Проверка выполняется каждые 5 минут.'
  } catch (cause) {
    error.value =
      cause instanceof ApiError && cause.status === 409
        ? 'Настройки изменены другим администратором. Ваши значения сохранены в форме; загрузите актуальные настройки перед повторной правкой.'
        : 'Не удалось сохранить правила. Проверьте столбец и сроки.'
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <section class="box" aria-labelledby="archive-settings-title" :aria-busy="busy">
    <h2 id="archive-settings-title" class="title is-5">Архивация</h2>
    <p class="mb-5">
      Общие правила для всей доски. Архив хранит карточки и проекты, чтобы их можно было найти и
      восстановить.
    </p>
    <p v-if="error" class="notification is-danger" role="alert">{{ error }}</p>
    <p v-if="message" class="notification is-success" role="status">{{ message }}</p>
    <form v-if="policy" @submit.prevent="check">
      <fieldset :disabled="busy">
        <BaseField
          label="Столбец «Готово»"
          for-id="archive-done-column"
          hint="Определяет прогресс всех проектов и столбец автоархивации карточек. При смене столбца отсчёт начинается заново."
        >
          <div class="select is-fullwidth">
            <select id="archive-done-column" v-model="policy.done_column_id">
              <option :value="null">Не выбран</option>
              <option v-for="column in columns" :key="column.id" :value="column.id">
                {{ column.name }}
              </option>
            </select>
          </div>
        </BaseField>
        <div class="columns mt-4">
          <div class="column">
            <h3 class="title is-6">Карточки</h3>
            <label class="checkbox mb-3"
              ><input v-model="policy.cards_enabled" type="checkbox" /> Автоматически архивировать
              карточки</label
            >
            <BaseField label="Срок для карточек, дней" for-id="archive-card-days">
              <input
                id="archive-card-days"
                v-model.number="policy.cards_days"
                class="input"
                type="number"
                min="1"
                max="3650"
                step="1"
                required
              />
            </BaseField>
            <p>
              После {{ policy.cards_days }} дней непрерывного нахождения в выбранном столбце. Правки
              текста срок не меняют.
            </p>
          </div>
          <div class="column">
            <h3 class="title is-6">Проекты</h3>
            <label class="checkbox mb-3"
              ><input v-model="policy.projects_enabled" type="checkbox" /> Автоматически
              архивировать проекты</label
            >
            <BaseField label="Срок для проектов, дней" for-id="archive-project-days">
              <input
                id="archive-project-days"
                v-model.number="policy.projects_days"
                class="input"
                type="number"
                min="1"
                max="3650"
                step="1"
                required
              />
            </BaseField>
            <p>
              После {{ policy.projects_days }} дней при 100% готовности, вместе с оставшимися
              активными карточками. Пустые проекты не архивируются.
            </p>
          </div>
        </div>
        <p class="help mb-4">
          День — 24 часа. При включении правила или сокращении срока учитывается уже прошедшее
          время. После восстановления из архива срок начинается заново.
        </p>
        <div v-if="preview" class="notification is-info is-light" role="status">
          Сейчас подходят: карточек — {{ preview.cards }}, проектов — {{ preview.projects }}.
          Карточки этих проектов могут входить в указанное число. Фоновый обработчик повторно
          проверит условия.
        </div>
        <div class="buttons">
          <BaseButton type="submit" tone="quiet">Проверить правила</BaseButton>
          <BaseButton :disabled="!preview || busy" @click="save">Сохранить правила</BaseButton>
        </div>
      </fieldset>
    </form>
    <BaseButton v-if="error" tone="quiet" :disabled="busy" @click="load"
      >Загрузить сохранённые настройки</BaseButton
    >
  </section>
</template>
