import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  base: '/app/docker-updater/',
  plugins: [
    vue(),
    tailwindcss()
  ],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/app/docker-updater/api': {
        target: 'http://localhost:9090',
        changeOrigin: true
      }
    }
  }
})
