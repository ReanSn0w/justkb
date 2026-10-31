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
  <section class="attachments">
    <h2>Вложения</h2>
    <label class="file-picker"
      ><span>Выбрать файл</span
      ><input
        type="file"
        :disabled="busy"
        @change="upload(($event.target as HTMLInputElement).files?.[0])" /></label
    ><progress v-if="busy" :value="progress" max="100">{{ progress }}%</progress>
    <p v-if="error" class="field__error" role="alert">
      {{ error }} <button v-if="lastFile" @click="upload(lastFile)">Повторить</button>
    </p>
    <ul>
      <li v-for="item in attachments" :key="item.id">
        <div>
          <a :href="cardsApi.attachmentUrl(item.id)">{{ item.original_name }}</a
          ><small
            >{{ item.media_type }} · {{ size(item.size) }} ·
            {{ item.author?.display_name || 'пользователь' }}</small
          >
        </div>
        <BaseButton tone="quiet" @click="remove(item)">Удалить</BaseButton>
      </li>
    </ul>
    <p v-if="!attachments.length" class="muted">Файлов пока нет.</p>
  </section>
</template>
