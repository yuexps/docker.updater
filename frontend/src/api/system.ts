import apiClient from './client'

// 拉取系统运行日志快照
export const getSystemLogsApi = (): Promise<{ logs: string[] }> => {
  return apiClient.get('/system/logs')
}

// 清空系统运行日志
export const clearSystemLogsApi = (): Promise<{ ok: boolean }> => {
  return apiClient.delete('/system/logs')
}
