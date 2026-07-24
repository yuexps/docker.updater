<template>
  <n-modal v-model:show="showModel" preset="dialog" title="暂挂升级检测" positive-text="确认暂挂" negative-text="取消"
    class="rounded-lg" @positive-click="emit('submit')">
    <div class="py-4 select-none">
      <label class="text-[12px] font-semibold uppercase tracking-wider text-body-muted block mb-2">搁置升级比对时长</label>
      <n-select v-model:value="daysModel" :options="deferOptions" />
    </div>
  </n-modal>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { NModal, NSelect } from 'naive-ui'

const props = defineProps<{
  show: boolean
  days: number
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'update:days', val: number): void
  (e: 'submit'): void
}>()

const showModel = computed({
  get: () => props.show,
  set: (val) => emit('update:show', val)
})

const daysModel = computed({
  get: () => props.days,
  set: (val) => emit('update:days', val)
})

const deferOptions = [
  { label: '暂挂 7 天', value: 7 },
  { label: '暂挂 14 天', value: 14 },
  { label: '暂挂 30 天', value: 30 },
  { label: '暂挂 90 天', value: 90 },
  { label: '永久暂挂', value: -1 }
]
</script>
