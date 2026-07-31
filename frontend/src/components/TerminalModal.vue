<template>
  <n-modal 
    :show="show" 
    :mask-closable="false"
    @update:show="emit('close')"
  >
    <div class="w-11/12 max-w-4xl rounded-xl overflow-hidden shadow-2xl bg-[#0d1117] border border-slate-700/80 flex flex-col h-[70vh] max-h-180 min-h-105 text-[#c9d1d9] font-mono select-none">
      
      <div class="h-10 px-3 bg-[#161b22] border-b border-[#30363d] flex items-center justify-between shrink-0 select-none">
        
        <div class="flex items-center space-x-3 min-w-0">
          <div class="flex items-center space-x-1.5 shrink-0 px-1">
            <span class="w-2.5 h-2.5 rounded-full bg-[#ff5f56] shadow-xs cursor-pointer hover:opacity-80" title="关闭" @click="emit('close')"></span>
            <span class="w-2.5 h-2.5 rounded-full bg-[#ffbd2e] shadow-xs"></span>
            <span class="w-2.5 h-2.5 rounded-full bg-[#27c93f] shadow-xs"></span>
          </div>
          
          <div class="flex items-center space-x-2 text-[12px] min-w-0">
            <div class="inline-flex items-center px-3 py-1 bg-[#0d1117] text-white rounded-t border-t-2 border-primary space-x-1.5 text-[11px] font-semibold">
              <svg class="w-3.5 h-3.5 text-emerald-400 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="4 17 10 11 4 5"></polyline>
                <line x1="12" y1="19" x2="20" y2="19"></line>
              </svg>
              <span>{{ title }}</span>
            </div>

            <span v-if="logRunning" class="px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-[10px] animate-pulse flex items-center gap-1.5 shrink-0">
              <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
              实时流监听中
            </span>
          </div>
        </div>

        <button 
          class="text-[#8b949e] hover:text-white transition-colors cursor-pointer p-1.5 rounded-md hover:bg-[#21262d]"
          title="关闭"
          @click="emit('close')"
        >
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>

      </div>

      <div class="flex-1 p-4 bg-[#0d1117] overflow-y-auto font-mono text-[12px] leading-relaxed select-text custom-terminal-scrollbar" ref="terminalLog">
        <div 
          v-for="(line, idx) in logLines" 
          :key="idx" 
          class="text-[#c9d1d9] py-0.5 px-1 rounded-sm break-all leading-relaxed select-text" 
          v-html="ansiToHtml(line)"
        ></div>
      </div>

      <div class="h-8.5 px-4 bg-[#161b22] border-t border-[#30363d] flex items-center justify-between shrink-0 select-none text-[11px] font-mono">
        <div class="flex items-center space-x-3 text-[#8b949e]">
          <span class="text-emerald-400 font-semibold bg-[#21262d] px-1.5 py-0.5 rounded text-[10px]">STREAM</span>
          <span>{{ logLines.length }} 行日志流</span>
        </div>
      </div>

    </div>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { NModal } from 'naive-ui'
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

<style scoped>
.custom-terminal-scrollbar::-webkit-scrollbar {
  width: 7px;
  height: 7px;
}
.custom-terminal-scrollbar::-webkit-scrollbar-track {
  background: #0d1117;
}
.custom-terminal-scrollbar::-webkit-scrollbar-thumb {
  background: #21262d;
  border-radius: 4px;
}
.custom-terminal-scrollbar::-webkit-scrollbar-thumb:hover {
  background: #30363d;
}
</style>
