<script setup lang="ts">
import { ref } from 'vue'
import { cardsApi, type Attachment } from '../../api'
import BaseButton from '../ui/BaseButton.vue'
import { useToastStore } from '../../stores/toasts'
const props = defineProps<{ cardId: string; attachments: Attachment[] }>()
const emit = defineEmits<{ change: [items: Attachment[]] }>()
const progress = ref(0),
  busy = ref(false),
  lastFile = ref<File | null>(null),
  error = ref(''),
  toasts = useToastStore()
const maxBytes = Number(import.meta.env.VITE_MAX_UPLOAD_BYTES || 25 * 1024 * 1024)
async function upload(file?: File) {
  if (!file) return
  lastFile.value = file
  if (file.size > maxBytes) {
    error.value = `Файл больше ${Math.round(maxBytes / 1024 / 1024)} МБ`
    return
  }
  busy.value = true
  error.value = ''
  progress.value = 0
  try {
    const item = await cardsApi.upload(
      props.cardId,
      file,
      (loaded, total) => (progress.value = total ? Math.round((loaded / total) * 100) : 0),
    )
    emit('change', [...props.attachments, item])
    lastFile.value = null
    toasts.show('Файл загружен')
  } catch {
    error.value = 'Не удалось загрузить файл'
  } finally {
    busy.value = false
  }
}
async function remove(item: Attachment) {
  if (!confirm(`Удалить ${item.original_name}?`)) return
  await cardsApi.deleteAttachment(item.id)
  emit(
    'change',
    props.attachments.filter((x) => x.id !== item.id),
  )
}
function size(value: number) {
  return value < 1024 * 1024
    ? `${Math.ceil(value / 1024)} КБ`
    : `${(value / 1024 / 1024).toFixed(1)} МБ`
}
</script>
<template>
  <section>
    <h2 class="title is-5">Вложения</h2>
    <div class="file has-name is-fullwidth">
      <label class="file-label">
        <input
          class="file-input"
          type="file"
          :disabled="busy"
          @change="upload(($event.target as HTMLInputElement).files?.[0])"
        />
        <span class="file-cta"><span class="file-label">Выбрать файл</span></span>
        <span class="file-name">{{ lastFile?.name || 'Файл не выбран' }}</span>
      </label>
    </div>
    <progress v-if="busy" class="progress is-primary mt-4" :value="progress" max="100">
      {{ progress }}%
    </progress>
    <div v-if="error" class="notification is-danger is-light mt-4" role="alert">
      {{ error }}
      <button v-if="lastFile" class="button is-danger is-light is-small" @click="upload(lastFile)">
        Повторить
      </button>
    </div>
    <div class="mt-5">
      <article v-for="item in attachments" :key="item.id" class="media">
        <div class="media-content">
          <div class="content">
            <p>
              <a :href="cardsApi.attachmentUrl(item.id)">{{ item.original_name }}</a>
              <br />
              <small
                >{{ item.media_type }} · {{ size(item.size) }} ·
                {{ item.author?.display_name || 'пользователь' }}</small
              >
            </p>
          </div>
        </div>
        <div class="media-right">
          <BaseButton tone="quiet" @click="remove(item)">Удалить</BaseButton>
        </div>
      </article>
    </div>
    <p v-if="!attachments.length" class="has-text-grey mt-4">Файлов пока нет.</p>
  </section>
</template>
