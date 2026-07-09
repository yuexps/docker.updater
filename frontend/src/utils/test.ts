import axios from 'axios'

// 模拟内存数据库
interface MockContainer {
  name: string
  image: string
  status: string // 'ok' | 'update' | 'deferred'
  defer_until: string | null
  checked_at: string
  has_rollback: boolean
  rollback_expires: string | null
  compose_project: string | null
  running: boolean
  local_digest: string
  remote_digest: string
}

let mockContainers: MockContainer[] = [
  {
    name: 'nginx-app',
    image: 'nginx:alpine',
    status: 'update',
    defer_until: null,
    checked_at: '2026-07-09 10:30',
    has_rollback: true,
    rollback_expires: '2026-07-16 10:30',
    compose_project: 'web-cluster',
    running: true,
    local_digest: 'sha256:4f082ec15d862f1c84f3c0eb51b14a601859c402130386cfb8109bfcf9446d3e',
    remote_digest: 'sha256:9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c3b2a1a0b9c8d7e6f5a4b3c2d1e0f9a8b'
  },
  {
    name: 'postgres-db',
    image: 'postgres:15-alpine',
    status: 'ok',
    defer_until: null,
    checked_at: '2026-07-09 11:15',
    has_rollback: false,
    rollback_expires: null,
    compose_project: null,
    running: true,
    local_digest: 'sha256:5a4b3c2d1e0f9a8b8c7d6e5f4a3b2c1d0c9b8a7f6e5d4c3b2a1a0b9c8d7e6f5a',
    remote_digest: 'sha256:5a4b3c2d1e0f9a8b8c7d6e5f4a3b2c1d0c9b8a7f6e5d4c3b2a1a0b9c8d7e6f5a'
  },
  {
    name: 'redis-cache',
    image: 'redis:7.2-alpine',
    status: 'ok',
    defer_until: null,
    checked_at: '2026-07-09 12:00',
    has_rollback: false,
    rollback_expires: null,
    compose_project: 'cache-stack',
    running: false,
    local_digest: 'sha256:8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d1c0b9a8f7e',
    remote_digest: 'sha256:8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d1c0b9a8f7e'
  },
  {
    name: 'node-api',
    image: 'node:20-alpine',
    status: 'deferred',
    defer_until: '2026-07-16',
    checked_at: '2026-07-09 09:45',
    has_rollback: false,
    rollback_expires: null,
    compose_project: 'web-cluster',
    running: true,
    local_digest: 'sha256:1a84f3c0eb51b14a601859c402130386cfb8109bfcf9446d3e4f082ec15d862f',
    remote_digest: 'sha256:f6e5d4c3b2a1a0b9c8d7e6f5a4b3c2d1e0f9a8b8c7d6e5f4a3b2c1d0c9b8a7f'
  },
  {
    name: 'python-worker',
    image: 'python:3.11-slim',
    status: 'update',
    defer_until: null,
    checked_at: '2026-07-09 08:20',
    has_rollback: true,
    rollback_expires: '2026-07-15 08:20',
    compose_project: null,
    running: false,
    local_digest: 'sha256:7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6',
    remote_digest: 'sha256:e0f9a8b8c7d6e5f4a3b2c1d0c9b8a7f6e5d4c3b2a1a0b9c8d7e6f5a4b3c2d1e'
  }
]

let activeMockWS: any = null

// 模拟镜像内存数据库
interface MockImage {
  id: string
  tags: string[]
  size: number
  created: number
  containers?: string[]
}

let mockImages: MockImage[] = [
  {
    id: 'sha256:4f082ec15d862f1c84f3c0eb51b14a601859c402130386cfb8109bfcf9446d3e',
    tags: ['nginx:alpine'],
    size: 45000000,
    created: 1783515000,
    containers: ['nginx-app']
  },
  {
    id: 'sha256:5a4b3c2d1e0f9a8b8c7d6e5f4a3b2c1d0c9b8a7f6e5d4c3b2a1a0b9c8d7e6f5a',
    tags: ['postgres:15-alpine'],
    size: 280000000,
    created: 1783500000,
    containers: ['postgres-db']
  },
  {
    id: 'sha256:8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d1c0b9a8f7e',
    tags: ['redis:7.2-alpine'],
    size: 38000000,
    created: 1783450000,
    containers: ['redis-cache']
  },
  {
    id: 'sha256:1a84f3c0eb51b14a601859c402130386cfb8109bfcf9446d3e4f082ec15d862f',
    tags: ['node:20-alpine'],
    size: 180000000,
    created: 1783400000,
    containers: ['node-api']
  },
  {
    id: 'sha256:7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6',
    tags: ['python:3.11-slim'],
    size: 120000000,
    created: 1783300000,
    containers: []
  },
  {
    id: 'sha256:9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c3b2a1a0b9c8d7e6f5a4b3c2d1e0f9a8b',
    tags: ['<none>:<none>'],
    size: 55000000,
    created: 1783100000,
    containers: []
  },
  {
    id: 'sha256:e0f9a8b8c7d6e5f4a3b2c1d0c9b8a7f6e5d4c3b2a1a0b9c8d7e6f5a4b3c2d1e',
    tags: ['<none>:<none>'],
    size: 110000000,
    created: 1783000000,
    containers: []
  }
]

// 动态计算镜像的使用容器
function getDynamicImages(): MockImage[] {
  return mockImages.map(img => {
    const matchedContainers = mockContainers.filter(c => {
      if (c.local_digest === img.id) return true
      const cleanImgName = c.image.replace('@sha256:', '')
      return img.tags.includes(cleanImgName)
    }).map(c => c.name)

    return {
      ...img,
      containers: matchedContainers
    }
  })
}

// 模拟配置持久化数据库
let mockSettings = {
  backup_enabled: true,
  backup_hours: 24,
  restart_stack: false,
  temp_mirrors: ['https://docker.m.daocloud.io', 'https://mirror.baidubce.com'],
  check_type: 'day',
  check_value: 1
}

// 模拟私有仓库数据库
interface RegistryItem {
  id: number
  registry: string
  username: string
  password?: string
  updated_at: string
}

let mockRegistries: RegistryItem[] = [
  { id: 1, registry: 'cr.io', username: 'admin', updated_at: '2026-07-09 10:00:00' }
]

// 模拟排队和活跃任务
interface TaskItem {
  container_name: string
  type: string // 'update' | 'rollback'
  added_at: string
}

let activeTask: TaskItem | null = null
let queuedTasks: TaskItem[] = []
let lastCheckTime = '2026-07-09 12:00:00'

// 模拟持久化升级历史记录数据库
let mockHistory = [
  {
    id: 1,
    ContainerName: 'nginx-app',
    Image: 'nginx:alpine',
    UpdatedAt: '2026-07-08 18:30:22',
    Status: 'success'
  },
  {
    id: 2,
    ContainerName: 'postgres-db',
    Image: 'postgres:15-alpine',
    UpdatedAt: '2026-07-07 09:15:10',
    Status: 'success'
  }
]

// 模拟持久化运行日志存储库
let mockUpdateLogs = new Map<string, string[]>()

// 初始化填充两个历史日志
mockUpdateLogs.set('nginx-app', [
  `[INFO] 开始对容器 nginx-app 进行升级操作...`,
  `[INFO] 正在分析本地与远端分发凭证...`,
  `[PULL] 正在从 registry 下载新镜像层: sha256:7f082ec15d86...`,
  `[PULL] 正在从 registry 下载新镜像层: sha256:1a84f3c0eb51...`,
  `[PULL] 最新镜像层拉取完毕，已校验校验和`,
  `[INFO] 正在创建升级前镜像备份点: nginx-app_old`,
  `[INFO] 成功记录重启配置策略.`,
  `[INFO] 正在优雅停止运行中的旧容器...`,
  `[INFO] 旧容器实例已清理`,
  `[INFO] 基于新拉取镜像重新组装参数重建中...`,
  `[INFO] 容器 nginx-app 正在唤醒启动...`,
  `[SUCCESS] 容器 nginx-app 已成功平滑升级至最新版本！`
])

mockUpdateLogs.set('postgres-db', [
  `[INFO] 开始对容器 postgres-db 进行升级操作...`,
  `[INFO] 正在分析本地与远端分发凭证...`,
  `[PULL] 正在从 registry 下载新镜像层: sha256:5a4b3c2d1e0f...`,
  `[PULL] 最新镜像层拉取完毕，已校验校验和`,
  `[INFO] 正在创建升级前镜像备份点: postgres-db_old`,
  `[INFO] 成功记录重启配置策略.`,
  `[INFO] 正在优雅停止运行中的旧容器...`,
  `[INFO] 旧容器实例已清理`,
  `[INFO] 基于新拉取镜像重新组装参数重建中...`,
  `[INFO] 容器 postgres-db 正在唤醒启动...`,
  `[SUCCESS] 容器 postgres-db 已成功平滑升级至最新版本！`
])

// 动态日志数据流生成器
function getMockTaskLogLines(containerName: string, mode: 'update' | 'rollback'): string[] {
  return mode === 'update'
    ? [
        `[INFO] 开始对容器 ${containerName} 进行升级操作...`,
        `[INFO] 正在分析本地与远端分发凭证...`,
        `[PULL] 正在从 registry 下载新镜像层: sha256:7f082ec15d86...`,
        `[PULL] 正在从 registry 下载新镜像层: sha256:1a84f3c0eb51...`,
        `[PULL] 最新镜像层拉取完毕，已校验校验和`,
        `[INFO] 正在创建升级前镜像备份点: ${containerName}_old`,
        `[INFO] 成功记录重启配置策略.`,
        `[INFO] 正在优雅停止运行中的旧容器...`,
        `[INFO] 旧容器实例已清理`,
        `[INFO] 基于新拉取镜像重新组装参数重建中...`,
        `[INFO] 容器 ${containerName} 正在唤醒启动...`,
        `[SUCCESS] 容器 ${containerName} 已成功平滑升级至最新版本！`
      ]
    : [
        `[INFO] 开启备份回退恢复机制: ${containerName}`,
        `[INFO] 正在检查物理备份容器 ${containerName}_old...`,
        `[INFO] 停止正在运行中的异常/新容器 ${containerName}...`,
        `[INFO] 正在解除其与 Docker 网桥的绑定...`,
        `[INFO] 重新映射磁盘挂载卷数据...`,
        `[INFO] 将物理备份容器还原并重命名为 ${containerName}`,
        `[INFO] 恢复原有的重启策略和主机名关联...`,
        `[SUCCESS] 容器 ${containerName} 已成功回滚至升级前备份点！`
      ]
}

function getMockStatusData() {
  return {
    containers: mockContainers,
    last_check: lastCheckTime,
    history: mockHistory,
    active: activeTask,
    queued: queuedTasks
  }
}

// 状态变更修改辅助函数
function updateContainerRunningState(name: string, running: boolean) {
  const c = mockContainers.find((item) => item.name === name)
  if (c) {
    c.running = running
    console.log(`[Mock DB] 容器 ${name} 运行状态已变更为: ${running}`)
    notifyWSStatusChange()
  }
}

function updateContainerDeferState(name: string, days: number) {
  const c = mockContainers.find((item) => item.name === name)
  if (c) {
    if (days > 0) {
      c.status = 'deferred'
      const targetDate = new Date()
      targetDate.setDate(targetDate.getDate() + days)
      c.defer_until = targetDate.toISOString().split('T')[0]
      console.log(`[Mock DB] 容器 ${name} 已成功暂挂 ${days} 天，至 ${c.defer_until}`)
    } else {
      c.status = 'update'
      c.defer_until = null
      console.log(`[Mock DB] 容器 ${name} 已恢复升级检测`)
    }
    notifyWSStatusChange()
  }
}

function updateContainerRollbackState(name: string, hasRollback: boolean) {
  const c = mockContainers.find((item) => item.name === name)
  if (c) {
    c.has_rollback = hasRollback
    c.rollback_expires = null
    console.log(`[Mock DB] 容器 ${name} 的回滚备份已清除`)
    notifyWSStatusChange()
  }
}

function updateContainerAfterUpgrade(name: string) {
  const c = mockContainers.find((item) => item.name === name)
  if (c) {
    c.status = 'ok'
    c.local_digest = c.remote_digest // 升级后本地与远程哈希对齐
    c.has_rollback = true // 产生了一个备份
    const expireDate = new Date()
    expireDate.setHours(expireDate.getHours() + 24)
    c.rollback_expires = expireDate.toISOString().replace('T', ' ').substring(0, 16)
    c.checked_at = new Date().toISOString().replace('T', ' ').substring(0, 16)
    console.log(`[Mock DB] 容器 ${name} 模拟升级流程完成，已归于最新版`)
    notifyWSStatusChange()
  }
}

function updateContainerAfterRollback(name: string) {
  const c = mockContainers.find((item) => item.name === name)
  if (c) {
    c.status = 'update'
    // 还原本地摘要到旧的值
    c.local_digest = 'sha256:rollback_old_hash_value_1234567890abcdef'
    c.has_rollback = false
    c.rollback_expires = null
    console.log(`[Mock DB] 容器 ${name} 模拟回滚还原完成`)
    notifyWSStatusChange()
  }
}

function notifyWSStatusChange() {
  if (activeMockWS) {
    activeMockWS.sendStatusUpdate()
  }
}

function getMockLogs(name: string): string {
  return `[Mock Logs for ${name}]
2026-07-09T12:00:00.124Z [info] Starting mock production build...
2026-07-09T12:00:01.345Z [info] Listening on HTTP connection: port 80
2026-07-09T12:00:05.678Z [info] Accepted external socket mapping from 127.0.0.1
2026-07-09T12:01:20.999Z [info] Database heartbeat validation: 0.12ms success
2026-07-09T12:02:40.456Z [info] Keep-alive worker check: status=idle load=0.01%
`
}

// 统一任务排队调度器
function addTaskToQueue(name: string, type: 'update' | 'rollback') {
  const newTask: TaskItem = {
    container_name: name,
    type,
    added_at: new Date().toISOString()
  }

  if (activeTask) {
    queuedTasks.push(newTask)
    console.log(`[Mock Queue] 任务已加入排队:`, newTask)
  } else {
    activeTask = newTask
    console.log(`[Mock Queue] 任务已立即运行:`, newTask)
    if (activeMockWS) {
      activeMockWS.startMockLogStream(name, type)
    }
  }
  notifyWSStatusChange()
}

// 升级或回滚结束出队
function onTaskFinished(name: string, type: 'update' | 'rollback') {
  const c = mockContainers.find((item) => item.name === name)
  const targetImage = c ? c.image : 'unknown'

  if (type === 'update') {
    updateContainerAfterUpgrade(name)
  } else {
    updateContainerAfterRollback(name)
  }

  // 模拟操作产生的日志，将其永久归档到我们的历史操作日志内存库
  const logLines = getMockTaskLogLines(name, type)
  mockUpdateLogs.set(name, logLines)

  // 追加到历史列表
  mockHistory.unshift({
    id: Date.now(),
    ContainerName: name,
    Image: targetImage,
    UpdatedAt: new Date().toISOString(),
    Status: 'success'
  })

  activeTask = null
  console.log(`[Mock Queue] 容器 ${name} 的 ${type} 任务已执行完毕，并归档历史记录`)

  if (queuedTasks.length > 0) {
    const nextTask = queuedTasks.shift()!
    activeTask = nextTask
    console.log(`[Mock Queue] 从队列拉起下一个任务:`, nextTask)
    if (activeMockWS) {
      activeMockWS.startMockLogStream(nextTask.container_name, nextTask.type as 'update' | 'rollback')
    }
  }
  notifyWSStatusChange()
}

// --- 劫持 Axios defaults.adapter ---
const originalAdapter = axios.defaults.adapter

axios.defaults.adapter = async function (config) {
  const url = config.url || ''
  const method = config.method?.toLowerCase()

  console.log(`[Mock Axios] ${method?.toUpperCase()} -> ${url}`, config.data)

  // 1. /api/status 容器状态汇总
  if (url.includes('/api/status')) {
    return {
      data: getMockStatusData(),
      status: 200,
      statusText: 'OK',
      headers: config.headers || {},
      config
    }
  }

  // 2. 容器生命周期控制
  const startMatch = url.match(/\/api\/container\/([^/]+)\/start/)
  if (startMatch) {
    const name = startMatch[1]
    updateContainerRunningState(name, true)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  const stopMatch = url.match(/\/api\/container\/([^/]+)\/stop/)
  if (stopMatch) {
    const name = stopMatch[1]
    updateContainerRunningState(name, false)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  const restartMatch = url.match(/\/api\/container\/([^/]+)\/restart/)
  if (restartMatch) {
    const name = restartMatch[1]
    updateContainerRunningState(name, true)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 3. 暂挂升级
  const deferMatch = url.match(/\/api\/defer\/([^/]+)/)
  if (deferMatch) {
    const name = deferMatch[1]
    let days = 7
    try {
      days = JSON.parse(config.data || '{}').days || 7
    } catch (e) {}
    updateContainerDeferState(name, days)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 4. 恢复检测
  const undeferMatch = url.match(/\/api\/undefer\/([^/]+)/)
  if (undeferMatch) {
    const name = undeferMatch[1]
    updateContainerDeferState(name, 0)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 5. 删除备份
  const backupDeleteMatch = url.match(/\/api\/backup\/([^/]+)/)
  if (backupDeleteMatch) {
    const name = backupDeleteMatch[1]
    updateContainerRollbackState(name, false)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 6. 获取常规日志
  const logsMatch = url.match(/\/api\/container\/([^/]+)\/logs/)
  if (logsMatch) {
    const name = logsMatch[1]
    return {
      data: { logs: getMockLogs(name) },
      status: 200,
      statusText: 'OK',
      headers: config.headers || {},
      config
    }
  }

  // 7. 触发升级 (加入任务排队调度系统)
  const updateMatch = url.match(/\/api\/update\/([^/]+)/)
  if (updateMatch) {
    const name = updateMatch[1]
    addTaskToQueue(name, 'update')
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 8. 触发回滚 (加入任务排队调度系统)
  const rollbackMatch = url.match(/\/api\/rollback\/([^/]+)/)
  if (rollbackMatch) {
    const name = rollbackMatch[1]
    addTaskToQueue(name, 'rollback')
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 9. 获取镜像管理列表 (动态计算占用容器)
  if (url.includes('/api/images') && method === 'get') {
    return {
      data: getDynamicImages(),
      status: 200,
      statusText: 'OK',
      headers: config.headers || {},
      config
    }
  }

  // 10. 删除指定镜像 (添加正被占用冲突校验)
  if (url.includes('/api/image') && method === 'delete') {
    const targetId = config.params?.id || ''
    const dynamicImages = getDynamicImages()
    const targetImg = dynamicImages.find(img => img.id === targetId)

    if (targetImg && targetImg.containers && targetImg.containers.length > 0) {
      console.warn(`[Mock DB] 删除镜像冲突，正被容器 ${targetImg.containers.join(',')} 使用: ${targetId}`)
      return {
        data: { error: `Conflict, image is being used by container(s): ${targetImg.containers.join(', ')}` },
        status: 500,
        statusText: 'Internal Server Error',
        headers: config.headers || {},
        config
      }
    }

    mockImages = mockImages.filter(img => img.id !== targetId)
    console.log(`[Mock DB] 镜像已强制删除: ${targetId}`)
    return {
      data: { ok: true },
      status: 200,
      statusText: 'OK',
      headers: config.headers || {},
      config
    }
  }

  // 11. 清理虚悬镜像 (Prune)
  if (url.includes('/api/images/prune') && method === 'post') {
    const dynamicImages = getDynamicImages()
    const dangling = dynamicImages.filter(img => img.tags.includes('<none>:<none>'))
    const space = dangling.reduce((acc, curr) => acc + curr.size, 0)
    const count = dangling.length
    mockImages = mockImages.filter(img => !img.tags.includes('<none>:<none>'))
    console.log(`[Mock DB] Prune 完成，释放空间 ${space} B，共清理 ${count} 个残留镜像`)
    return {
      data: {
        space_reclaimed: space,
        deleted_count: count
      },
      status: 200,
      statusText: 'OK',
      headers: config.headers || {},
      config
    }
  }

  // 12. 取消排队等待的任务
  const cancelTaskMatch = url.match(/\/api\/tasks\/cancel\/([^/]+)/)
  if (cancelTaskMatch && method === 'post') {
    const name = cancelTaskMatch[1]
    const beforeLen = queuedTasks.length
    queuedTasks = queuedTasks.filter(t => t.container_name !== name)
    const success = queuedTasks.length < beforeLen
    console.log(`[Mock Queue] 取消任务排队: ${name}, 成功: ${success}`)
    notifyWSStatusChange()
    return {
      data: { success },
      status: 200,
      statusText: 'OK',
      headers: config.headers || {},
      config
    }
  }

  // 14. 触发手动检查新版本
  if (url.includes('/api/check') && method === 'post') {
    // 异步 1.5 秒后完成，改变 postgres-db 状态，模拟发现新版本
    setTimeout(() => {
      lastCheckTime = new Date().toISOString().replace('T', ' ').substring(0, 19)
      const pDb = mockContainers.find(c => c.name === 'postgres-db')
      if (pDb && pDb.status === 'ok') {
        pDb.status = 'update'
        pDb.remote_digest = 'sha256:new_postgres_remote_digest_hash_val_9999'
        pDb.checked_at = lastCheckTime.substring(0, 16)
        console.log('[Mock DB] 后台版本检测完成：发现 postgres-db 存在新镜像版本！')
      }
      notifyWSStatusChange()
    }, 1500)
    
    return {
      data: { ok: true },
      status: 200,
      statusText: 'OK',
      headers: config.headers || {},
      config
    }
  }

  // 15. 获取/删除持久化升级回滚历史操作日志
  const updateLogMatch = url.match(/\/api\/update-log\/([^/]+)/)
  if (updateLogMatch) {
    const name = updateLogMatch[1]
    if (method === 'get') {
      const hasLog = mockUpdateLogs.has(name)
      return {
        data: {
          found: hasLog,
          logs: mockUpdateLogs.get(name) || []
        },
        status: 200,
        statusText: 'OK',
        headers: config.headers || {},
        config
      }
    } else if (method === 'delete') {
      mockUpdateLogs.delete(name)
      mockHistory = mockHistory.filter(h => h.ContainerName !== name)
      console.log(`[Mock DB] 持久化操作日志与历史记录已清除: ${name}`)
      notifyWSStatusChange()
      return {
        data: { ok: true },
        status: 200,
        statusText: 'OK',
        headers: config.headers || {},
        config
      }
    }
  }

  // 12.5 获取系统源加速镜像
  if (url.includes('/api/settings/system-mirrors')) {
    return {
      data: ['https://registry.docker-cn.com', 'https://docker.mirrors.ustc.edu.cn'],
      status: 200,
      statusText: 'OK',
      headers: config.headers || {},
      config
    }
  }

  // 13. 获取/保存系统配置
  if (url.includes('/api/settings')) {
    if (method === 'get') {
      return {
        data: mockSettings,
        status: 200,
        statusText: 'OK',
        headers: config.headers || {},
        config
      }
    } else if (method === 'post') {
      try {
        const body = JSON.parse(config.data || '{}')
        mockSettings.backup_enabled = body.backup_enabled ?? mockSettings.backup_enabled
        mockSettings.backup_hours = body.backup_hours ?? mockSettings.backup_hours
        mockSettings.restart_stack = body.restart_stack ?? mockSettings.restart_stack
        mockSettings.temp_mirrors = body.temp_mirrors ?? mockSettings.temp_mirrors
        mockSettings.check_type = body.check_type ?? mockSettings.check_type
        mockSettings.check_value = body.check_value ?? mockSettings.check_value
        console.log('[Mock DB] 配置已保存:', mockSettings)
      } catch (e) {}
      return {
        data: { ok: true },
        status: 200,
        statusText: 'OK',
        headers: config.headers || {},
        config
      }
    }
  }

  // 16. 私有仓凭据列表的增删改查
  if (url.includes('/api/registries')) {
    if (method === 'get') {
      return {
        data: mockRegistries,
        status: 200,
        statusText: 'OK',
        headers: config.headers || {},
        config
      }
    } else if (method === 'post') {
      try {
        const cred = JSON.parse(config.data || '{}')
        if (cred.id && cred.id > 0) {
          const exist = mockRegistries.find(r => r.id === cred.id)
          if (exist) {
            exist.registry = cred.registry || exist.registry
            exist.username = cred.username || exist.username
            exist.updated_at = new Date().toISOString().replace('T', ' ').substring(0, 19)
            console.log('[Mock DB] 私有仓库凭据已更新:', exist)
          }
        } else {
          const newId = mockRegistries.length > 0 ? Math.max(...mockRegistries.map(r => r.id)) + 1 : 1
          const newReg = {
            id: newId,
            registry: cred.registry,
            username: cred.username,
            updated_at: new Date().toISOString().replace('T', ' ').substring(0, 19)
          }
          mockRegistries.push(newReg)
          console.log('[Mock DB] 私有仓库凭证已新增:', newReg)
        }
      } catch (e) {}
      return {
        data: { ok: true },
        status: 200,
        statusText: 'OK',
        headers: config.headers || {},
        config
      }
    }
  }

  const deleteRegMatch = url.match(/\/api\/registries\/([^/]+)/)
  if (deleteRegMatch && method === 'delete') {
    const targetId = parseInt(deleteRegMatch[1], 10)
    mockRegistries = mockRegistries.filter(r => r.id !== targetId)
    console.log(`[Mock DB] 私有仓库凭据已删除: id=${targetId}`)
    return {
      data: { ok: true },
      status: 200,
      statusText: 'OK',
      headers: config.headers || {},
      config
    }
  }

  // 其他请求使用原装 adapter
  if (originalAdapter) {
    // @ts-ignore
    return originalAdapter(config)
  }

  return {
    data: {},
    status: 404,
    statusText: 'Not Found',
    headers: {},
    config
  }
}

// --- 劫持全局 window.WebSocket ---
const OriginalWebSocket = window.WebSocket

class MockWebSocket {
  url: string
  readyState: number = 0 // CONNECTING
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onerror: ((err: any) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null

  constructor(url: string) {
    this.url = url
    console.log(`[Mock WS] 拦截连接建立: ${url}`)

    setTimeout(() => {
      this.readyState = 1 // OPEN
      if (this.onopen) this.onopen()

      // 连接成功后发回第一版数据
      this.sendStatusUpdate()
      this.startSysLogBroadcast()
    }, 150)

    activeMockWS = this
  }

  send(data: string) {
    try {
      const parsed = JSON.parse(data)
      console.log('[Mock WS] 接收到上行消息:', parsed)

      if (parsed.type === 'ping') {
        if (this.onmessage && this.readyState === 1) {
          this.onmessage(
            new MessageEvent('message', {
              data: JSON.stringify({
                type: 'pong'
              })
            })
          )
        }
      } else if (parsed.type === 'subscribe' && parsed.target?.startsWith('logs:')) {
        const containerName = parsed.target.replace('logs:', '')
        // 外部 HTTP 已触发升级或回滚日志推送，这里配合完成数据填充
      }
    } catch (e) {}
  }

  close() {
    this.readyState = 3 // CLOSED
    if (this.onclose) this.onclose()
    if (activeMockWS === this) {
      activeMockWS = null
    }
  }

  sendStatusUpdate() {
    if (this.onmessage && this.readyState === 1) {
      this.onmessage(
        new MessageEvent('message', {
          data: JSON.stringify({
            type: 'status',
            payload: getMockStatusData()
          })
        })
      )
    }
  }

  startSysLogBroadcast() {
    const sysLogInterval = setInterval(() => {
      if (this.readyState !== 1) {
        clearInterval(sysLogInterval)
        return
      }
      if (this.onmessage) {
        const level = Math.random() > 0.35 ? '[INFO]' : Math.random() > 0.5 ? '[WARNING]' : '[SUCCESS]'
        const line = `${new Date().toLocaleTimeString()} ${level} 巡检模拟: 正在对所有外部镜像源进行连通性确认...`
        this.onmessage(
          new MessageEvent('message', {
            data: JSON.stringify({
              type: 'syslog',
              payload: { line }
            })
          })
        )
      }
    }, 12000)
  }

  startMockLogStream(containerName: string, mode: 'update' | 'rollback') {
    let step = 0
    const logs = getMockTaskLogLines(containerName, mode)

    const logInterval = setInterval(() => {
      if (this.readyState !== 1) {
        clearInterval(logInterval)
        return
      }

      if (step >= logs.length) {
        clearInterval(logInterval)
        onTaskFinished(containerName, mode)
        return
      }

      if (this.onmessage) {
        this.onmessage(
          new MessageEvent('message', {
            data: JSON.stringify({
              type: 'log',
              payload: {
                container: containerName,
                task: mode,
                message: logs[step]
              }
            })
          })
        )
      }
      step++
    }, 1000)
  }
}

// @ts-ignore
window.WebSocket = function (url: string) {
  if (url.includes('/app/docker-updater/api/ws')) {
    return new MockWebSocket(url)
  }
  return new OriginalWebSocket(url)
}
// @ts-ignore
window.WebSocket.prototype = OriginalWebSocket.prototype

console.log('[Mock System] 成功拦截全局 Axios 与 WebSocket 连接，启动本地纯交互测试数据填充器！')
