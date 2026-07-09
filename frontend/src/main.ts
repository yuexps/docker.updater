import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'

const mountApp = () => {
  const app = createApp(App)
  app.use(router)
  app.mount('#app')
}

if (import.meta.env.DEV) {
  import('./utils/test').then(() => {
    mountApp()
  })
} else {
  mountApp()
}
