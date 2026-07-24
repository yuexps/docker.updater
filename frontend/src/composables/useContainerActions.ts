import { ref, nextTick, onScopeDispose } from 'vue'
import { useMessage, useDialog } from 'naive-ui'
import axios from 'axios'
import wsService from '../utils/websocket'

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
  const copyToClipboard = (text: string) => {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(() => {
        message?.success('已复制镜像名称到剪贴板')
      }).catch(() => {
        message?.error('复制失败')
      })
    } else {
      const input = document.createElement('input')
      input.setAttribute('value', text)
      document.body.appendChild(input)
      input.select()
      try {
        document.execCommand('copy')
        message?.success('已复制镜像名称到剪贴板')
      } catch (err) {
        message?.error('复制失败')
      }
      document.body.removeChild(input)
    }
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
      if (msg.includes('[SUCCESS]') || msg.includes('[ERROR]')) {
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
          if (msg.includes('[SUCCESS]') || msg.includes('[ERROR]')) {
            unsub()
            resolve()
          }
        })
      })
    }

    logLines.value.push(`========================================`)
    logLines.value.push(`[SUCCESS] 批量升级队列全部作业运行完毕`)
    logRunning.value = false
    if (onClearSelection) onClearSelection()
  }

  // 回滚单个容器
  const rollbackContainer = (name: string) => {
    dialog?.warning({
      title: '确认回滚容器',
      content: `你是否确定要将 ${name} 重建还原为上一次升级前的备份镜像？该过程会造成容器短暂中断重启。`,
      positiveText: '确认还原',
      negativeText: '取消',
      onPositiveClick: () => {
        return new Promise<void>((resolve) => {
          logLines.value = []
          logModalVisible.value = true
          logRunning.value = true

          axios.get(`${apiBase}/rollback/${name}`)
            .then(() => resolve())
            .catch(() => {
              message?.error('触发回滚失败')
              logRunning.value = false
              resolve()
            })

          if (activeUnsubscribeLogs) activeUnsubscribeLogs()
          activeUnsubscribeLogs = wsService.subscribeLogs(name, ({ message: msg }) => {
            logLines.value.push(msg)
            scrollTerminal()
            if (msg.includes('[SUCCESS]') || msg.includes('[ERROR]')) {
              logRunning.value = false
            }
          })
        })
      }
    })
  }

  // 删除备份容器
  const deleteBackup = (name: string, onSuccess?: () => void) => {
    dialog?.warning({
      title: '清除备份容器',
      content: `清理备份容器 ${name}_backup_docker_updater 将释放本地磁盘空间，但这之后您将无法直接一键回滚。确定清除吗？`,
      positiveText: '确认清除',
      negativeText: '取消',
      onPositiveClick: () => {
        return new Promise<void>(async (resolve, reject) => {
          try {
            await axios.delete(`${apiBase}/backup/${name}`)
            message?.success('备份容器已彻底清除')
            if (onSuccess) onSuccess()
            resolve()
          } catch (err) {
            message?.error('清除备份失败')
            reject()
          }
        })
      }
    })
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
      await axios.post(`${apiBase}/container/${name}/start`)
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
      await axios.post(`${apiBase}/container/${name}/stop`)
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
      await axios.post(`${apiBase}/container/${name}/restart`)
      message?.success(`已成功重启容器 ${name}`)
    } catch {
      message?.error(`重启容器 ${name} 失败`)
    } finally {
      operatingContainers.value.delete(actionKey)
    }
  }

  // 检查单个容器更新
  const checkContainer = async (name: string) => {
    const actionKey = `${name}:check`
    operatingContainers.value.add(actionKey)
    try {
      const res = await axios.post(`${apiBase}/check/${name}`)
      if (res.data?.has_update) {
        message?.success(`检测完毕: 容器 ${name} 发现新版本镜像`)
      } else {
        message?.success(`检测完毕: 容器 ${name} 当前镜像已是最新`)
      }
    } catch {
      message?.error(`检测容器 ${name} 版本失败`)
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
    rollbackContainer,
    deleteBackup,
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
