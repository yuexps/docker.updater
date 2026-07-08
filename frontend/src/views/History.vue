<template>
  <div class="view-fade-in">
      <!-- Page Header -->
      <div class="flex items-center justify-between mb-8 select-none">
        <div>
          <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">升级历史</h1>
        </div>
      </div>

      <!-- History Table Card -->
      <div class="apple-card rounded-lg p-8">
        <div v-if="loading" class="flex justify-center py-20">
          <n-spin size="large" />
        </div>

        <div v-else-if="history.length === 0" class="text-body-muted text-center py-12 text-[14px]">
          无历史升级或回滚记录
        </div>

        <div v-else class="overflow-x-auto">
          <table class="w-full text-left text-[14px] text-slate-700 leading-relaxed border-collapse">
            <thead>
              <tr class="border-b border-hairline text-slate-400 font-semibold">
                <th class="py-3 px-4 font-semibold text-[12px] uppercase tracking-wider">目标容器</th>
                <th class="py-3 px-4 font-semibold text-[12px] uppercase tracking-wider">拉取镜像名</th>
                <th class="py-3 px-4 font-semibold text-[12px] uppercase tracking-wider">执行完成时间</th>
                <th class="py-3 px-4 font-semibold text-[12px] uppercase tracking-wider">运行结果</th>
                <th class="py-3 px-4 font-semibold text-[12px] uppercase tracking-wider text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr 
                v-for="h in history" 
                :key="h.ID" 
                class="border-b border-hairline/60 hover:bg-slate-50 transition-colors"
              >
                <td class="py-4 px-4 font-semibold text-slate-800 break-all">{{ h.ContainerName }}</td>
                <td class="py-4 px-4 font-mono text-[12px] break-all max-w-xs">{{ h.Image }}</td>
                <td class="py-4 px-4 text-slate-500 text-[12px]">{{ formatCheckTime(h.UpdatedAt) }}</td>
                <td class="py-4 px-4">
                  <span 
                    :class="h.Status === 'success' ? 'text-emerald-600' : 'text-red-600'"
                    class="font-bold text-[12px]"
                  >
                    {{ h.Status === 'success' ? '[升级成功]' : '[执行失败]' }}
                  </span>
                </td>
                <td class="py-4 px-4 text-right">
                  <n-button 
                    size="small" 
                    round 
                    secondary 
                    class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale"
                    @click="viewHistoricalLog(h.ContainerName)"
                  >
                    查看详情日志
                  </n-button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

    <!-- Modal 1: Pure Flat Dark Terminal -->
    <n-modal 
      v-model:show="logModalVisible" 
      :mask-closable="false" 
      preset="card" 
      class="max-w-3xl bg-slate-900 text-white rounded-lg"
      title="升级作业历史日志"
    >
      <div class="bg-black p-4 rounded font-mono text-[12px] h-[400px] overflow-y-auto border border-slate-800" ref="terminalLog">
        <div v-for="(line, idx) in logLines" :key="idx" class="py-0.5 break-all">
          <span v-if="line.includes('[SUCCESS]')" class="text-green-400 font-semibold">{{ line }}</span>
          <span v-else-if="line.includes('[ERROR]')" class="text-red-400 font-semibold">{{ line }}</span>
          <span v-else-if="line.includes('[WARNING]')" class="text-amber-400 font-semibold">{{ line }}</span>
          <span v-else-if="line.includes('[PULL]')" class="text-sky-400">{{ line }}</span>
          <span v-else class="text-slate-300">{{ line }}</span>
        </div>
      </div>
      <template #action>
        <div class="flex justify-end">
          <n-button type="primary" round size="medium" @click="logModalVisible = false">
            关闭窗口
          </n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { NButton, NSpin, NModal, useMessage } from 'naive-ui'
import axios from 'axios'
import wsService from '../utils/websocket'

const apiBase = '/app/docker-updater/api'
const message = useMessage()

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

const formatCheckTime = (isoStr: string) => {
  if (!isoStr) return '无'
  const date = new Date(isoStr)
  return date.toLocaleString()
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
