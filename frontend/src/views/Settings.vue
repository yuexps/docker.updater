<template>
  <div class="view-fade-in">
    <!-- Page Header -->
    <div class="flex items-center justify-between mb-8 select-none">
      <div class="flex items-baseline space-x-3">
        <h1 class="text-[28px] font-semibold tracking-tight text-slate-800 apple-headline">设置</h1>
        <span 
          class="text-[12px] text-body-muted transition-all duration-300 select-none" 
          :class="showFeedback ? 'opacity-100 translate-x-0' : 'opacity-0 -translate-x-1 pointer-events-none'"
        >
          {{ feedbackText }}
        </span>
      </div>
    </div>

    <!-- Configuration Cards stack -->
    <div class="space-y-6">
      
      <!-- Card 1: Backup Toggle -->
      <div class="apple-card rounded-lg p-6 flex items-center justify-between min-h-[90px]">
        <div>
          <div class="text-[15px] font-semibold text-slate-800">自动备份服务容器</div>
          <div class="text-[13px] text-body-muted mt-1">在容器更新重建前克隆备份旧版实例，若出现故障则支持自愈回退。</div>
        </div>
        <n-switch v-model:value="settings.backup_enabled" @update:value="autoSaveSettings" />
      </div>

      <!-- Card 2: Expiry Hours -->
      <div 
        class="apple-card rounded-lg p-6 flex items-center justify-between min-h-[90px] transition-opacity duration-200"
        :class="settings.backup_enabled ? 'opacity-100' : 'opacity-50 pointer-events-none select-none'"
      >
        <div>
          <div class="text-[15px] font-semibold text-slate-800">备份自动清除周期</div>
          <div class="text-[13px] text-body-muted mt-1">超出保留期后，系统后台会自动物理清除旧容器备份以释放磁盘空间。</div>
        </div>
        <div class="w-[180px]">
          <n-select 
            v-model:value="settings.backup_hours" 
            :options="hoursOptions" 
            :disabled="!settings.backup_enabled" 
            @update:value="autoSaveSettings"
          />
        </div>
      </div>

      <!-- Card 3: Sibling Restart Toggle -->
      <div class="apple-card rounded-lg p-6 flex items-center justify-between min-h-[90px]">
        <div>
          <div class="text-[15px] font-semibold text-slate-800">自动重启同 Compose 项目服务</div>
          <div class="text-[13px] text-body-muted mt-1">当服务更新重建后，自动重启同 Compose 项目下的其它关联服务。</div>
        </div>
        <n-switch v-model:value="settings.restart_stack" @update:value="autoSaveSettings" />
      </div>

      <!-- Card 4: Private Registry Credentials Management -->
      <div class="apple-card rounded-lg p-6 space-y-4">
        <div>
          <div class="text-[15px] font-semibold text-slate-800">Registry 凭证</div>
          <div class="text-[13px] text-body-muted mt-1">配置镜像源仓库（如阿里云、自建 Harbor 等）的认证账号，用于拉取和比对私有镜像。</div>
        </div>

        <!-- Credentials List -->
        <div v-if="registries.length > 0" class="border border-hairline rounded overflow-hidden divide-y divide-hairline">
          <div v-for="r in registries" :key="r.id" class="flex items-center justify-between p-4 bg-slate-50/10">
            <div>
              <div class="text-[14px] font-semibold text-slate-800 break-all pr-4">{{ r.registry }}</div>
              <div class="text-[12px] text-body-muted mt-0.5">Username: {{ r.username }}</div>
            </div>
            <div class="flex items-center space-x-2 shrink-0">
              <n-button size="tiny" round class="active-scale" @click="editRegistry(r)">
                编辑
              </n-button>
              <n-button size="tiny" type="error" round class="active-scale" @click="deleteRegistry(r.id)">
                删除
              </n-button>
            </div>
          </div>
        </div>

        <div v-else class="text-center py-8 text-body-muted text-[13px] bg-canvas-parchment rounded border border-dashed border-hairline select-none">
          暂未配置任何 Registry 凭证
        </div>

        <div class="pt-2">
          <n-button 
            secondary 
            round 
            size="small" 
            class="active-scale bg-surface-pearl border border-divider-soft text-slate-700 w-full"
            @click="openAddRegistryModal"
          >
            添加 Registry 凭证
          </n-button>
        </div>
      </div>

      <!-- Card 5: System Registry Mirrors (Read-Only) -->
      <div class="apple-card rounded-lg p-6 space-y-4">
        <div>
          <div class="text-[15px] font-semibold text-slate-800">系统 Registry Mirrors</div>
          <div class="text-[13px] text-body-muted mt-1">只读获取宿主机本地全局生效的 Docker 镜像加速源。</div>
        </div>

        <div v-if="systemMirrors.length > 0" class="border border-hairline rounded overflow-hidden divide-y divide-hairline">
          <div v-for="m in systemMirrors" :key="m" class="p-3.5 text-[13px] font-mono text-slate-700 bg-slate-50/10 break-all select-all">
            {{ m }}
          </div>
        </div>
        <div v-else class="text-center py-6 text-body-muted text-[13px] bg-canvas-parchment rounded border border-dashed border-hairline select-none">
          当前宿主机系统未配置任何全局镜像加速源
        </div>
      </div>

      <!-- Card 6: Temporary Registry Mirrors (Read & Write) -->
      <div class="apple-card rounded-lg p-6 space-y-4">
        <div>
          <div class="text-[15px] font-semibold text-slate-800">临时 Pull Mirrors</div>
          <div class="text-[13px] text-body-muted mt-1">添加仅在此升级器内生效的临时加速源。拉取官方镜像时，会自动在此加速下载，不重启系统 Docker，且不修改宿主机全局配置。</div>
        </div>

        <!-- Temporary Mirrors List -->
        <div v-if="settings.temp_mirrors && settings.temp_mirrors.length > 0" class="border border-hairline rounded overflow-hidden divide-y divide-hairline">
          <div v-for="(m, idx) in settings.temp_mirrors" :key="idx" class="flex items-center justify-between p-3.5 bg-slate-50/10">
            <span class="text-[13px] font-mono text-slate-700 break-all pr-4">{{ m }}</span>
            <n-button size="tiny" type="error" round class="active-scale shrink-0" @click="removeTempMirror(idx)">
              删除
            </n-button>
          </div>
        </div>

        <div v-else class="text-center py-8 text-body-muted text-[13px] bg-canvas-parchment rounded border border-dashed border-hairline select-none">
          暂未配置任何临时 Pull Mirrors
        </div>

        <div class="pt-2 flex space-x-2">
          <n-input v-model:value="newTempMirror" placeholder="例如: https://docker.m.daocloud.io" />
          <n-button 
            secondary 
            round 
            size="medium" 
            class="active-scale bg-surface-pearl border border-divider-soft text-slate-700 shrink-0" 
            @click="addTempMirror"
          >
            添加
          </n-button>
        </div>
      </div>

    </div>

    <!-- Registry Credentials Modal -->
    <n-modal 
      v-model:show="registryModalVisible" 
      preset="dialog" 
      :title="editingRegistryId ? '编辑 Registry 凭证' : '添加 Registry 凭证'"
      positive-text="保存凭据"
      negative-text="取消"
      @positive-click="submitRegistry"
    >
      <div class="space-y-4 py-4">
        <div>
          <label class="text-[12px] font-semibold uppercase tracking-wider text-slate-500 block mb-1">Registry 域名</label>
          <n-input v-model:value="registryForm.registry" placeholder="例如: registry.cn-hangzhou.aliyuncs.com" />
        </div>
        <div>
          <label class="text-[12px] font-semibold uppercase tracking-wider text-slate-500 block mb-1">Username</label>
          <n-input v-model:value="registryForm.username" placeholder="请输入用户名" />
        </div>
        <div>
          <label class="text-[12px] font-semibold uppercase tracking-wider text-slate-500 block mb-1">Password / Token</label>
          <n-input v-model:value="registryForm.password" type="password" show-password-on="click" placeholder="请输入密码或 Token" />
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NButton, NSwitch, NSelect, NInput, NModal, useMessage, useDialog } from 'naive-ui'
import axios from 'axios'

const apiBase = '/app/docker-updater/api'
const message = useMessage()
const dialog = useDialog()

interface SettingsData {
  backup_enabled: boolean;
  backup_hours: number;
  restart_stack: boolean;
  temp_mirrors: string[];
}

const settings = ref<SettingsData>({
  backup_enabled: false,
  backup_hours: 24,
  restart_stack: false,
  temp_mirrors: []
})

const registries = ref<any[]>([])
const registryModalVisible = ref<boolean>(false)
const editingRegistryId = ref<number | null>(null)
const registryForm = ref({
  registry: '',
  username: '',
  password: ''
})

const systemMirrors = ref<string[]>([])
const newTempMirror = ref<string>('')

const saving = ref<boolean>(false)
const showFeedback = ref<boolean>(false)
const feedbackText = ref<string>('')
let feedbackTimer: any = null

const hoursOptions = [
  { label: '保留 12 小时', value: 12 },
  { label: '保留 24 小时 (1天)', value: 24 },
  { label: '保留 72 小时 (3天)', value: 72 },
  { label: '保留 168 小时 (7天)', value: 168 }
]

const loadSettings = async () => {
  try {
    const res = await axios.get(`${apiBase}/settings`)
    settings.value = res.data
    if (!settings.value.temp_mirrors) {
      settings.value.temp_mirrors = []
    }
  } catch (err) {
    message.error('载入配置失败')
  }
}

const autoSaveSettings = async () => {
  if (feedbackTimer) {
    clearTimeout(feedbackTimer)
  }
  
  feedbackText.value = '正在同步...'
  showFeedback.value = true
  saving.value = true
  
  try {
    await axios.post(`${apiBase}/settings`, settings.value)
    saving.value = false
    feedbackText.value = '配置已自动保存'
    
    // 2.5秒后渐变淡出
    feedbackTimer = setTimeout(() => {
      showFeedback.value = false
    }, 2500)
  } catch (err) {
    saving.value = false
    feedbackText.value = '保存配置失败'
    feedbackTimer = setTimeout(() => {
      showFeedback.value = false
    }, 3000)
  }
}

const loadRegistries = async () => {
  try {
    const res = await axios.get(`${apiBase}/registries`)
    registries.value = res.data || []
  } catch (err) {
    message.error('载入私有源列表失败')
  }
}

const loadSystemMirrors = async () => {
  try {
    const res = await axios.get(`${apiBase}/settings/system-mirrors`)
    systemMirrors.value = res.data || []
  } catch (err) {
    // 静默忽略
  }
}

const addTempMirror = async () => {
  const url = newTempMirror.value.trim()
  if (!url) return
  if (!settings.value.temp_mirrors) {
    settings.value.temp_mirrors = []
  }
  if (!settings.value.temp_mirrors.includes(url)) {
    settings.value.temp_mirrors.push(url)
    newTempMirror.value = ''
    await autoSaveSettings()
  } else {
    message.warning('该加速源已存在')
  }
}

const removeTempMirror = async (index: number) => {
  if (settings.value.temp_mirrors) {
    settings.value.temp_mirrors.splice(index, 1)
    await autoSaveSettings()
  }
}

const openAddRegistryModal = () => {
  editingRegistryId.value = null
  registryForm.value = { registry: '', username: '', password: '' }
  registryModalVisible.value = true
}

const editRegistry = (item: any) => {
  editingRegistryId.value = item.id
  registryForm.value = {
    registry: item.registry,
    username: item.username,
    password: item.password
  }
  registryModalVisible.value = true
}

const submitRegistry = async () => {
  if (!registryForm.value.registry || !registryForm.value.username || !registryForm.value.password) {
    message.warning('请填写完整的认证信息')
    return false
  }

  try {
    await axios.post(`${apiBase}/registries`, {
      id: editingRegistryId.value || 0,
      registry: registryForm.value.registry,
      username: registryForm.value.username,
      password: registryForm.value.password
    })
    message.success('凭证保存成功')
    loadRegistries()
  } catch (err) {
    message.error('保存凭证失败')
    return false
  }
}

const deleteRegistry = (id: number) => {
  dialog.warning({
    title: '删除 Registry 凭据',
    content: '确认删除该 Registry 镜像认证凭证吗？删除后将导致针对该镜像源的容器服务无法正常升级。',
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await axios.delete(`${apiBase}/registries/${id}`)
        message.success('凭证删除成功')
        loadRegistries()
      } catch (err) {
        message.error('删除凭证失败')
      }
    }
  })
}

onMounted(() => {
  loadSettings()
  loadRegistries()
  loadSystemMirrors()
})
</script>
