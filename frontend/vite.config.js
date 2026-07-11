import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Dev server proxies /api to the local FastAPI backend (run on port 8010).
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8010', changeOrigin: true, ws: true },
      '/uploads': { target: 'http://127.0.0.1:8010', changeOrigin: true },
    },
  },
})
