import { apiErrorFrom, httpClient, type ApiError } from '../../app/httpClient'
import { createRefreshCoordinator } from './refreshCoordinator'

export interface AdminIdentity {
  id: number
  username: string
}

export interface AuthSession {
  accessToken: string
  accessExpiresAt: string
  admin: AdminIdentity
}

export interface LoginCredentials {
  username: string
  password: string
}

async function requestSession(
  url: '/auth/login' | '/auth/refresh',
  data?: LoginCredentials,
): Promise<AuthSession> {
  try {
    const response = await httpClient.post<AuthSession>(url, data)
    return response.data
  } catch (error: unknown) {
    throw apiErrorFrom(error)
  }
}

export function loginSession(credentials: LoginCredentials) {
  return requestSession('/auth/login', credentials)
}

// 页面恢复和 401 重试共享同一个协调器，StrictMode 重挂载也不会轮换两次令牌。
export const refreshSession = createRefreshCoordinator(() =>
  requestSession('/auth/refresh'),
)

export async function logoutSession(): Promise<void> {
  try {
    await httpClient.post('/auth/logout')
  } catch (error: unknown) {
    throw apiErrorFrom(error)
  }
}

export function isApiError(value: unknown): value is ApiError {
  return (
    typeof value === 'object' &&
    value !== null &&
    'status' in value &&
    'code' in value &&
    'message' in value
  )
}
