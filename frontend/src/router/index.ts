import { createRouter, createWebHistory, RouteRecordRaw } from 'vue-router'
import Dashboard from '../views/Dashboard.vue'
import Containers from '../views/Containers.vue'
import Images from '../views/Images.vue'
import History from '../views/History.vue'
import Settings from '../views/Settings.vue'
import Tasks from '../views/Tasks.vue'
import Logs from '../views/Logs.vue'

const routes: Array<RouteRecordRaw> = [
  {
    path: '/',
    name: 'Dashboard',
    component: Dashboard
  },
  {
    path: '/containers',
    name: 'Containers',
    component: Containers
  },
  {
    path: '/images',
    name: 'Images',
    component: Images
  },
  {
    path: '/history',
    name: 'History',
    component: History
  },
  {
    path: '/tasks',
    name: 'Tasks',
    component: Tasks
  },
  {
    path: '/logs',
    name: 'Logs',
    component: Logs
  },
  {
    path: '/settings',
    name: 'Settings',
    component: Settings
  }
]

const router = createRouter({
  history: createWebHistory('/app/docker-updater/'),
  routes
})

export default router
