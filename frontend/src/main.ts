import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'

const mountApp = () => {
  const app = createApp(App)
  app.use(router)
  
  // 路由切换后自动释放先前点击元素的焦点，防止移动端按钮/底栏在切换页面后残留 hover/focus 高亮
  router.afterEach(() => {
    if (document.activeElement && typeof (document.activeElement as any).blur === 'function') {
      (document.activeElement as HTMLElement).blur()
    }
  })
  
  app.mount('#app')
}

if (import.meta.env.DEV) {
  import('./utils/test').then(() => {
    mountApp()
  })
} else {
  mountApp()
}

// 强制阻止 iOS Safari 双指捏合缩放手势
document.addEventListener('gesturestart', (event) => {
  event.preventDefault();
});

// 强制阻止 iOS Safari 快速双击缩放行为，同时避免干扰输入框聚焦
let lastTouchEnd = 0;
document.addEventListener('touchend', (event) => {
  const now = new Date().getTime();
  if (now - lastTouchEnd <= 300) {
    const target = event.target as HTMLElement;
    // 如果点击的是输入框、文本域或富文本编辑区，不予拦截，确保虚拟键盘可正常弹出
    if (
      target &&
      (target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.tagName === 'SELECT' ||
        target.isContentEditable)
    ) {
      return;
    }
    event.preventDefault(); // 拦截 300ms 内的双击缩放
  }
  lastTouchEnd = now;
}, false);

// 阻止多点触控的默认缩放事件
document.addEventListener('touchstart', (event) => {
  if (event.touches.length > 1) {
    event.preventDefault();
  }
}, { passive: false });

// 移动端点击非交互空白区域时，自动释放当前激活的输入框或按钮焦点，隐藏软键盘并恢复状态
document.addEventListener('touchstart', (event) => {
  const target = event.target as HTMLElement;
  if (!target) return;

  // 识别是否点击在可交互的可聚焦元素上
  const isInteractive =
    target.tagName === 'INPUT' ||
    target.tagName === 'TEXTAREA' ||
    target.tagName === 'SELECT' ||
    target.isContentEditable ||
    target.closest('button') ||
    target.closest('a') ||
    target.closest('.n-button') ||
    target.closest('[role="button"]');

  if (!isInteractive && document.activeElement && typeof (document.activeElement as any).blur === 'function') {
    (document.activeElement as HTMLElement).blur();
  }
}, { passive: true });
