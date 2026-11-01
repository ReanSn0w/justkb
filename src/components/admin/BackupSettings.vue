<script setup lang="ts">
import { ref } from 'vue'
import { adminApi, ApiError } from '../../api'
import { useToastStore } from '../../stores/toasts'
import BaseButton from '../ui/BaseButton.vue'
import BaseField from '../ui/BaseField.vue'

const file = ref<File | null>(null)
const confirmation = ref('')
const busy = ref(false)
const progress = ref(0)
const error = ref('')
const restored = ref(false)
const toasts = useToastStore()

async function createBackup() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    const result = await adminApi.createBackup()
    const url = URL.createObjectURL(result.blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = result.filename
    anchor.click()
    URL.revokeObjectURL(url)
    toasts.show('Резервная копия создана')
  } catch {
    error.value = 'Не удалось создать резервную копию'
  } finally {
    busy.value = false
  }
}

function selectFile(event: Event) {
  file.value = (event.target as HTMLInputElement).files?.[0] || null
  confirmation.value = ''
  restored.value = false
  error.value = ''
}

async function restoreBackup() {
  if (!file.value || confirmation.value !== 'RESTORE' || busy.value) return
  if (!confirm('Заменить текущую базу и все локальные файлы данными из резервной копии?')) return
  busy.value = true
  progress.value = 0
  error.value = ''
  restored.value = false
  try {
    await adminApi.restoreBackup(file.value, (loaded, total) => {
      progress.value = total > 0 ? Math.round((loaded / total) * 100) : 0
    })
    restored.value = true
    file.value = null
    confirmation.value = ''
  } catch (cause) {
    error.value =
      cause instanceof ApiError && cause.code === 'backup_schema_mismatch'
        ? 'Версия схемы резервной копии не совпадает с текущей системой'
        : cause instanceof ApiError && cause.code === 'invalid_backup'
          ? 'Файл не является корректной резервной копией системы'
          : 'Восстановление не выполнено. Проверьте состояние сервиса и журналы перед повторной попыткой.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="error" class="notification is-danger" role="alert">{{ error }}</div>
  <div v-if="restored" class="notification is-success" role="status">
    Система восстановлена. Текущая сессия могла измениться —
    <a href="/">перезагрузите приложение</a>.
  </div>
  <div class="columns is-variable is-5">
    <div class="column">
      <section class="box">
        <h2 class="title is-4">Создать резервную копию</h2>
        <div class="content">
          <p>
            Архив содержит данные PostgreSQL, вложения и аватары. На время создания система
            ненадолго приостанавливает остальные запросы, чтобы snapshot оставался согласованным.
          </p>
        </div>
        <BaseButton :disabled="busy" @click="createBackup">
          {{ busy ? 'Подготовка…' : 'Скачать резервную копию' }}
        </BaseButton>
      </section>
    </div>
    <div class="column">
      <section class="box">
        <h2 class="title is-4">Восстановить систему</h2>
        <div class="notification is-danger">
          Восстановление заменит всех пользователей, задачи, настройки и локальные файлы. Архив
          должен быть создан той же версией схемы системы.
        </div>
        <div class="file has-name is-fullwidth mb-4">
          <label class="file-label">
            <input
              class="file-input"
              type="file"
              accept=".tar.gz,application/gzip"
              @change="selectFile"
            />
            <span class="file-cta"><span class="file-label">Выбрать backup</span></span>
            <span class="file-name">{{ file?.name || 'Файл не выбран' }}</span>
          </label>
        </div>
        <BaseField label="Подтверждение" hint="Введите RESTORE">
          <input v-model.trim="confirmation" class="input" autocomplete="off" />
        </BaseField>
        <progress v-if="busy && progress" class="progress is-danger" :value="progress" max="100">
          {{ progress }}%
        </progress>
        <BaseButton
          tone="danger"
          :disabled="!file || confirmation !== 'RESTORE' || busy"
          @click="restoreBackup"
        >
          Восстановить из backup
        </BaseButton>
      </section>
    </div>
  </div>
</template>
