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
  },
  build: {
    chunkSizeWarningLimit: 1500,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules/naive-ui')) {
            return 'naive-ui'
          }
          if (id.includes('node_modules/vue-codemirror') || id.includes('node_modules/@codemirror')) {
            return 'codemirror'
          }
        }
      }
    }
  }
})
