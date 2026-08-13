import { ref, nextTick, onScopeDispose } from 'vue'
import wsService from '../utils/websocket'
import { copyToClipboard as copyToClipboardUtil } from '../utils/clipboard'
import { message as discreteMessage } from '../utils/discrete'
import {
  updateContainerApi,
  getContainerLogsApi,
  deferContainerApi,
  undeferContainerApi,
  startContainerApi,
  stopContainerApi,
  restartContainerApi,
  checkContainerApi
} from '../api/container'

export function useContainerActions() {
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

  const shortDigest = (digest: string) => {
    if (!digest) return ''
    return digest.startsWith('sha256:') ? digest.slice(7, 19) : digest.slice(0, 12)
  }

  const copyToClipboard = (text: string, label: string = '镜像名称') => {
    return copyToClipboardUtil(text, label, discreteMessage)
  }

  const updateContainer = (name: string, targetImage?: string) => {
    logLines.value = []
    logModalVisible.value = true
    logRunning.value = true

    discreteMessage.info('升级任务已启动，请查看任务日志确认最终结果')

    updateContainerApi(name, targetImage)

    if (activeUnsubscribeLogs) activeUnsubscribeLogs()
    activeUnsubscribeLogs = wsService.subscribeLogs(name, ({ message: msg }) => {
      logLines.value.push(msg)
      scrollTerminal()
      if (msg.includes('恢复成功') || msg.includes('任务全部完成')) {
        logRunning.value = false
        discreteMessage.success(`容器 ${name} 升级成功`)
      } else if (msg.includes('[ERROR]')) {
        logRunning.value = false
        discreteMessage.error(`容器 ${name} 升级失败，详情查看日志`)
      }
    })
  }

  const startBulkUpdate = async (targets: string[], onClearSelection?: () => void) => {
    if (targets.length === 0) return

    logLines.value = []
    logModalVisible.value = true
    logRunning.value = true
    logLines.value.push(`[INFO] 批量升级开始，队列总数: ${targets.length}`)
    discreteMessage.info('升级任务已启动，请查看任务日志确认最终结果')

    for (let i = 0; i < targets.length; i++) {
      const name = targets[i]
      logLines.value.push(`========================================`)
      logLines.value.push(`[INFO] 正在升级当前服务 (${i + 1}/${targets.length}): ${name}`)
      scrollTerminal()

      await new Promise<void>((resolve) => {
        updateContainerApi(name).catch(() => {
          logLines.value.push(`[ERROR] 触发容器 ${name} 升级失败`)
          resolve()
        })

        const unsub = wsService.subscribeLogs(name, ({ message: msg }) => {
          logLines.value.push(msg)
          scrollTerminal()
          if (msg.includes('任务全部完成') || msg.includes('恢复成功') || msg.includes('[ERROR]')) {
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

  const parseErrorMessage = (err: any): string => {
    if (err?.response?.data) {
      const d = err.response.data
      if (typeof d === 'string') return d
      if (typeof d === 'object' && (d.error || d.message)) return d.error || d.message
    }
    if (err?.message) {
      if (err.message.includes('timeout of') && err.message.includes('exceeded')) return '请求超时'
      return err.message
    }
    return '未知错误'
  }

  const showLogs = async (name: string) => {
    try {
      const data = await getContainerLogsApi(name)
      diagnosticsLogs.value = data.logs || '未检索到输出日志'
      diagnosticsVisible.value = true
    } catch (err: any) {
      discreteMessage.error(`获取容器 ${name} 日志失败: ${parseErrorMessage(err)}`)
    }
  }

  const openDeferModal = (name: string) => {
    deferTarget.value = name
    deferDays.value = 7
    deferModalVisible.value = true
  }

  const submitDefer = async (onSuccess?: () => void) => {
    const target = deferTarget.value
    const days = deferDays.value
    deferModalVisible.value = false
    try {
      await deferContainerApi(target, days)
      discreteMessage.success(`已暂挂服务 ${target} 的版本检测`)
      if (onSuccess) onSuccess()
    } catch (err: any) {
      discreteMessage.error(`暂挂服务 ${target} 失败: ${parseErrorMessage(err)}`)
    }
  }

  const undeferContainer = async (name: string, onSuccess?: () => void) => {
    try {
      await undeferContainerApi(name)
      discreteMessage.success(`已恢复服务 ${name} 版本正常检测`)
      if (onSuccess) onSuccess()
    } catch (err: any) {
      discreteMessage.error(`恢复服务 ${name} 检测失败: ${parseErrorMessage(err)}`)
    }
  }

  const startContainer = async (name: string) => {
    const actionKey = `${name}:start`
    operatingContainers.value.add(actionKey)
    try {
      await startContainerApi(name)
      discreteMessage.success(`已成功启动容器 ${name}`)
    } catch (err: any) {
      discreteMessage.error(`启动容器 ${name} 失败: ${parseErrorMessage(err)}`)
    } finally {
      operatingContainers.value.delete(actionKey)
    }
  }

  const stopContainer = async (name: string) => {
    const actionKey = `${name}:stop`
    operatingContainers.value.add(actionKey)
    try {
      await stopContainerApi(name)
      discreteMessage.success(`已成功停止容器 ${name}`)
    } catch (err: any) {
      discreteMessage.error(`停止容器 ${name} 失败: ${parseErrorMessage(err)}`)
    } finally {
      operatingContainers.value.delete(actionKey)
    }
  }

  const restartContainer = async (name: string) => {
    const actionKey = `${name}:restart`
    operatingContainers.value.add(actionKey)
    try {
      await restartContainerApi(name)
      discreteMessage.success(`已成功重启容器 ${name}`)
    } catch (err: any) {
      discreteMessage.error(`重启容器 ${name} 失败: ${parseErrorMessage(err)}`)
    } finally {
      operatingContainers.value.delete(actionKey)
    }
  }

  const checkContainer = async (name: string) => {
    const actionKey = `${name}:check`
    operatingContainers.value.add(actionKey)
    try {
      await checkContainerApi(name)
      discreteMessage.info(`容器 ${name} 版本检测已触发，结果稍后自动刷新`)
    } catch (err: any) {
      discreteMessage.error(`触发容器 ${name} 版本检测失败: ${parseErrorMessage(err)}`)
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
    onScopeDispose(cleanup)
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
      discreteMessage.warning('请输入要升级的目标版本或镜像名')
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
