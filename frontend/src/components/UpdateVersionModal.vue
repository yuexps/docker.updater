<template>
  <n-modal v-model:show="showModel" preset="dialog" title="修改容器版本" positive-text="确认修改" negative-text="取消"
    class="rounded-lg max-w-[420px]" @positive-click="emit('submit')">
    <div class="py-4 select-none space-y-4">
      <div>
        <label class="text-[12px] font-semibold uppercase tracking-wider text-body-muted block mb-1.5">当前镜像</label>
        <div
          class="text-[13px] font-mono text-slate-700 bg-slate-50 border border-hairline rounded-lg p-2.5 break-all select-all">
          {{ currentImage }}
        </div>
      </div>

      <div>
        <label class="text-[12px] font-semibold uppercase tracking-wider text-body-muted block mb-1.5">可选版本列表</label>
        <n-select v-model:value="selectValue" :options="selectOptions" :loading="loadingTags" placeholder="请输入或下拉获取版本号" class="rounded-lg" />
      </div>

      <div v-if="selectValue === 'custom'">
        <label class="text-[12px] font-semibold uppercase tracking-wider text-body-muted block mb-1.5">目标版本 /
          镜像名</label>
        <n-input v-model:value="versionModel" placeholder="例如: 8.0 或 mysql:8.0" class="rounded-lg"
          @keyup.enter="emit('submit')" />
        <div class="text-[11px] text-body-muted mt-2 leading-relaxed">
          若仅输入 Tag（例如 5.7），将自动拼装原镜像库名（例如: {{ currentImage.split(':')[0] }}:5.7）；若包含冒号，将直接使用完整镜像名.
        </div>
      </div>
    </div>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NModal, NInput, NSelect } from 'naive-ui'
import axios from 'axios'

const props = defineProps<{
  show: boolean
  currentImage: string
  version: string
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'update:version', val: string): void
  (e: 'submit'): void
}>()

const showModel = computed({
  get: () => props.show,
  set: (val) => emit('update:show', val)
})

const versionModel = computed({
  get: () => props.version,
  set: (val) => emit('update:version', val)
})

const selectValue = ref<string>('custom')
const loadingTags = ref<boolean>(false)
const remoteTags = ref<string[]>([])

// 封装下拉菜单选项，默认包含自定义输入项
const selectOptions = computed(() => {
  const options = [{ label: '自定义输入', value: 'custom' }]
  remoteTags.value.forEach(tag => {
    options.push({ label: tag, value: tag })
  })
  return options
})

// 拉取远程仓库的 tags 列表 (最多展示 20 个)
const fetchTags = async () => {
  loadingTags.value = true
  remoteTags.value = []
  try {
    const res = await axios.get(`/app/docker-updater/api/image/tags?image=${encodeURIComponent(props.currentImage)}`)
    if (Array.isArray(res.data)) {
      remoteTags.value = res.data
    }
  } catch (err) {
    console.warn('[UpdateModal] 获取镜像 tags 列表失败:', err)
  } finally {
    loadingTags.value = false
  }
}

// 侦听弹窗显隐状态，开启时初始化拉取 tags 列表
watch(() => props.show, (newVal) => {
  if (newVal) {
    selectValue.value = 'custom'
    fetchTags()
  }
})

// 侦听下拉框选项，选中具体 tag 时将其自动同步覆写至输入框中
watch(selectValue, (newVal) => {
  if (newVal !== 'custom') {
    versionModel.value = newVal
  }
})

// 侦听输入框内容手动更改，若用户修改的值不在远端拉取的 tags 列表中，自动切回 custom
watch(versionModel, (newVal) => {
  if (newVal !== selectValue.value && newVal !== '') {
    if (!remoteTags.value.includes(newVal)) {
      selectValue.value = 'custom'
    } else {
      selectValue.value = newVal
    }
  }
})
</script>
