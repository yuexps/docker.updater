import axios from 'axios'

// --- 仿真内存数据库状态定义 ---
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

interface MockImage {
  id: string
  tags: string[]
  size: number
  created: number
  containers?: string[]
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
  type: 'update' | 'rollback'
  added_at: string
}

class SimulationDatabase {
  containers: MockContainer[] = []
  images: MockImage[] = []
  settings = {
    backup_enabled: true,
    backup_hours: 24,
    restart_stack: false,
    temp_mirrors: ['https://docker.m.daocloud.io', 'https://mirror.baidubce.com'],
    check_type: 'day',
    check_value: 1
  }
  registries: RegistryItem[] = []
  history: any[] = []
  updateLogs = new Map<string, string[]>()
  activeTask: TaskItem | null = null
  queuedTasks: TaskItem[] = []
  lastCheckTime = '2026-07-09 12:00:00'

  // 仿真控制器变量
  upgradeMode: 'success' | 'fail_pull' | 'fail_start' = 'success'
  isWSConnected = true

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

    this.images = [
      {
        id: 'sha256:4f082ec15d862f1c84f3c0eb51b14a601859c402130386cfb8109bfcf9446d3e',
        tags: ['nginx:alpine'],
        size: 45000000,
        created: 1783515000
      },
      {
        id: 'sha256:5a4b3c2d1e0f9a8b8c7d6e5f4a3b2c1d0c9b8a7f6e5d4c3b2a1a0b9c8d7e6f5a',
        tags: ['postgres:15-alpine'],
        size: 280000000,
        created: 1783500000
      },
      {
        id: 'sha256:8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d1c0b9a8f7e',
        tags: ['redis:7.2-alpine'],
        size: 38000000,
        created: 1783450000
      },
      {
        id: 'sha256:1a84f3c0eb51b14a601859c402130386cfb8109bfcf9446d3e4f082ec15d862f',
        tags: ['node:20-alpine'],
        size: 180000000,
        created: 1783400000
      },
      {
        id: 'sha256:7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6',
        tags: ['python:3.11-slim'],
        size: 120000000,
        created: 1783300000
      },
      {
        id: 'sha256:9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c3b2a1a0b9c8d7e6f5a4b3c2d1e0f9a8b',
        tags: ['<none>:<none>'],
        size: 55000000,
        created: 1783100000
      },
      {
        id: 'sha256:e0f9a8b8c7d6e5f4a3b2c1d0c9b8a7f6e5d4c3b2a1a0b9c8d7e6f5a4b3c2d1e',
        tags: ['<none>:<none>'],
        size: 110000000,
        created: 1783000000
      }
    ]

    this.registries = [
      { id: 1, registry: 'cr.io', username: 'admin', updated_at: '2026-07-09 10:00:00' }
    ]

    this.history = [
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

    this.updateLogs.clear()
    this.updateLogs.set('nginx-app', [
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

    this.activeTask = null
    this.queuedTasks = []
    this.lastCheckTime = '2026-07-09 12:00:00'
  }

  // 模拟版本检测发现更新
  triggerUpdateFound() {
    this.lastCheckTime = new Date().toISOString().replace('T', ' ').substring(0, 19)
    const pDb = this.containers.find(c => c.name === 'postgres-db')
    if (pDb && pDb.status === 'ok') {
      pDb.status = 'update'
      pDb.remote_digest = 'sha256:new_postgres_remote_digest_hash_val_9999'
      pDb.checked_at = this.lastCheckTime.substring(0, 16)
      console.log('[Mock DB] 可视化触发检测：postgres-db 发现可用升级！')
    }
    const rCache = this.containers.find(c => c.name === 'redis-cache')
    if (rCache && rCache.status === 'ok') {
      rCache.status = 'update'
      rCache.remote_digest = 'sha256:new_redis_remote_digest_hash_val_8888'
      rCache.checked_at = this.lastCheckTime.substring(0, 16)
      console.log('[Mock DB] 可视化触发检测：redis-cache 发现可用升级！')
    }
    notifyWSStatusChange()
  }

  getDynamicImages(): MockImage[] {
    return this.images.map(img => {
      const matchedContainers = this.containers.filter(c => {
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

  updateContainerRunningState(name: string, running: boolean) {
    const c = this.containers.find((item) => item.name === name)
    if (c) {
      c.running = running
      console.log(`[Mock DB] 容器 ${name} 运行状态已变更为: ${running}`)
      notifyWSStatusChange()
    }
  }

  updateContainerDeferState(name: string, days: number) {
    const c = this.containers.find((item) => item.name === name)
    if (c) {
      if (days > 0) {
        c.status = 'deferred'
        const targetDate = new Date()
        targetDate.setDate(targetDate.getDate() + days)
        c.defer_until = targetDate.toISOString().split('T')[0]
        console.log(`[Mock DB] 容器 ${name} 已暂挂 ${days} 天，至 ${c.defer_until}`)
      } else {
        c.status = 'update'
        c.defer_until = null
        console.log(`[Mock DB] 容器 ${name} 已恢复升级检测`)
      }
      notifyWSStatusChange()
    }
  }

  updateContainerRollbackState(name: string, hasRollback: boolean) {
    const c = this.containers.find((item) => item.name === name)
    if (c) {
      c.has_rollback = hasRollback
      c.rollback_expires = null
      console.log(`[Mock DB] 容器 ${name} 的回滚备份已清除`)
      notifyWSStatusChange()
    }
  }

  // 根据当前升级模式进行最终状态转移
  onTaskFinished(name: string, type: 'update' | 'rollback') {
    const c = this.containers.find((item) => item.name === name)
    const targetImage = c ? c.image : 'unknown'
    let finalStatus = 'success'
    let errorMsg = ''

    if (type === 'update') {
      if (this.upgradeMode === 'success') {
        c!.status = 'ok'
        c!.local_digest = c!.remote_digest // 升级成功，对齐摘要
        c!.has_rollback = true
        const expireDate = new Date()
        expireDate.setHours(expireDate.getHours() + 24)
        c!.rollback_expires = expireDate.toISOString().replace('T', ' ').substring(0, 16)
        c!.checked_at = new Date().toISOString().replace('T', ' ').substring(0, 16)
        c!.running = true
        console.log(`[Mock DB] 容器 ${name} 升级成功`)
      } else if (this.upgradeMode === 'fail_pull') {
        finalStatus = 'failed'
        errorMsg = 'error: Connection timed out to registry'
        console.log(`[Mock DB] 容器 ${name} 由于拉取失败升级未改变状态`)
      } else if (this.upgradeMode === 'fail_start') {
        finalStatus = 'failed'
        errorMsg = 'error: Container failed to stay up'
        // 自愈回滚，维持原状
        c!.status = 'update'
        c!.has_rollback = false
        c!.rollback_expires = null
        c!.running = true
        console.log(`[Mock DB] 容器 ${name} 启动失败并成功自愈回滚`)
      }
    } else {
      // 回滚
      c!.status = 'update'
      c!.local_digest = 'sha256:rollback_old_hash_value_1234567890abcdef'
      c!.has_rollback = false
      c!.rollback_expires = null
      c!.running = true
      console.log(`[Mock DB] 容器 ${name} 回滚成功`)
    }

    // 归档日志到持久化内存
    const logLines = this.getMockTaskLogLines(name, type)
    this.updateLogs.set(name, logLines)

    // 写入升级历史表
    this.history.unshift({
      id: Date.now(),
      ContainerName: name,
      Image: targetImage,
      UpdatedAt: new Date().toISOString(),
      Status: finalStatus === 'success' ? 'success' : errorMsg
    })

    this.activeTask = null
    console.log(`[Mock Queue] 容器 ${name} 的 ${type} 任务已处理完毕，状态: ${finalStatus}`)

    // 调出队列的下一个任务
    if (this.queuedTasks.length > 0) {
      const nextTask = this.queuedTasks.shift()!
      this.activeTask = nextTask
      console.log(`[Mock Queue] 从队列拉起下一个任务:`, nextTask)
      if (activeMockWS) {
        activeMockWS.startMockLogStream(nextTask.container_name, nextTask.type)
      }
    }
    notifyWSStatusChange()
  }

  // 动态生成升级/回滚任务日志流数据
  getMockTaskLogLines(containerName: string, mode: 'update' | 'rollback'): string[] {
    if (mode === 'rollback') {
      return [
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

    // update 模式，根据仿真模式决定日志
    if (this.upgradeMode === 'success') {
      return [
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
    } else if (this.upgradeMode === 'fail_pull') {
      return [
        `[INFO] 开始对容器 ${containerName} 进行升级操作...`,
        `[INFO] 正在分析本地与远端分发凭证...`,
        `[PULL] 正在从 registry 下载新镜像层: sha256:7f082ec15d86...`,
        `[ERROR] 镜像拉取失败: Connection timed out to registry (网络仿真异常)`,
        `[ERROR] 容器 ${containerName} 升级流程中断退出`
      ]
    } else {
      // fail_start 启动失败自愈
      return [
        `[INFO] 开始对容器 ${containerName} 进行升级操作...`,
        `[INFO] 正在分析本地与远端分发凭证...`,
        `[PULL] 正在从 registry 下载新镜像层: sha256:7f082ec15d86...`,
        `[PULL] 最新镜像层拉取完毕，已校验校验和`,
        `[INFO] 正在创建升级前镜像备份点: ${containerName}_old`,
        `[INFO] 成功记录重启配置策略.`,
        `[INFO] 正在优雅停止运行中的旧容器...`,
        `[INFO] 旧容器实例已清理`,
        `[INFO] 基于新拉取镜像重新组装参数重建中...`,
        `[INFO] 容器 ${containerName} 正在唤醒启动...`,
        `[ERROR] 启动新容器失败: port 80 is already allocated by another service`,
        `[INFO] 检测到容器启动异常，正在执行安全回滚自愈机制...`,
        `[INFO] 正在物理清除故障新容器实例...`,
        `[INFO] 将备份容器 ${containerName}_old 重命名还原为 ${containerName}`,
        `[INFO] 恢复原有的重启策略策略和网络网关绑定`,
        `[SUCCESS] 回滚恢复成功，原容器已上线运行`
      ]
    }
  }

  getMockLogs(name: string): string {
    return `[Mock Logs for ${name}]
2026-07-09T12:00:00.124Z [info] Starting mock production build...
2026-07-09T12:00:01.345Z [info] Listening on HTTP connection: port 80
2026-07-09T12:00:05.678Z [info] Accepted external socket mapping from 127.0.0.1
2026-07-09T12:01:20.999Z [info] Database heartbeat validation: 0.12ms success
2026-07-09T12:02:40.456Z [info] Keep-alive worker check: status=idle load=0.01%
`
  }

  addTaskToQueue(name: string, type: 'update' | 'rollback') {
    const newTask: TaskItem = {
      container_name: name,
      type,
      added_at: new Date().toISOString()
    }

    if (this.activeTask) {
      this.queuedTasks.push(newTask)
      console.log(`[Mock Queue] 任务已加入排队:`, newTask)
    } else {
      this.activeTask = newTask
      console.log(`[Mock Queue] 任务已立即运行:`, newTask)
      if (activeMockWS) {
        activeMockWS.startMockLogStream(name, type)
      }
    }
    notifyWSStatusChange()
  }
}

// 实例化全局仿真数据库
const db = new SimulationDatabase()
let activeMockWS: any = null

function notifyWSStatusChange() {
  if (activeMockWS) {
    activeMockWS.sendStatusUpdate()
  }
}

// --- 劫持 Axios defaults.adapter ---
const originalAdapter = axios.defaults.adapter

axios.defaults.adapter = async function (config) {
  const url = config.url || ''
  const method = config.method?.toLowerCase()

  console.log(`[Mock Axios] ${method?.toUpperCase()} -> ${url}`, config.data)

  // 1. /api/status 状态汇总
  if (url.includes('/api/status')) {
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
      headers: config.headers || {},
      config
    }
  }

  // 1.5 获取可用镜像 Tags 仿真
  if (url.includes('/api/image/tags')) {
    return {
      data: [
        '1.27.2', '1.27.1', '1.27.0', '1.26.5', '1.26.4', '1.26.3', '1.26.2', '1.26.1', '1.26.0',
        '1.25.9', '1.25.8', '1.25.7', '1.25.6', '1.25.5', '1.25.4', '1.25.3', '1.25.2', '1.25.1', '1.25.0',
        '1.24.4'
      ],
      status: 200,
      statusText: 'OK',
      headers: config.headers || {},
      config
    }
  }

  // 2. 容器生命周期控制
  const startMatch = url.match(/\/api\/container\/([^/]+)\/start/)
  if (startMatch) {
    db.updateContainerRunningState(startMatch[1], true)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  const stopMatch = url.match(/\/api\/container\/([^/]+)\/stop/)
  if (stopMatch) {
    db.updateContainerRunningState(stopMatch[1], false)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  const restartMatch = url.match(/\/api\/container\/([^/]+)\/restart/)
  if (restartMatch) {
    db.updateContainerRunningState(restartMatch[1], true)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 3. 暂挂升级
  const deferMatch = url.match(/\/api\/defer\/([^/]+)/)
  if (deferMatch) {
    let days = 7
    try {
      days = JSON.parse(config.data || '{}').days || 7
    } catch (e) {}
    db.updateContainerDeferState(deferMatch[1], days)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 4. 恢复检测
  const undeferMatch = url.match(/\/api\/undefer\/([^/]+)/)
  if (undeferMatch) {
    db.updateContainerDeferState(undeferMatch[1], 0)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 5. 删除备份
  const backupDeleteMatch = url.match(/\/api\/backup\/([^/]+)/)
  if (backupDeleteMatch) {
    db.updateContainerRollbackState(backupDeleteMatch[1], false)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 6. 获取常规日志
  const logsMatch = url.match(/\/api\/container\/([^/]+)\/logs/)
  if (logsMatch) {
    return {
      data: { logs: db.getMockLogs(logsMatch[1]) },
      status: 200,
      statusText: 'OK',
      headers: config.headers || {},
      config
    }
  }

  // 7. 触发升级
  const updateMatch = url.match(/\/api\/update\/([^/]+)/)
  if (updateMatch) {
    db.addTaskToQueue(updateMatch[1], 'update')
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 8. 触发回滚
  const rollbackMatch = url.match(/\/api\/rollback\/([^/]+)/)
  if (rollbackMatch) {
    db.addTaskToQueue(rollbackMatch[1], 'rollback')
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 9. 获取镜像列表
  if (url.includes('/api/images') && method === 'get') {
    return {
      data: db.getDynamicImages(),
      status: 200,
      statusText: 'OK',
      headers: config.headers || {},
      config
    }
  }

  // 10. 删除指定镜像
  if (url.includes('/api/image') && method === 'delete') {
    const targetId = config.params?.id || ''
    const dynamicImages = db.getDynamicImages()
    const targetImg = dynamicImages.find(img => img.id === targetId)

    if (targetImg && targetImg.containers && targetImg.containers.length > 0) {
      return {
        data: { error: `Conflict, image is being used by container(s): ${targetImg.containers.join(', ')}` },
        status: 500,
        statusText: 'Internal Server Error',
        headers: config.headers || {},
        config
      }
    }

    db.images = db.images.filter(img => img.id !== targetId)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 11. 清理虚悬镜像 (Prune)
  if (url.includes('/api/images/prune') && method === 'post') {
    const dynamicImages = db.getDynamicImages()
    const dangling = dynamicImages.filter(img => img.tags.includes('<none>:<none>'))
    const space = dangling.reduce((acc, curr) => acc + curr.size, 0)
    const count = dangling.length
    db.images = db.images.filter(img => !img.tags.includes('<none>:<none>'))
    return {
      data: { space_reclaimed: space, deleted_count: count },
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
    const beforeLen = db.queuedTasks.length
    db.queuedTasks = db.queuedTasks.filter(t => t.container_name !== name)
    const success = db.queuedTasks.length < beforeLen
    notifyWSStatusChange()
    return { data: { success }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 14. 触发手动检查新版本
  if (url.includes('/api/check') && method === 'post') {
    setTimeout(() => {
      db.triggerUpdateFound()
    }, 1200)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 15. 获取/删除历史操作日志
  const updateLogMatch = url.match(/\/api\/update-log\/([^/]+)/)
  if (updateLogMatch) {
    const name = updateLogMatch[1]
    if (method === 'get') {
      const hasLog = db.updateLogs.has(name)
      return {
        data: { found: hasLog, logs: db.updateLogs.get(name) || [] },
        status: 200,
        statusText: 'OK',
        headers: config.headers || {},
        config
      }
    } else if (method === 'delete') {
      db.updateLogs.delete(name)
      db.history = db.history.filter(h => h.ContainerName !== name)
      notifyWSStatusChange()
      return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
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

  // 13. 获取/保存配置
  if (url.includes('/api/settings')) {
    if (method === 'get') {
      return { data: db.settings, status: 200, statusText: 'OK', headers: config.headers || {}, config }
    } else if (method === 'post') {
      try {
        const body = JSON.parse(config.data || '{}')
        db.settings = { ...db.settings, ...body }
      } catch (e) {}
      return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
    }
  }

  // 13.5 获取/清理系统运行日志 Mock
  if (url.includes('/api/system/logs')) {
    if (method === 'get') {
      const mockSysLogs = [
        `2026/07/09 21:00:00 [INFO] 正在启动 docker-updater...`,
        `2026/07/09 21:00:01 [INFO] 数据库初始化成功 (sqlite CGO-free)`,
        `2026/07/09 21:00:02 [INFO] 任务队列: 初始化全局任务队列管理器并启动后台 Worker 协程`,
        `2026/07/09 21:00:03 [INFO] 成功加载定时任务调度器 (cron)`,
        `2026/07/09 21:05:00 [INFO] 开始扫描本地活动容器进行版本比对检查...`,
        `2026/07/09 21:05:02 [INFO] 服务 nginx-app 存在可用升级 (本地: sha256:old_nginx_digest, 远端: sha256:new_nginx_digest)`,
        `2026/07/09 21:05:03 [INFO] 容器扫描比对检查结束。当前共发现 1 个待升级服务。`,
        `2026/07/09 21:10:00 [INFO] 开始执行定时任务自动清理失效备份点...`,
        `2026/07/09 21:10:01 [SUCCESS] 清理历史过期备份容器成功 (共清理 0 个容器)`
      ]
      return { data: { logs: mockSysLogs }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
    } else if (method === 'delete') {
      return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
    }
  }

  // 16. 私有仓凭据列表的增删改查
  if (url.includes('/api/registries')) {
    if (method === 'get') {
      return { data: db.registries, status: 200, statusText: 'OK', headers: config.headers || {}, config }
    } else if (method === 'post') {
      try {
        const cred = JSON.parse(config.data || '{}')
        if (cred.id && cred.id > 0) {
          const exist = db.registries.find(r => r.id === cred.id)
          if (exist) {
            exist.registry = cred.registry || exist.registry
            exist.username = cred.username || exist.username
            exist.updated_at = new Date().toISOString().replace('T', ' ').substring(0, 19)
          }
        } else {
          const newId = db.registries.length > 0 ? Math.max(...db.registries.map(r => r.id)) + 1 : 1
          db.registries.push({
            id: newId,
            registry: cred.registry,
            username: cred.username,
            updated_at: new Date().toISOString().replace('T', ' ').substring(0, 19)
          })
        }
      } catch (e) {}
      return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
    }
  }

  const deleteRegMatch = url.match(/\/api\/registries\/([^/]+)/)
  if (deleteRegMatch && method === 'delete') {
    const targetId = parseInt(deleteRegMatch[1], 10)
    db.registries = db.registries.filter(r => r.id !== targetId)
    return { data: { ok: true }, status: 200, statusText: 'OK', headers: config.headers || {}, config }
  }

  // 其他请求调用原装
  if (originalAdapter) {
    // @ts-ignore
    return originalAdapter(config)
  }

  return { data: {}, status: 404, statusText: 'Not Found', headers: {}, config }
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
  sysLogInterval: any = null

  constructor(url: string) {
    this.url = url
    console.log(`[Mock WS] 拦截连接建立: ${url}`)

    setTimeout(() => {
      if (!db.isWSConnected) {
        this.readyState = 3 // CLOSED
        if (this.onclose) this.onclose()
        return
      }

      this.readyState = 1 // OPEN
      if (this.onopen) this.onopen()

      // 连接成功后下发第一包状态
      this.sendStatusUpdate()
      this.startSysLogBroadcast()
    }, 150)

    activeMockWS = this
  }

  send(data: string) {
    try {
      const parsed = JSON.parse(data)
      if (parsed.type === 'ping') {
        if (this.onmessage && this.readyState === 1) {
          this.onmessage(
            new MessageEvent('message', {
              data: JSON.stringify({ type: 'pong' })
            })
          )
        }
      }
    } catch (e) {}
  }

  close() {
    this.readyState = 3 // CLOSED
    this.stopSysLogBroadcast()
    if (this.onclose) this.onclose()
    if (activeMockWS === this) {
      activeMockWS = null
    }
  }

  // 被调试控制面板调用的强制关闭，用来模拟网络断线
  simulateDisconnect() {
    this.readyState = 3
    this.stopSysLogBroadcast()
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
            payload: {
              containers: db.containers,
              last_check: db.lastCheckTime,
              history: db.history,
              active: db.activeTask,
              queued: db.queuedTasks
            }
          })
        })
      )
    }
  }

  startSysLogBroadcast() {
    this.sysLogInterval = setInterval(() => {
      if (this.readyState !== 1) {
        clearInterval(this.sysLogInterval)
        return
      }
      if (this.onmessage) {
        const level = Math.random() > 0.4 ? '[INFO]' : Math.random() > 0.5 ? '[WARNING]' : '[SUCCESS]'
        const now = new Date()
        const dateStr = now.getFullYear() + '/' + 
          String(now.getMonth() + 1).padStart(2, '0') + '/' + 
          String(now.getDate()).padStart(2, '0') + ' ' + 
          now.toTimeString().split(' ')[0]
        const line = `${dateStr} ${level} [Simulation] 守护进程巡检中: 正在拉取 Docker Hub 连通情况...`
        this.onmessage(
          new MessageEvent('message', {
            data: JSON.stringify({
              type: 'syslog',
              payload: { line }
            })
          })
        )
      }
    }, 15000)
  }

  stopSysLogBroadcast() {
    if (this.sysLogInterval) {
      clearInterval(this.sysLogInterval)
    }
  }

  startMockLogStream(containerName: string, mode: 'update' | 'rollback') {
    let step = 0
    const logs = db.getMockTaskLogLines(containerName, mode)

    const logInterval = setInterval(() => {
      if (this.readyState !== 1) {
        clearInterval(logInterval)
        return
      }

      if (step >= logs.length) {
        clearInterval(logInterval)
        db.onTaskFinished(containerName, mode)
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
    }, 850)
  }
}

window.WebSocket = function (url: string | URL, protocols?: string | string[]) {
  const urlStr = typeof url === 'string' ? url : url.toString()
  if (urlStr.includes('/app/docker-updater/api/ws')) {
    return new MockWebSocket(urlStr) as any
  }
  return new OriginalWebSocket(url, protocols)
} as any
window.WebSocket.prototype = OriginalWebSocket.prototype


function injectSimulationPanel() {
  if (document.getElementById('simulation-panel-hud')) return

  const panel = document.createElement('div')
  panel.id = 'simulation-panel-hud'

  // 极简扁平样式
  panel.style.position = 'fixed'
  panel.style.bottom = '20px'
  panel.style.right = '20px'
  panel.style.zIndex = '999999'
  panel.style.width = '240px'
  panel.style.backgroundColor = '#ffffff'
  panel.style.border = '1px solid #d1d5db'
  panel.style.borderRadius = '8px'
  panel.style.boxShadow = '0 4px 12px rgba(0, 0, 0, 0.08)'
  panel.style.fontFamily = 'Consolas, Monaco, monospace, sans-serif'
  panel.style.fontSize = '11px'
  panel.style.color = '#111827'
  panel.style.transition = 'width 0.2s ease, opacity 0.2s ease'
  panel.style.overflow = 'hidden'
  panel.style.userSelect = 'none'

  // 适配暗色模式检测
  const updateTheme = () => {
    const isDark = document.documentElement.classList.contains('dark')
    if (isDark) {
      panel.style.backgroundColor = '#1f2937'
      panel.style.border = '1px solid #374151'
      panel.style.boxShadow = '0 4px 12px rgba(0, 0, 0, 0.3)'
      panel.style.color = '#f9fafb'
    } else {
      panel.style.backgroundColor = '#ffffff'
      panel.style.border = '1px solid #d1d5db'
      panel.style.boxShadow = '0 4px 12px rgba(0, 0, 0, 0.08)'
      panel.style.color = '#111827'
    }
  }

  // 观察暗色模式类名变化
  const observer = new MutationObserver(updateTheme)
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  updateTheme()

  // 面板头部，用于折叠控制和拖拽操作
  const header = document.createElement('div')
  header.style.display = 'flex'
  header.style.justifyContent = 'space-between'
  header.style.alignItems = 'center'
  header.style.padding = '8px 12px'
  header.style.backgroundColor = 'rgba(0, 0, 0, 0.03)'
  header.style.borderBottom = '1px solid rgba(0, 0, 0, 0.08)'
  header.style.cursor = 'grab'

  const title = document.createElement('span')
  title.innerText = '仿真调试'
  title.style.fontWeight = '700'
  title.style.color = '#0066cc'
  title.style.fontSize = '10px'
  title.style.letterSpacing = '0.5px'

  const toggleBtn = document.createElement('span')
  toggleBtn.innerText = '[-]'
  toggleBtn.style.fontSize = '10px'
  toggleBtn.style.cursor = 'pointer'
  toggleBtn.style.fontWeight = 'bold'
  toggleBtn.style.color = '#6b7280'

  header.appendChild(title)
  header.appendChild(toggleBtn)
  panel.appendChild(header)

  // 面板主体内容区
  const body = document.createElement('div')
  body.style.padding = '12px'
  body.style.display = 'flex'
  body.style.flexDirection = 'column'
  body.style.gap = '8px'

  // --- 控件 1：触发可用升级 ---
  const triggerBtn = document.createElement('button')
  triggerBtn.innerText = '模拟发现可用更新'
  styleButton(triggerBtn, '#0066cc')
  triggerBtn.onclick = () => {
    db.triggerUpdateFound()
    showToast('已模拟注入新镜像版本')
  }
  body.appendChild(triggerBtn)

  // --- 控件 2：设置升级成功/失败模式 ---
  const modeLabel = document.createElement('div')
  modeLabel.innerText = '升级部署模拟场景：'
  modeLabel.style.fontWeight = '600'
  modeLabel.style.fontSize = '9px'
  modeLabel.style.color = '#6b7280'
  body.appendChild(modeLabel)

  const modeSelect = document.createElement('select')
  modeSelect.style.width = '100%'
  modeSelect.style.padding = '5px 8px'
  modeSelect.style.borderRadius = '4px'
  modeSelect.style.border = '1px solid #d1d5db'
  modeSelect.style.backgroundColor = 'transparent'
  modeSelect.style.color = 'inherit'
  modeSelect.style.outline = 'none'
  modeSelect.style.fontSize = '11px'
  modeSelect.style.cursor = 'pointer'
  modeSelect.style.fontFamily = 'inherit'

  const opt1 = document.createElement('option')
  opt1.value = 'success'
  opt1.innerText = '常规升级成功'
  opt1.style.backgroundColor = 'var(--n-color, #fff)'
  opt1.style.color = '#111827'

  const opt2 = document.createElement('option')
  opt2.value = 'fail_pull'
  opt2.innerText = '模拟镜像拉取超时'
  opt2.style.backgroundColor = 'var(--n-color, #fff)'
  opt2.style.color = '#111827'

  const opt3 = document.createElement('option')
  opt3.value = 'fail_start'
  opt3.innerText = '模拟闪退并自愈回滚'
  opt3.style.backgroundColor = 'var(--n-color, #fff)'
  opt3.style.color = '#111827'

  modeSelect.appendChild(opt1)
  modeSelect.appendChild(opt2)
  modeSelect.appendChild(opt3)

  modeSelect.onchange = (e) => {
    db.upgradeMode = (e.target as HTMLSelectElement).value as any
    showToast(`升级场景切换为: ${(e.target as HTMLSelectElement).selectedOptions[0].text}`)
  }
  body.appendChild(modeSelect)

  // --- 控件 3：模拟网络断线重连 ---
  const wsBtn = document.createElement('button')
  wsBtn.innerText = '模拟 WebSocket 断开'
  styleButton(wsBtn, '#ef4444')
  wsBtn.onclick = () => {
    if (db.isWSConnected) {
      db.isWSConnected = false
      wsBtn.innerText = '恢复 WebSocket 连接'
      wsBtn.style.backgroundColor = 'rgba(16, 185, 129, 0.08)'
      wsBtn.style.border = '1px solid rgba(16, 185, 129, 0.2)'
      wsBtn.style.color = '#10b981'
      if (activeMockWS) {
        activeMockWS.simulateDisconnect()
      }
      showToast('WebSocket 通道已切断')
    } else {
      db.isWSConnected = true
      wsBtn.innerText = '模拟 WebSocket 断开'
      wsBtn.style.backgroundColor = 'rgba(239, 68, 68, 0.08)'
      wsBtn.style.border = '1px solid rgba(239, 68, 68, 0.2)'
      wsBtn.style.color = '#ef4444'
      showToast('WebSocket 通道已恢复，重连中')
    }
  }
  body.appendChild(wsBtn)

  // --- 控件 4：重置数据 ---
  const resetBtn = document.createElement('button')
  resetBtn.innerText = '重置测试数据'
  styleButton(resetBtn, '#6b7280')
  resetBtn.onclick = () => {
    db.reset()
    notifyWSStatusChange()
    showToast('测试状态库已重置')
  }
  body.appendChild(resetBtn)

  panel.appendChild(body)
  document.body.appendChild(panel)

  // 折叠状态逻辑
  let isFolded = false
  const toggleFold = () => {
    isFolded = !isFolded
    if (isFolded) {
      body.style.display = 'none'
      toggleBtn.innerText = '[+]'
      panel.style.width = '150px'
    } else {
      body.style.display = 'flex'
      toggleBtn.innerText = '[-]'
      panel.style.width = '240px'
    }
  }

  // 点击折叠按钮触发
  toggleBtn.onclick = (e) => {
    e.stopPropagation()
    toggleFold()
  }

  // 双击头部也可触发折叠
  header.ondblclick = (e) => {
    e.stopPropagation()
    toggleFold()
  }

  // --- 统一的鼠标与触摸拖拽逻辑 ---
  let isDragging = false
  let startX = 0
  let startY = 0
  let startLeft = 0
  let startTop = 0

  const onDragStart = (clientX: number, clientY: number, target: EventTarget | null) => {
    if (target === toggleBtn) return
    isDragging = true
    startX = clientX
    startY = clientY

    const rect = panel.getBoundingClientRect()
    startLeft = rect.left
    startTop = rect.top

    // 转换为 left/top 绝对物理定位以支持拖拽
    panel.style.bottom = 'auto'
    panel.style.right = 'auto'
    panel.style.left = startLeft + 'px'
    panel.style.top = startTop + 'px'
    header.style.cursor = 'grabbing'
  }

  const onDragMove = (clientX: number, clientY: number) => {
    if (!isDragging) return
    const deltaX = clientX - startX
    const deltaY = clientY - startY

    let newLeft = startLeft + deltaX
    let newTop = startTop + deltaY

    // 限制拖拽边界在可视屏幕范围之内
    const maxLeft = window.innerWidth - panel.offsetWidth
    const maxTop = window.innerHeight - panel.offsetHeight
    newLeft = Math.max(0, Math.min(newLeft, maxLeft))
    newTop = Math.max(0, Math.min(newTop, maxTop))

    panel.style.left = newLeft + 'px'
    panel.style.top = newTop + 'px'
  }

  const onDragEnd = () => {
    if (isDragging) {
      isDragging = false
      header.style.cursor = 'grab'
    }
  }

  // 1. 鼠标事件
  header.addEventListener('mousedown', (e: MouseEvent) => {
    onDragStart(e.clientX, e.clientY, e.target)
    e.preventDefault()
  })

  document.addEventListener('mousemove', (e: MouseEvent) => {
    onDragMove(e.clientX, e.clientY)
  })

  document.addEventListener('mouseup', () => {
    onDragEnd()
  })

  // 2. 移动端触摸事件 (Touch Events)
  header.addEventListener('touchstart', (e: TouchEvent) => {
    const touch = e.touches[0]
    onDragStart(touch.clientX, touch.clientY, e.target)
    if (e.target !== toggleBtn) {
      e.preventDefault()
    }
  }, { passive: false })

  document.addEventListener('touchmove', (e: TouchEvent) => {
    if (!isDragging) return
    const touch = e.touches[0]
    onDragMove(touch.clientX, touch.clientY)
    e.preventDefault() // 阻止移动端页面跟手滚动，彻底解决视口跟着一块跑的问题
  }, { passive: false })

  document.addEventListener('touchend', () => {
    onDragEnd()
  })
}

// 辅助样式定制函数，打造极致质朴、简洁的测试按钮质感
function styleButton(btn: HTMLButtonElement, color: string = '#0066cc') {
  btn.style.width = '100%'
  btn.style.padding = '5px 10px'
  btn.style.borderRadius = '4px'
  btn.style.border = '1px solid rgba(0, 0, 0, 0.08)'
  btn.style.outline = 'none'
  btn.style.fontSize = '11px'
  btn.style.fontWeight = '600'
  btn.style.cursor = 'pointer'
  btn.style.fontFamily = 'inherit'

  if (color === '#ef4444') {
    btn.style.backgroundColor = 'rgba(239, 68, 68, 0.08)'
    btn.style.border = '1px solid rgba(239, 68, 68, 0.2)'
    btn.style.color = '#ef4444'
  } else if (color === '#6b7280') {
    btn.style.backgroundColor = 'rgba(107, 114, 128, 0.08)'
    btn.style.border = '1px solid rgba(107, 114, 128, 0.2)'
    btn.style.color = '#6b7280'
  } else {
    btn.style.backgroundColor = 'rgba(0, 102, 204, 0.08)'
    btn.style.border = '1px solid rgba(0, 102, 204, 0.2)'
    btn.style.color = '#0066cc'
  }
}

// 仿真控制台的浮动小提示（Toast）
function showToast(msg: string) {
  const toast = document.createElement('div')
  toast.innerText = msg
  toast.style.position = 'fixed'
  toast.style.bottom = '20px'
  toast.style.left = '50%'
  toast.style.transform = 'translateX(-50%) translateY(20px)'
  toast.style.zIndex = '9999999'
  toast.style.padding = '6px 12px'
  toast.style.borderRadius = '4px'
  toast.style.backgroundColor = '#1f2937'
  toast.style.border = '1px solid #374151'
  toast.style.color = '#f9fafb'
  toast.style.fontSize = '10px'
  toast.style.fontWeight = '600'
  toast.style.opacity = '0'
  toast.style.boxShadow = '0 2px 8px rgba(0,0,0,0.15)'
  toast.style.transition = 'all 0.2s ease'
  document.body.appendChild(toast)

  setTimeout(() => {
    toast.style.opacity = '1'
    toast.style.transform = 'translateX(-50%) translateY(0)'
  }, 20)

  setTimeout(() => {
    toast.style.opacity = '0'
    toast.style.transform = 'translateX(-50%) translateY(-10px)'
    setTimeout(() => {
      document.body.removeChild(toast)
    }, 200)
  }, 2000)
}

if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', injectSimulationPanel)
  } else {
    injectSimulationPanel()
  }
}

console.log('[Mock System] 成功劫持全局 Axios 与 WebSocket API，极简仿真控制面板 HUD 已注入宿主页面！')


