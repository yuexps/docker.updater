<template>
  <div class="view-fade-in flex flex-col h-full overflow-hidden">
    <!-- Page Header -->
    <div class="shrink-0 px-4 md:px-8 lg:px-10 pt-4 md:pt-8 lg:pt-10 pb-4 md:pb-6 select-none bg-canvas-parchment">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">镜像管理</h1>
        </div>
        <div class="flex items-center shrink-0">
          <n-button 
            type="primary" 
            round 
            size="small"
            class="active-scale font-semibold"
            :loading="pruning"
            @click="pruneImages"
          >
            清理残留镜像
          </n-button>
        </div>
      </div>
    </div>

    <!-- 页面内容 -->
    <div class="flex-1 min-w-0 overflow-y-auto px-4 md:px-8 lg:px-10 pb-24">
      <!-- Overview Stats -->
      <div class="grid grid-cols-2 gap-4 md:gap-6 mb-10 select-none">
        <!-- 镜像总数 -->
        <div class="apple-card p-5 sm:p-6 rounded-lg hover:shadow-[0_12px_30px_rgba(0,102,204,0.03)] hover:border-primary/30 transition-all duration-300 bg-white">
          <span class="text-[12px] font-semibold text-body-muted uppercase tracking-wider block">镜像总数</span>
          <span class="text-[36px] font-bold tracking-tight block mt-2 text-primary leading-none">{{ images.length }}</span>
        </div>

        <!-- 无标签残留 -->
        <div 
          class="apple-card p-5 sm:p-6 rounded-lg transition-all duration-300 bg-white"
          :class="[
            danglingCount > 0 
              ? 'hover:shadow-[0_12px_30px_rgba(245,158,11,0.03)] hover:border-amber-500/30' 
              : 'hover:shadow-[0_12px_30px_rgba(0,0,0,0.02)]'
          ]"
        >
          <span class="text-[12px] font-semibold text-body-muted uppercase tracking-wider block">无标签残留</span>
          <div class="flex items-baseline justify-between mt-2">
            <span 
              class="text-[36px] font-bold tracking-tight leading-none"
              :class="danglingCount > 0 ? 'text-amber-600' : 'text-slate-600'"
            >
              {{ danglingCount }}
            </span>
          </div>
        </div>
      </div>

      <!-- Image Grid -->
      <div>
        <div v-if="loading" class="flex justify-center py-20">
          <n-spin size="large" />
        </div>

        <div v-else-if="images.length === 0" class="apple-card p-12 text-center text-body-muted rounded-lg select-none">
          未在宿主机上发现 Docker 镜像
        </div>

        <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-5">
          <image-card 
            v-for="img in images" 
            :key="img.id"
            :image="img"
            @delete="deleteImage"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { NButton, NSpin, useMessage, useDialog } from 'naive-ui'
import axios from 'axios'
import ImageCard from '../components/ImageCard.vue'

const apiBase = '/app/docker-updater/api'
const message = useMessage()
const dialog = useDialog()

interface ImageItem {
  id: string;
  tags: string[];
  size: number;
  created: number;
  containers?: string[];
  architecture?: string;
}

const images = ref<ImageItem[]>([])
const loading = ref<boolean>(false)
const pruning = ref<boolean>(false)

// 虚悬镜像数量 (RepoTags 为 <none>:<none>)
const danglingCount = computed(() => {
  return images.value.filter(img => isDangling(img)).length
})

const isDangling = (img: ImageItem): boolean => {
  return img.tags.includes('<none>:<none>')
}


const fetchImages = async () => {
  loading.value = true
  try {
    const res = await axios.get(`${apiBase}/images`)
    images.value = res.data || []
  } catch (err) {
    message.error('载入镜像清单失败')
  } finally {
    loading.value = false
  }
}

// Dangling Prune 清理
const pruneImages = async () => {
  dialog.warning({
    title: 'Prune 残留镜像清理',
    content: '清理动作会安全删除全部未关联任何运行或停止容器的无标签残留镜像，用以释放系统存储空间。确认清理吗？',
    positiveText: '确认清理',
    negativeText: '取消',
    onPositiveClick: async () => {
      pruning.value = true
      try {
        const res = await axios.post(`${apiBase}/images/prune`)
        const space = formatSize(res.data.space_reclaimed)
        message.success(`清理完成，成功释放 ${space} 空间，共清理 ${res.data.deleted_count} 个残留镜像。`)
        fetchImages()
      } catch (err: any) {
        message.error('清理失败: ' + (err.response?.data?.error || err.message))
      } finally {
        pruning.value = false
      }
    }
  })
}

// 彻底删除指定镜像
const deleteImage = async (id: string) => {
  dialog.warning({
    title: '确认删除镜像',
    content: '如果该镜像正被某些活动容器使用，删除可能会报错或导致关联服务失效。确认强制删除吗？',
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await axios.delete(`${apiBase}/image`, { params: { id } })
        message.success('镜像已成功删除')
        fetchImages()
      } catch (err: any) {
        message.error('删除镜像失败: ' + (err.response?.data?.error || err.message))
      }
    }
  })
}


const formatSize = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}


onMounted(() => {
  fetchImages()
})
</script>
