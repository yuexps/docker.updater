import { defineStore } from 'pinia'
import wsService from '../utils/websocket'
import { getComposeProjectsApi, handleComposeLifecycleApi } from '../api/compose'

export interface ContainerItem {
  name: string
  image: string
  status: 'ok' | 'update' | 'deferred' | string
  defer_until: string | null
  checked_at: string
  compose_project: string | null
  running: boolean
  local_digest: string
  remote_digest: string
  local_version?: string
  remote_version?: string
  check_method?: 'RepoDigest' | 'ConfigDigest' | string
}

export interface ComposeContainerItem {
  name: string
  service: string
  image: string
  state: string
  has_update: boolean
  local_version: string
  remote_version: string
}

export interface ComposeProject {
  name: string
  working_dir: string
  config_files: string
  status: 'running' | 'partial' | 'stopped'
  services_count: number
  has_update: boolean
  containers: ComposeContainerItem[]
}

export interface TaskItem {
  container_name: string
  type: 'update'
  added_at: string
}

export const useDockerStore = defineStore('dockerStore', {
  state: () => ({
    containers: [] as ContainerItem[],
    composeProjects: [] as ComposeProject[],
    lastCheckTime: '' as string,
    history: [] as any[],
    activeTask: null as TaskItem | null,
    queuedTasks: [] as TaskItem[],
    loading: false as boolean,
    isWsConnected: false as boolean,
    isInitialized: false as boolean,
    lastComposeFetchTime: 0 as number,
  }),

  getters: {
    // 强制按容器名称字母升序排列
    sortedContainers(state): ContainerItem[] {
      return [...state.containers].sort((a, b) => a.name.localeCompare(b.name))
    },

    // 强制按 Compose 项目名称字母升序排列
    sortedComposeProjects(state): ComposeProject[] {
      return [...state.composeProjects].sort((a, b) => a.name.localeCompare(b.name))
    },

    // 存在可用升级的容器列表
    updateAvailableContainers(state): ContainerItem[] {
      return state.containers.filter(c => c.status === 'update')
    },

    // 待升级容器数量
    updateCount(): number {
      return this.updateAvailableContainers.length
    },

    // 暂挂中的容器数量
    deferredCount(state): number {
      return state.containers.filter(c => c.status === 'deferred').length
    }
  },

  actions: {
    // 初始化 WebSocket 状态订阅
    initWebSocket() {
      if (this.isInitialized) return
      this.isInitialized = true

      wsService.subscribeStatus((payload) => {
        if (!payload) return
        this.containers = payload.containers || []
        this.lastCheckTime = payload.last_check || ''
        this.history = payload.history || []
        this.activeTask = payload.active || null
        this.queuedTasks = payload.queued || []
        this.isWsConnected = wsService.isConnected

        // 状态推送时同步刷新 Compose 项目
        this.fetchComposeProjects(true)
      })
    },

    // 异步拉取 Compose 编排项目列表
    async fetchComposeProjects(silent = false) {
      const now = Date.now()
      if (silent && now - this.lastComposeFetchTime < 3000) {
        return
      }
      this.lastComposeFetchTime = now

      if (!silent && this.composeProjects.length === 0) {
        this.loading = true
      }
      try {
        const data = await getComposeProjectsApi()
        this.composeProjects = data || []
      } catch (e: any) {
        console.error('[Pinia Store] 拉取 Compose 项目失败:', e)
      } finally {
        this.loading = false
      }
    },

    // 发送 Compose 生命周期控制 (start/stop/restart)
    async handleComposeLifecycle(projectName: string, action: 'start' | 'stop' | 'restart') {
      await handleComposeLifecycleApi(projectName, action)
      await this.fetchComposeProjects(true)
    }
  }
})

