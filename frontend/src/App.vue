<template>
  <n-config-provider :theme-overrides="themeOverrides">
    <n-dialog-provider>
      <n-message-provider>
        <div class="flex min-h-screen bg-canvas-parchment font-sans text-ink">

          <!-- Left Sidebar Navigation -->
          <div
            class="w-[220px] shrink-0 bg-white border-r border-hairline flex flex-col justify-between p-6 select-none">
            <div class="space-y-8">
              <!-- Sidebar Header -->
              <div class="pb-4 border-b border-hairline">
                <span class="text-[17px] font-semibold tracking-tight apple-headline">Docker Updater</span>
              </div>

              <!-- Nav Items -->
              <div class="space-y-1">
                <router-link to="/"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  控制面板
                </router-link>
                <router-link to="/containers"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/containers' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  容器列表
                </router-link>
                <router-link to="/images"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/images' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  镜像管理
                </router-link>
                <router-link to="/history"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/history' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  升级历史
                </router-link>
                <router-link to="/tasks"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/tasks' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  任务队列
                </router-link>
                <router-link to="/logs"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/logs' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  运行日志
                </router-link>
                <router-link to="/settings"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/settings' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  设置
                </router-link>
              </div>
            </div>
          </div>

          <!-- Right Content Container -->
          <div class="flex-1 min-w-0 h-screen overflow-y-auto bg-canvas-parchment p-8 lg:p-10">
            <router-view />
          </div>

        </div>
      </n-message-provider>
    </n-dialog-provider>
  </n-config-provider>
</template>

<script setup>
import { NConfigProvider, NMessageProvider, NDialogProvider } from 'naive-ui'
import { useRoute } from 'vue-router'

const route = useRoute()

// 映射 Apple 设计规范的系统级别主题覆盖
const themeOverrides = {
  common: {
    primaryColor: '#0066cc',         // Action Blue
    primaryColorHover: '#0071e3',
    primaryColorPressed: '#0052a3',
    primaryColorSuppl: '#0066cc',
    borderRadius: '18px',            // 统一的大圆角
    textColorBase: '#1d1d1f'         // Obsidian 字体色
  },
  Button: {
    borderRadiusMedium: '9999px',    // 主按钮胶囊圆角
    borderRadiusSmall: '8px',        // 辅助工具按钮圆角
    borderRadiusTiny: '8px',
    textColorPrimary: '#ffffff'
  },
  Card: {
    borderRadius: '18px',
    borderColor: '#e0e0e0'
  },
  Switch: {
    railColorActive: '#0066cc'
  },
  Modal: {
    borderRadius: '18px'
  }
}
</script>
