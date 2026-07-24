<template>
  <n-modal 
    :show="show" 
    preset="card" 
    class="w-[94%] max-w-3xl bg-slate-900 text-white rounded-lg border border-slate-800 overflow-hidden"
    @close="emit('close')"
  >
    <template #header>
      <div class="flex items-center space-x-2">
        <!-- Mac 控制小圆点 -->
        <div class="flex space-x-1.5 shrink-0 select-none">
          <span class="w-3 h-3 rounded-full bg-[#ff5f56]"></span>
          <span class="w-3 h-3 rounded-full bg-[#ffbd2e]"></span>
          <span class="w-3 h-3 rounded-full bg-[#27c93f]"></span>
        </div>
        <span class="text-slate-200 text-sm font-semibold tracking-wide ml-3 select-none">容器运行日志 (stdout/stderr)</span>
      </div>
    </template>
    
    <div class="bg-black text-slate-300 p-4 rounded-xl font-mono text-[12px] h-75 sm:h-100 overflow-y-auto border border-slate-850">
      <pre class="whitespace-pre-wrap font-sans text-[12px] leading-relaxed select-all">{{ logs }}</pre>
    </div>
    
    <template #action>
      <div class="flex justify-end">
        <n-button round size="medium" type="primary" class="active-scale" @click="emit('close')">
          关闭窗口
        </n-button>
      </div>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { NModal, NButton } from 'naive-ui'

defineProps<{
  show: boolean
  logs: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()
</script>
