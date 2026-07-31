import apiClient from './client'
import type { ComposeProject } from '../stores/useDockerStore'

// 获取 Compose 项目列表
export const getComposeProjectsApi = (): Promise<ComposeProject[]> => {
  return apiClient.get('/compose/projects')
}

// 变更 Compose 项目生命周期状态
export const handleComposeLifecycleApi = (project: string, action: 'start' | 'stop' | 'restart'): Promise<any> => {
  return apiClient.post('/compose/lifecycle', { project, action })
}

// 获取 Compose 项目 YAML 内容
export const getComposeContentApi = (project: string): Promise<{ project: string; working_dir?: string; file_path: string; content: string }> => {
  return apiClient.get('/compose/config', { params: { project }, suppressToast: true } as any)
}

// 保存 Compose 项目 YAML 内容
export const saveComposeContentApi = (project: string, content: string): Promise<{ ok?: boolean; error?: string }> => {
  return apiClient.post('/compose/config', { project, content }, { suppressToast: true } as any)
}
