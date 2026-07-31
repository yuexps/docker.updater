<template>
  <div class="view-fade-in flex flex-col h-full overflow-hidden">
    <!-- Page Header -->
    <div class="shrink-0 px-3 md:px-5 lg:px-6 pt-3 md:pt-4 lg:pt-5 pb-3 md:pb-4 select-none bg-canvas-parchment">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">镜像列表</h1>
        </div>
        <div class="flex items-center shrink-0">
          <n-button type="primary" round size="small" class="active-scale font-semibold" :loading="pruning"
            @click="pruneImages">
            清理残留镜像
          </n-button>
        </div>
      </div>
    </div>

    <!-- 页面内容 -->
    <div class="flex-1 min-w-0 overflow-y-auto px-3 md:px-5 lg:px-6 pb-24">
      <!-- Overview Stats -->
      <div class="grid grid-cols-2 gap-4 md:gap-6 mb-10 select-none">
        <!-- 镜像总数 -->
        <div
          class="apple-card p-5 sm:p-6 rounded-lg transition-all duration-300 cursor-pointer select-none"
          :class="[
            currentFilter === 'all'
              ? 'border-2 border-primary! bg-white'
              : 'border-2 border-slate-100 bg-white hover:border-primary/35'
          ]"
          @click="currentFilter = 'all'"
        >
          <span class="text-[12px] font-semibold text-body-muted uppercase tracking-wider block">镜像总数</span>
          <span class="text-[36px] font-bold tracking-tight block mt-2 text-primary leading-none">{{ images.length
            }}</span>
        </div>

        <!-- 无标签残留 -->
        <div 
          class="apple-card p-5 sm:p-6 rounded-lg transition-all duration-300 cursor-pointer select-none"
          :class="[
            currentFilter === 'dangling'
              ? 'border-2 border-amber-500! bg-white'
              : 'border-2 border-slate-100 bg-white hover:border-amber-500/35'
          ]"
          @click="currentFilter = currentFilter === 'dangling' ? 'all' : 'dangling'"
        >
          <span class="text-[12px] font-semibold text-body-muted uppercase tracking-wider block">无标签残留</span>
          <div class="flex items-baseline justify-between mt-2">
            <span class="text-[36px] font-bold tracking-tight leading-none"
              :class="danglingCount > 0 || currentFilter === 'dangling' ? 'text-amber-600' : 'text-slate-600'">
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

        <div v-else-if="filteredImages.length === 0" class="apple-card p-12 text-center text-body-muted rounded-lg select-none">
          {{ images.length === 0 ? '未在宿主机上发现 Docker 镜像' : '未发现满足筛选条件的镜像' }}
        </div>

        <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(310px,1fr))] gap-5">
          <image-card v-for="img in filteredImages" :key="img.id" :image="img" @delete="deleteImage" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { NButton, NSpin, useDialog } from 'naive-ui'
import { getImagesApi, pruneImagesApi, deleteImageApi, type ImageItem } from '../api/image'
import { message } from '../utils/discrete'
import ImageCard from '../components/ImageCard.vue'

const dialog = useDialog()

const images = ref<ImageItem[]>([])
const currentFilter = ref<'all' | 'dangling'>('all')
const loading = ref<boolean>(false)
const pruning = ref<boolean>(false)

const danglingCount = computed(() => {
  return images.value.filter(img => isDangling(img)).length
})

const isDangling = (img: ImageItem): boolean => {
  return img.tags.includes('<none>:<none>')
}

const filteredImages = computed(() => {
  return images.value.filter(img => {
    if (currentFilter.value === 'dangling') {
      return isDangling(img)
    }
    return true
  })
})

const fetchImages = async () => {
  loading.value = true
  try {
    const data = await getImagesApi()
    images.value = data || []
  } catch (err) {
    message.error('载入镜像清单失败')
  } finally {
    loading.value = false
  }
}

const pruneImages = () => {
  dialog.warning({
    title: 'Prune 残留镜像清理',
    content: '清理动作会安全删除全部未关联任何运行或停止容器的无标签残留镜像，用以释放系统存储空间。确认清理吗？',
    positiveText: '确认清理',
    negativeText: '取消',
    onPositiveClick: () => {
      executePrune()
    }
  })
}

const parseErrorMessage = (err: any): string => {
  if (err?.response?.data) {
    const d = err.response.data
    if (typeof d === 'string') return d
    if (typeof d === 'object' && (d.error || d.message)) return d.error || d.message
  }
  if (err?.message) return err.message
  return '未知错误'
}

const executePrune = async () => {
  pruning.value = true
  try {
    const res = await pruneImagesApi()
    const space = formatSize(res.space_reclaimed)
    message.success(`清理完成，成功释放 ${space} 空间，共清理 ${res.deleted_count} 个残留镜像。`)
    fetchImages()
  } catch (err: any) {
    message.error(`清理无用虚悬镜像失败: ${parseErrorMessage(err)}`)
  } finally {
    pruning.value = false
  }
}

const deleteImage = (id: string, tag?: string) => {
  const displayTag = tag && tag !== '<none>:<none>' ? tag : id.slice(0, 12)
  dialog.warning({
    title: '确认删除镜像',
    content: `确认删除镜像 [${displayTag}] 吗？如果该镜像正被活动容器依赖，删除可能会报错。`,
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: () => {
      executeDeleteImage(id, displayTag)
    }
  })
}

const executeDeleteImage = async (id: string, displayTag?: string) => {
  const targetName = displayTag || id.slice(0, 12)
  try {
    await deleteImageApi(id)
    message.success(`已成功删除镜像 [${targetName}]`)
    fetchImages()
  } catch (err: any) {
    message.error(`删除镜像 [${targetName}] 失败: ${parseErrorMessage(err)}`)
  }
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
