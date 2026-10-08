import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { publicShell } from './pwa-build.ts'

export default defineConfig({
  plugins: [react(), tailwindcss(), publicShell()],
  server: {
    host: '127.0.0.1',
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/ready': 'http://127.0.0.1:8080',
      '/health': 'http://127.0.0.1:8080',
    },
  },
  build: { outDir: '../build/web', emptyOutDir: true },
})
