import type { MessageApi } from 'naive-ui'
import { message as discreteMessage } from './discrete'

export async function copyToClipboard(
  text: string,
  label: string = '内容',
  message?: MessageApi | null
): Promise<boolean> {
  const msg = message || discreteMessage

  if (!text) {
    if (msg) msg.warning(`无法复制空的 ${label}`)
    return false
  }

  const cleanText = text.trim()
  let success = false

  if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
    try {
      await navigator.clipboard.writeText(cleanText)
      success = true
    } catch {
      success = false
    }
  }

  if (!success) {
    try {
      const textarea = document.createElement('textarea')
      textarea.value = cleanText
      textarea.setAttribute('readonly', '')
      textarea.style.position = 'absolute'
      textarea.style.left = '-9999px'
      textarea.style.top = (window.pageYOffset || document.documentElement.scrollTop) + 'px'
      textarea.style.fontSize = '12pt'

      document.body.appendChild(textarea)
      textarea.focus()
      textarea.select()
      textarea.setSelectionRange(0, textarea.value.length)

      success = document.execCommand('copy')
      document.body.removeChild(textarea)
    } catch {
      success = false
    }
  }

  if (msg) {
    const preview = cleanText.length > 20 ? cleanText.slice(0, 20) + '...' : cleanText
    if (success) {
      msg.success(`已复制 ${label} [${preview}] 到剪贴板`)
    } else {
      msg.error(`复制 ${label} [${preview}] 失败，请手动选中文本复制`)
    }
  }

  return success
}
