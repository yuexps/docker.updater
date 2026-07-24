<template>
  <div class="view-fade-in flex flex-col h-full overflow-hidden">
    <!-- 页面头部 -->
    <div class="shrink-0 px-3 md:px-5 lg:px-6 pt-3 md:pt-4 lg:pt-5 pb-3 md:pb-4 select-none bg-canvas-parchment">
      <div class="flex items-center justify-between">
        <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">概览</h1>
        
        <div class="flex items-center space-x-2 shrink-0">
          <n-button 
            :loading="checking" 
            round 
            secondary
            size="small"
            class="active-scale"
            @click="checkUpdates"
          >
            {{ checking ? '检测中...' : '检测更新' }}
          </n-button>
        </div>
      </div>
      
    </div>

    <!-- 页面内容 -->
    <div class="flex-1 min-w-0 overflow-y-auto px-3 md:px-5 lg:px-6 pb-24">
      <!-- 统计卡片区 -->
      <div class="grid grid-cols-2 gap-4 md:gap-6 mb-10 select-none">
        <!-- 待升级卡片 -->
        <div 
          class="apple-card p-5 sm:p-6 rounded-lg transition-all duration-300 cursor-pointer select-none"
          :class="[
            currentFilter === 'update'
              ? 'border-2 border-primary! bg-white'
              : 'border-2 border-slate-100 bg-white hover:border-primary/35'
          ]"
          @click="currentFilter = 'update'"
        >
          <span class="text-[12px] font-semibold text-body-muted uppercase tracking-wider block">待升级容器</span>
          <span class="text-[36px] font-bold tracking-tight block mt-2 text-primary leading-none">{{ updateCount }}</span>
        </div>

        <!-- 已暂挂卡片 -->
        <div 
          class="apple-card p-5 sm:p-6 rounded-lg transition-all duration-300 cursor-pointer select-none"
          :class="[
            currentFilter === 'deferred'
              ? 'border-2 border-amber-500! bg-white'
              : 'border-2 border-slate-100 bg-white hover:border-amber-500/35'
          ]"
          @click="currentFilter = 'deferred'"
        >
          <span class="text-[12px] font-semibold text-body-muted uppercase tracking-wider block">已暂挂检测</span>
          <span class="text-[36px] font-bold tracking-tight block mt-2 text-amber-600 leading-none">{{ deferredCount }}</span>
        </div>
      </div>

      <!-- 可升级容器列表 -->
      <div>
        <div class="flex items-baseline justify-between mb-6 select-none">
          <h2 class="text-[21px] font-semibold tracking-tight apple-headline">可升级容器</h2>
          <span v-if="lastCheck" class="text-[12px] text-body-muted font-normal flex items-center">
            <svg class="w-3.5 h-3.5 mr-1 text-slate-400 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10" />
              <path d="M12 6v6l4 2" />
            </svg>
            上次检测: {{ formatCheckTime(lastCheck) }}
          </span>
        </div>

        <div v-if="!loading && updatableContainerNames.length > 0 && selectedContainers.length > 0" class="flex items-center justify-between mb-5 bg-white/40 dark:bg-zinc-900/10 px-4 py-2.5 rounded-xl border border-hairline select-none">
          <div class="flex items-center space-x-2">
            <n-checkbox 
              :checked="isAllSelected" 
              :indeterminate="isSomeSelected"
              @update:checked="handleSelectAll"
            >
              <span class="text-[13px] font-medium text-slate-700">全选</span>
            </n-checkbox>
            <span class="text-[11px] text-body-muted">已选 {{ selectedContainers.length }} 个</span>
          </div>

          <div class="flex items-center gap-2">
            <n-button 
              round 
              secondary
              size="small"
              class="active-scale bg-surface-pearl border border-divider-soft text-slate-700 font-medium"
              @click="selectedContainers = []"
            >
              取消
            </n-button>
            <n-button 
              type="primary"
              round
              size="small"
              class="active-scale font-semibold"
              :disabled="selectedContainers.length === 0"
              @click="actions.startBulkUpdate(selectedContainers, () => selectedContainers = [])"
            >
              批量升级
            </n-button>
          </div>
        </div>

        <div v-if="loading" class="flex justify-center py-20">
          <n-spin size="large" />
        </div>

        <!-- 均已是最新空状态 -->
        <div v-else-if="pendingContainers.length === 0" class="apple-card rounded-lg p-8 sm:p-16 flex flex-col items-center justify-center text-center select-none bg-white">
          <div class="w-16 h-16 rounded-full bg-emerald-50 border border-emerald-100 flex items-center justify-center text-emerald-500 mb-6 shadow-xs">
            <svg class="w-8 h-8" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
              <polyline points="22 4 12 14.01 9 11.01" />
            </svg>
          </div>
          <h3 class="text-[17px] font-bold text-slate-800 mb-2">
            {{ currentFilter === 'deferred' ? '暂无被暂挂的容器' : '所有本地容器已是最新版本' }}
          </h3>
          <p class="text-[13px] text-body-muted max-w-sm leading-relaxed mb-0">
            {{ currentFilter === 'deferred' ? '未检测到任何被设置暂挂检测的本地运行容器。' : '未检测到待升级的本地运行容器。系统将会在后台定时检测镜像版本。您也可以手动触发即时检测。' }}
          </p>
        </div>

        <!-- 待升级/已暂挂列表 -->
        <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(310px,1fr))] gap-5">
          <container-card
            v-for="c in pendingContainers" 
            :key="c.name"
            :container="c"
            mode="dashboard"
            :show-checkbox="c.status === 'update'"
            :checked="selectedContainers.includes(c.name)"
            @update:checked="(val) => toggleSelect(c.name, val)"
            @update="actions.updateContainer(c.name)"
            @defer="actions.openDeferModal(c.name)"
            @undefer="actions.undeferContainer(c.name)"
            @update-to-version="actions.openUpdateVersionModal(c.name, c.image)"
            @check="actions.checkContainer(c.name)"
          />
        </div>
      </div>
    </div>

    <!-- 弹窗一：公共 Mac 日志终端 -->
    <terminal-modal
      :show="actions.logModalVisible.value"
      :log-lines="actions.logLines.value"
      :log-running="actions.logRunning.value"
      @close="actions.closeLogModal"
    />

    <!-- 弹窗二：公共暂挂选择框 -->
    <defer-modal
      v-model:show="actions.deferModalVisible.value"
      v-model:days="actions.deferDays.value"
      @submit="actions.submitDefer()"
    />

    <!-- 弹窗三：指定版本升级选择框 -->
    <update-version-modal
      v-model:show="actions.updateVersionModalVisible.value"
      v-model:version="actions.updateVersionValue.value"
      :current-image="actions.updateVersionCurrentImage.value"
      @submit="actions.submitUpdateVersion()"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { NButton, NSpin, NCheckbox, useMessage } from 'naive-ui'
import axios from 'axios'
import wsService from '../utils/websocket'
import { useContainerActions } from '../composables/useContainerActions'
import ContainerCard from '../components/ContainerCard.vue'
import TerminalModal from '../components/TerminalModal.vue'
import DeferModal from '../components/DeferModal.vue'
import UpdateVersionModal from '../components/UpdateVersionModal.vue'

const apiBase = '/app/docker-updater/api'

const containers = ref<any[]>([])
const lastCheck = ref<string>('')
const loading = ref<boolean>(false)
const checking = ref<boolean>(false)

const selectedContainers = ref<string[]>([])
const currentFilter = ref<'update' | 'deferred'>('update')
const message = useMessage()

// 初始化共享 Composable 业务方法
const actions = useContainerActions()

let unsubscribeStatus: (() => void) | null = null

// 待更新或已挂起列表
const pendingContainers = computed(() => {
  return containers.value
    .filter(c => c.status === currentFilter.value)
    .slice()
    .sort((a, b) => a.name.localeCompare(b.name))
})

const updateCount = computed(() => {
  return containers.value.filter(c => c.status === 'update').length
})
const deferredCount = computed(() => {
  return containers.value.filter(c => c.status === 'deferred').length
})

// 所有可升级容器名列表
const updatableContainerNames = computed(() => {
  return pendingContainers.value
    .filter(c => c.status === 'update')
    .map(c => c.name)
})

// 是否为全选状态
const isAllSelected = computed(() => {
  const names = updatableContainerNames.value
  if (names.length === 0) return false
  return names.every(name => selectedContainers.value.includes(name))
})

// 是否为半选状态
const isSomeSelected = computed(() => {
  const names = updatableContainerNames.value
  if (names.length === 0) return false
  const selectedCount = names.filter(name => selectedContainers.value.includes(name)).length
  return selectedCount > 0 && selectedCount < names.length
})

// 全选操作逻辑
const handleSelectAll = (checked: boolean) => {
  const names = updatableContainerNames.value
  if (checked) {
    names.forEach(name => {
      if (!selectedContainers.value.includes(name)) {
        selectedContainers.value.push(name)
      }
    })
  } else {
    selectedContainers.value = selectedContainers.value.filter(
      name => !names.includes(name)
    )
  }
}

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
  actions.cleanup()
})
</script>
