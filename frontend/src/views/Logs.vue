<template>
  <div class="view-fade-in flex flex-col h-full overflow-hidden">
    <!-- Page Header -->
    <div class="shrink-0 px-4 md:px-8 lg:px-10 pt-3 md:pt-4 lg:pt-5 pb-3 md:pb-4 select-none bg-canvas-parchment">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">运行日志</h1>
        </div>
        <div class="flex items-center space-x-3">
          <n-button 
            type="error" 
            round 
            size="small" 
            ghost
            class="active-scale text-[12px] font-medium"
            :loading="clearing"
            @click="clearSystemLogs"
          >
            清空日志
          </n-button>
        </div>
      </div>
    </div>

    <!-- 页面内容 -->
    <div class="flex-1 min-w-0 flex flex-col min-h-0 px-4 md:px-8 lg:px-10 pb-20 md:pb-6 lg:pb-8">
      <!-- Logs Content Card (Flex fill) -->
      <div class="apple-card p-4 sm:p-6 rounded-lg flex-1 flex flex-col min-h-0 bg-white">
        
        <!-- Logs Control Toolbar -->
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-5 pb-4 border-b border-divider-soft">
          <!-- Level Filter Segment Control -->
          <div class="flex items-center p-0.5 bg-slate-100/80 rounded-full border border-slate-200/40 shrink-0 select-none">
            <button 
              v-for="lvl in levels" 
              :key="lvl.value"
              @click="selectedLevel = lvl.value"
              class="px-3.5 py-1.5 rounded-full text-[12px] font-medium transition-all duration-200 cursor-pointer text-center flex-1 sm:flex-initial"
              :class="selectedLevel === lvl.value ? 'bg-primary text-white shadow-xs font-semibold' : 'text-slate-500 hover:text-slate-800'"
            >
              {{ lvl.label }}
            </button>
          </div>

          <!-- Search Input -->
          <div class="relative min-w-0 w-full sm:max-w-xs">
            <input 
              v-model="searchQuery" 
              type="text" 
              placeholder="搜索日志关键字..."
              class="w-full pl-9 pr-4 py-1.5 bg-slate-50 border border-hairline rounded-full text-[13px] text-slate-700 placeholder-slate-400 focus:outline-none focus:border-primary focus:bg-white transition-all font-sans"
            />
            <span class="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 select-none pointer-events-none">
              <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="11" cy="11" r="8" />
                <line x1="21" y1="21" x2="16.65" y2="16.65" />
              </svg>
            </span>
          </div>
        </div>

        <!-- Main Content Area -->
        <div v-if="loading" class="flex justify-center items-center flex-1">
          <n-spin size="large" />
        </div>

        <div v-else-if="logLines.length === 0" class="text-body-muted text-center flex-1 flex items-center justify-center text-[14px] select-none">
          当前暂无系统运行日志输出
        </div>

        <!-- Logs Container & Floating Button Wrapper -->
        <div v-else class="flex-1 min-h-0 relative flex flex-col">
          <!-- Logs Output Container -->
          <div 
            ref="logContainer"
            @scroll="handleScroll"
            class="font-mono text-[12px] sm:text-[13px] leading-relaxed text-slate-600 bg-slate-50/30 p-4 sm:p-5 rounded-lg border border-hairline flex-1 overflow-y-auto scrollbar-thin select-text"
          >
            <div v-if="filteredLines.length === 0" class="text-body-muted text-center py-10 text-[13px] select-none">
              未检索到匹配的日志行
            </div>
            <div v-else>
              <div 
                v-for="(line, idx) in filteredLines" 
                :key="idx" 
                class="py-1 flex items-start border-b border-slate-100/50 last:border-b-0 hover:bg-slate-100/30 transition-colors"
              >
                <!-- 日志内容 -->
                <div class="flex-1 break-all whitespace-pre-wrap">
                  <template v-for="parsed in [parseLine(line)]">
                    <template v-if="parsed">
                      <!-- 级别气泡徽章 -->
                      <span 
                        :class="parsed.badgeClass" 
                        class="inline-block px-1.5 py-0.25 rounded text-[10px] font-bold mr-2 uppercase tracking-wide select-none"
                      >
                        {{ parsed.level }}
                      </span>
                      <!-- 文本内容 -->
                      <span :class="parsed.textClass">{{ parsed.content }}</span>
                    </template>
                    <template v-else>
                      <span class="text-slate-600">{{ line }}</span>
                    </template>
                  </template>
                </div>
              </div>
            </div>
          </div>

          <!-- Floating Back to Bottom Button -->
          <transition name="fade">
            <button 
              v-show="showScrollBtn"
              @click="scrollToBottom"
              class="absolute right-4 bottom-4 sm:right-6 sm:bottom-6 w-9 h-9 sm:w-10 sm:h-10 rounded-full bg-primary text-white flex items-center justify-center shadow-lg hover:bg-primary-focus active-scale transition-all duration-200 cursor-pointer select-none border border-primary/10 z-10"
              title="回到底部"
            >
              <svg class="w-4.5 h-4.5 sm:w-5 sm:h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="6 9 12 15 18 9" />
              </svg>
            </button>
          </transition>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, computed } from 'vue'
import { NButton, NSpin, useMessage } from 'naive-ui'
import axios from 'axios'
import wsService from '../utils/websocket'

const apiBase = '/app/docker-updater/api'
const message = useMessage()

const logLines = ref<string[]>([])
const loading = ref<boolean>(false)
const clearing = ref<boolean>(false)
const logContainer = ref<HTMLDivElement | null>(null)

let unsubscribeSysLog: (() => void) | null = null

const selectedLevel = ref('ALL')
const searchQuery = ref('')
const showScrollBtn = ref(false)

const levels = [
  { label: 'ALL', value: 'ALL' },
  { label: 'INFO', value: 'INFO' },
  { label: 'SUCCESS', value: 'SUCCESS' },
  { label: 'WARN', value: 'WARN' },
  { label: 'ERROR', value: 'ERROR' },
  { label: 'DEBUG', value: 'DEBUG' }
]

const filteredLines = computed(() => {
  let lines = logLines.value
  
  if (selectedLevel.value !== 'ALL') {
    if (selectedLevel.value === 'WARN') {
      lines = lines.filter(line => line.includes('[WARN]') || line.includes('[WARNING]'))
    } else {
      lines = lines.filter(line => line.includes(`[${selectedLevel.value}]`))
    }
  }
  
  if (searchQuery.value.trim()) {
    const query = searchQuery.value.toLowerCase()
    lines = lines.filter(line => line.toLowerCase().includes(query))
  }
  
  return lines
})

interface ParsedLine {
  level: string
  content: string
  badgeClass: string
  textClass: string
}

const parseLine = (line: string): ParsedLine | null => {
  const targetLevels = ['ERROR', 'INFO', 'WARN', 'WARNING', 'DEBUG', 'SUCCESS'] as const
  for (const lvl of targetLevels) {
    const tag = `[${lvl}]`
    if (line.includes(tag)) {
      const parts = line.split(tag)
      const content = parts.slice(1).join(tag).trim()
      let badgeClass = ''
      let textClass = 'text-slate-600'
      const displayLevel = (lvl === 'WARNING' || lvl === 'WARN') ? 'WARN' : lvl
      
      if (lvl === 'ERROR') {
        badgeClass = 'bg-rose-50 border border-rose-100 text-rose-600'
        textClass = 'text-rose-600/90 font-medium'
      } else if (lvl === 'INFO') {
        badgeClass = 'bg-blue-50 border border-blue-100 text-primary'
        textClass = 'text-slate-700'
      } else if (lvl === 'WARN' || lvl === 'WARNING') {
        badgeClass = 'bg-amber-50 border border-amber-200 text-amber-600'
        textClass = 'text-amber-700 font-medium'
      } else if (lvl === 'DEBUG') {
        badgeClass = 'bg-purple-50 border border-purple-100 text-purple-600'
        textClass = 'text-slate-500 font-mono'
      } else if (lvl === 'SUCCESS') {
        badgeClass = 'bg-emerald-50 border border-emerald-100 text-emerald-600'
        textClass = 'text-emerald-700 font-medium'
      }
      
      return {
        level: displayLevel,
        content: content || line,
        badgeClass,
        textClass
      }
    }
  }
  return null
}

const handleScroll = (e: Event) => {
  const el = e.target as HTMLDivElement
  if (!el) return
  
  // 滚动位置离开底部超过 100px 时显示悬浮按钮
  const threshold = 100
  const isFarFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight > threshold
  showScrollBtn.value = isFarFromBottom
}

const scrollToBottom = () => {
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
      showScrollBtn.value = false
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
  clearing.value = true
  try {
    const res = await axios.delete(`${apiBase}/system/logs`)
    if (res.data.ok) {
      message.success('已成功清空系统运行日志')
      logLines.value = []
      showScrollBtn.value = false
    }
  } catch {
    message.error('清空日志文件失败')
  } finally {
    clearing.value = false
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
  height: 6px;
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

/* 悬浮按钮淡入淡出与滑入动效 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
