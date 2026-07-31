<template>
  <div class="view-fade-in flex flex-col h-full overflow-hidden">
    <!-- 页面头部 -->
    <div class="shrink-0 px-3 md:px-5 lg:px-6 pt-3 md:pt-4 lg:pt-5 pb-3 md:pb-4 select-none bg-canvas-parchment">
      <div class="flex items-center justify-between">
        <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">容器列表</h1>
      </div>
    </div>

    <!-- 页面内容 -->
    <div class="flex-1 min-w-0 overflow-y-auto px-3 md:px-5 lg:px-6 pb-24">
      <!-- 容器网格 -->
      <div>
        <div v-if="dockerStore.loading && containers.length === 0" class="flex justify-center py-20">
          <n-spin size="large" />
        </div>

        <div v-else-if="containers.length === 0"
          class="apple-card p-12 text-center text-body-muted rounded-lg bg-white">
          未发现活动或已停止的容器
        </div>

        <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(310px,1fr))] gap-5">
          <container-card v-for="c in sortedContainers" :id="'container-card-' + c.name" :key="c.name" :container="c"
            mode="full" :show-lifecycle="true"
            :class="{ 'border-2! border-primary! shadow-md shadow-blue-500/10': c.name === highlightContainerName }"
            :operating-containers="actions.operatingContainers.value" @lifecycle="(act) => handleLifecycle(c.name, act)"
            @update="actions.updateContainer(c.name)" @defer="actions.openDeferModal(c.name)"
            @undefer="actions.undeferContainer(c.name)" @show-logs="actions.showLogs(c.name)"
            @update-to-version="actions.openUpdateVersionModal(c.name, c.image)"
            @check="actions.checkContainer(c.name)" />
        </div>
      </div>
    </div>

    <!-- 弹窗一：公共 Mac 日志终端 -->
    <terminal-modal :show="actions.logModalVisible.value" :log-lines="actions.logLines.value"
      :log-running="actions.logRunning.value" @close="actions.closeLogModal" />

    <!-- 弹窗二：公共暂挂选择框 -->
    <defer-modal v-model:show="actions.deferModalVisible.value" v-model:days="actions.deferDays.value"
      @submit="actions.submitDefer()" />

    <!-- 弹窗三：公共诊断运行日志 -->
    <diagnostics-modal :show="actions.diagnosticsVisible.value" :logs="actions.diagnosticsLogs.value"
      @close="actions.diagnosticsVisible.value = false" />

    <!-- 弹窗四：指定版本升级选择框 -->
    <update-version-modal v-model:show="actions.updateVersionModalVisible.value"
      v-model:version="actions.updateVersionValue.value" :current-image="actions.updateVersionCurrentImage.value"
      @submit="actions.submitUpdateVersion()" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { NSpin } from 'naive-ui'
import { useContainerActions } from '../composables/useContainerActions'
import { useDockerStore } from '../stores/useDockerStore'
import ContainerCard from '../components/ContainerCard.vue'
import TerminalModal from '../components/TerminalModal.vue'
import DeferModal from '../components/DeferModal.vue'
import DiagnosticsModal from '../components/DiagnosticsModal.vue'
import UpdateVersionModal from '../components/UpdateVersionModal.vue'

const route = useRoute()
const dockerStore = useDockerStore()
const highlightContainerName = ref<string>((route.query.name as string) || '')

// 直接订阅 Pinia 状态仓库中强排序的容器列表
const sortedContainers = computed(() => dockerStore.sortedContainers)
const containers = computed(() => dockerStore.containers)

// 初始化共享 Composable 业务方法
const actions = useContainerActions()

const handleLifecycle = (name: string, act: 'start' | 'stop' | 'restart') => {
  if (act === 'start') {
    actions.startContainer(name)
  } else if (act === 'stop') {
    actions.stopContainer(name)
  } else if (act === 'restart') {
    actions.restartContainer(name)
  }
}

const checkAndScrollHighlight = () => {
  if (route.query.name) {
    highlightContainerName.value = route.query.name as string
    nextTick(() => {
      const el = document.getElementById('container-card-' + highlightContainerName.value)
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }
    })
  }
}

watch(() => sortedContainers.value, () => {
  checkAndScrollHighlight()
}, { immediate: true })

onUnmounted(() => {
  actions.cleanup()
})
</script>
