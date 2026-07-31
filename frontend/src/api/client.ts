import axios from 'axios'
import { message as discreteMessage } from '../utils/discrete'

const apiClient = axios.create({
  baseURL: '/app/docker-updater/api',
  timeout: 30000
})

const formatErrorMessage = (rawMsg: string): string => {
  if (!rawMsg) return '网络请求发生未知异常'
  if (rawMsg === 'Network Error') return '网络连接异常，请检查后端服务状态'
  if (rawMsg.includes('timeout of') && rawMsg.includes('exceeded')) return '网络请求超时，请稍后重试'
  if (rawMsg.includes('Request failed with status code 404')) return '请求的资源或接口不存在 (404)'
  if (rawMsg.includes('Request failed with status code 500')) return '服务器内部处理异常 (500)'
  if (rawMsg.includes('Request failed with status code 403')) return '无权限访问该资源 (403)'
  return rawMsg
}

apiClient.interceptors.response.use(
  (response) => response.data,
  (error) => {
    let errMsg = '网络请求发生未知异常'

    if (error.response?.data) {
      const data = error.response.data
      if (typeof data === 'string') {
        errMsg = data
      } else if (typeof data === 'object') {
        errMsg = data.error || data.message || JSON.stringify(data)
      }
    } else if (error.message) {
      errMsg = error.message
    }

    errMsg = formatErrorMessage(errMsg)

    if (errMsg && !error.config?.suppressToast) {
      discreteMessage.error(`请求失败: ${errMsg}`)
    }
    return Promise.reject(error)
  }
)

export default apiClient
