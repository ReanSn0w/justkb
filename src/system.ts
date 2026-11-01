import { readonly, ref } from 'vue'
import { systemApi } from './api'

const name = ref('JKB')

export const systemName = readonly(name)

export async function loadSystemName() {
  try {
    const system = await systemApi.get()
    if (system.name.trim()) name.value = system.name.trim()
  } catch {
    // Keep the built-in name when the runtime configuration cannot be loaded.
  }
  document.title = name.value
}
