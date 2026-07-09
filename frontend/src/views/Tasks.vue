<template>
  <div class="view-fade-in">
    <!-- Page Header -->
    <div class="flex items-center justify-between mb-8 select-none">
      <div>
        <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">任务队列</h1>
      </div>
    </div>

    <!-- Active Task Card Section -->
    <div class="mb-10">
      <h2 class="text-[15px] font-semibold text-slate-500 uppercase tracking-wider mb-4 select-none">当前活跃任务</h2>
      
      <div v-if="activeTask" class="apple-card p-6 rounded-lg flex items-center justify-between min-h-[80px]">
        <div class="space-y-1.5">
          <div class="flex items-center space-x-3">
            <span class="text-[17px] font-semibold text-slate-800">
              {{ activeTask.type === 'update' ? '容器升级:' : '容器回滚:' }} {{ activeTask.container_name }}
            </span>
            <n-tag type="primary" round size="small" class="animate-pulse">运行中</n-tag>
          </div>
          <div class="text-[13px] text-body-muted font-mono select-none">
            入队时间: {{ formatDate(activeTask.added_at) }}
          </div>
        </div>
      </div>

      <div v-else class="apple-card p-8 text-center text-body-muted rounded-lg select-none">
        当前没有正在运行的升级或回滚任务
      </div>
    </div>

    <!-- Waiting Queue Table Section -->
    <div>
      <h2 class="text-[15px] font-semibold text-slate-500 uppercase tracking-wider mb-4 select-none">等待排队队列</h2>
      
      <div v-if="queuedTasks.length > 0" class="apple-card rounded-lg p-4 sm:p-6 overflow-hidden">
        <div class="overflow-x-auto">
          <table class="w-full border-collapse text-left min-w-[550px]">
            <thead>
              <tr class="border-b border-hairline bg-slate-50/50 text-[13px] font-semibold text-slate-500 select-none">
                <th class="py-4 px-6 w-[100px]">排队序号</th>
                <th class="py-4 px-6">容器名称</th>
                <th class="py-4 px-6 w-[140px]">任务类型</th>
                <th class="py-4 px-6">入队时间</th>
                <th class="py-4 px-6 text-right w-[150px]">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-hairline">
              <tr 
                v-for="(task, index) in queuedTasks" 
                :key="task.container_name"
                class="text-[14px] text-slate-700 hover:bg-slate-50/30 transition-colors"
              >
                <td class="py-4 px-6 font-mono font-semibold text-slate-500">
                  #{{ index + 1 }}
                </td>
                <td class="py-4 px-6 font-semibold text-slate-800">
                  {{ task.container_name }}
                </td>
                <td class="py-4 px-6">
                  <n-tag v-if="task.type === 'update'" size="small" round>升级</n-tag>
                  <n-tag v-else type="warning" size="small" round>回滚</n-tag>
                </td>
                <td class="py-4 px-6 text-[13px] text-body-muted font-mono">
                  {{ formatDate(task.added_at) }}
                </td>
                <td class="py-4 px-6 text-right">
                  <n-button 
                    type="error"
                    size="small"
                    quaternary
                    round
                    class="active-scale"
                    @click="cancelQueuedTask(task.container_name)"
                  >
                    取消排队
                  </n-button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-else class="apple-card p-8 text-center text-body-muted rounded-lg select-none">
        排队队列中目前没有等待的任务
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { NButton, NTag, useMessage } from 'naive-ui'
import axios from 'axios'
import wsService from '../utils/websocket'

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

const cancelQueuedTask = async (name: string) => {
  try {
    const res = await axios.post(`${apiBase}/tasks/cancel/${name}`)
    if (res.data.success) {
      message.success(`已成功移出排队任务: ${name}`)
    } else {
      message.error('无法取消该任务，可能它已开始运行')
    }
  } catch (err) {
    message.error('取消排队请求失败')
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
})
</script>
