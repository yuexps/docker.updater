import { createDiscreteApi } from 'naive-ui'

// 创建全局独立消息与对话框实例，脱离组件生命周期约束
const { message, dialog } = createDiscreteApi(
  ['message', 'dialog'],
  {
    messageProviderProps: {
      max: 5,
      keepAliveOnHover: true
    }
  }
)

export { message, dialog }
export default message
