<template>
  <div class="view-fade-in">
      <!-- Page Header -->
      <div class="flex items-center justify-between mb-8 select-none">
        <div>
          <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">容器列表</h1>
        </div>
      </div>

      <!-- Container Grid -->
      <div>
        <div v-if="loading" class="flex justify-center py-20">
          <n-spin size="large" />
        </div>

        <div v-else-if="containers.length === 0" class="apple-card p-12 text-center text-body-muted rounded-lg">
          未发现活动或已停止的容器
        </div>

        <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          <div 
            v-for="c in containers" 
            :key="c.name"
            class="apple-card rounded-lg p-4 sm:p-6 flex flex-col justify-between min-h-[220px] hover:border-primary transition-all duration-200"
          >
            <!-- Top Info -->
            <div>
              <div class="flex items-start justify-between">
                <span class="text-[17px] font-semibold text-ink tracking-tight break-all">
                  {{ c.name }}
                </span>

                <div class="flex items-center space-x-2">
                  <span class="text-[12px] font-normal text-body-muted">
                    {{ c.status === 'update' ? '待升级' : c.status === 'deferred' ? '已暂挂' : '最新' }}
                  </span>
                  <div 
                    class="w-2 h-2 rounded-full"
                    :class="[
                      c.status === 'update' ? 'status-dot-red' : 
                      c.status === 'deferred' ? 'status-dot-amber' : 
                      'status-dot-green'
                    ]"
                  ></div>
                </div>
              </div>

              <!-- Details -->
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

              <!-- 运行中：停止 + 重启 -->
              <n-button
                v-if="c.running"
                size="small"
                round
                secondary
                :loading="operatingContainers.has(c.name + ':stop')"
                :disabled="operatingContainers.has(c.name + ':stop')"
                class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale"
                @click="stopContainer(c.name)"
              >
                停止
              </n-button>

              <n-button
                v-if="c.running"
                size="small"
                round
                secondary
                :loading="operatingContainers.has(c.name + ':restart')"
                :disabled="operatingContainers.has(c.name + ':restart')"
                class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale"
                @click="restartContainer(c.name)"
              >
                重启
              </n-button>

              <!-- 已停止：启动 -->
              <n-button
                v-if="!c.running"
                type="primary"
                size="small"
                round
                :loading="operatingContainers.has(c.name + ':start')"
                :disabled="operatingContainers.has(c.name + ':start')"
                class="active-scale"
                @click="startContainer(c.name)"
              >
                启动
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
      title="升级部署作业进度"
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
        <label class="text-[12px] font-semibold uppercase tracking-wider text-body-muted block mb-2">搁置比对检测时长</label>
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
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { 
  NButton, NSpin, NModal, NSelect, 
  useMessage, useDialog 
} from 'naive-ui'
import axios from 'axios'
import wsService from '../utils/websocket'

const apiBase = '/app/docker-updater/api'

const containers = ref<any[]>([])
const loading = ref<boolean>(false)

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
const operatingContainers = ref<Set<string>>(new Set()) // 操作中的容器+动作，防止重复点击

// 辅助：标记操作状态并刷新返回函数
const withOp = async (key: string, fn: () => Promise<void>) => {
  operatingContainers.value = new Set([...operatingContainers.value, key])
  try {
    await fn()
  } finally {
    const next = new Set(operatingContainers.value)
    next.delete(key)
    operatingContainers.value = next
  }
}


const startContainer = (name: string) =>
  withOp(`${name}:start`, async () => {
    try {
      await axios.post(`${apiBase}/container/${name}/start`)
      message.success(`${name} 已启动`)
    } catch (err: any) {
      message.error('启动失败: ' + (err.response?.data?.error || err.message))
    }
  })

const stopContainer = (name: string) =>
  withOp(`${name}:stop`, async () => {
    try {
      await axios.post(`${apiBase}/container/${name}/stop`)
      message.success(`${name} 已停止`)
    } catch (err: any) {
      message.error('停止失败: ' + (err.response?.data?.error || err.message))
    }
  })

const restartContainer = (name: string) =>
  withOp(`${name}:restart`, async () => {
    try {
      await axios.post(`${apiBase}/container/${name}/restart`)
      message.success(`${name} 已重启`)
    } catch (err: any) {
      message.error('重启失败: ' + (err.response?.data?.error || err.message))
    }
  })

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
    message.success(`已成功暂挂容器 ${deferTarget.value} 的版本检测`)
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

onMounted(() => {
  loading.value = true
  unsubscribeStatus = wsService.subscribeStatus((payload) => {
    containers.value = payload.containers || []
    loading.value = false
  })
})

onUnmounted(() => {
  if (unsubscribeStatus) unsubscribeStatus()
  if (activeUnsubscribeLogs) activeUnsubscribeLogs()
})
</script>
