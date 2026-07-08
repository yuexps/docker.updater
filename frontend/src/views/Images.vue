<template>
  <div class="view-fade-in">
      <!-- Page Header -->
      <div class="flex items-center justify-between mb-8 select-none">
        <div>
          <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">镜像管理</h1>
        </div>
        <div class="flex items-center space-x-3">
          <n-button 
            type="primary" 
            round 
            size="small"
            class="active-scale"
            :loading="pruning"
            @click="pruneImages"
          >
            清理虚悬镜像 (Prune)
          </n-button>
        </div>
      </div>

      <!-- Overview Stats -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-10 select-none">
        <div class="apple-card p-6 rounded-lg flex items-center justify-between">
          <div>
            <span class="text-[12px] font-normal text-body-muted uppercase tracking-wider">镜像总数</span>
            <span class="text-[34px] font-semibold tracking-tight block mt-2 text-primary">{{ images.length }}</span>
          </div>
        </div>
        <div class="apple-card p-6 rounded-lg flex items-center justify-between">
          <div>
            <span class="text-[12px] font-normal text-body-muted uppercase tracking-wider">Dangling 虚悬镜像</span>
            <span 
              class="text-[34px] font-semibold tracking-tight block mt-2"
              :class="danglingCount > 0 ? 'text-amber-600' : 'text-slate-600'"
            >
              {{ danglingCount }}
            </span>
          </div>
          <div v-if="danglingCount > 0" class="w-10 h-10 rounded-full bg-amber-50/10 flex items-center justify-center">
            <div class="w-2.5 h-2.5 rounded-full status-dot-amber"></div>
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

        <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          <div 
            v-for="img in images" 
            :key="img.id"
            class="apple-card rounded-lg p-6 flex flex-col justify-between min-h-[180px] hover:border-primary transition-all duration-200"
            :class="{'border-amber-400 bg-amber-50/5': isDangling(img)}"
          >
            <!-- Top Details -->
            <div>
              <div class="flex items-start justify-between mb-4">
                <div class="space-y-1 max-w-[70%]">
                  <div 
                    v-for="tag in img.tags" 
                    :key="tag"
                    class="text-[14px] font-semibold text-slate-800 break-all leading-tight"
                  >
                    {{ tag }}
                  </div>
                </div>
                
                <n-tag v-if="isDangling(img)" type="warning" round size="small">
                  虚悬
                </n-tag>
                <n-tag v-else-if="isInUse(img)" type="success" round size="small">
                  在用
                </n-tag>
                <n-tag v-else type="default" round size="small">
                  未占用
                </n-tag>
              </div>

              <!-- Metadata -->
              <div class="space-y-1 text-[12px] font-normal text-body-muted font-mono">
                <div class="truncate"><span class="font-sans text-slate-500 font-semibold">Image ID:</span> {{ formatID(img.id) }}</div>
                <div><span class="font-sans text-slate-500 font-semibold">大小:</span> {{ formatSize(img.size) }}</div>
                <div><span class="font-sans text-slate-500 font-semibold">创建时间:</span> {{ formatDate(img.created) }}</div>
                <div class="truncate">
                  <span class="font-sans text-slate-500 font-semibold">容器占用:</span>
                  <span v-if="isInUse(img)" class="text-primary font-semibold font-sans ml-1">{{ img.containers!.join(', ') }}</span>
                  <span v-else class="text-slate-400 font-sans ml-1">暂无容器引用</span>
                </div>
              </div>
            </div>

            <!-- Actions -->
            <div class="mt-6 pt-4 border-t border-hairline flex justify-end">
              <n-button 
                type="error"
                size="small"
                round
                ghost
                class="active-scale"
                @click="deleteImage(img.id)"
              >
                物理删除
              </n-button>
            </div>
          </div>
        </div>
      </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { NButton, NSpin, NTag, useMessage, useDialog } from 'naive-ui'
import axios from 'axios'

const apiBase = '/app/docker-updater/api'
const message = useMessage()
const dialog = useDialog()

interface ImageItem {
  id: string;
  tags: string[];
  size: number;
  created: number;
  containers?: string[];
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

// 是否被至少一个容器实际引用
const isInUse = (img: ImageItem): boolean => {
  return !!(img.containers && img.containers.length > 0)
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
    title: 'Prune 虚悬镜像清理',
    content: '清理动作会安全删除全部未关联任何运行或停止容器的虚悬 (Dangling) 镜像，用以释放系统存储空间。确认清理吗？',
    positiveText: '确认清理',
    negativeText: '取消',
    onPositiveClick: async () => {
      pruning.value = true
      try {
        const res = await axios.post(`${apiBase}/images/prune`)
        const space = formatSize(res.data.space_reclaimed)
        message.success(`清理完成，成功释放 ${space} 空间，共清理 ${res.data.deleted_count} 个虚悬镜像。`)
        fetchImages()
      } catch (err: any) {
        message.error('清理失败: ' + (err.response?.data?.error || err.message))
      } finally {
        pruning.value = false
      }
    }
  })
}

// 物理删除指定镜像
const deleteImage = async (id: string) => {
  dialog.warning({
    title: '确认物理删除镜像',
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

const formatID = (rawId: string): string => {
  return rawId.replace('sha256:', '').substring(0, 12)
}

const formatSize = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const formatDate = (timestamp: number): string => {
  const date = new Date(timestamp * 1000)
  return date.toLocaleDateString()
}

onMounted(() => {
  fetchImages()
})
</script>
