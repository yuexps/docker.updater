import apiClient from './client'

// 将任务移出排队队列
export const cancelTaskApi = (name: string): Promise<{ success: boolean }> => {
  return apiClient.post(`/tasks/cancel/${name}`)
}

// 获取容器升级过程的历史日志
export const getUpdateLogApi = (name: string): Promise<{ found: boolean; logs: string[] }> => {
  return apiClient.get(`/update-log/${name}`)
}
