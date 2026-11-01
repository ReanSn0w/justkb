import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'
import { loadSystemName } from './system'
import 'bulma/css/bulma.min.css'

async function bootstrap() {
  await loadSystemName()
  createApp(App).use(createPinia()).use(router).mount('#app')
}

void bootstrap()
