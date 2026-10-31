import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useToastStore = defineStore('toasts', () => {
  const items = ref<{ id: number; message: string; tone: 'info' | 'error' }[]>([])
  let id = 0
  function show(message: string, tone: 'info' | 'error' = 'info') {
    const item = { id: ++id, message, tone }
    items.value.push(item)
    window.setTimeout(() => remove(item.id), 4500)
  }
  function remove(value: number) {
    items.value = items.value.filter((item) => item.id !== value)
  }
  return { items, show, remove }
})
