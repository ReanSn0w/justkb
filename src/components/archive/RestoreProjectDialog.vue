<script setup lang="ts">
import { ref, watch } from 'vue'
import { archiveApi, epicsApi, type Epic } from '../../api'
import { archiveError } from '../../features/archive'
import BaseDialog from '../ui/BaseDialog.vue'
import BaseButton from '../ui/BaseButton.vue'
const props = defineProps<{ project: Epic | null }>()
const emit = defineEmits<{ close: []; restored: [Epic] }>()
const mode = ref<'project_only' | 'with_cards'>('with_cards')
const busy = ref(false),
  error = ref('')
const current = ref<Epic | null>(null)
watch(
  () => props.project,
  (value) => {
    current.value = value
    error.value = ''
    mode.value = 'with_cards'
  },
  { immediate: true },
)
async function restore() {
  if (!current.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    const restored = await archiveApi.restoreProject(
      current.value.id,
      current.value.version,
      mode.value,
    )
    emit('restored', restored)
  } catch (cause) {
    error.value = archiveError(cause)
  } finally {
    busy.value = false
  }
}
async function reload() {
  if (!current.value) return
  busy.value = true
  try {
    current.value = await epicsApi.get(current.value.id)
    error.value = ''
  } catch (cause) {
    error.value = archiveError(cause)
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <BaseDialog :open="!!project" title="Восстановить проект" @close="!busy && emit('close')">
    <template v-if="current">
      <p class="mb-4">{{ current.name }}</p>
      <fieldset :disabled="busy">
        <label class="radio is-block mb-3"
          ><input v-model="mode" type="radio" value="project_only" name="restore-project-mode" />
          Только проект</label
        >
        <label class="radio is-block"
          ><input v-model="mode" type="radio" value="with_cards" name="restore-project-mode" />
          Проект и карточки, архивированные вместе с ним ({{ current.restorable_cards }})</label
        >
      </fieldset>
      <p class="help mt-4">
        Ранее отдельно архивированные карточки останутся в архиве. Срок автоархивации начнётся
        заново.
      </p>
      <p v-if="error" class="notification is-danger mt-4" role="alert">{{ error }}</p>
    </template>
    <template #actions>
      <BaseButton tone="quiet" :disabled="busy" @click="emit('close')">Отмена</BaseButton>
      <BaseButton v-if="error" tone="quiet" :disabled="busy" @click="reload"
        >Обновить данные</BaseButton
      >
      <BaseButton :disabled="busy || !current?.is_archived" @click="restore"
        >Восстановить</BaseButton
      >
    </template>
  </BaseDialog>
</template>
