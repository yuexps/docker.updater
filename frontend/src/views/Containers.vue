<template>
  <div class="view-fade-in flex flex-col h-full overflow-hidden">
    <!-- 页面头部 -->
    <div class="shrink-0 px-4 md:px-8 lg:px-10 pt-3 md:pt-4 lg:pt-5 pb-3 md:pb-4 select-none bg-canvas-parchment">
      <div class="flex items-center justify-between">
        <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">容器列表</h1>
      </div>
    </div>

    <!-- 页面内容 -->
    <div class="flex-1 min-w-0 overflow-y-auto px-4 md:px-8 lg:px-10 pb-24">
      <!-- 容器网格 -->
      <div>
        <div v-if="loading" class="flex justify-center py-20">
          <n-spin size="large" />
        </div>

        <div v-else-if="containers.length === 0" class="apple-card p-12 text-center text-body-muted rounded-lg bg-white">
          未发现活动或已停止的容器
        </div>

        <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-5">
          <container-card
            v-for="c in sortedContainers" 
            :key="c.name"
            :container="c"
            mode="full"
            :show-lifecycle="true"
            :operating-containers="actions.operatingContainers.value"
            @lifecycle="(act) => handleLifecycle(c.name, act)"
            @update="actions.updateContainer(c.name)"
            @rollback="actions.rollbackContainer(c.name)"
            @defer="actions.openDeferModal(c.name)"
            @undefer="actions.undeferContainer(c.name)"
            @delete-backup="actions.deleteBackup(c.name)"
            @show-logs="actions.showLogs(c.name)"
            @update-to-version="actions.openUpdateVersionModal(c.name, c.image)"
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

    <!-- 弹窗三：公共诊断运行日志 -->
    <diagnostics-modal
      :show="actions.diagnosticsVisible.value"
      :logs="actions.diagnosticsLogs.value"
      @close="actions.diagnosticsVisible.value = false"
    />

    <!-- 弹窗四：指定版本升级选择框 -->
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
import { NSpin } from 'naive-ui'
import wsService from '../utils/websocket'
import { useContainerActions } from '../composables/useContainerActions'
import ContainerCard from '../components/ContainerCard.vue'
import TerminalModal from '../components/TerminalModal.vue'
import DeferModal from '../components/DeferModal.vue'
import DiagnosticsModal from '../components/DiagnosticsModal.vue'
import UpdateVersionModal from '../components/UpdateVersionModal.vue'

const containers = ref<any[]>([])
const loading = ref<boolean>(false)

// 按容器名称字母顺序锁定渲染次序，避免状态改变推送后位置乱跳
const sortedContainers = computed(() => {
  return containers.value.slice().sort((a, b) => a.name.localeCompare(b.name))
})

// 初始化共享 Composable 业务方法
const actions = useContainerActions()

let unsubscribeStatus: (() => void) | null = null

const handleLifecycle = (name: string, act: 'start' | 'stop' | 'restart') => {
  if (act === 'start') {
    actions.startContainer(name)
  } else if (act === 'stop') {
    actions.stopContainer(name)
  } else if (act === 'restart') {
    actions.restartContainer(name)
  }
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
  actions.cleanup()
})
</script>
