<template>
  <n-modal
    :show="show"
    :mask-closable="false"
    @update:show="$emit('close')"
  >
    <div class="w-11/12 max-w-5xl rounded-xl overflow-hidden shadow-2xl bg-[#1e1e1e] border border-slate-700/80 flex flex-col h-[75vh] max-h-205 min-h-130 text-[#cccccc] font-mono select-none">
      
      <div class="h-10 px-3 bg-[#252526] border-b border-[#333333] flex items-center justify-between shrink-0 select-none">
        <div class="flex items-center space-x-3 min-w-0">
          <div class="flex items-center space-x-1.5 shrink-0 px-1">
            <span class="w-2.5 h-2.5 rounded-full bg-[#ff5f56] shadow-xs cursor-pointer hover:opacity-80" title="关闭" @click="$emit('close')"></span>
            <span class="w-2.5 h-2.5 rounded-full bg-[#ffbd2e] shadow-xs"></span>
            <span class="w-2.5 h-2.5 rounded-full bg-[#27c93f] shadow-xs"></span>
          </div>
          
          <div class="flex items-center space-x-2 text-[12px] min-w-0">
            <div class="inline-flex items-center px-3 py-1 bg-[#1e1e1e] text-white rounded-t border-t-2 border-primary space-x-1.5 text-[11px] font-semibold">
              <svg class="w-3.5 h-3.5 text-amber-400 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
                <polyline points="14 2 14 8 20 8"></polyline>
              </svg>
              <span>{{ projectName }}</span>
            </div>

            <span class="text-[#858585] truncate text-[11px] opacity-80 hidden md:inline ml-2" :title="filePath">
              {{ filePath || 'docker-compose.yml' }}
            </span>
          </div>
        </div>

        <button 
          class="text-[#858585] hover:text-white transition-colors cursor-pointer p-1.5 rounded-md hover:bg-[#37373d]"
          title="关闭"
          @click="$emit('close')"
        >
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>

      </div>

      <div class="flex-1 relative overflow-hidden bg-[#1e1e1e]">
        <div v-if="loading" class="flex justify-center items-center w-full h-full bg-[#1e1e1e]">
          <n-spin size="large" />
        </div>
        
        <codemirror
          v-else
          v-model="content"
          placeholder="version: '3.8'..."
          :style="{ height: '100%', width: '100%', fontSize: '13px' }"
          :autofocus="true"
          :indent-with-tab="true"
          :tab-size="2"
          :extensions="extensions"
        />
      </div>

      <div class="h-9 px-4 bg-[#252526] border-t border-[#333333] flex items-center justify-between shrink-0 select-none text-[11px] font-mono">
        <div class="flex items-center space-x-3 text-[#858585]">
          <span class="text-slate-300 font-semibold bg-[#333333] px-1.5 py-0.5 rounded text-[10px]">YAML</span>
          <div class="flex items-center space-x-1.5">
            <span>{{ lineCount }} 行</span>
            <span>·</span>
            <span>{{ content.length }} 字符</span>
          </div>
        </div>

        <n-button 
          type="primary" 
          round 
          size="tiny" 
          :loading="saving"
          class="font-semibold active-scale px-4 shadow-xs"
          @click="handleSave"
        >
          保存
        </n-button>
      </div>

    </div>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { NModal, NButton, NSpin } from 'naive-ui'
import { Codemirror } from 'vue-codemirror'
import { yaml } from '@codemirror/lang-yaml'
import { oneDark } from '@codemirror/theme-one-dark'
import { getComposeContentApi, saveComposeContentApi } from '../api/compose'
import { message } from '../utils/discrete'

const props = defineProps<{
  show: boolean
  projectName: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const loading = ref<boolean>(false)
const saving = ref<boolean>(false)
const content = ref<string>('')
const filePath = ref<string>('')

const extensions = [yaml(), oneDark]

const lineCount = computed(() => {
  if (!content.value) return 1
  return content.value.split('\n').length
})

const fetchConfig = async () => {
  if (!props.projectName) return
  loading.value = true
  try {
    const data = await getComposeContentApi(props.projectName)
    if (data) {
      content.value = data.content || ''
      filePath.value = data.file_path || ''
    }
  } catch (err: any) {
    const errorMsg = err?.response?.data?.error || err?.response?.data?.message || err?.message || '未知错误'
    message.error(`加载 Compose 项目 [${props.projectName}] 配置文件失败: ${errorMsg}`)
  } finally {
    loading.value = false
  }
}

watch(() => props.show, (newVal) => {
  if (newVal && props.projectName) {
    fetchConfig()
  }
})

const handleSave = async () => {
  if (!props.projectName) return
  saving.value = true
  try {
    await saveComposeContentApi(props.projectName, content.value)
    message.success(`项目 ${props.projectName} 配置文件保存成功！请点击【重建】以应用最新配置。`)
    emit('saved')
    emit('close')
  } catch (err: any) {
    const errorMsg = err?.response?.data?.error || err?.response?.data?.message || err?.message || '未知错误'
    message.error(`保存 Compose 项目 [${props.projectName}] 配置文件失败: ${errorMsg}`)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
:deep(.cm-editor) {
  height: 100% !important;
  background-color: #1e1e1e !important;
}
:deep(.cm-gutters) {
  background-color: #1e1e1e !important;
  border-right: 1px solid #2d2d2d !important;
  color: #5a5a5a !important;
}
:deep(.cm-scroller) {
  overflow: auto !important;
}
</style>
