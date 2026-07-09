<template>
  <div class="view-fade-in">
      <!-- Page Header -->
      <div class="flex items-center justify-between mb-8 select-none">
        <div>
          <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">概览</h1>
        </div>
        <div class="flex items-center space-x-3">
          <n-button 
            v-if="selectedContainers.length > 0"
            type="primary"
            round
            size="small"
            class="active-scale"
            @click="startBulkUpdate"
          >
            批量升级 ({{ selectedContainers.length }})
          </n-button>
          <n-button 
            :loading="checking" 
            round 
            secondary
            size="small"
            class="active-scale"
            @click="checkUpdates"
          >
            {{ checking ? '正在检测...' : '检测新版本' }}
          </n-button>
        </div>
      </div>

      <!-- Stats Grid -->
      <div class="grid grid-cols-3 gap-3 md:gap-6 mb-10 select-none">
        <!-- Card 1 -->
        <div class="apple-card p-3 sm:p-6 rounded-lg">
          <span class="text-[12px] font-normal text-body-muted uppercase tracking-wider">待升级</span>
          <span class="text-[34px] font-semibold tracking-tight block mt-2 text-primary">{{ updateCount }}</span>
        </div>
        <!-- Card 2 -->
        <div class="apple-card p-3 sm:p-6 rounded-lg">
          <span class="text-[12px] font-normal text-body-muted uppercase tracking-wider">已暂挂</span>
          <span class="text-[34px] font-semibold tracking-tight block mt-2 text-amber-600">{{ deferredCount }}</span>
        </div>
        <!-- Card 3 -->
        <div class="apple-card p-3 sm:p-6 rounded-lg">
          <span class="text-[12px] font-normal text-body-muted uppercase tracking-wider">最后检测</span>
          <span class="text-[11px] sm:text-[14px] font-semibold block mt-4 text-slate-700 break-all">{{ formatCheckTime(lastCheck) }}</span>
        </div>
      </div>

      <!-- Section: Pending Updates Grid -->
      <div>
        <h2 class="text-[21px] font-semibold tracking-tight mb-6 apple-headline">可升级容器</h2>

        <div v-if="loading" class="flex justify-center py-20">
          <n-spin size="large" />
        </div>

        <div v-else-if="pendingContainers.length === 0" class="apple-card p-12 text-center text-body-muted rounded-lg select-none">
          本地运行容器均已是最新
        </div>

        <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          <div 
            v-for="c in pendingContainers" 
            :key="c.name"
            class="apple-card rounded-lg p-4 sm:p-6 flex flex-col justify-between min-h-[220px] hover:border-primary transition-all duration-200"
          >
            <!-- Top Header -->
            <div>
              <div class="flex items-start justify-between">
                <div class="flex items-center space-x-3">
                  <n-checkbox 
                    v-if="c.status === 'update'"
                    :checked="selectedContainers.includes(c.name)"
                    @update:checked="(val) => toggleSelect(c.name, val)"
                  />
                  <span class="text-[17px] font-semibold text-ink tracking-tight break-all">
                    {{ c.name }}
                  </span>
                </div>

                <div class="flex items-center space-x-2">
                  <span class="text-[12px] font-normal text-body-muted">
                    {{ c.status === 'update' ? '待升级' : '已暂挂' }}
                  </span>
                  <div 
                    class="w-2 h-2 rounded-full"
                    :class="c.status === 'update' ? 'status-dot-red' : 'status-dot-amber'"
                  ></div>
                </div>
              </div>

              <!-- Image details -->
              <div class="mt-4 space-y-2">
                <div class="bg-canvas-parchment p-2 rounded border border-hairline font-mono text-[12px] text-slate-600 break-all select-all">
                  {{ c.image }}
                </div>
                <div class="flex flex-wrap gap-x-4 gap-y-1 text-[12px] font-normal text-body-muted">
                  <span v-if="c.compose_project">
                    <span class="font-semibold text-slate-600">Compose:</span> {{ c.compose_project }}
                  </span>
                  <span>
                    <span class="font-semibold text-slate-600">状态:</span> {{ c.running ? '运行中' : '已停止' }}
                  </span>
                  <span v-if="c.defer_until">
                    <span class="font-semibold text-slate-600">挂起至:</span> {{ c.defer_until }}
                  </span>
                </div>
              </div>
            </div>

            <!-- Action buttons -->
            <div class="mt-6 pt-4 border-t border-hairline flex flex-wrap gap-2">
              <n-button 
                v-if="c.status === 'update'"
                type="primary"
                size="small"
                round
                class="active-scale"
                @click="updateContainer(c.name)"
              >
                升级
              </n-button>
              
              <n-button 
                v-if="c.has_rollback"
                size="small"
                round
                secondary
                class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale"
                @click="rollbackContainer(c.name)"
              >
                回滚
              </n-button>

              <n-button 
                v-if="c.status === 'update'"
                size="small"
                round
                secondary
                class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale"
                @click="openDeferModal(c.name)"
              >
                暂挂
              </n-button>

              <n-button 
                v-if="c.status === 'deferred'"
                size="small"
                round
                secondary
                class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale"
                @click="undeferContainer(c.name)"
              >
                恢复检测
              </n-button>

              <n-button 
                v-if="c.has_rollback"
                size="small"
                round
                secondary
                class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale"
                @click="deleteBackup(c.name)"
              >
                清除备份
              </n-button>

              <n-button 
                size="small"
                round
                secondary
                class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale ml-auto"
                @click="showLogs(c.name)"
              >
                日志
              </n-button>
            </div>
          </div>
        </div>
      </div>

    <!-- Modal 1: Pure Flat Dark Terminal -->
    <n-modal 
      v-model:show="logModalVisible" 
      :mask-closable="false" 
      preset="card" 
      class="max-w-3xl bg-slate-900 text-white rounded-lg"
      title="升级部署进度"
    >
      <div class="bg-black p-4 rounded font-mono text-[12px] h-[400px] overflow-y-auto border border-slate-800" ref="terminalLog">
        <div v-for="(line, idx) in logLines" :key="idx" class="py-0.5 break-all">
          <span v-if="line.includes('[SUCCESS]')" class="text-green-400 font-semibold">{{ line }}</span>
          <span v-else-if="line.includes('[ERROR]')" class="text-red-400 font-semibold">{{ line }}</span>
          <span v-else-if="line.includes('[WARNING]')" class="text-amber-400 font-semibold">{{ line }}</span>
          <span v-else-if="line.includes('[PULL]')" class="text-sky-400">{{ line }}</span>
          <span v-else class="text-slate-300">{{ line }}</span>
        </div>
        <div v-if="logRunning" class="text-white animate-pulse mt-2">[JOB RUNNING] 正在监听日志输出流...</div>
      </div>
      <template #action>
        <div class="flex justify-end space-x-2">
          <n-button v-if="logRunning" secondary round size="medium" @click="logModalVisible = false">
            后台运行
          </n-button>
          <n-button v-else type="primary" round size="medium" @click="closeLogModal">
            关闭窗口
          </n-button>
        </div>
      </template>
    </n-modal>

    <!-- Modal 2: Defer Select -->
    <n-modal 
      v-model:show="deferModalVisible" 
      preset="dialog" 
      title="暂挂升级检测"
      positive-text="确认暂挂"
      negative-text="取消"
      @positive-click="submitDefer"
    >
      <div class="py-4">
        <label class="text-[12px] font-semibold uppercase tracking-wider text-body-muted block mb-2">搁置升级比对时长</label>
        <n-select v-model:value="deferDays" :options="deferOptions" />
      </div>
    </n-modal>

    <!-- Modal 3: Diagnostics Log -->
    <n-modal 
      v-model:show="diagnosticsVisible" 
      preset="card" 
      class="max-w-3xl"
      title="容器运行日志 (stdout/stderr)"
    >
      <div class="bg-black text-slate-300 p-4 rounded font-mono text-[12px] h-[400px] overflow-y-auto border border-slate-900">
        <pre class="whitespace-pre-wrap font-sans text-[12px] leading-relaxed">{{ diagnosticsLogs }}</pre>
      </div>
      <template #action>
        <div class="flex justify-end">
          <n-button round size="medium" @click="diagnosticsVisible = false">关闭窗口</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, nextTick } from 'vue'
import { 
  NButton, NSpin, NCheckbox, NModal, NSelect, 
  useMessage, useDialog 
} from 'naive-ui'
import axios from 'axios'
import wsService from '../utils/websocket'

const apiBase = '/app/docker-updater/api'

const containers = ref<any[]>([])
const lastCheck = ref<string>('')
const loading = ref<boolean>(false)
const checking = ref<boolean>(false)

const selectedContainers = ref<string[]>([])

const logModalVisible = ref<boolean>(false)
const logLines = ref<string[]>([])
const logRunning = ref<boolean>(false)
const terminalLog = ref<HTMLDivElement | null>(null)

const deferModalVisible = ref<boolean>(false)
const deferTarget = ref<string>('')
const deferDays = ref<number>(7)
const deferOptions = [
  { label: '暂挂 7 天', value: 7 },
  { label: '暂挂 14 天', value: 14 },
  { label: '暂挂 30 天', value: 30 },
  { label: '暂挂 90 天', value: 90 }
]

const diagnosticsVisible = ref<boolean>(false)
const diagnosticsLogs = ref<string>('')

const message = useMessage()
const dialog = useDialog()

let unsubscribeStatus: (() => void) | null = null
let activeUnsubscribeLogs: (() => void) | null = null

// 待更新或已挂起列表
const pendingContainers = computed(() => {
  return containers.value.filter(c => c.status === 'update' || c.status === 'deferred')
})

const updateCount = computed(() => {
  return containers.value.filter(c => c.status === 'update').length
})
const deferredCount = computed(() => {
  return containers.value.filter(c => c.status === 'deferred').length
})

const checkUpdates = async () => {
  checking.value = true
  try {
    await axios.post(`${apiBase}/check`)
    message.success('版本检测已在后台触发')
  } catch (err) {
    message.error('触发检测失败')
  } finally {
    checking.value = false
  }
}

const toggleSelect = (name: string, checked: boolean) => {
  if (checked) {
    if (!selectedContainers.value.includes(name)) {
      selectedContainers.value.push(name)
    }
  } else {
    selectedContainers.value = selectedContainers.value.filter(n => n !== name)
  }
}

const updateContainer = (name: string) => {
  logLines.value = []
  logModalVisible.value = true
  logRunning.value = true

  axios.get(`${apiBase}/update/${name}`).catch(err => {
    message.error('触发升级失败')
    logRunning.value = false
  })

  activeUnsubscribeLogs = wsService.subscribeLogs(name, ({ message: msg }) => {
    logLines.value.push(msg)
    scrollTerminal()
    if (msg.includes('[SUCCESS]') || msg.includes('[ERROR]')) {
      logRunning.value = false
    }
  })
}

const startBulkUpdate = async () => {
  const targets = [...selectedContainers.value]
  selectedContainers.value = []
  
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
}

const rollbackContainer = (name: string) => {
  dialog.warning({
    title: '确认回滚容器',
    content: `你是否确定要将 ${name} 重建还原为上一次升级前的备份镜像？该过程会造成容器短暂中断重启。`,
    positiveText: '确认还原',
    negativeText: '取消',
    onPositiveClick: () => {
      logLines.value = []
      logModalVisible.value = true
      logRunning.value = true

      axios.get(`${apiBase}/rollback/${name}`).catch(err => {
        message.error('触发回滚失败')
        logRunning.value = false
      })

      activeUnsubscribeLogs = wsService.subscribeLogs(name, ({ message: msg }) => {
        logLines.value.push(msg)
        scrollTerminal()
        if (msg.includes('[SUCCESS]') || msg.includes('[ERROR]')) {
          logRunning.value = false
        }
      })
    }
  })
}

const deleteBackup = async (name: string) => {
  dialog.warning({
    title: '清除备份容器',
    content: `清理备份容器 ${name}_old 将释放本地磁盘空间，但这之后您将无法直接一键回滚。确定清除吗？`,
    positiveText: '确认清除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await axios.delete(`${apiBase}/backup/${name}`)
        message.success('备份容器已彻底清除')
      } catch (err) {
        message.error('清除备份失败')
      }
    }
  })
}

const showLogs = async (name: string) => {
  try {
    const res = await axios.get(`${apiBase}/container/${name}/logs`)
    diagnosticsLogs.value = res.data.logs || '未检索到输出日志'
    diagnosticsVisible.value = true
  } catch (err) {
    message.error('获取容器日志失败')
  }
}

const openDeferModal = (name: string) => {
  deferTarget.value = name
  deferDays.value = 7
  deferModalVisible.value = true
}

const submitDefer = async () => {
  try {
    await axios.post(`${apiBase}/defer/${deferTarget.value}`, { days: deferDays.value })
    message.success(`已暂挂服务 ${deferTarget.value} 的版本检测`)
  } catch (err) {
    message.error('暂挂失败')
  }
}

const undeferContainer = async (name: string) => {
  try {
    await axios.post(`${apiBase}/undefer/${name}`)
    message.success('已恢复版本正常检测')
  } catch (err) {
    message.error('恢复检测失败')
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

const formatCheckTime = (isoStr: string) => {
  if (!isoStr) return '无'
  const date = new Date(isoStr)
  return date.toLocaleString()
}

onMounted(() => {
  loading.value = true
  unsubscribeStatus = wsService.subscribeStatus((payload) => {
    containers.value = payload.containers || []
    lastCheck.value = payload.last_check || ''
    loading.value = false
  })
})

onUnmounted(() => {
  if (unsubscribeStatus) unsubscribeStatus()
  if (activeUnsubscribeLogs) activeUnsubscribeLogs()
})
</script>
