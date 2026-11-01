<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { tokensApi, type ApiToken, type IssuedToken } from '../api'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseField from '../components/ui/BaseField.vue'
import BaseDialog from '../components/ui/BaseDialog.vue'
import { useToastStore } from '../stores/toasts'
const items = ref<ApiToken[]>([]),
  name = ref(''),
  read = ref(true),
  write = ref(false),
  expires = ref(''),
  issued = ref<IssuedToken | null>(null),
  toasts = useToastStore(),
  mcpUrl = computed(() => `${location.origin}/mcp`)
async function load() {
  items.value = (await tokensApi.list()).items
}
onMounted(load)
async function create() {
  const scopes = []
  if (read.value) scopes.push('cards:read')
  if (write.value) scopes.push('cards:write')
  issued.value = await tokensApi.create({
    name: name.value,
    scopes,
    expires_at: expires.value ? new Date(expires.value).toISOString() : null,
  })
  name.value = ''
  await load()
}
function closeSecret() {
  issued.value = null
}
async function copy() {
  if (!issued.value) return
  await navigator.clipboard.writeText(issued.value.token)
  toasts.show('Токен скопирован')
}
async function revoke(item: ApiToken) {
  if (!confirm(`Отозвать токен «${item.name}»?`)) return
  await tokensApi.revoke(item.id)
  await load()
}
function status(item: ApiToken) {
  if (item.revoked_at) return 'отозван'
  if (item.expires_at && new Date(item.expires_at) < new Date()) return 'истёк'
  return 'активен'
}
</script>
<template>
  <section class="section">
    <div class="container">
      <header class="mb-5">
        <h1 class="title">API-токены</h1>
        <p class="subtitle is-6">Персональный доступ для агентов и автоматизации.</p>
      </header>
      <div class="columns is-variable is-5">
        <div class="column">
          <form class="box" @submit.prevent="create">
            <h2 class="title is-4">Новый токен</h2>
            <BaseField label="Название">
              <input v-model.trim="name" class="input" required placeholder="Codex" />
            </BaseField>
            <fieldset class="field">
              <legend class="label">Права</legend>
              <label class="checkbox mr-4">
                <input v-model="read" type="checkbox" /> Чтение карточек
              </label>
              <label class="checkbox">
                <input v-model="write" type="checkbox" /> Изменение карточек
              </label>
            </fieldset>
            <BaseField label="Срок действия" hint="Необязательно">
              <input v-model="expires" class="input" type="datetime-local" />
            </BaseField>
            <BaseButton type="submit" :disabled="!read && !write">Выпустить токен</BaseButton>
          </form>
        </div>
        <div class="column">
          <aside class="box">
            <h2 class="title is-4">Remote MCP</h2>
            <div class="content">
              <p>Endpoint:</p>
              <pre><code>{{ mcpUrl }}</code></pre>
              <p>Передайте токен заголовком:</p>
              <pre><code>Authorization: Bearer jkb_pat_…</code></pre>
              <p class="has-text-grey">Не добавляйте токен в URL или репозиторий.</p>
            </div>
          </aside>
        </div>
      </div>
      <div class="box">
        <h2 class="title is-4">Выданные токены</h2>
        <article v-for="item in items" :key="item.id" class="media">
          <div class="media-content">
            <div class="content">
              <p>
                <strong>{{ item.name }}</strong>
                <br />
                <code>{{ item.token_prefix }}…</code>
                <br />
                <small>
                  {{ item.scopes.join(', ') }} · {{ status(item) }}
                  <template v-if="item.last_used_at">
                    · использован {{ new Date(item.last_used_at).toLocaleString() }}</template
                  >
                  <template v-if="item.expires_at">
                    · до {{ new Date(item.expires_at).toLocaleString() }}</template
                  >
                </small>
              </p>
            </div>
          </div>
          <div class="media-right">
            <BaseButton v-if="!item.revoked_at" tone="danger" @click="revoke(item)">
              Отозвать
            </BaseButton>
          </div>
        </article>
        <p v-if="!items.length" class="has-text-grey">Токенов пока нет.</p>
      </div>
      <BaseDialog :open="Boolean(issued)" title="Токен выпущен" @close="closeSecret">
        <div class="content">
          <p>
            <strong>Скопируйте токен сейчас.</strong> После закрытия он больше не будет показан.
          </p>
          <pre><code>{{ issued?.token }}</code></pre>
        </div>
        <template #actions>
          <BaseButton tone="quiet" @click="copy">Копировать</BaseButton>
          <BaseButton @click="closeSecret">Готово</BaseButton>
        </template>
      </BaseDialog>
    </div>
  </section>
</template>
