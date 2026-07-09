<template>
  <div 
    class="apple-card rounded-lg p-4.5 flex flex-col justify-between min-h-[200px] hover:border-primary hover:shadow-[0_12px_28px_rgba(0,0,0,0.04)] transition-all duration-300 bg-white"
  >
    <!-- 顶部主要信息展示 -->
    <div>
      <div class="flex items-start">
        <!-- 左侧：选择复选框与堆栈图标 -->
        <div class="flex items-center space-x-2 shrink-0">
          <n-checkbox 
            v-if="showCheckbox"
            :checked="checked"
            class="mr-1 touch-auto"
            @update:checked="(val) => emit('update:checked', val)"
          />
          <div class="w-10 h-10 rounded-xl bg-slate-50 border border-slate-100 flex items-center justify-center text-slate-500 shadow-xs shrink-0">
            <svg class="w-5 h-5 text-slate-500" viewBox="0 0 32 32">
              <path d="M17 13V6H8v16h16v-9zm-7-5h5v5h-5zm0 7h5v5h-5zm12 5h-5v-5h5z" fill="currentColor"></path>
              <path d="M28 11h-9V2h9zm-7-2h5V4h-5z" fill="currentColor"></path>
              <path d="M28 20h-2v2h2v6H4v-6h2v-2H4a2.002 2.002 0 0 0-2 2v6a2.002 2.002 0 0 0 2 2h24a2.002 2.002 0 0 0 2-2v-6a2.002 2.002 0 0 0-2-2z" fill="currentColor"></path>
              <circle cx="7" cy="25" r="1" fill="currentColor"></circle>
            </svg>
          </div>
        </div>

        <!-- 右侧内容区：包含容器名、运行状态、镜像/Compose 徽章 -->
        <div class="ml-3 flex-1 min-w-0">
          <!-- 第一行：容器名与运行状态 -->
          <div class="flex items-center justify-between min-w-0">
            <div class="text-[16px] font-bold text-slate-800 tracking-tight truncate select-all" :title="container.name">
              {{ container.name }}
            </div>
            <!-- 运行状态 -->
            <div class="shrink-0 ml-2">
              <div 
                class="flex items-center space-x-1 px-2 py-0.5 rounded-full text-[11px] font-medium border"
                :class="[
                  container.running 
                    ? 'bg-emerald-50 text-emerald-700 border-emerald-100' 
                    : 'bg-slate-50 text-slate-500 border-slate-200'
                ]"
              >
                <span v-if="container.running" class="relative flex h-1.5 w-1.5">
                  <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                  <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500"></span>
                </span>
                <span v-else class="h-1.5 w-1.5 rounded-full bg-slate-400"></span>
                <span>{{ container.running ? '运行中' : '已停止' }}</span>
              </div>
            </div>
          </div>

          <!-- 第二行：镜像名与 Compose 徽章（放置于下方，可延伸至右侧） -->
          <div class="flex items-center gap-1.5 mt-1.5 min-w-0">
            <!-- 镜像名徽章 -->
            <div 
              class="inline-flex items-center px-2 py-0.5 rounded-md bg-slate-100 text-[10px] font-semibold text-slate-500 border border-slate-200/40 max-w-[240px] min-w-0 truncate cursor-pointer hover:bg-slate-200/60 hover:text-slate-700 transition-colors"
              :title="'点击复制完整镜像: ' + container.image"
              @click="copyText(container.image)"
            >
              <svg class="w-2.5 h-2.5 mr-1 shrink-0 text-slate-400" viewBox="0 0 24 24">
                <g fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M12 3l8 4.5v9L12 21l-8-4.5v-9L12 3"></path>
                  <path d="M12 12l8-4.5"></path>
                  <path d="M12 12v9"></path>
                  <path d="M12 12L4 7.5"></path>
                </g>
              </svg>
              <span class="truncate">{{ container.image }}</span>
            </div>

            <!-- Compose 项目徽章 -->
            <div 
              v-if="container.compose_project" 
              class="inline-flex items-center px-2 py-0.5 rounded-md bg-slate-100 text-[10px] font-semibold text-slate-500 border border-slate-200/40 max-w-[150px] shrink-0 truncate"
              :title="container.compose_project"
            >
              <svg class="w-2.5 h-2.5 mr-1 shrink-0 text-slate-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="18" cy="5" r="3"/>
                <circle cx="6" cy="12" r="3"/>
                <circle cx="18" cy="19" r="3"/>
                <path d="M9 10.5l6-3.5M9 13.5l6 3.5"/>
              </svg>
              <span class="truncate">{{ container.compose_project }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 详细镜像元数据及比对 -->
      <div class="mt-4 space-y-2.5">
        <!-- 有更新状态：本地与最新镜像 SHA 比对 -->
        <div v-if="container.status === 'update'" class="bg-blue-50/40 border border-blue-100/50 rounded-xl p-3 h-[76px] flex flex-col justify-between">
          <div class="flex items-center justify-between text-[11px] font-semibold text-slate-500">
            <span>版本摘要比对</span>
            <span class="text-[10px] font-normal text-slate-400" v-if="container.checked_at">检测于: {{ container.checked_at }}</span>
          </div>
          
          <div class="flex items-center space-x-2 text-[11px] font-mono justify-between">
            <div class="bg-white border border-slate-200 px-2.5 py-1 rounded-md text-slate-600 truncate max-w-[120px] xs:max-w-[140px] sm:max-w-[160px] flex items-center" :title="container.local_digest">
              <span class="text-[10px] text-slate-400 font-sans font-semibold mr-1.5 shrink-0">本地</span>
              <span>{{ shortDigest(container.local_digest) || '未知' }}</span>
            </div>
            
            <svg class="w-3.5 h-3.5 text-blue-500 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M5 12h14M12 5l7 7-7 7"/>
            </svg>
            
            <div class="bg-blue-500 text-white px-2.5 py-1 rounded-md truncate max-w-[120px] xs:max-w-[140px] sm:max-w-[160px] flex items-center" :title="container.remote_digest">
              <span class="text-[10px] text-blue-200 font-sans font-semibold mr-1.5 shrink-0">最新</span>
              <span class="font-bold">{{ shortDigest(container.remote_digest) || '获取中' }}</span>
            </div>
          </div>
        </div>

        <!-- 已暂挂状态：仅显示暂挂时间提示 -->
        <div v-else-if="container.status === 'deferred'" class="bg-amber-50/40 border border-amber-100/50 rounded-xl p-3 h-[76px] flex flex-col justify-between">
          <div class="flex items-center justify-between text-[11px] font-semibold text-amber-700">
            <span>版本摘要比对</span>
            <span class="text-[10px] font-normal text-amber-500/80" v-if="container.checked_at">检测于: {{ container.checked_at }}</span>
          </div>
          
          <div class="text-[11px] text-amber-700 flex items-center py-1 mt-1">
            <svg class="w-3.5 h-3.5 mr-1.5 text-amber-600 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"/>
              <path d="M12 6v6l4 2"/>
            </svg>
            <span class="font-medium">检测已暂挂至: {{ container.defer_until || '无限期' }}</span>
          </div>
        </div>

        <!-- 当前已是最新状态展示 -->
        <div v-else class="bg-emerald-50/10 border border-emerald-100/30 rounded-xl p-3 h-[76px] flex flex-col justify-between">
          <div class="flex items-center justify-between text-[11px] font-semibold text-emerald-700">
            <span>版本摘要比对</span>
            <span class="text-[10px] font-normal text-slate-400" v-if="container.checked_at">检测于: {{ container.checked_at }}</span>
          </div>
          
          <div class="text-[11px] text-emerald-700 flex items-center py-1 mt-1">
            <svg class="w-3.5 h-3.5 mr-1.5 text-emerald-500 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
              <path d="M22 4L12 14.01l-3-3"/>
            </svg>
            <span class="font-medium">当前镜像已是最新版本</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 底部操作按钮区域（利用 actionsContainerRef 实时测量宽度防换行） -->
    <div 
      ref="actionsContainerRef"
      class="mt-4 pt-3.5 border-t border-slate-100 flex flex-wrap gap-2 items-center min-w-0"
    >
      <!-- 循环渲染物理平铺的按钮，确保物理显示顺序始终符合规定 -->
      <template v-for="act in layoutResult.flatActions" :key="act.key">
        <!-- 升级 -->
        <n-button 
          v-if="act.key === 'update'"
          type="primary"
          size="small"
          round
          class="active-scale shadow-xs font-semibold shrink-0"
          @click="emit('update')"
        >
          升级
        </n-button>

        <!-- 启动 -->
        <n-button 
          v-else-if="act.key === 'start'"
          size="small"
          round
          secondary
          type="success"
          class="active-scale font-medium animate-none shrink-0"
          :disabled="operatingContainers?.has(container.name + ':start')"
          @click="emit('lifecycle', 'start')"
        >
          启动
        </n-button>

        <!-- 停止 -->
        <n-button 
          v-else-if="act.key === 'stop'"
          size="small"
          round
          secondary
          type="error"
          class="active-scale font-medium animate-none shrink-0"
          :disabled="operatingContainers?.has(container.name + ':stop') || operatingContainers?.has(container.name + ':restart')"
          @click="emit('lifecycle', 'stop')"
        >
          停止
        </n-button>

        <!-- 重启 -->
        <n-button 
          v-else-if="act.key === 'restart'"
          size="small"
          round
          secondary
          class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale font-medium animate-none shrink-0"
          :disabled="operatingContainers?.has(container.name + ':stop') || operatingContainers?.has(container.name + ':restart')"
          @click="emit('lifecycle', 'restart')"
        >
          重启
        </n-button>

        <!-- 日志 -->
        <n-button 
          v-else-if="act.key === 'show-logs'"
          size="small"
          round
          secondary
          class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale font-medium animate-none shrink-0"
          @click="emit('show-logs')"
        >
          日志
        </n-button>

        <!-- 暂挂 -->
        <n-button 
          v-else-if="act.key === 'defer'"
          size="small"
          round
          secondary
          class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale font-medium animate-none shrink-0"
          @click="emit('defer')"
        >
          暂挂
        </n-button>

        <!-- 恢复检测 -->
        <n-button 
          v-else-if="act.key === 'undefer'"
          size="small"
          round
          secondary
          class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale font-medium animate-none shrink-0"
          @click="emit('undefer')"
        >
          恢复检测
        </n-button>

        <!-- 回滚 -->
        <n-button 
          v-else-if="act.key === 'rollback'"
          size="small"
          round
          secondary
          class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale font-medium animate-none shrink-0"
          @click="emit('rollback')"
        >
          回滚
        </n-button>

        <!-- 清除备份 -->
        <n-button 
          v-else-if="act.key === 'delete-backup'"
          size="small"
          round
          secondary
          class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale font-medium animate-none shrink-0"
          title="删除回滚备份镜像释放空间"
          @click="emit('delete-backup')"
        >
          清除备份
        </n-button>

        <!-- 指定版本升级 -->
        <n-button 
          v-else-if="act.key === 'update-to-version'"
          size="small"
          round
          secondary
          class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale font-medium animate-none shrink-0"
          @click="emit('update-to-version')"
        >
          修改版本
        </n-button>
      </template>

      <!-- 更多操作下拉菜单 (固定渲染在末尾) -->
      <n-dropdown 
        v-if="layoutResult.dropdownItems.length > 0"
        trigger="click" 
        :options="layoutResult.dropdownItems" 
        @select="handleMoreSelect"
      >
        <n-button 
          size="small" 
          round 
          secondary 
          class="bg-surface-pearl border border-divider-soft text-slate-700 active-scale font-medium animate-none shrink-0"
        >
          <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/>
          </svg>
        </n-button>
      </n-dropdown>
    </div>

    <!-- 隐藏测宽影子 DOM 容器：仅限 ResizeObserver 静态测量实际字号/字体的按钮物理边框 -->
    <div class="absolute opacity-0 pointer-events-none select-none -z-50 flex items-center gap-2" style="top: -9999px; left: -9999px;">
      <n-button :id="container.name + '-m2'" size="small" round secondary>测试</n-button>
      <n-button :id="container.name + '-m4'" size="small" round secondary>恢复检测</n-button>
      <n-button :id="container.name + '-mmore'" size="small" round secondary>
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/>
        </svg>
      </n-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, nextTick } from 'vue'
import { NButton, NCheckbox, NDropdown, useMessage } from 'naive-ui'

const props = withDefaults(defineProps<{
  container: any
  showCheckbox?: boolean
  checked?: boolean
  showLifecycle?: boolean
  operatingContainers?: Set<string>
  mode?: 'dashboard' | 'full'
}>(), {
  mode: 'full'
})

const emit = defineEmits<{
  (e: 'update:checked', val: boolean): void
  (e: 'update'): void
  (e: 'rollback'): void
  (e: 'defer'): void
  (e: 'undefer'): void
  (e: 'delete-backup'): void
  (e: 'show-logs'): void
  (e: 'update-to-version'): void
  (e: 'lifecycle', action: 'start' | 'stop' | 'restart'): void
}>()

const message = useMessage()

// 隐藏影子测宽数据
const realWidths = ref({
  char2: 56,
  char4: 80,
  more: 34
})

// 动作项接口
interface ActionItem {
  key: string
  label: string
  width: number
  priority: number
}

// 物理容器尺寸实时测量
const actionsContainerRef = ref<HTMLDivElement | null>(null)
const containerWidth = ref(350) // 预设默认容器宽度
let resizeObserver: ResizeObserver | null = null

// 整理所有本卡片可能具备的操作项，并按照指定显示优先级排队
const allAvailableActions = computed(() => {
  const list: ActionItem[] = []

  // 1. 升级 (优先级 1)
  if (props.container.status === 'update') {
    list.push({ key: 'update', label: '升级', width: realWidths.value.char2, priority: 1 })
  }

  // 2. 启动/停止 (优先级 2)
  if (props.mode === 'full' && props.showLifecycle) {
    if (props.container.running) {
      list.push({ key: 'stop', label: '停止', width: realWidths.value.char2, priority: 2 })
    } else {
      list.push({ key: 'start', label: '启动', width: realWidths.value.char2, priority: 2 })
    }
  }

  // 3. 重启 (优先级 3)
  if (props.mode === 'full' && props.showLifecycle && props.container.running) {
    list.push({ key: 'restart', label: '重启', width: realWidths.value.char2, priority: 3 })
  }

  // 4. 日志 (优先级 4)
  if (props.mode === 'full') {
    list.push({ key: 'show-logs', label: '日志', width: realWidths.value.char2, priority: 4 })
  }

  // 5. 暂挂 (优先级 5)
  if (props.container.status === 'update') {
    list.push({ key: 'defer', label: '暂挂', width: realWidths.value.char2, priority: 5 })
  } else if (props.container.status === 'deferred') {
    list.push({ key: 'undefer', label: '恢复检测', width: realWidths.value.char4, priority: 5 })
  }

  // 6. 回滚 (优先级 6)
  if (props.mode === 'full' && props.container.has_rollback) {
    list.push({ key: 'rollback', label: '回滚', width: realWidths.value.char2, priority: 6 })
  }

  // 7. 清除备份 (优先级 7)
  if (props.mode === 'full' && props.container.has_rollback) {
    list.push({ key: 'delete-backup', label: '清除备份', width: realWidths.value.char4, priority: 7 })
  }

  // 8. 指定版本升级 (优先级 8)
  if (props.mode === 'full') {
    list.push({ key: 'update-to-version', label: '修改版本', width: realWidths.value.char4, priority: 8 })
  }

  return list.sort((a, b) => a.priority - b.priority)
})

// 物理宽度剪裁决策
const layoutResult = computed(() => {
  const list = allAvailableActions.value
  const limit = containerWidth.value
  
  // 计算全放开需要的宽度之和
  let totalWidthNeeded = 0
  for (let i = 0; i < list.length; i++) {
    totalWidthNeeded += list[i].width + (i > 0 ? 8 : 0) // gap 为 8px
  }

  // 状况一：当前可用空间完全能在一行塞下所有按钮
  if (totalWidthNeeded <= limit) {
    return {
      flatActions: list,
      dropdownItems: []
    }
  }

  // 状况二：空间不足，需扣减出 更多下拉按钮宽度 和 8px (间距) 预留，依优先级裁剪
  const maxFlatWidth = limit - realWidths.value.more - 8
  const flatActions: ActionItem[] = []
  const dropdownItems: any[] = []
  
  let currentWidth = 0
  let isFirst = true
  
  for (let item of list) {
    const itemWidthWithGap = item.width + (isFirst ? 0 : 8)
    if (currentWidth + itemWidthWithGap <= maxFlatWidth) {
      flatActions.push(item)
      currentWidth += itemWidthWithGap
      isFirst = false
    } else {
      dropdownItems.push({
        label: item.label,
        key: item.key,
        disabled: item.key === 'restart' && (props.operatingContainers?.has(props.container.name + ':restart') || props.operatingContainers?.has(props.container.name + ':stop'))
      })
    }
  }

  return {
    flatActions,
    dropdownItems
  }
})

// 下拉菜单分发器
const handleMoreSelect = (key: string) => {
  if (key === 'restart') {
    emit('lifecycle', 'restart')
  } else if (key === 'defer') {
    emit('defer')
  } else if (key === 'undefer') {
    emit('undefer')
  } else if (key === 'rollback') {
    emit('rollback')
  } else if (key === 'delete-backup') {
    emit('delete-backup')
  } else if (key === 'update-to-version') {
    emit('update-to-version')
  }
}

// 镜像摘要哈希缩略展示
const shortDigest = (digest: string) => {
  if (!digest) return ''
  if (digest.startsWith('sha256:')) {
    return digest.slice(7, 19)
  }
  return digest.slice(0, 12)
}

// 复制镜像名称到剪贴板
const copyText = (text: string) => {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(() => {
      message?.success('已复制镜像名称到剪贴板')
    }).catch(() => {
      message?.error('复制失败')
    })
  } else {
    const input = document.createElement('input')
    input.setAttribute('value', text)
    document.body.appendChild(input)
    input.select()
    try {
      document.execCommand('copy')
      message?.success('已复制镜像名称到剪贴板')
    } catch {
      message?.error('复制失败')
    }
    document.body.removeChild(input)
  }
}

onMounted(() => {
  nextTick(() => {
    const m2 = document.getElementById(props.container.name + '-m2')
    const m4 = document.getElementById(props.container.name + '-m4')
    const mmore = document.getElementById(props.container.name + '-mmore')
    
    if (m2 && m2.offsetWidth > 0) realWidths.value.char2 = m2.offsetWidth
    if (m4 && m4.offsetWidth > 0) realWidths.value.char4 = m4.offsetWidth
    if (mmore && mmore.offsetWidth > 0) realWidths.value.more = mmore.offsetWidth
  })

  if (actionsContainerRef.value) {
    resizeObserver = new ResizeObserver((entries) => {
      for (let entry of entries) {
        containerWidth.value = entry.contentRect.width
      }
    })
    resizeObserver.observe(actionsContainerRef.value)
  }
})

onUnmounted(() => {
  if (resizeObserver) {
    resizeObserver.disconnect()
  }
})
</script>
