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
  <section class="page tokens-page">
    <header>
      <div>
        <h1>API-токены</h1>
        <p class="muted">Персональный доступ для агентов и автоматизации.</p>
      </div>
    </header>
    <div class="token-grid">
      <form class="panel" @submit.prevent="create">
        <h2>Новый токен</h2>
        <BaseField label="Название"
          ><input v-model.trim="name" required placeholder="Codex"
        /></BaseField>
        <fieldset>
          <legend>Права</legend>
          <label class="check"><input v-model="read" type="checkbox" />Чтение карточек</label
          ><label class="check"><input v-model="write" type="checkbox" />Изменение карточек</label>
        </fieldset>
        <BaseField label="Срок действия" hint="Необязательно"
          ><input v-model="expires" type="datetime-local" /></BaseField
        ><BaseButton type="submit" :disabled="!read && !write">Выпустить токен</BaseButton>
      </form>
      <aside class="panel mcp-help">
        <h2>Remote MCP</h2>
        <p>Endpoint:</p>
        <code>{{ mcpUrl }}</code>
        <p>Передайте токен заголовком:</p>
        <code>Authorization: Bearer jkb_pat_…</code>
        <p class="muted">Не добавляйте токен в URL или репозиторий.</p>
      </aside>
    </div>
    <div class="panel token-list">
      <h2>Выданные токены</h2>
      <article v-for="item in items" :key="item.id">
        <div>
          <strong>{{ item.name }}</strong
          ><code>{{ item.token_prefix }}…</code
          ><small
            >{{ item.scopes.join(', ') }} · {{ status(item)
            }}<template v-if="item.last_used_at">
              · использован {{ new Date(item.last_used_at).toLocaleString() }}</template
            ><template v-if="item.expires_at">
              · до {{ new Date(item.expires_at).toLocaleString() }}</template
            ></small
          >
        </div>
        <BaseButton v-if="!item.revoked_at" tone="danger" @click="revoke(item)"
          >Отозвать</BaseButton
        >
      </article>
      <p v-if="!items.length" class="muted">Токенов пока нет.</p>
    </div>
    <BaseDialog :open="Boolean(issued)" title="Токен выпущен" @close="closeSecret"
      ><p><strong>Скопируйте токен сейчас.</strong> После закрытия он больше не будет показан.</p>
      <pre class="secret">{{ issued?.token }}</pre>
      <template #actions
        ><BaseButton tone="quiet" @click="copy">Копировать</BaseButton
        ><BaseButton @click="closeSecret">Готово</BaseButton></template
      ></BaseDialog
    >
  </section>
</template>
