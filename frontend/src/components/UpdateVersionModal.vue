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
        <label class="text-[12px] font-semibold uppercase tracking-wider text-body-muted block mb-1.5">目标版本 /
          镜像名</label>
        <n-input v-model:value="versionModel" placeholder="例如: 8.0 或 mysql:8.0" class="rounded-lg"
          @keyup.enter="emit('submit')" />
        <div class="text-[11px] text-body-muted mt-2 leading-relaxed">
          若仅输入 Tag（例如 5.7），将自动拼装原镜像库名（例如: {{ currentImage.split(':')[0] }}:5.7）；若包含冒号，将直接使用完整镜像名。
        </div>
      </div>
    </div>
  </n-modal>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { NModal, NInput } from 'naive-ui'

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
</script>
