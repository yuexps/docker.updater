<template>
  <n-modal 
    :show="show" 
    :mask-closable="false" 
    preset="card" 
    class="w-[94%] max-w-3xl bg-slate-900 text-white rounded-lg border border-slate-800 overflow-hidden"
    :segmented="{ content: 'soft', action: 'soft' }"
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
        <span class="text-slate-200 text-sm font-semibold tracking-wide ml-3 select-none">{{ title }}</span>
      </div>
    </template>
    
    <div class="bg-black p-4 rounded-xl font-mono text-[12px] h-75 sm:h-100 overflow-y-auto border border-slate-850 select-text" ref="terminalLog">
      <div 
        v-for="(line, idx) in logLines" 
        :key="idx" 
        class="text-slate-300 py-0.5 px-1.5 rounded-sm break-all leading-relaxed select-text" 
        v-html="ansiToHtml(line)"
      ></div>
      <div v-if="logRunning" class="text-slate-400 animate-pulse mt-2 px-2 text-[11px] flex items-center gap-2 select-none">
        <span class="w-2 h-2 rounded-full bg-emerald-400"></span>
        [JOB RUNNING] 正在监听实时日志输出流...
      </div>
    </div>
    
    <template #action>
      <div class="flex justify-end space-x-2">
        <n-button v-if="logRunning" secondary round size="medium" class="active-scale text-slate-300" @click="emit('close')">
          后台运行
        </n-button>
        <n-button v-else type="primary" round size="medium" class="active-scale" @click="emit('close')">
          关闭窗口
        </n-button>
      </div>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { NModal, NButton } from 'naive-ui'
import { ansiToHtml } from '../utils/ansi'

const props = withDefaults(
  defineProps<{
    show: boolean
    logLines: string[]
    logRunning: boolean
    title?: string
  }>(),
  {
    title: '升级部署进度'
  }
)

const emit = defineEmits<{
  (e: 'close'): void
}>()

const terminalLog = ref<HTMLDivElement | null>(null)

watch([() => props.logLines.length, () => props.show], () => {
  nextTick(() => {
    if (terminalLog.value) {
      terminalLog.value.scrollTop = terminalLog.value.scrollHeight
    }
  })
})
</script>
