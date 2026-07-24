<template>
  <div class="view-fade-in flex flex-col h-full overflow-hidden">
    <!-- 顶栏区域 -->
    <div class="shrink-0 px-3 md:px-5 lg:px-6 pt-3 md:pt-4 lg:pt-5 pb-3 md:pb-4 select-none bg-canvas-parchment">
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-3 md:gap-4">
        <!-- 视图标题 -->
        <div class="flex items-center justify-between md:justify-start shrink-0">
          <h1 class="text-[24px] md:text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">运行日志</h1>

          <!-- 移动端清空按钮 -->
          <div class="md:hidden">
            <n-button type="error" round size="small" ghost class="active-scale text-[12px] font-medium"
              :loading="clearing" @click="clearSystemLogs">
              清空日志
            </n-button>
          </div>
        </div>

        <!-- 日志等级筛选器 -->
        <div class="flex-1 flex items-center justify-center">
          <div
            class="flex items-center p-0.5 bg-slate-100/80 rounded-full border border-slate-200/40 shrink-0 select-none overflow-x-auto scrollbar-none w-full sm:w-auto">
            <button v-for="lvl in levels" :key="lvl.value" @click="selectedLevel = lvl.value"
              class="px-3.5 py-1 md:py-1.5 rounded-full text-[12px] font-medium transition-all duration-200 cursor-pointer text-center shrink-0 flex-1 sm:flex-none min-w-14"
              :class="selectedLevel === lvl.value ? 'bg-primary text-white shadow-xs font-semibold' : 'text-slate-500 hover:text-slate-800'">
              {{ lvl.label }}
            </button>
          </div>
        </div>

        <!-- 搜索与桌面端清空按钮 -->
        <div class="flex items-center gap-2.5 md:gap-3 shrink-0">
          <!-- 关键字搜索框 -->
          <div class="relative min-w-0 w-full sm:w-52 md:w-56 lg:w-60">
            <input v-model="searchQuery" type="text" placeholder="搜索日志关键字..."
              class="w-full pl-9 pr-4 py-1 md:py-1.5 bg-white sm:bg-slate-50 border border-hairline rounded-full text-[13px] text-slate-700 placeholder-slate-400 focus:outline-none focus:border-primary focus:bg-white transition-all font-sans shadow-2xs sm:shadow-none" />
            <span class="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 select-none pointer-events-none">
              <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"
                stroke-linecap="round" stroke-linejoin="round">
                <circle cx="11" cy="11" r="8" />
                <line x1="21" y1="21" x2="16.65" y2="16.65" />
              </svg>
            </span>
          </div>

          <!-- 桌面端清空按钮 -->
          <div class="hidden md:block shrink-0">
            <n-button type="error" round size="small" ghost class="active-scale text-[12px] font-medium"
              :loading="clearing" @click="clearSystemLogs">
              清空日志
            </n-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 主内容容器 -->
    <div class="flex-1 min-w-0 flex flex-col min-h-0 px-1.5 md:px-2 lg:px-2.5 pb-2 md:pb-1.5 lg:pb-2">
      <div class="apple-card p-2 sm:p-3 rounded-lg flex-1 flex flex-col min-h-0 bg-white">

        <!-- 加载与空状态 -->
        <div v-if="loading" class="flex justify-center items-center flex-1">
          <n-spin size="large" />
        </div>

        <div v-else-if="logLines.length === 0"
          class="text-body-muted text-center flex-1 flex items-center justify-center text-[14px] select-none">
          当前暂无系统运行日志输出
        </div>

        <!-- 日志容器 -->
        <div v-else class="flex-1 min-h-0 relative flex flex-col">
          <div ref="logContainer" @scroll="handleScroll"
            class="flex-1 overflow-y-auto scrollbar-thin select-text space-y-2 p-1">
            <div v-if="filteredLines.length === 0" class="text-body-muted text-center py-10 text-[13px] select-none">
              未检索到匹配的日志行
            </div>
            <div v-else class="space-y-2">
              <!-- 单条日志容器（内联文本流支持贴左折行） -->
              <div v-for="(line, idx) in filteredLines" :key="idx"
                class="p-2.5 sm:p-3 rounded-lg border border-slate-200/60 bg-slate-50/40 hover:bg-white hover:border-slate-300 hover:shadow-2xs transition-all text-[12px] font-mono leading-relaxed">
                <template v-for="parsed in [parseLine(line)]" :key="idx">
                  <template v-if="parsed">
                    <div class="break-all whitespace-pre-wrap">
                      <!-- 时间戳 -->
                      <span v-if="parsed.time"
                        class="inline-block mr-2.5 text-slate-400 text-[11.5px] align-baseline">
                        {{ parsed.time }}
                      </span>

                      <!-- 日志等级徽章 -->
                      <span :class="parsed.badgeClass"
                        class="inline-block mr-2.5 px-1.5 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider align-baseline">
                        {{ parsed.level }}
                      </span>

                      <!-- 日志正文 -->
                      <span :class="parsed.textClass">
                        {{ parsed.content }}
                      </span>
                    </div>
                  </template>

                  <!-- 无标签日志正文 -->
                  <template v-else>
                    <div class="break-all whitespace-pre-wrap text-slate-600">
                      {{ line }}
                    </div>
                  </template>
                </template>
              </div>
            </div>
          </div>

          <!-- 快捷置底按钮 -->
          <transition name="fade">
            <button v-show="showScrollBtn" @click="scrollToBottom"
              class="absolute right-4 bottom-4 sm:right-6 sm:bottom-6 w-9 h-9 sm:w-10 sm:h-10 rounded-full bg-primary text-white flex items-center justify-center shadow-lg hover:bg-primary-focus active-scale transition-all duration-200 cursor-pointer select-none border border-primary/10 z-10"
              title="回到底部">
              <svg class="w-4.5 h-4.5 sm:w-5 sm:h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="6 9 12 15 18 9" />
              </svg>
            </button>
          </transition>
        </div>
      </div>
    </div>

    <!-- 移动端底栏占位 -->
    <div class="h-16 md:hidden shrink-0"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, computed } from 'vue'
import { NButton, NSpin, useMessage, useDialog } from 'naive-ui'
import axios from 'axios'
import wsService from '../utils/websocket'

const apiBase = '/app/docker-updater/api'
const message = useMessage()
const dialog = useDialog()

// 状态声明与元素引用
const logLines = ref<string[]>([])
const loading = ref<boolean>(false)
const clearing = ref<boolean>(false)
const logContainer = ref<HTMLDivElement | null>(null)

// 订阅取消句柄
let unsubscribeSysLog: (() => void) | null = null

// 筛选与滚动组件状态
const selectedLevel = ref('ALL')
const searchQuery = ref('')
const showScrollBtn = ref(false)

const levels = [
  { label: 'ALL', value: 'ALL' },
  { label: 'INFO', value: 'INFO' },
  { label: 'WARN', value: 'WARN' },
  { label: 'ERROR', value: 'ERROR' }
]

// 过滤后的日志列表
const filteredLines = computed(() => {
  let lines = logLines.value

  if (selectedLevel.value !== 'ALL') {
    lines = lines.filter(line => line.includes(`[${selectedLevel.value}]`))
  }

  if (searchQuery.value.trim()) {
    const query = searchQuery.value.toLowerCase()
    lines = lines.filter(line => line.toLowerCase().includes(query))
  }

  return lines
})

// 解析后日志格式定义
interface ParsedLine {
  time: string
  level: string
  content: string
  badgeClass: string
  textClass: string
}

// 日志等级标签解析器
const parseLine = (line: string): ParsedLine | null => {
  const targetLevels = ['ERROR', 'WARN', 'INFO'] as const
  for (const lvl of targetLevels) {
    const tag = `[${lvl}]`
    if (line.includes(tag)) {
      const idx = line.indexOf(tag)
      const timePart = line.substring(0, idx).trim()
      const content = line.substring(idx + tag.length).trim()

      let badgeClass = ''
      let textClass = 'text-slate-700'

      if (lvl === 'ERROR') {
        badgeClass = 'bg-rose-100 text-rose-700 border border-rose-200'
        textClass = 'text-rose-700 font-medium'
      } else if (lvl === 'WARN') {
        badgeClass = 'bg-amber-100 text-amber-800 border border-amber-200'
        textClass = 'text-amber-800 font-medium'
      } else {
        badgeClass = 'bg-blue-100 text-blue-700 border border-blue-200'
        textClass = 'text-slate-700'
      }

      return {
        time: timePart,
        level: lvl,
        content: content || line,
        badgeClass,
        textClass
      }
    }
  }
  return null
}

// 日志容器滚动监听
const handleScroll = (e: Event) => {
  const el = e.target as HTMLDivElement
  if (!el) return

  const threshold = 100
  const isFarFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight > threshold
  showScrollBtn.value = isFarFromBottom
}

// 容器视图滚动至底部
const scrollToBottom = () => {
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
      showScrollBtn.value = false
    }
  })
}

// 拉取历史日志快照
const fetchSystemLogs = async () => {
  loading.value = true
  try {
    const res = await axios.get(`${apiBase}/system/logs`)
    logLines.value = res.data.logs || []
  } catch {
    message.error('加载系统运行日志失败')
  } finally {
    loading.value = false
    nextTick(() => {
      scrollToBottom()
    })
  }
}

// 确认清空系统运行日志
const clearSystemLogs = () => {
  dialog.warning({
    title: '确认清空运行日志',
    content: '确定要清空当前系统运行日志吗？该操作不可撤销。',
    positiveText: '确认清空',
    negativeText: '取消',
    onPositiveClick: () => {
      return new Promise<void>(async (resolve, reject) => {
        clearing.value = true
        try {
          const res = await axios.delete(`${apiBase}/system/logs`)
          if (res.data.ok) {
            message.success('已成功清空系统运行日志')
            logLines.value = []
            showScrollBtn.value = false
            resolve()
          } else {
            reject()
          }
        } catch {
          message.error('清空日志文件失败')
          reject()
        } finally {
          clearing.value = false
        }
      })
    }
  })
}

// 生命周期挂载与卸载
onMounted(async () => {
  await fetchSystemLogs()

  unsubscribeSysLog = wsService.subscribeSysLog((line: string) => {
    if (line.trim()) {
      logLines.value.push(line)
      if (logLines.value.length > 1000) {
        logLines.value.shift()
      }
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

.scrollbar-none::-webkit-scrollbar {
  display: none;
}

.scrollbar-none {
  -ms-overflow-style: none;
  scrollbar-width: none;
}

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
