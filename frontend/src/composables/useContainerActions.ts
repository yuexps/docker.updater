import { ref, nextTick, onScopeDispose } from 'vue'
import { useMessage, useDialog } from 'naive-ui'
import axios from 'axios'
import wsService from '../utils/websocket'
import { copyToClipboard as copyToClipboardUtil } from '../utils/clipboard'

export function useContainerActions(apiBase = '/app/docker-updater/api') {
  const message = useMessage()
  const dialog = useDialog()

  const operatingContainers = ref<Set<string>>(new Set())
  const logModalVisible = ref<boolean>(false)
  const logLines = ref<string[]>([])
  const logRunning = ref<boolean>(false)
  const terminalLog = ref<HTMLDivElement | null>(null)

  const deferModalVisible = ref<boolean>(false)
  const deferTarget = ref<string>('')
  const deferDays = ref<number>(7)

  const diagnosticsVisible = ref<boolean>(false)
  const diagnosticsLogs = ref<string>('')

  const updateVersionModalVisible = ref<boolean>(false)
  const updateVersionTarget = ref<string>('')
  const updateVersionCurrentImage = ref<string>('')
  const updateVersionValue = ref<string>('')

  let activeUnsubscribeLogs: (() => void) | null = null

  // 缩略哈希算法
  const shortDigest = (digest: string) => {
    if (!digest) return ''
    if (digest.startsWith('sha256:')) {
      return digest.slice(7, 19)
    }
    return digest.slice(0, 12)
  }

  // 复制文本至剪贴板
  const copyToClipboard = (text: string, label: string = '镜像名称') => {
    return copyToClipboardUtil(text, label, message)
  }

  // 升级单个容器
  const updateContainer = (name: string, targetImage?: string) => {
    logLines.value = []
    logModalVisible.value = true
    logRunning.value = true

    const url = targetImage
      ? `${apiBase}/update/${name}?target_image=${encodeURIComponent(targetImage)}`
      : `${apiBase}/update/${name}`

    axios.get(url).catch(() => {
      message?.error('触发升级失败')
      logRunning.value = false
    })

    if (activeUnsubscribeLogs) activeUnsubscribeLogs()
    activeUnsubscribeLogs = wsService.subscribeLogs(name, ({ message: msg }) => {
      logLines.value.push(msg)
      scrollTerminal()
      if (msg.includes('任务全部完成') || msg.includes('已成功') || msg.includes('恢复成功') || msg.includes('[ERROR]')) {
        logRunning.value = false
      }
    })
  }

  // 批量升级
  const startBulkUpdate = async (targets: string[], onClearSelection?: () => void) => {
    if (targets.length === 0) return

    logLines.value = []
    logModalVisible.value = true
    logRunning.value = true

    logLines.value.push(`[INFO] 批量升级开始，队列总数: ${targets.length}`)

    for (let i = 0; i < targets.length; i++) {
      const name = targets[i]
      logLines.value.push(`========================================`)
      logLines.value.push(`[INFO] 正在升级当前服务 (${i + 1}/${targets.length}): ${name}`)
      scrollTerminal()

      await new Promise<void>((resolve) => {
        axios.get(`${apiBase}/update/${name}`).catch(() => {
          logLines.value.push(`[ERROR] 触发容器 ${name} 升级失败`)
          resolve()
        })

        const unsub = wsService.subscribeLogs(name, ({ message: msg }) => {
          logLines.value.push(msg)
          scrollTerminal()
          if (msg.includes('任务全部完成') || msg.includes('已成功') || msg.includes('恢复成功') || msg.includes('[ERROR]')) {
            unsub()
            resolve()
          }
        })
      })
    }

    logLines.value.push(`========================================`)
    logLines.value.push(`[INFO] 批量升级队列全部作业运行完毕`)
    logRunning.value = false
    if (onClearSelection) onClearSelection()
  }

  // 获取运行日志
  const showLogs = async (name: string) => {
    try {
      const res = await axios.get(`${apiBase}/container/${name}/logs`)
      diagnosticsLogs.value = res.data.logs || '未检索到输出日志'
      diagnosticsVisible.value = true
    } catch (err) {
      message?.error('获取容器日志失败')
    }
  }

  // 暂挂控制
  const openDeferModal = (name: string) => {
    deferTarget.value = name
    deferDays.value = 7
    deferModalVisible.value = true
  }

  const submitDefer = async (onSuccess?: () => void) => {
    try {
      await axios.post(`${apiBase}/defer/${deferTarget.value}`, { days: deferDays.value })
      message?.success(`已暂挂服务 ${deferTarget.value} 的版本检测`)
      deferModalVisible.value = false
      if (onSuccess) onSuccess()
    } catch (err) {
      message?.error('暂挂失败')
    }
  }

  const undeferContainer = async (name: string, onSuccess?: () => void) => {
    try {
      await axios.post(`${apiBase}/undefer/${name}`)
      message?.success('已恢复版本正常检测')
      if (onSuccess) onSuccess()
    } catch (err) {
      message?.error('恢复检测失败')
    }
  }

  // 容器生命周期 API
  const startContainer = async (name: string) => {
    const actionKey = `${name}:start`
    operatingContainers.value.add(actionKey)
    try {
      await axios.post(`${apiBase}/container/${name}/start`, null, { timeout: 20000 })
      message?.success(`已成功启动容器 ${name}`)
    } catch {
      message?.error(`启动容器 ${name} 失败`)
    } finally {
      operatingContainers.value.delete(actionKey)
    }
  }

  const stopContainer = async (name: string) => {
    const actionKey = `${name}:stop`
    operatingContainers.value.add(actionKey)
    try {
      await axios.post(`${apiBase}/container/${name}/stop`, null, { timeout: 15000 })
      message?.success(`已成功停止容器 ${name}`)
    } catch {
      message?.error(`停止容器 ${name} 失败`)
    } finally {
      operatingContainers.value.delete(actionKey)
    }
  }

  const restartContainer = async (name: string) => {
    const actionKey = `${name}:restart`
    operatingContainers.value.add(actionKey)
    try {
      await axios.post(`${apiBase}/container/${name}/restart`, null, { timeout: 25000 })
      message?.success(`已成功重启容器 ${name}`)
    } catch {
      message?.error(`重启容器 ${name} 失败`)
    } finally {
      operatingContainers.value.delete(actionKey)
    }
  }

  // 检查单个容器更新（异步触发，结果由 WS 推送刷新）
  const checkContainer = async (name: string) => {
    const actionKey = `${name}:check`
    operatingContainers.value.add(actionKey)
    try {
      await axios.post(`${apiBase}/check/${name}`)
      message?.info(`容器 ${name} 版本检测已触发，结果稍后自动刷新`)
    } catch {
      message?.error(`触发容器 ${name} 版本检测失败`)
    } finally {
      operatingContainers.value.delete(actionKey)
    }
  }

  const closeLogModal = () => {
    if (activeUnsubscribeLogs) {
      activeUnsubscribeLogs()
      activeUnsubscribeLogs = null
    }
    logModalVisible.value = false
    logLines.value = []
  }

  const scrollTerminal = () => {
    nextTick(() => {
      if (terminalLog.value) {
        terminalLog.value.scrollTop = terminalLog.value.scrollHeight
      }
    })
  }

  const cleanup = () => {
    if (activeUnsubscribeLogs) {
      activeUnsubscribeLogs()
      activeUnsubscribeLogs = null
    }
  }

  try {
    onScopeDispose(() => {
      cleanup()
    })
  } catch (e) {
    // 忽略非组件上下文调用的异常
  }

  const openUpdateVersionModal = (name: string, currentImage: string) => {
    updateVersionTarget.value = name
    updateVersionCurrentImage.value = currentImage
    updateVersionValue.value = ''
    updateVersionModalVisible.value = true
  }

  const submitUpdateVersion = () => {
    if (!updateVersionValue.value.trim()) {
      message?.warning('请输入要升级的目标版本或镜像名')
      return
    }
    const target = updateVersionTarget.value
    const val = updateVersionValue.value.trim()
    updateVersionModalVisible.value = false
    updateContainer(target, val)
  }

  return {
    operatingContainers,
    logModalVisible,
    logLines,
    logRunning,
    terminalLog,
    deferModalVisible,
    deferTarget,
    deferDays,
    deferOptions: [
      { label: '暂挂 7 天', value: 7 },
      { label: '暂挂 14 天', value: 14 },
      { label: '暂挂 30 天', value: 30 },
      { label: '暂挂 90 天', value: 90 }
    ],
    diagnosticsVisible,
    diagnosticsLogs,
    updateVersionModalVisible,
    updateVersionTarget,
    updateVersionCurrentImage,
    updateVersionValue,

    shortDigest,
    copyToClipboard,
    updateContainer,
    startBulkUpdate,
    showLogs,
    openDeferModal,
    submitDefer,
    undeferContainer,
    startContainer,
    stopContainer,
    restartContainer,
    closeLogModal,
    scrollTerminal,
    cleanup,
    openUpdateVersionModal,
    submitUpdateVersion,
    checkContainer
  }
}
