<template>
  <n-config-provider :theme="isDark ? darkTheme : null" :theme-overrides="isDark ? darkThemeOverrides : themeOverrides" :locale="zhCN" :date-locale="dateZhCN">
    <n-dialog-provider>
      <n-message-provider>
        <div class="flex min-h-screen bg-canvas-parchment font-sans text-ink">

          <!-- Left Sidebar Navigation -->
          <div
            class="hidden md:flex w-[180px] shrink-0 bg-white border-r border-hairline flex-col justify-between p-6 select-none">
            <div class="space-y-8">
              <!-- Sidebar Header -->
              <div class="pb-4 border-b border-hairline whitespace-nowrap">
                <span class="text-[17px] font-semibold tracking-tight apple-headline">Docker Updater</span>
              </div>

              <!-- Nav Items -->
              <div class="space-y-1">
                <router-link to="/"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  <svg class="w-5 h-5 mr-3 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 0 1 6 3.75h2.25A2.25 2.25 0 0 1 10.5 6v2.25a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM3.75 15.75A2.25 2.25 0 0 1 6 13.5h2.25a2.25 2.25 0 0 1 2.25 2.25V18a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 6a2.25 2.25 0 0 1 2.25-2.25H18A2.25 2.25 0 0 1 20.25 6v2.25A2.25 2.25 0 0 1 18 10.5h-2.25a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 15.75a2.25 2.25 0 0 1 2.25-2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-2.25A2.25 2.25 0 0 1 13.5 18v-2.25Z" />
                  </svg>
                  <span>概览</span>
                </router-link>
                <router-link to="/containers"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/containers' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  <svg class="w-5 h-5 mr-3 shrink-0" viewBox="0 0 32 32">
                    <path d="M17 13V6H8v16h16v-9zm-7-5h5v5h-5zm0 7h5v5h-5zm12 5h-5v-5h5z" fill="currentColor"></path>
                    <path d="M28 11h-9V2h9zm-7-2h5V4h-5z" fill="currentColor"></path>
                    <path d="M28 20h-2v2h2v6H4v-6h2v-2H4a2.002 2.002 0 0 0-2 2v6a2.002 2.002 0 0 0 2 2h24a2.002 2.002 0 0 0 2-2v-6a2.002 2.002 0 0 0-2-2z" fill="currentColor"></path>
                    <circle cx="7" cy="25" r="1" fill="currentColor"></circle>
                  </svg>
                  <span>容器</span>
                </router-link>
                <router-link to="/images"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/images' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  <svg class="w-5 h-5 mr-3 shrink-0" viewBox="0 0 24 24">
                    <g fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M12 3l8 4.5v9L12 21l-8-4.5v-9L12 3"></path>
                      <path d="M12 12l8-4.5"></path>
                      <path d="M12 12v9"></path>
                      <path d="M12 12L4 7.5"></path>
                    </g>
                  </svg>
                  <span>镜像</span>
                </router-link>
                <router-link to="/history"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/history' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  <svg class="w-5 h-5 mr-3 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-9-9c2.53 0 4.87 1.045 6.57 2.73M21 2.25v4.5m0 0h-4.5" />
                  </svg>
                  <span>历史</span>
                </router-link>
                <router-link to="/tasks"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/tasks' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  <svg class="w-5 h-5 mr-3 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
                  </svg>
                  <span>任务</span>
                </router-link>
                <router-link to="/logs"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/logs' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  <svg class="w-5 h-5 mr-3 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
                  </svg>
                  <span>日志</span>
                </router-link>
                <router-link to="/settings"
                  class="flex items-center px-4 py-2.5 rounded-lg text-[14px] transition-all active-scale"
                  :class="[route.path === '/settings' ? 'bg-primary text-white font-semibold shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800']">
                  <svg class="w-5 h-5 mr-3 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.43l-1.003.828c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.43l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z" />
                    <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
                  </svg>
                  <span>设置</span>
                </router-link>
              </div>
            </div>
          </div>

          <!-- Bottom Navigation Bar for Mobile -->
          <div
            class="fixed bottom-0 left-0 right-0 h-16 bg-white/90 backdrop-blur-md border-t border-hairline flex md:hidden items-center justify-around z-50 px-2 py-1 select-none shadow-lg">
            <router-link to="/"
              class="flex flex-col items-center justify-center flex-1 min-w-0 transition-all active-scale"
              :class="[route.path === '/' ? 'text-primary font-semibold' : 'text-slate-500']">
              <svg class="w-5 h-5 mb-0.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 0 1 6 3.75h2.25A2.25 2.25 0 0 1 10.5 6v2.25a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM3.75 15.75A2.25 2.25 0 0 1 6 13.5h2.25a2.25 2.25 0 0 1 2.25 2.25V18a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 6a2.25 2.25 0 0 1 2.25-2.25H18A2.25 2.25 0 0 1 20.25 6v2.25A2.25 2.25 0 0 1 18 10.5h-2.25a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 15.75a2.25 2.25 0 0 1 2.25-2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-2.25A2.25 2.25 0 0 1 13.5 18v-2.25Z" />
              </svg>
              <span class="text-[10px] tracking-tight">概览</span>
            </router-link>
            
            <router-link to="/containers"
              class="flex flex-col items-center justify-center flex-1 min-w-0 transition-all active-scale"
              :class="[route.path === '/containers' ? 'text-primary font-semibold' : 'text-slate-500']">
              <svg class="w-5 h-5 mb-0.5" viewBox="0 0 32 32">
                <path d="M17 13V6H8v16h16v-9zm-7-5h5v5h-5zm0 7h5v5h-5zm12 5h-5v-5h5z" fill="currentColor"></path>
                <path d="M28 11h-9V2h9zm-7-2h5V4h-5z" fill="currentColor"></path>
                <path d="M28 20h-2v2h2v6H4v-6h2v-2H4a2.002 2.002 0 0 0-2 2v6a2.002 2.002 0 0 0 2 2h24a2.002 2.002 0 0 0 2-2v-6a2.002 2.002 0 0 0-2-2z" fill="currentColor"></path>
                <circle cx="7" cy="25" r="1" fill="currentColor"></circle>
              </svg>
              <span class="text-[10px] tracking-tight">容器</span>
            </router-link>

            <router-link to="/images"
              class="flex flex-col items-center justify-center flex-1 min-w-0 transition-all active-scale"
              :class="[route.path === '/images' ? 'text-primary font-semibold' : 'text-slate-500']">
              <svg class="w-5 h-5 mb-0.5" viewBox="0 0 24 24">
                <g fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M12 3l8 4.5v9L12 21l-8-4.5v-9L12 3"></path>
                  <path d="M12 12l8-4.5"></path>
                  <path d="M12 12v9"></path>
                  <path d="M12 12L4 7.5"></path>
                </g>
              </svg>
              <span class="text-[10px] tracking-tight">镜像</span>
            </router-link>

            <router-link to="/history"
              class="flex flex-col items-center justify-center flex-1 min-w-0 transition-all active-scale"
              :class="[route.path === '/history' ? 'text-primary font-semibold' : 'text-slate-500']">
              <svg class="w-5 h-5 mb-0.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-9-9c2.53 0 4.87 1.045 6.57 2.73M21 2.25v4.5m0 0h-4.5" />
              </svg>
              <span class="text-[10px] tracking-tight">历史</span>
            </router-link>

            <router-link to="/tasks"
              class="flex flex-col items-center justify-center flex-1 min-w-0 transition-all active-scale"
              :class="[route.path === '/tasks' ? 'text-primary font-semibold' : 'text-slate-500']">
              <svg class="w-5 h-5 mb-0.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
              </svg>
              <span class="text-[10px] tracking-tight">任务</span>
            </router-link>

            <router-link to="/logs"
              class="flex flex-col items-center justify-center flex-1 min-w-0 transition-all active-scale"
              :class="[route.path === '/logs' ? 'text-primary font-semibold' : 'text-slate-500']">
              <svg class="w-5 h-5 mb-0.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
              </svg>
              <span class="text-[10px] tracking-tight">日志</span>
            </router-link>

            <router-link to="/settings"
              class="flex flex-col items-center justify-center flex-1 min-w-0 transition-all active-scale"
              :class="[route.path === '/settings' ? 'text-primary font-semibold' : 'text-slate-500']">
              <svg class="w-5 h-5 mb-0.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.43l-1.003.828c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.43l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z" />
                <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
              </svg>
              <span class="text-[10px] tracking-tight">设置</span>
            </router-link>
          </div>

          <!-- Right Content Container -->
          <div class="flex-1 min-w-0 h-screen flex flex-col bg-canvas-parchment overflow-hidden">
            <router-view />
          </div>

        </div>
      </n-message-provider>
    </n-dialog-provider>
  </n-config-provider>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { NConfigProvider, NMessageProvider, NDialogProvider, zhCN, dateZhCN, darkTheme } from 'naive-ui'
import { useRoute } from 'vue-router'

const route = useRoute()

// 深色模式状态与系统监听
const isDark = ref(false)
let mediaQuery = null

const handleThemeChange = (e) => {
  isDark.value = e.matches
  if (isDark.value) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

onMounted(() => {
  mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  handleThemeChange(mediaQuery)
  // 现代浏览器支持 addEventListener，部分旧版本支持 addListener，此处使用标准方法
  mediaQuery.addEventListener('change', handleThemeChange)
})

onUnmounted(() => {
  if (mediaQuery) {
    mediaQuery.removeEventListener('change', handleThemeChange)
  }
})

// 映射 Apple 设计规范的系统级别主题覆盖 (浅色模式)
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

// 映射 Apple 设计规范的系统级别主题覆盖 (深色模式)
const darkThemeOverrides = {
  common: {
    primaryColor: '#0066cc',         // Action Blue
    primaryColorHover: '#0071e3',
    primaryColorPressed: '#0052a3',
    primaryColorSuppl: '#0066cc',
    borderRadius: '18px',            // 统一的大圆角
    textColorBase: '#f5f5f7'         // 浅灰色字体色
  },
  Button: {
    borderRadiusMedium: '9999px',
    borderRadiusSmall: '8px',
    borderRadiusTiny: '8px',
    textColorPrimary: '#ffffff'
  },
  Card: {
    borderRadius: '18px',
    borderColor: '#2c2c2e'
  },
  Switch: {
    railColorActive: '#0066cc'
  },
  Modal: {
    borderRadius: '18px'
  }
}
</script>
