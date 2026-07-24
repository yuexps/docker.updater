import { AnsiUp } from 'ansi_up'
import stripAnsiLib from 'strip-ansi'

const ansiUp = new AnsiUp()
// 自动将 < > & 等 HTML 敏感字符转义，防止 XSS
ansiUp.escape_html = true

/**
 * 将包含 ANSI 转义代码的文本直接转换为带有 CSS 样式的 safe HTML
 */
export function ansiToHtml(input: string): string {
  if (!input) return ''
  return ansiUp.ansi_to_html(input)
}

/**
 * 擦除 ANSI 代码及控制序列，返回纯文本
 */
export function stripAnsi(input: string): string {
  if (!input) return ''
  return stripAnsiLib(input)
}
