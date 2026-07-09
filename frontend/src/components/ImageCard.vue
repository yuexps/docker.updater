<template>
  <div 
    class="apple-card rounded-lg p-4.5 flex flex-col justify-between min-h-[175px] hover:border-primary hover:shadow-[0_12px_28px_rgba(0,0,0,0.04)] transition-all duration-300 bg-white"
    :class="{'border-amber-400/60 bg-amber-50/10': isDangling}"
  >
    <!-- 顶部主要信息 -->
    <div>
      <div class="flex items-start justify-between">
        <div class="flex items-center min-w-0 flex-1">
          <!-- 镜像图标 -->
          <div class="w-10 h-10 rounded-xl bg-slate-50 border border-slate-100 flex items-center justify-center text-slate-500 shadow-xs shrink-0">
            <svg class="w-5 h-5 text-slate-500" viewBox="0 0 24 24">
              <g fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                <path d="M12 3l8 4.5v9L12 21l-8-4.5v-9L12 3"></path>
                <path d="M12 12l8-4.5"></path>
                <path d="M12 12v9"></path>
                <path d="M12 12L4 7.5"></path>
              </g>
            </svg>
          </div>

          <!-- 解析后的 Repository 与 Tag 信息 -->
          <div class="ml-3 min-w-0 flex-1">
            <div class="flex items-center space-x-1.5 min-w-0">
              <div 
                class="text-[16px] font-bold text-slate-800 tracking-tight truncate select-all"
                :class="{'text-slate-400': isDangling}"
                :title="parsedInfo.repo"
              >
                {{ parsedInfo.repo }}
              </div>
              <!-- 如果有多个 Tag，显示 +N 药丸 -->
              <n-tooltip v-if="image.tags.length > 1" trigger="hover" placement="top">
                <template #trigger>
                  <span class="cursor-help shrink-0 bg-slate-100 text-[10px] text-slate-500 font-bold px-1 py-0.5 rounded border border-slate-200/50">
                    +{{ image.tags.length - 1 }}
                  </span>
                </template>
                <div class="space-y-1 font-mono text-[11px] p-1 max-w-[280px]">
                  <div class="font-semibold text-slate-300 border-b border-slate-700 pb-1 mb-1">所有关联标签:</div>
                  <div v-for="tag in image.tags" :key="tag" class="text-white break-all">{{ tag }}</div>
                </div>
              </n-tooltip>
            </div>
            
            <!-- 主 Tag 显示 -->
            <div class="mt-1 flex items-center space-x-1.5">
              <span 
                class="inline-flex items-center px-1.5 py-0.5 rounded-md bg-blue-50/80 text-[10px] font-bold text-primary border border-blue-100/50 truncate max-w-[150px]"
                :class="{'bg-amber-50 text-amber-600 border-amber-200/50': isDangling}"
                :title="parsedInfo.tag"
              >
                {{ parsedInfo.tag }}
              </span>
              <button 
                v-if="!isDangling"
                class="text-slate-400 hover:text-primary transition-colors p-0.5 rounded hover:bg-slate-50 cursor-pointer shrink-0"
                title="复制完整镜像名称"
                @click="copyText(image.tags[0])"
              >
                <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                  <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                </svg>
              </button>
            </div>
          </div>
        </div>

        <!-- 状态指示徽章 -->
        <div class="shrink-0 ml-2">
          <div 
            class="flex items-center space-x-1 px-2.5 py-0.5 rounded-full text-[11px] font-medium border"
            :class="[
              isDangling
                ? 'bg-amber-50 text-amber-700 border-amber-100'
                : isInUse
                  ? 'bg-emerald-50 text-emerald-700 border-emerald-100'
                  : 'bg-slate-50 text-slate-500 border-slate-200'
            ]"
          >
            <span v-if="isInUse" class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
            <span v-else-if="isDangling" class="h-1.5 w-1.5 rounded-full bg-amber-500"></span>
            <span v-else class="h-1.5 w-1.5 rounded-full bg-slate-400"></span>
            <span>{{ isDangling ? '无标签' : isInUse ? '在用' : '未占用' }}</span>
          </div>
        </div>
      </div>

      <!-- 详细元数据 -->
      <div class="mt-4 space-y-2.5 text-[12px] font-normal text-body-muted">
        <!-- 第一行属性：大小 与 创建时间 -->
        <div class="grid grid-cols-2 gap-4">
          <div class="flex flex-col">
            <span class="text-[10px] text-slate-400 font-semibold uppercase tracking-wider">大小</span>
            <span class="text-slate-700 font-medium mt-0.5 font-mono text-[13px]">{{ formatSize(image.size) }}</span>
          </div>
          <div class="flex flex-col">
            <span class="text-[10px] text-slate-400 font-semibold uppercase tracking-wider">创建时间</span>
            <span class="text-slate-700 font-medium mt-0.5 font-mono text-[13px]">{{ formatDate(image.created) }}</span>
          </div>
        </div>

        <!-- 第二行属性：容器占用 与 架构 -->
        <div class="grid grid-cols-2 gap-4">
          <!-- 容器占用（左侧） -->
          <div class="flex flex-col">
            <span class="text-[10px] text-slate-400 font-semibold uppercase tracking-wider">容器占用</span>
            <div class="mt-1 flex flex-wrap gap-1.5 min-h-[22px]">
              <template v-if="isInUse">
                <!-- 最多渲染前2个引用容器 -->
                <span 
                  v-for="c in visibleContainers" 
                  :key="c"
                  class="inline-flex items-center px-2 py-0.5 rounded-md bg-slate-50 text-[11px] text-slate-600 border border-slate-200/40 font-mono select-all truncate max-w-[85px] xs:max-w-[100px]"
                  :title="c"
                >
                  {{ c }}
                </span>
                <!-- 超出折叠标签 -->
                <n-tooltip v-if="image.containers && image.containers.length > 2" trigger="hover" placement="top">
                  <template #trigger>
                    <span class="cursor-help inline-flex items-center px-1.5 py-0.5 rounded-md bg-slate-100 text-[11px] text-slate-500 font-bold border border-slate-200/50">
                      +{{ image.containers.length - 2 }}
                    </span>
                  </template>
                  <div class="space-y-1 font-mono text-[11px] p-1 max-w-[240px]">
                    <div class="font-semibold text-slate-300 border-b border-slate-700 pb-1 mb-1">正在使用该镜像的容器:</div>
                    <div v-for="c in image.containers" :key="c" class="text-white break-all">{{ c }}</div>
                  </div>
                </n-tooltip>
              </template>
              <span v-else class="text-slate-400/80 text-[11px] mt-0.5 italic">暂无容器引用</span>
            </div>
          </div>

          <!-- 架构（右侧） -->
          <div class="flex flex-col min-w-0">
            <span class="text-[10px] text-slate-400 font-semibold uppercase tracking-wider">架构</span>
            <span 
              class="text-slate-700 font-medium mt-1 font-mono text-[12px] sm:text-[13px] truncate" 
              :title="image.architecture"
            >
              {{ image.architecture || '未知' }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- 底部操作区域 -->
    <div class="mt-4 pt-3.5 border-t border-slate-100 flex justify-between items-center">
      <!-- 镜像 ID -->
      <div class="flex items-center space-x-1.5 min-w-0">
        <span class="text-[11px] text-slate-400 font-mono tracking-tight truncate">ID: {{ shortID }}</span>
        <button 
          class="text-slate-400 hover:text-primary transition-colors p-1 rounded hover:bg-slate-50 cursor-pointer shrink-0"
          title="复制完整镜像 ID"
          @click="copyText(image.id)"
        >
          <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
          </svg>
        </button>
      </div>

      <!-- 操作按钮 -->
      <div class="flex items-center space-x-2 shrink-0">
        <n-tooltip v-if="isInUse" trigger="hover" placement="top">
          <template #trigger>
            <span>
              <n-button 
                type="error"
                size="small"
                round
                ghost
                disabled
                class="text-[12px] font-medium opacity-50 cursor-not-allowed"
              >
                删除
              </n-button>
            </span>
          </template>
          <div class="text-[11px] p-1 max-w-[240px] leading-relaxed">
            该镜像正被以下容器使用，无法删除：
            <div class="font-semibold text-amber-400 mt-1 font-mono break-all">{{ image.containers?.join(', ') }}</div>
          </div>
        </n-tooltip>
        <n-button 
          v-else
          type="error"
          size="small"
          round
          ghost
          class="active-scale text-[12px] font-medium"
          @click="emit('delete', image.id)"
        >
          删除
        </n-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NTooltip, useMessage } from 'naive-ui'

interface ImageItem {
  id: string;
  tags: string[];
  size: number;
  created: number;
  containers?: string[];
  architecture?: string;
}

const props = defineProps<{
  image: ImageItem
}>()

const emit = defineEmits<{
  (e: 'delete', id: string): void
}>()

const message = useMessage()

// 状态判定
const isDangling = computed(() => {
  return props.image.tags.includes('<none>:<none>')
})

const isInUse = computed(() => {
  return !!(props.image.containers && props.image.containers.length > 0)
})

// 解析第一个 Tag 来获取 Repository 和 Tag
const parsedInfo = computed(() => {
  const firstTag = props.image.tags[0]
  if (!firstTag || firstTag === '<none>:<none>') {
    return { repo: '<none>', tag: '<none>' }
  }
  
  const lastColon = firstTag.lastIndexOf(':')
  const lastSlash = firstTag.lastIndexOf('/')
  
  if (lastColon > lastSlash) {
    return {
      repo: firstTag.substring(0, lastColon),
      tag: firstTag.substring(lastColon + 1)
    }
  }
  
  return { repo: firstTag, tag: 'latest' }
})

// 镜像 ID 缩略显示
const shortID = computed(() => {
  return props.image.id.replace('sha256:', '').substring(0, 12)
})

// 只显示前 2 个关联容器，多余的折叠
const visibleContainers = computed(() => {
  if (!props.image.containers) return []
  return props.image.containers.slice(0, 2)
})

// 格式化大小
const formatSize = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 格式化创建时间
const formatDate = (timestamp: number): string => {
  const date = new Date(timestamp * 1000)
  return date.toLocaleDateString()
}

// 一键复制
const copyText = (text: string) => {
  const cleanText = text.replace('sha256:', '')
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(cleanText).then(() => {
      message?.success('已复制到剪贴板')
    }).catch(() => {
      message?.error('复制失败')
    })
  } else {
    const input = document.createElement('input')
    input.setAttribute('value', cleanText)
    document.body.appendChild(input)
    input.select()
    try {
      document.execCommand('copy')
      message?.success('已复制到剪贴板')
    } catch {
      message?.error('复制失败')
    }
    document.body.removeChild(input)
  }
}
</script>
