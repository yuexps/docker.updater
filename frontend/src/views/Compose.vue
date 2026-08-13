<template>
  <div class="view-fade-in flex flex-col h-full overflow-hidden">
    <!-- 页面头部 -->
    <div class="shrink-0 px-3 md:px-5 lg:px-6 pt-3 md:pt-4 lg:pt-5 pb-3 md:pb-4 select-none bg-canvas-parchment">
      <div class="flex items-center justify-between">
        <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">Compose 项目</h1>
      </div>
    </div>

    <!-- 页面内容主体：Compose 编排管理卡片列表 -->
    <div class="flex-1 min-w-0 overflow-y-auto px-3 md:px-5 lg:px-6 pb-24">

      <!-- 加载状态 -->
      <div v-if="dockerStore.loading && filteredProjects.length === 0" class="flex justify-center py-20">
        <n-spin size="large" />
      </div>

      <!-- 空状态 -->
      <div v-else-if="filteredProjects.length === 0"
        class="apple-card p-12 text-center text-body-muted rounded-lg bg-white">
        未发现包含 com.docker.compose.project Label 的编排项目
      </div>

      <!-- Compose 项目列表卡片 -->
      <div v-else class="space-y-5">
        <div v-for="project in filteredProjects" :key="project.name" :id="'compose-card-' + project.name"
          class="apple-card rounded-lg p-5 flex flex-col justify-between transition-all duration-300 bg-white"
          :class="{ 'border-2! border-primary! shadow-md shadow-blue-500/10': project.name === highlightProjectName, 'hover:border-primary/40': project.name !== highlightProjectName }">

          <!-- 项目头部：组合名称、路径元数据与项目级动作 -->
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-slate-100">

            <div class="flex items-center space-x-3 min-w-0">
              <!-- 标志图标 -->
              <div
                class="w-10 h-10 rounded-xl bg-slate-50 border border-slate-100 flex items-center justify-center text-slate-500 shrink-0">
                <svg class="w-5 h-5 text-slate-500" viewBox="0 0 24 24">
                  <g fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path
                      d="M22 12.54c-1.804-.345-2.701-1.08-3.523-2.94c-.487.696-1.102 1.568-.92 2.4c.028.238-.32 1.002-.557 1H3c0 5.208 3.164 7 6.196 7c4.124.022 7.828-1.376 9.854-5c1.146-.101 2.296-1.505 2.95-2.46z">
                    </path>
                    <path d="M5 10h3v3H5z"></path>
                    <path d="M8 10h3v3H8z"></path>
                    <path d="M11 10h3v3h-3z"></path>
                    <path d="M8 7h3v3H8z"></path>
                    <path d="M11 7h3v3h-3z"></path>
                    <path d="M11 4h3v3h-3z"></path>
                    <path d="M4.571 18c1.5 0 2.047-.074 2.958-.78"></path>
                    <path d="M10 16v.01"></path>
                  </g>
                </svg>
              </div>

              <div class="min-w-0">
                <div class="flex items-center space-x-2">
                  <span class="text-[18px] font-bold text-slate-800 tracking-tight truncate select-all"
                    :title="project.name">
                    {{ project.name }}
                  </span>

                  <!-- 项目运行状态 -->
                  <div class="shrink-0">
                    <div class="flex items-center space-x-1 px-2 py-0.5 rounded-full text-[11px] font-medium border"
                      :class="[
                        project.status === 'running'
                          ? 'bg-emerald-50 text-emerald-700 border-emerald-100'
                          : project.status === 'partial'
                            ? 'bg-amber-50 text-amber-700 border-amber-100'
                            : 'bg-slate-50 text-slate-500 border-slate-200'
                      ]">
                      <span v-if="project.status === 'running'" class="relative flex h-1.5 w-1.5">
                        <span
                          class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                        <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500"></span>
                      </span>
                      <span v-else-if="project.status === 'partial'"
                        class="h-1.5 w-1.5 rounded-full bg-amber-500"></span>
                      <span v-else class="h-1.5 w-1.5 rounded-full bg-slate-400"></span>
                      <span>{{ project.status === 'running' ? '运行中' : project.status === 'partial' ? '部分运行' : '已停止'
                      }}</span>
                    </div>
                  </div>

                  <!-- 可升级状态指示 -->
                  <span v-if="project.has_update"
                    class="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-50 text-amber-700 border border-amber-200 shrink-0">
                    存在可更新服务
                  </span>
                </div>

                <div class="flex items-center gap-2 mt-1 min-w-0 text-[11px]">
                  <span
                    class="inline-flex items-center px-2 py-0.5 rounded-md bg-slate-100 font-semibold text-slate-500 border border-slate-200/40 shrink-0 whitespace-nowrap">
                    {{ project.containers.length }} 服务
                  </span>
                  <span v-if="project.working_dir" class="font-mono text-slate-400 truncate min-w-0 flex-1"
                    :title="project.working_dir">
                    {{ project.working_dir }}
                  </span>
                </div>
              </div>
            </div>

            <!-- 项目级控制按钮组 -->
            <div class="shrink-0">
              <n-space :size="8" align="center" class="flex-wrap">
                <!-- 按钮 1: 启动/停止 动态切换 -->
                <n-button v-if="project.status === 'running' || project.status === 'partial'" size="small" round
                  secondary type="error" class="active-scale font-medium border border-rose-200/60 shadow-2xs"
                  @click="handleBatchLifecycle(project, 'stop')">
                  停止
                </n-button>
                <n-button v-else size="small" round secondary type="success"
                  class="active-scale font-medium border border-emerald-200/60 shadow-2xs"
                  @click="handleBatchLifecycle(project, 'start')">
                  启动
                </n-button>

                <!-- 按钮 2: 重启 -->
                <n-button size="small" round secondary
                  class="bg-slate-50 border border-slate-200 text-slate-700 active-scale font-medium shadow-2xs"
                  @click="handleBatchLifecycle(project, 'restart')">
                  重启
                </n-button>

                <!-- 按钮 3: 重建 -->
                <n-button size="small" round secondary type="warning"
                  class="active-scale font-medium border border-amber-200/60 shadow-2xs"
                  @click="handleBatchLifecycle(project, 'recreate')">
                  重建
                </n-button>

                <!-- 按钮 4: 编辑 -->
                <n-button size="small" round secondary
                  class="bg-slate-50 border border-slate-200 text-slate-700 active-scale font-medium shadow-2xs"
                  @click="openEditor(project.name)">
                  <template #icon>
                    <svg class="w-3.5 h-3.5 text-slate-500" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                      stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                      <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                    </svg>
                  </template>
                  编辑
                </n-button>
              </n-space>
            </div>

          </div>

          <!-- 组内 Compose Services 拓扑列表 -->
          <div class="mt-4">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div v-for="item in project.containers" :key="item.name"
                class="p-3 rounded-lg border border-slate-100 bg-slate-50/50 flex items-center justify-between gap-3">

                <div class="flex items-center space-x-2.5 min-w-0">
                  <span class="h-2 w-2 rounded-full shrink-0"
                    :class="item.state === 'running' ? 'bg-emerald-500' : 'bg-slate-300'" />

                  <div class="min-w-0">
                    <div class="flex items-center space-x-1.5">
                      <span class="text-[14px] font-bold text-slate-800 tracking-tight truncate select-all">{{
                        item.service ||
                        item.name }}</span>

                      <!-- 容器 Badge 徽章 -->
                      <span v-if="item.name"
                        class="inline-flex items-center px-1.5 py-0.5 rounded-md bg-slate-100/90 text-[10px] font-mono font-semibold text-slate-500 border border-slate-200/60 shrink-0 cursor-pointer hover:bg-slate-200/80 hover:text-slate-700 transition-colors"
                        :title="'点击在容器列表中定位: ' + item.name" @click.stop="navigateToContainer(item.name)">
                        <svg class="w-2.5 h-2.5 mr-1 text-slate-400 shrink-0" viewBox="0 0 32 32" fill="currentColor">
                          <path d="M17 13V6H8v16h16v-9zm-7-5h5v5h-5zm0 7h5v5h-5zm12 5h-5v-5h5z"></path>
                        </svg>
                        {{ item.name }}
                      </span>
                    </div>
                    <div class="text-[11px] text-slate-400 font-mono truncate" :title="item.image">
                      {{ item.image }}
                    </div>
                  </div>
                </div>

                <div class="shrink-0 flex items-center space-x-2">
                  <span v-if="item.has_update" class="inline-flex items-center">
                    <n-button size="tiny" type="primary" round class="px-2.5 font-medium active-scale shadow-xs"
                      @click="triggerContainerUpgrade(item)">
                      升级
                    </n-button>
                  </span>

                  <span v-else class="text-[11px] text-slate-400 font-medium px-1">
                    {{ item.state === 'running' ? '运行中' : '已停止' }}
                  </span>
                </div>

              </div>
            </div>
          </div>

        </div>
      </div>

    </div>

    <!-- 升级实时日志终端弹窗 -->
    <TerminalModal :show="logModalVisible" title="升级部署进度" :log-lines="logLines" :log-running="logRunning"
      @close="closeLogModal" />

    <!-- Compose 配置文件编辑器弹窗 -->
    <ComposeEditorModal :show="showEditorModal" :project-name="editingProject" @close="showEditorModal = false"
      @saved="dockerStore.fetchComposeProjects(true)" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NSpin, NSpace, useDialog } from 'naive-ui'
import TerminalModal from '../components/TerminalModal.vue'
import ComposeEditorModal from '../components/ComposeEditorModal.vue'
import { useContainerActions } from '../composables/useContainerActions'
import { useDockerStore, ComposeProject, ComposeContainerItem } from '../stores/useDockerStore'
import { message } from '../utils/discrete'

const dialog = useDialog()
const route = useRoute()
const router = useRouter()
const dockerStore = useDockerStore()

const currentFilter = ref<'all' | 'update'>('all')
const showEditorModal = ref<boolean>(false)
const editingProject = ref<string>('')
const highlightProjectName = ref<string>('')

const {
  logModalVisible,
  logLines,
  logRunning,
  updateContainer,
  closeLogModal
} = useContainerActions()

const navigateToContainer = (containerName: string) => {
  if (!containerName) return
  router.push({
    path: '/containers',
    query: { name: containerName }
  })
}

const filteredProjects = computed(() => {
  let list = dockerStore.sortedComposeProjects
  if (currentFilter.value === 'update') {
    list = list.filter(p => p.has_update)
  }
  return list
})

const openEditor = (projectName: string) => {
  editingProject.value = projectName
  showEditorModal.value = true
}

const handleBatchLifecycle = (project: ComposeProject, action: 'start' | 'stop' | 'restart' | 'recreate') => {
  const actionText = action === 'start' ? '启动' : action === 'stop' ? '停止' : action === 'restart' ? '重启' : '重建'
  dialog.warning({
    title: `确认${actionText}项目`,
    content: `确定要对项目 [${project.name}] 下的 ${project.containers.length} 个服务执行 ${actionText} 操作吗？`,
    positiveText: '确认',
    negativeText: '取消',
    onPositiveClick: () => {
      executeLifecycle(project.name, action, actionText)
    }
  })
}

const executeLifecycle = async (projectName: string, action: 'start' | 'stop' | 'restart' | 'recreate', actionText: string) => {
  try {
    await dockerStore.handleComposeLifecycle(projectName, action)
    message.success(`已成功下发 Compose 项目 [${projectName}] 的 ${actionText} 命令`)
  } catch {
    message.error(`对 Compose 项目 [${projectName}] 执行 ${actionText} 失败`)
  }
}

const triggerContainerUpgrade = (container: ComposeContainerItem) => {
  updateContainer(container.name)
}

const handleQueryProjectHighlight = () => {
  const targetProject = route.query.project as string
  if (targetProject) {
    highlightProjectName.value = targetProject
    setTimeout(() => {
      const el = document.getElementById(`compose-card-${targetProject}`)
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }
    }, 150)
  }
}

watch(() => route.query.project, () => {
  handleQueryProjectHighlight()
})

onMounted(async () => {
  await dockerStore.fetchComposeProjects()
  handleQueryProjectHighlight()
})
</script>
