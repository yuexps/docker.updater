<template>
  <div class="view-fade-in flex flex-col h-[calc(100vh-80px)] lg:h-[calc(100vh-100px)]">
    <!-- Page Header -->
    <div class="flex items-center justify-between mb-6 shrink-0 select-none">
      <div>
        <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">运行日志</h1>
      </div>
      <div class="flex items-center space-x-3">
        <n-button 
          type="error" 
          round 
          size="small" 
          ghost
          class="active-scale"
          @click="clearSystemLogs"
        >
          清空日志
        </n-button>
      </div>
    </div>

    <!-- Logs Content Card (Flex fill) -->
    <div class="apple-card p-6 rounded-lg flex-1 flex flex-col min-h-0">
      <div v-if="loading" class="flex justify-center items-center flex-1">
        <n-spin size="large" />
      </div>

      <div v-else-if="logLines.length === 0" class="text-body-muted text-center flex-1 flex items-center justify-center text-[14px] select-none">
        当前暂无系统运行日志输出
      </div>

      <!-- Logs Output Container -->
      <div 
        v-else 
        ref="logContainer"
        class="font-mono text-[13px] leading-relaxed text-slate-600 bg-slate-50/50 p-6 rounded-lg border border-hairline flex-1 overflow-y-auto scrollbar-thin select-text"
      >
        <div v-for="(line, idx) in logLines" :key="idx" class="py-0.5 whitespace-pre-wrap">
          <span v-if="line.includes('[SUCCESS]')" class="text-emerald-600 font-semibold">{{ line }}</span>
          <span v-else-if="line.includes('[ERROR]')" class="text-rose-600 font-semibold">{{ line }}</span>
          <span v-else-if="line.includes('[WARNING]')" class="text-amber-600 font-semibold">{{ line }}</span>
          <span v-else-if="line.includes('[PULL]')" class="text-sky-600">{{ line }}</span>
          <span v-else class="text-slate-600">{{ line }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { NButton, NSpin, useMessage } from 'naive-ui'
import axios from 'axios'
import wsService from '../utils/websocket'

const apiBase = '/app/docker-updater/api'
const message = useMessage()

const logLines = ref<string[]>([])
const loading = ref<boolean>(false)
const logContainer = ref<HTMLDivElement | null>(null)

let unsubscribeSysLog: (() => void) | null = null

const scrollToBottom = () => {
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
}

// 加载历史日志快照
const fetchSystemLogs = async () => {
  loading.value = true
  try {
    const res = await axios.get(`${apiBase}/system/logs`)
    logLines.value = res.data.logs || []
    scrollToBottom()
  } catch {
    message.error('加载系统运行日志失败')
  } finally {
    loading.value = false
  }
}

const clearSystemLogs = async () => {
  try {
    const res = await axios.delete(`${apiBase}/system/logs`)
    if (res.data.ok) {
      message.success('已成功清空系统运行日志')
      logLines.value = []
    }
  } catch {
    message.error('清空日志文件失败')
  }
}

onMounted(async () => {
  // 先拉取历史快照
  await fetchSystemLogs()

  // 再订阅 WebSocket 实时推送新行
  unsubscribeSysLog = wsService.subscribeSysLog((line: string) => {
    if (line.trim()) {
      logLines.value.push(line)
      scrollToBottom()
    }
  })
})

onUnmounted(() => {
  if (unsubscribeSysLog) unsubscribeSysLog()
})
</script>

<style scoped>
.scrollbar-thin::-webkit-scrollbar {
  width: 6px;
}
.scrollbar-thin::-webkit-scrollbar-track {
  background: transparent;
}
.scrollbar-thin::-webkit-scrollbar-thumb {
  background: #e2e8f0;
  border-radius: 3px;
}
.scrollbar-thin::-webkit-scrollbar-thumb:hover {
  background: #cbd5e1;
}
</style>
