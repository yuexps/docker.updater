<template>
  <div class="view-fade-in flex flex-col h-full overflow-hidden">
    <!-- Page Header -->
    <div class="shrink-0 px-3 md:px-5 lg:px-6 pt-3 md:pt-4 lg:pt-5 pb-3 md:pb-4 select-none bg-canvas-parchment">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">任务队列</h1>
        </div>
      </div>
    </div>

    <!-- 页面内容 -->
    <div class="flex-1 min-w-0 overflow-y-auto px-3 md:px-5 lg:px-6 pb-24">
      <!-- Active Task Card Section -->
      <div class="mb-10">
        <h2 class="text-[15px] font-semibold text-slate-500 uppercase tracking-wider mb-4 select-none">当前活跃任务</h2>
        
        <div v-if="activeTask" class="apple-card p-5 sm:p-6 rounded-lg flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-white transition-all duration-300 hover:shadow-[0_8px_24px_rgba(0,0,0,0.03)] hover:border-primary/20">
          <div class="flex items-start space-x-3.5 min-w-0 flex-1">
            <!-- 动态旋转加载图标 -->
            <div class="w-10 h-10 rounded-xl bg-blue-50/60 border border-blue-100 flex items-center justify-center text-primary shadow-xs shrink-0 select-none">
              <svg class="w-5 h-5 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <line x1="12" y1="2" x2="12" y2="6" />
                <line x1="12" y1="18" x2="12" y2="22" />
                <line x1="4.93" y1="4.93" x2="7.76" y2="7.76" />
                <line x1="16.24" y1="16.24" x2="19.07" y2="19.07" />
                <line x1="2" y1="12" x2="6" y2="12" />
                <line x1="18" y1="12" x2="22" y2="12" />
                <line x1="4.93" y1="19.07" x2="7.76" y2="16.24" />
                <line x1="16.24" y1="7.76" x2="19.07" y2="4.93" />
              </svg>
            </div>

            <!-- 文字信息 -->
            <div class="min-w-0 flex-1 space-y-1.5">
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-[17px] font-bold text-slate-800 tracking-tight truncate select-all">
                  {{ activeTask.container_name }}
                </span>
                <span 
                  class="px-2 py-0.5 rounded-full text-[10px] font-bold border shrink-0 select-none"
                  :class="activeTask.type === 'update' 
                    ? 'bg-blue-50 text-primary border-blue-100' 
                    : 'bg-amber-50 text-amber-700 border-amber-100'"
                >
                  {{ activeTask.type === 'update' ? '容器升级' : '容器回滚' }}
                </span>
              </div>
              
              <!-- 入队时间 (带小图标) -->
              <div class="flex items-center text-[12px] text-body-muted font-medium select-none">
                <svg class="w-3.5 h-3.5 mr-1.5 text-slate-400 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="12" cy="12" r="10"/>
                  <polyline points="12 6 12 12 16 14"/>
                </svg>
                <span class="font-mono">{{ formatDate(activeTask.added_at) }}</span>
              </div>
            </div>
          </div>

          <!-- 右侧状态指示与操作 -->
          <div class="flex items-center shrink-0 sm:border-l sm:border-slate-100 sm:pl-6 pt-3 sm:pt-0 border-t border-slate-100 sm:border-t-0 justify-end gap-3 select-none">
            <div class="flex items-center gap-3">
              <n-button 
                size="small" 
                round 
                secondary 
                class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale font-medium text-[12px] px-4"
                @click="viewActiveTaskLog(activeTask.container_name)"
              >
                日志
              </n-button>
              <div class="flex items-center gap-2 px-3 py-1 rounded-full text-[12px] font-semibold bg-blue-50/50 text-primary border border-blue-100/50">
                <span class="h-2 w-2 rounded-full bg-primary animate-pulse"></span>
                <span>运行中</span>
              </div>
            </div>
          </div>
        </div>

        <div v-else class="apple-card p-8 text-center text-body-muted rounded-lg select-none bg-white">
          当前没有正在运行的升级或回滚任务
        </div>
      </div>

      <!-- Waiting Queue Cards Section -->
      <div class="mb-10">
        <h2 class="text-[15px] font-semibold text-slate-500 uppercase tracking-wider mb-4 select-none">等待执行任务</h2>
        
        <div v-if="queuedTasks.length > 0" class="space-y-4">
          <div 
            v-for="(task, index) in queuedTasks" 
            :key="task.container_name"
            class="apple-card rounded-lg p-5 flex flex-col md:flex-row md:items-center md:justify-between gap-4 bg-white transition-all duration-300 hover:shadow-[0_8px_24px_rgba(0,0,0,0.03)] hover:border-primary/20"
          >
            <!-- 左侧排队信息 -->
            <div class="flex items-center space-x-3.5 min-w-0 flex-1">
              <!-- 排队序号 -->
              <div class="w-10 h-10 rounded-xl bg-slate-50 border border-slate-100 flex items-center justify-center text-slate-500 font-bold font-mono text-[14px] shrink-0 select-none">
                #{{ index + 1 }}
              </div>

              <!-- 容器与类型 -->
              <div class="min-w-0 flex-1 space-y-1.5">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-[16px] font-bold text-slate-800 tracking-tight truncate select-all">
                    {{ task.container_name }}
                  </span>
                  <span 
                    class="px-2 py-0.5 rounded-full text-[10px] font-bold border shrink-0 select-none"
                    :class="task.type === 'update' 
                      ? 'bg-blue-50 text-primary border-blue-100' 
                      : 'bg-amber-50 text-amber-700 border-amber-100'"
                  >
                    {{ task.type === 'update' ? '升级' : '回滚' }}
                  </span>
                </div>

                <!-- 入队时间 (带小图标) -->
                <div class="flex items-center text-[12px] text-body-muted font-medium select-none">
                  <svg class="w-3.5 h-3.5 mr-1.5 text-slate-400 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="12" cy="12" r="10"/>
                    <polyline points="12 6 12 12 16 14"/>
                  </svg>
                  <span class="font-mono">{{ formatDate(task.added_at) }}</span>
                </div>
              </div>
            </div>

            <!-- 右侧操作 -->
            <div class="flex flex-row md:flex-col items-center md:items-end justify-end md:justify-center shrink-0 border-t border-slate-100 md:border-0 pt-3 md:pt-0 gap-3">
              <n-button 
                type="error"
                size="small"
                ghost
                round
                :loading="cancelLoadingMap[task.container_name]"
                class="active-scale text-[12px] font-medium px-4"
                @click="cancelQueuedTask(task.container_name)"
              >
                取消排队
              </n-button>
            </div>
          </div>
        </div>

        <div v-else class="apple-card p-8 text-center text-body-muted rounded-lg select-none bg-white">
          排队队列中目前没有等待的任务
        </div>
      </div>
    </div>

    <!-- 公共 Mac 日志终端弹窗 -->
    <terminal-modal
      :show="logModalVisible"
      :log-lines="logLines"
      :log-running="logRunning"
      title="实时部署日志"
      @close="closeLogModal"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { NButton, useMessage } from 'naive-ui'
import axios from 'axios'
import wsService from '../utils/websocket'
import TerminalModal from '../components/TerminalModal.vue'

const apiBase = '/app/docker-updater/api'
const message = useMessage()

interface TaskItem {
  container_name: string;
  type: string;
  added_at: string;
}

const activeTask = ref<TaskItem | null>(null)
const queuedTasks = ref<TaskItem[]>([])
let unsubscribeStatus: (() => void) | null = null

// 活跃任务日志相关状态
const logModalVisible = ref<boolean>(false)
const logLines = ref<string[]>([])
const logRunning = ref<boolean>(false)
let activeUnsubscribeLogs: (() => void) | null = null

const viewActiveTaskLog = async (name: string) => {
  logLines.value = []
  logModalVisible.value = true
  logRunning.value = true

  // 1. 先尝试获取历史日志，使终端有内容显示
  try {
    const res = await axios.get(`${apiBase}/update-log/${name}`)
    if (res.data.found) {
      logLines.value = res.data.logs || []
    }
  } catch (err) {
    console.error('拉取当前活跃任务历史日志失败:', err)
  }

  // 2. 然后通过 WebSocket 订阅实时日志流
  if (activeUnsubscribeLogs) activeUnsubscribeLogs()
  activeUnsubscribeLogs = wsService.subscribeLogs(name, ({ message: msg }) => {
    logLines.value.push(msg)
    if (msg.includes('[SUCCESS]') || msg.includes('[ERROR]')) {
      logRunning.value = false
    }
  })
}

const closeLogModal = () => {
  logModalVisible.value = false
  if (activeUnsubscribeLogs) {
    activeUnsubscribeLogs()
    activeUnsubscribeLogs = null
  }
}

const cancelLoadingMap = ref<Record<string, boolean>>({})

const cancelQueuedTask = async (name: string) => {
  if (cancelLoadingMap.value[name]) return
  cancelLoadingMap.value[name] = true
  try {
    const res = await axios.post(`${apiBase}/tasks/cancel/${name}`)
    if (res.data.success) {
      message.success(`已成功将容器移出排队: ${name}`)
    } else {
      message.error('无法取消该任务，可能它已开始运行')
    }
  } catch (err) {
    message.error('取消排队请求失败')
  } finally {
    cancelLoadingMap.value[name] = false
  }
}

const formatDate = (isoStr: string) => {
  if (!isoStr) return ''
  try {
    const d = new Date(isoStr)
    return d.toLocaleString()
  } catch (e) {
    return isoStr
  }
}

onMounted(() => {
  unsubscribeStatus = wsService.subscribeStatus((payload) => {
    activeTask.value = payload.active
    queuedTasks.value = payload.queued || []
  })
})

onUnmounted(() => {
  if (unsubscribeStatus) {
    unsubscribeStatus()
  }
  if (activeUnsubscribeLogs) {
    activeUnsubscribeLogs()
  }
})
</script>
