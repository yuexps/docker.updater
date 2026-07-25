import type { MessageApi } from 'naive-ui'

/**
 * 统一剪贴板复制工具函数
 * @param text 要复制的目标文本内容
 * @param label 提示标签 (例如 "镜像名称"、"Digest 摘要")
 * @param message Naive UI useMessage 实例 (可选，传入则自动触发 Toast 提示)
 */
export async function copyToClipboard(
  text: string,
  label: string = '内容',
  message?: MessageApi | null
): Promise<boolean> {
  if (!text) {
    if (message) message.warning(`无法复制空的 ${label}`)
    return false
  }

  const cleanText = text.trim()
  let success = false

  // 1. 优先尝试现代 Clipboard API
  if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
    try {
      await navigator.clipboard.writeText(cleanText)
      success = true
    } catch (e) {
      // 若因非 SecureContext 或权限问题失败，静默进入二次 Fallback 级联保障
      success = false
    }
  }

  // 2. 级联降级备用方案：兼容 HTTP 内网 IP 访问与移动端浏览器
  if (!success) {
    try {
      const textarea = document.createElement('textarea')
      textarea.value = cleanText

      // 设置 readonly 防止移动端唤起系统软键盘
      textarea.setAttribute('readonly', '')

      // 隐秘防坍塌定位
      textarea.style.position = 'absolute'
      textarea.style.left = '-9999px'
      textarea.style.top = (window.pageYOffset || document.documentElement.scrollTop) + 'px'
      textarea.style.fontSize = '12pt'
      textarea.style.border = '0'
      textarea.style.padding = '0'
      textarea.style.margin = '0'

      document.body.appendChild(textarea)

      // iOS Safari 兼容全选适配
      textarea.focus()
      textarea.select()
      textarea.setSelectionRange(0, textarea.value.length)

      success = document.execCommand('copy')
      document.body.removeChild(textarea)
    } catch (err) {
      success = false
    }
  }

  // 3. 结果统一 Toast 提示
  if (message) {
    if (success) {
      message.success(`已复制 ${label} 到剪贴板`)
    } else {
      message.error(`复制 ${label} 失败，请手动选中文本复制`)
    }
  }

  return success
}
