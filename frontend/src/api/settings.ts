import apiClient from './client'

export interface SettingsData {
  auto_update_enabled: boolean
  auto_update_whitelist: string[]
  dockerhub_mirrors: string[]
  ghcr_mirrors: string[]
  check_type: string
  check_value: number
  notify_enabled: boolean
  notify_type: string
  smtp_host: string
  smtp_port: string
  smtp_username: string
  smtp_password: string
  smtp_ssl: boolean
  smtp_to: string
  smtp_subject_template: string
  smtp_body_template: string
  smtp_subject_template_check: string
  smtp_body_template_check: string
  webhook_url: string
  webhook_method: string
  webhook_type: string
  webhook_template_custom: string
  webhook_template_wechat: string
  webhook_template_dingtalk: string
  webhook_template_feishu: string
  webhook_template_check_custom: string
  webhook_template_check_wechat: string
  webhook_template_check_dingtalk: string
  webhook_template_check_feishu: string
  [key: string]: any
}

export interface RegistryItem {
  id: number
  registry: string
  username: string
  password?: string
}

// 获取系统配置
export const getSettingsApi = (): Promise<SettingsData> => {
  return apiClient.get('/settings')
}

// 保存系统配置
export const saveSettingsApi = (settings: Partial<SettingsData>): Promise<any> => {
  return apiClient.post('/settings', settings)
}

// 测试推送通知 (抑制全局 Toast，由 Settings 组件定制弹窗)
export const testNotificationApi = (data: any): Promise<{ response?: string }> => {
  return apiClient.post('/settings/test-email', data, { suppressToast: true } as any)
}

// 获取私有仓库凭据列表
export const getRegistriesApi = (): Promise<RegistryItem[]> => {
  return apiClient.get('/registries')
}

// 保存/添加私有仓库凭据
export const saveRegistryApi = (data: { id?: number | null; registry: string; username: string; password?: string }): Promise<any> => {
  return apiClient.post('/registries', data)
}

// 删除私有仓库凭据
export const deleteRegistryApi = (id: number): Promise<any> => {
  return apiClient.delete(`/registries/${id}`)
}
