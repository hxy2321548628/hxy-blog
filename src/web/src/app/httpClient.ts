import axios from 'axios'

export interface ApiError {
  status: number
  code: string
  message: string
}

// 浏览器只与同源 /api 通信；刷新令牌由 HttpOnly Cookie 自动携带，前端代码不可读取。
export const httpClient = axios.create({
  baseURL: '/api',
  timeout: 10_000,
  withCredentials: true,
})

export function apiErrorFrom(error: unknown): ApiError {
  if (axios.isAxiosError(error)) {
    const payload: unknown = error.response?.data
    if (
      typeof payload === 'object' &&
      payload !== null &&
      'code' in payload &&
      'message' in payload &&
      typeof payload.code === 'string' &&
      typeof payload.message === 'string'
    ) {
      return {
        status: error.response?.status ?? 0,
        code: payload.code,
        message: payload.message,
      }
    }
  }
  return { status: 0, code: 'NETWORK_ERROR', message: '无法连接服务器' }
}
