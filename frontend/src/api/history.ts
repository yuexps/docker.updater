import apiClient from './client'

export interface HistoryItem {
  ID: number
  ContainerName: string
  Image: string
  UpdatedAt: string
  Status: string
}

// 删除指定单条历史记录
export const deleteHistoryApi = (id: number): Promise<{ ok: boolean }> => {
  return apiClient.delete(`/history/${id}`)
}

// 清空所有历史记录
export const clearHistoryApi = (): Promise<{ ok: boolean }> => {
  return apiClient.delete('/history')
}
