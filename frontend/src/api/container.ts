import apiClient from './client'

export const updateContainerApi = (name: string, targetImage?: string): Promise<any> => {
  const url = targetImage
    ? `/update/${name}?target_image=${encodeURIComponent(targetImage)}`
    : `/update/${name}`
  return apiClient.get(url, { suppressToast: true, timeout: 6000 } as any).catch(() => {})
}

export const getContainerLogsApi = (name: string): Promise<{ logs: string }> => {
  return apiClient.get(`/container/${name}/logs`, { suppressToast: true } as any)
}

export const deferContainerApi = (name: string, days: number): Promise<any> => {
  return apiClient.post(`/defer/${name}`, { days }, { suppressToast: true } as any)
}

export const undeferContainerApi = (name: string): Promise<any> => {
  return apiClient.post(`/undefer/${name}`, null, { suppressToast: true } as any)
}

export const startContainerApi = (name: string): Promise<any> => {
  return apiClient.post(`/container/${name}/start`, null, { timeout: 20000, suppressToast: true } as any)
}

export const stopContainerApi = (name: string): Promise<any> => {
  return apiClient.post(`/container/${name}/stop`, null, { timeout: 15000, suppressToast: true } as any)
}

export const restartContainerApi = (name: string): Promise<any> => {
  return apiClient.post(`/container/${name}/restart`, null, { timeout: 25000, suppressToast: true } as any)
}

export const checkContainerApi = (name: string): Promise<any> => {
  return apiClient.post(`/check/${name}`, null, { suppressToast: true } as any)
}

export const checkAllContainersApi = (): Promise<any> => {
  return apiClient.post('/check')
}
