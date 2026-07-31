import apiClient from './client'

export interface ImageItem {
  id: string
  tags: string[]
  size: number
  created: number
  containers?: string[]
  architecture?: string
}

// 获取宿主机镜像列表
export const getImagesApi = (): Promise<ImageItem[]> => {
  return apiClient.get('/images')
}

// 获取镜像可用的 Tag 列表
export const getImageTagsApi = (image: string): Promise<string[]> => {
  return apiClient.get('/image/tags', { params: { image }, suppressToast: true } as any)
}

// 清理无标签残留镜像
export const pruneImagesApi = (): Promise<{ deleted_count: number; space_reclaimed: number }> => {
  return apiClient.post('/images/prune', null, { suppressToast: true } as any)
}

// 删除指定 ID 镜像
export const deleteImageApi = (id: string): Promise<any> => {
  return apiClient.delete('/image', { params: { id }, suppressToast: true } as any)
}
