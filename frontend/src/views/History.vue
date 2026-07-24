<template>
  <div class="view-fade-in flex flex-col h-full overflow-hidden">
    <!-- Page Header -->
    <div class="shrink-0 px-3 md:px-5 lg:px-6 pt-3 md:pt-4 lg:pt-5 pb-3 md:pb-4 select-none bg-canvas-parchment">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">升级历史</h1>
        </div>
        <n-button 
          v-if="history.length > 0"
          type="error"
          size="tiny"
          quaternary
          round
          class="active-scale text-[12px] font-medium"
          @click="clearAllHistory"
        >
          清空历史记录
        </n-button>
      </div>
    </div>

    <!-- 页面内容 -->
    <div class="flex-1 min-w-0 overflow-y-auto px-3 md:px-5 lg:px-6 pb-24">
      <div v-if="loading" class="flex justify-center py-20">
        <n-spin size="large" />
      </div>

      <div v-else-if="history.length === 0" class="apple-card rounded-lg p-8 text-center text-body-muted text-[14px] bg-white">
        无历史升级或回滚记录
      </div>

      <div v-else class="space-y-4">
        <!-- 响应式历史记录卡片 -->
        <div 
          v-for="h in history" 
          :key="h.ID" 
          class="apple-card rounded-lg p-5 flex flex-col md:flex-row md:items-center md:justify-between gap-4 bg-white transition-all duration-300 hover:shadow-[0_8px_24px_rgba(0,0,0,0.03)] hover:border-primary/20"
        >
          <!-- 左侧/上半部分：容器与镜像 -->
          <div class="flex items-start space-x-3.5 min-w-0 flex-1">
            <!-- 状态指示图标 -->
            <div 
              class="w-10 h-10 rounded-xl flex items-center justify-center shrink-0 border transition-colors select-none"
              :class="h.Status === 'success' 
                ? 'bg-emerald-50/50 border-emerald-100/80 text-emerald-600' 
                : 'bg-rose-50/50 border-rose-100/80 text-rose-600'"
            >
              <!-- 成功：打勾图标 -->
              <svg v-if="h.Status === 'success'" class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="20 6 9 17 4 12" />
              </svg>
              <!-- 失败：打叉图标 -->
              <svg v-else class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </div>

            <!-- 文字信息 -->
            <div class="min-w-0 flex-1 space-y-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-[16px] font-bold text-slate-800 tracking-tight truncate select-all">{{ h.ContainerName }}</span>
                <span 
                  class="px-2 py-0.5 rounded-full text-[10px] font-bold border shrink-0 select-none"
                  :class="h.Status === 'success' 
                    ? 'bg-emerald-50 text-emerald-700 border-emerald-100' 
                    : 'bg-rose-50 text-rose-700 border-rose-100'"
                >
                  {{ h.Status === 'success' ? '升级成功' : '执行失败' }}
                </span>
              </div>
              <div class="flex items-center space-x-1.5 text-body-muted text-[12px] font-mono break-all pr-2">
                <span class="truncate max-w-70 sm:max-w-md md:max-w-lg lg:max-w-xl select-all" :title="h.Image">{{ h.Image }}</span>
                <button 
                  class="text-slate-400 hover:text-primary transition-colors p-0.5 rounded hover:bg-slate-100 cursor-pointer shrink-0"
                  title="复制镜像名称"
                  @click="copyText(h.Image)"
                >
                  <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                  </svg>
                </button>
              </div>
            </div>
          </div>

          <!-- 右侧/底部按钮栏：时间、日志、删除 -->
          <div class="flex flex-row items-center justify-between md:justify-end gap-4 border-t border-slate-100 md:border-0 pt-3 md:pt-0 shrink-0 select-none">
            <!-- 时间 -->
            <div class="flex items-center text-[12px] text-body-muted font-medium">
              <svg class="w-3.5 h-3.5 mr-1.5 text-slate-400 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10"/>
                <polyline points="12 6 12 12 16 14"/>
              </svg>
              <span class="font-mono">{{ formatCheckTime(h.UpdatedAt) }}</span>
            </div>
            
            <!-- 按钮组 -->
            <div class="flex items-center gap-3">
              <n-button 
                size="small" 
                round 
                secondary 
                class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale font-medium text-[12px] px-4"
                @click="viewHistoricalLog(h.ContainerName)"
              >
                日志
              </n-button>
              <n-button 
                type="error"
                size="small"
                ghost
                round
                class="active-scale text-[12px] font-medium px-4"
                @click="deleteHistoryItem(h.ID, h.ContainerName)"
              >
                删除
              </n-button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 公共 Mac 日志终端弹窗 -->
    <terminal-modal
      :show="logModalVisible"
      :log-lines="logLines"
      :log-running="false"
      title="升级历史日志"
      @close="logModalVisible = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { NButton, NSpin, useMessage, useDialog } from 'naive-ui'
import axios from 'axios'
import wsService from '../utils/websocket'
import TerminalModal from '../components/TerminalModal.vue'

const apiBase = '/app/docker-updater/api'
const message = useMessage()
const dialog = useDialog()

interface HistoryItem {
  ID: number;
  ContainerName: string;
  Image: string;
  UpdatedAt: string;
  Status: string;
}

const history = ref<HistoryItem[]>([])
const loading = ref<boolean>(false)

const logModalVisible = ref<boolean>(false)
const logLines = ref<string[]>([])

let unsubscribeStatus: (() => void) | null = null

const viewHistoricalLog = async (name: string) => {
  try {
    const res = await axios.get(`${apiBase}/update-log/${name}`)
    if (res.data.found) {
      logLines.value = res.data.logs || []
      logModalVisible.value = true
    } else {
      message.warning('找不到该容器对应的历史升级文件日志')
    }
  } catch (err) {
    message.error('拉取历史日志失败')
  }
}

const deleteHistoryItem = (id: number, name: string) => {
  dialog.warning({
    title: '确认删除记录',
    content: `确认删除容器 ${name} 的这次历史升级记录吗？这也会删除本地持久化的日志文件，且不可恢复。`,
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: () => {
      return new Promise<void>(async (resolve, reject) => {
        try {
          const res = await axios.delete(`${apiBase}/history/${id}`)
          if (res.data.ok) {
            message.success(`已成功删除该条记录`)
            resolve()
          } else {
            reject()
          }
        } catch (err) {
          message.error('删除历史记录失败')
          reject()
        }
      })
    }
  })
}

const clearAllHistory = () => {
  dialog.warning({
    title: '确认清空历史',
    content: '确认清空全部升级历史记录吗？该操作同时会彻底删除本地产生的全部日志文件，且不可恢复。',
    positiveText: '确认清空',
    negativeText: '取消',
    onPositiveClick: () => {
      return new Promise<void>(async (resolve, reject) => {
        try {
          const res = await axios.delete(`${apiBase}/history`)
          if (res.data.ok) {
            message.success('已成功清空所有历史记录')
            resolve()
          } else {
            reject()
          }
        } catch (err) {
          message.error('清空历史记录失败')
          reject()
        }
      })
    }
  })
}

const formatCheckTime = (isoStr: string) => {
  if (!isoStr) return '无'
  const date = new Date(isoStr)
  return date.toLocaleString()
}

const copyText = (text: string) => {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(() => {
      message.success('已复制到剪贴板')
    }).catch(() => {
      message.error('复制失败')
    })
  } else {
    const input = document.createElement('input')
    input.setAttribute('value', text)
    document.body.appendChild(input)
    input.select()
    try {
      document.execCommand('copy')
      message.success('已复制到剪贴板')
    } catch {
      message.error('复制失败')
    }
    document.body.removeChild(input)
  }
}

onMounted(() => {
  loading.value = true
  unsubscribeStatus = wsService.subscribeStatus((payload) => {
    history.value = payload.history || []
    loading.value = false
  })
})

onUnmounted(() => {
  if (unsubscribeStatus) unsubscribeStatus()
})
</script>

