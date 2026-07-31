import axios from 'axios'
import apiClient from '../api/client'

interface MockContainer {
  name: string
  image: string
  status: string
  defer_until: string | null
  checked_at: string
  compose_project: string | null
  running: boolean
  local_digest: string
  remote_digest: string
  local_version?: string
  remote_version?: string
  check_method?: 'RepoDigest' | 'ConfigDigest'
}

interface MockImage {
  id: string
  tags: string[]
  size: number
  created: number
  containers?: string[]
  architecture?: string
  version?: string
}

interface RegistryItem {
  id: number
  registry: string
  username: string
  password?: string
  updated_at: string
}

interface TaskItem {
  container_name: string
  type: 'update'
  added_at: string
}

class SimulationDatabase {
  containers: MockContainer[] = []
  images: MockImage[] = []
  settings = {
    temp_mirrors: ['https://docker.m.daocloud.io', 'https://mirror.baidubce.com'],
    dockerhub_mirrors: ['https://docker.m.daocloud.io'],
    ghcr_mirrors: [],
    check_type: 'day',
    check_value: 1,
    auto_update_enabled: false,
    notify_enabled: false,
    notify_type: 'email'
  }
  registries: RegistryItem[] = []
  history: any[] = []
  updateLogs = new Map<string, string[]>()
  activeTask: TaskItem | null = null
  queuedTasks: TaskItem[] = []
  lastCheckTime = '2026-07-09 12:00:00'
  upgradeMode: 'success' | 'fail_pull' | 'fail_start' = 'success'
  isWSConnected = true
  composeConfigs: Map<string, string> = new Map()

  constructor() {
    this.reset()
  }

  reset() {
    this.containers = [
      {
        name: 'nginx-app',
        image: 'nginx:alpine',
        status: 'update',
        defer_until: null,
        checked_at: '2026-07-09 10:30',
        compose_project: 'web-cluster',
        running: true,
        local_digest: 'sha256:4f082ec15d862f1c84f3c0eb51b14a601859c402130386cfb8109bfcf9446d3e',
        remote_digest: 'sha256:9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c3b2a1a0b9c8d7e6f5a4b3c2d1e0f9a8b',
        local_version: '1.25.3',
        remote_version: '1.25.4',
        check_method: 'RepoDigest'
      },
      {
        name: 'web-api',
        image: 'node:20-alpine',
        status: 'ok',
        defer_until: null,
        checked_at: '2026-07-09 10:35',
        compose_project: 'web-cluster',
        running: true,
        local_digest: 'sha256:7f082ec15d862f1c84f3c0eb51b14a601859c402130386cfb8109bfcf9446d3e',
        remote_digest: 'sha256:7f082ec15d862f1c84f3c0eb51b14a601859c402130386cfb8109bfcf9446d3e',
        local_version: '20.11.0',
        remote_version: '20.11.0',
        check_method: 'RepoDigest'
      },
      {
        name: 'postgres-db',
        image: 'postgres:15-alpine',
        status: 'ok',
        defer_until: null,
        checked_at: '2026-07-09 10:40',
        compose_project: 'db-stack',
        running: true,
        local_digest: 'sha256:1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b',
        remote_digest: 'sha256:1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b',
        local_version: '15.5',
        remote_version: '15.5',
        check_method: 'RepoDigest'
      },
      {
        name: 'redis-cache',
        image: 'redis:7-alpine',
        status: 'deferred',
        defer_until: '2026-08-01 00:00:00',
        checked_at: '2026-07-09 09:00',
        compose_project: null,
        running: true,
        local_digest: 'sha256:8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b2c1d0e9f8a7b',
        remote_digest: 'sha256:8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b2c1d0e9f8a7b',
        local_version: '7.2.3',
        remote_version: '7.2.4',
        check_method: 'RepoDigest'
      },
      {
        name: 'custom-config-app',
        image: 'myrepo/myapp:latest',
        status: 'update',
        defer_until: null,
        checked_at: '2026-07-09 11:00',
        compose_project: null,
        running: false,
        local_digest: 'sha256:5555555555555555555555555555555555555555555555555555555555555555',
        remote_digest: 'sha256:6666666666666666666666666666666666666666666666666666666666666666',
        check_method: 'ConfigDigest'
      }
    ]

    this.images = [
      {
        id: 'sha256:4f082ec15d862f1c84f3c0eb51b14a601859c402130386cfb8109bfcf9446d3e',
        tags: ['nginx:alpine', 'nginx:1.25.3-alpine'],
        size: 42580000,
        created: 1704067200,
        containers: ['nginx-app'],
        architecture: 'amd64'
      },
      {
        id: 'sha256:7f082ec15d862f1c84f3c0eb51b14a601859c402130386cfb8109bfcf9446d3e',
        tags: ['node:20-alpine'],
        size: 178900000,
        created: 1704153600,
        containers: ['web-api'],
        architecture: 'amd64'
      },
      {
        id: 'sha256:1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b',
        tags: ['postgres:15-alpine'],
        size: 234500000,
        created: 1704240000,
        containers: ['postgres-db'],
        architecture: 'amd64'
      },
      {
        id: 'sha256:dangling00112233445566778899aabbccddeeff00112233445566778899aabb',
        tags: ['<none>:<none>'],
        size: 154000000,
        created: 1701000000,
        containers: [],
        architecture: 'amd64'
      }
    ]

    this.registries = [
      {
        id: 1,
        registry: 'docker.io',
        username: 'dev_user',
        updated_at: '2026-07-01 10:00:00'
      },
      {
        id: 2,
        registry: 'ghcr.io',
        username: 'github_user',
        updated_at: '2026-07-02 11:30:00'
      }
    ]

    this.history = [
      {
        ID: 101,
        ContainerName: 'nginx-app',
        Image: 'nginx:alpine',
        OldDigest: 'sha256:old_nginx_digest_001',
        NewDigest: 'sha256:4f082ec15d862f1c84f3c0eb51b14a601859c402130386cfb8109bfcf9446d3e',
        Status: 'success',
        UpdatedAt: '2026-07-08T15:30:00Z'
      }
    ]

    this.updateLogs.set('nginx-app', [
      '[INFO] 依赖拉取开始: nginx:alpine',
      '[INFO] 正在与远端 Registry 进行网络交互与层校验...',
      '[INFO] 成功同步最新镜像层组件 Digest: sha256:4f082e...',
      '[INFO] 停止原旧容器进程: nginx-app',
      '[INFO] 重建启动服务容器: nginx-app',
      '[INFO] 探活检查成功，服务运行正常！',
      '[INFO] 任务全部完成'
    ])
  }

  getDynamicImages(): MockImage[] {
    const activeImageMap = new Map<string, string[]>()
    this.containers.forEach(c => {
      const matched = this.images.find(img => img.tags.includes(c.image))
      if (matched) {
        if (!activeImageMap.has(matched.id)) activeImageMap.set(matched.id, [])
        activeImageMap.get(matched.id)!.push(c.name)
      }
    })
    return this.images.map(img => ({
      ...img,
      containers: activeImageMap.get(img.id) || []
    }))
  }

  updateContainerRunningState(name: string, running: boolean) {
    const target = this.containers.find(c => c.name === name)
    if (target) {
      target.running = running
      notifyWSStatusChange()
    }
  }

  updateContainerDeferState(name: string, days: number) {
    const target = this.containers.find(c => c.name === name)
    if (target) {
      if (days <= 0) {
        target.status = 'ok'
        target.defer_until = null
      } else {
        target.status = 'deferred'
        const date = new Date()
        date.setDate(date.getDate() + days)
        target.defer_until = date.toISOString().replace('T', ' ').substring(0, 19)
      }
      notifyWSStatusChange()
    }
  }

  addTaskToQueue(name: string) {
    const target = this.containers.find(c => c.name === name)
    if (!target) return
    const nowStr = new Date().toISOString()
    const task: TaskItem = { container_name: name, type: 'update', added_at: nowStr }

    if (this.activeTask) {
      if (!this.queuedTasks.some(t => t.container_name === name)) {
        this.queuedTasks.push(task)
      }
    } else {
      this.activeTask = task
      this.runMockTaskProcessing(name)
    }
    notifyWSStatusChange()
  }

  runMockTaskProcessing(name: string) {
    const logs: string[] = []
    this.updateLogs.set(name, logs)

    const pushLog = (msg: string) => {
      logs.push(msg)
      if (activeMockWS) {
        activeMockWS.sendLogLine(name, msg)
      }
    }

    pushLog(`[INFO] 开始处理容器 ${name} 的升级任务`)

    setTimeout(() => {
      if (this.upgradeMode === 'fail_pull') {
        pushLog(`[ERROR] 拉取远端镜像网络失败`)
        this.finishTaskProcessing(name, false)
        return
      }
      pushLog(`[INFO] 成功拉取最新镜像层`)

      setTimeout(() => {
        if (this.upgradeMode === 'fail_start') {
          pushLog(`[ERROR] 探活检查失败，启动异常`)
          pushLog(`[INFO] 开始秒级恢复原 Spec 备份容器...`)
          setTimeout(() => {
            pushLog(`[INFO] 恢复成功，已还原至原镜像配置`)
            this.finishTaskProcessing(name, false)
          }, 800)
          return
        }

        const target = this.containers.find(c => c.name === name)
        if (target) {
          target.status = 'ok'
          target.local_digest = target.remote_digest
          target.running = true
        }

        this.history.unshift({
          ID: Date.now(),
          ContainerName: name,
          Image: target?.image || 'unknown',
          OldDigest: 'sha256:old_digest',
          NewDigest: target?.remote_digest || 'sha256:new_digest',
          Status: 'success',
          UpdatedAt: new Date().toISOString()
        })

        pushLog(`[INFO] 容器 ${name} 探活正常`)
        pushLog(`[INFO] 任务全部完成`)
        this.finishTaskProcessing(name, true)
      }, 1000)
    }, 1000)
  }

  finishTaskProcessing(_name: string, _success: boolean) {
    this.activeTask = null
    if (this.queuedTasks.length > 0) {
      const nextTask = this.queuedTasks.shift()!
      this.activeTask = nextTask
      this.runMockTaskProcessing(nextTask.container_name)
    }
    notifyWSStatusChange()
  }

  triggerUpdateFound() {
    this.containers.forEach((c, idx) => {
      if (idx % 2 === 0) {
        c.status = 'update'
        c.remote_digest = 'sha256:new_simulated_digest_' + Date.now()
      }
      c.checked_at = new Date().toISOString().replace('T', ' ').substring(0, 16)
    })
    this.lastCheckTime = new Date().toISOString().replace('T', ' ').substring(0, 19)
    notifyWSStatusChange()
  }

  batchComposeLifecycle(projectName: string, action: 'start' | 'stop' | 'restart') {
    const list = this.containers.filter(c => c.compose_project === projectName)
    list.forEach(c => {
      if (action === 'start' || action === 'restart') c.running = true
      else if (action === 'stop') c.running = false
    })
    notifyWSStatusChange()
  }

  getComposeProjects(): any[] {
    const projectMap = new Map<string, any>()
    this.containers.forEach(c => {
      if (!c.compose_project) return
      const projName = c.compose_project
      if (!projectMap.has(projName)) {
        projectMap.set(projName, {
          name: projName,
          working_dir: `/volume1/docker/${projName}`,
          config_files: `/volume1/docker/${projName}/docker-compose.yml`,
          containers: [],
          status: 'running',
          services_count: 0,
          has_update: false
        })
      }
      const proj = projectMap.get(projName)
      const hasUp = c.status === 'update'
      if (hasUp) proj.has_update = true

      proj.containers.push({
        name: c.name,
        service: c.name.split('-')[0] || c.name,
        image: c.image,
        state: c.running ? 'running' : 'stopped',
        has_update: hasUp,
        local_version: c.local_version || 'v1.0.0',
        remote_version: c.remote_version || (hasUp ? 'v1.1.0' : 'v1.0.0')
      })
    })

    const result: any[] = []
    projectMap.forEach(proj => {
      proj.services_count = proj.containers.length
      const runningCount = proj.containers.filter((item: any) => item.state === 'running').length
      if (runningCount === proj.services_count) proj.status = 'running'
      else if (runningCount === 0) proj.status = 'stopped'
      else proj.status = 'partial'
      result.push(proj)
    })
    return result
  }
}

const db = new SimulationDatabase()
let activeMockWS: any = null

function notifyWSStatusChange() {
  if (activeMockWS) {
    activeMockWS.sendStatusUpdate()
  }
}

const mockAdapter = async function (config: any) {
  const fullUrl = config.url || ''
  const method = config.method?.toLowerCase()
  const cleanPath = fullUrl.split('?')[0]

  console.log(`[Mock API] ${method?.toUpperCase()} -> ${cleanPath}`)

  if (cleanPath.endsWith('/status')) {
    return {
      data: {
        containers: db.containers,
        last_check: db.lastCheckTime,
        history: db.history,
        active: db.activeTask,
        queued: db.queuedTasks
      },
      status: 200,
      statusText: 'OK',
      headers: {},
      config
    }
  }

  if (cleanPath.endsWith('/compose/projects')) {
    return { data: db.getComposeProjects(), status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.endsWith('/compose/config')) {
    if (method === 'get') {
      const projName = config.params?.project || 'web-cluster'
      const defaultYaml = `version: '3.8'\nservices:\n  web:\n    image: nginx:alpine\n`
      return {
        data: {
          project: projName,
          file_path: `/volume1/docker/${projName}/docker-compose.yml`,
          content: db.composeConfigs.get(projName) || defaultYaml
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config
      }
    } else {
      try {
        const payload = JSON.parse(config.data || '{}')
        if (payload.project && payload.content) {
          db.composeConfigs.set(payload.project, payload.content)
        }
      } catch {}
      return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
    }
  }

  if (cleanPath.endsWith('/compose/lifecycle')) {
    try {
      const payload = JSON.parse(config.data || '{}')
      if (payload.project && payload.action) {
        db.batchComposeLifecycle(payload.project, payload.action)
      }
    } catch {}
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.endsWith('/image/tags')) {
    return {
      data: ['1.27.2', '1.27.1', '1.25.4', '1.25.3', '1.24.4'],
      status: 200,
      statusText: 'OK',
      headers: {},
      config
    }
  }

  if (cleanPath.includes('/container/') && cleanPath.endsWith('/start')) {
    const match = cleanPath.match(/\/container\/([^/]+)\/start/)
    if (match) db.updateContainerRunningState(match[1], true)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.includes('/container/') && cleanPath.endsWith('/stop')) {
    const match = cleanPath.match(/\/container\/([^/]+)\/stop/)
    if (match) db.updateContainerRunningState(match[1], false)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.includes('/container/') && cleanPath.endsWith('/restart')) {
    const match = cleanPath.match(/\/container\/([^/]+)\/restart/)
    if (match) db.updateContainerRunningState(match[1], true)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.includes('/container/') && cleanPath.endsWith('/logs')) {
    const match = cleanPath.match(/\/container\/([^/]+)\/logs/)
    const name = match ? match[1] : 'app'
    return { data: { logs: db.updateLogs.get(name)?.join('\n') || '模拟容器运行日志...' }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.includes('/defer/')) {
    const match = cleanPath.match(/\/defer\/([^/]+)/)
    if (match) db.updateContainerDeferState(match[1], 7)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.includes('/undefer/')) {
    const match = cleanPath.match(/\/undefer\/([^/]+)/)
    if (match) db.updateContainerDeferState(match[1], 0)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.includes('/update/')) {
    const match = cleanPath.match(/\/update\/([^/]+)/)
    if (match) db.addTaskToQueue(match[1])
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.endsWith('/images')) {
    return { data: db.getDynamicImages(), status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.endsWith('/images/prune')) {
    return { data: { space_reclaimed: 154000000, deleted_count: 1 }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.endsWith('/settings')) {
    if (method === 'get') return { data: db.settings, status: 200, statusText: 'OK', headers: {}, config }
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.endsWith('/settings/test-email')) {
    return { data: { ok: true, response: 'SMTP 测试连接成功' }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.endsWith('/registries')) {
    if (method === 'get') return { data: db.registries, status: 200, statusText: 'OK', headers: {}, config }
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.endsWith('/system/logs')) {
    return { data: { logs: ['2026-07-09 12:00:00 [INFO] 仿真日志'] }, status: 200, statusText: 'OK', headers: {}, config }
  }

  if (cleanPath.includes('/update-log/')) {
    const match = cleanPath.match(/\/update-log\/([^/]+)/)
    const name = match ? match[1] : ''
    if (method === 'get') {
      const logs = db.updateLogs.get(name) || [
        `[INFO] 正在初始化部署容器: ${name}`,
        `[INFO] 成功同步最新镜像层组件 Digest: sha256:4f082e...`,
        `[INFO] 容器探活正常`,
        `[INFO] 任务全部完成`
      ]
      return { data: { found: true, logs }, status: 200, statusText: 'OK', headers: {}, config }
    } else if (method === 'delete') {
      db.updateLogs.delete(name)
      db.history = db.history.filter(h => h.ContainerName !== name)
      notifyWSStatusChange()
      return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
    }
  }

  if (cleanPath.includes('/history')) {
    const match = cleanPath.match(/\/history\/([^/]+)/)
    if (method === 'delete') {
      if (match) {
        const targetId = parseInt(match[1], 10)
        db.history = db.history.filter(h => h.ID !== targetId)
      } else {
        db.history = []
      }
      notifyWSStatusChange()
      return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
    }
  }

  if (cleanPath.endsWith('/check')) {
    setTimeout(() => db.triggerUpdateFound(), 1000)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
  }

  return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
}

axios.defaults.adapter = mockAdapter as any
if (apiClient && apiClient.defaults) {
  apiClient.defaults.adapter = mockAdapter as any
}

const originalFetch = window.fetch
window.fetch = async function (input: RequestInfo | URL, init?: RequestInit) {
  const urlStr = typeof input === 'string' ? input : input instanceof Request ? input.url : input.toString()
  const cleanPath = urlStr.split('?')[0]

  if (cleanPath.endsWith('/status')) {
    return new Response(
      JSON.stringify({
        containers: db.containers,
        last_check: db.lastCheckTime,
        history: db.history,
        active: db.activeTask,
        queued: db.queuedTasks
      }),
      { status: 200, headers: { 'Content-Type': 'application/json' } }
    )
  }
  if (cleanPath.endsWith('/compose/projects')) {
    return new Response(
      JSON.stringify(db.getComposeProjects()),
      { status: 200, headers: { 'Content-Type': 'application/json' } }
    )
  }
  if (cleanPath.includes('/api/')) {
    return new Response(JSON.stringify({ ok: true }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
  return originalFetch(input, init)
}

class MockWebSocket {
  url: string
  readyState: number = 0
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onerror: ((err: any) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null

  constructor(url: string) {
    this.url = url
    setTimeout(() => {
      this.readyState = 1
      if (this.onopen) this.onopen()
      this.sendStatusUpdate()
    }, 150)
    activeMockWS = this
  }

  send(data: string) {
    try {
      const parsed = JSON.parse(data)
      if (parsed.type === 'ping' && this.onmessage && this.readyState === 1) {
        this.onmessage(new MessageEvent('message', { data: JSON.stringify({ type: 'pong' }) }))
      }
    } catch {}
  }

  close() {
    this.readyState = 3
    if (this.onclose) this.onclose()
    if (activeMockWS === this) activeMockWS = null
  }

  sendStatusUpdate() {
    if (this.readyState === 1 && this.onmessage) {
      this.onmessage(
        new MessageEvent('message', {
          data: JSON.stringify({
            type: 'status',
            containers: db.containers,
            last_check: db.lastCheckTime,
            history: db.history,
            active: db.activeTask,
            queued: db.queuedTasks
          })
        })
      )
    }
  }

  sendLogLine(containerName: string, messageText: string) {
    if (this.readyState === 1 && this.onmessage) {
      this.onmessage(
        new MessageEvent('message', {
          data: JSON.stringify({
            type: 'log',
            container: containerName,
            message: messageText
          })
        })
      )
    }
  }
}

// @ts-ignore
window.WebSocket = MockWebSocket

console.log('[Mock System] 成功重构并激活 DEV 全量网络与 Websocket 仿真引擎')
