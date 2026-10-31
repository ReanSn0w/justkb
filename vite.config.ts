import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: { outDir: 'internal/frontend/dist', emptyOutDir: true },
  server: { proxy: { '/api': 'http://localhost:8080', '/mcp': 'http://localhost:8080' } },
  test: { environment: 'jsdom', setupFiles: ['./src/test/setup.ts'], css: true },
})
