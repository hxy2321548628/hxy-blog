import { describe, expect, it } from 'vitest'
import authReducer, {
  restoreSession,
  sessionCleared,
  sessionReceived,
} from './authSlice'

describe('authReducer', () => {
  it('只在内存状态中保存管理员和访问令牌', () => {
    const state = authReducer(
      undefined,
      sessionReceived({
        accessToken: 'access-token',
        accessExpiresAt: '2026-10-01T10:10:00Z',
        admin: { id: 1, username: 'admin' },
      }),
    )

    expect(state.status).toBe('authenticated')
    expect(state.accessToken).toBe('access-token')
    expect(state.admin?.username).toBe('admin')
  })

  it('退出时完整清除会话', () => {
    const authenticated = authReducer(
      undefined,
      sessionReceived({
        accessToken: 'access-token',
        accessExpiresAt: '2026-10-01T10:10:00Z',
        admin: { id: 1, username: 'admin' },
      }),
    )

    expect(authReducer(authenticated, sessionCleared())).toMatchObject({
      status: 'anonymous',
      accessToken: null,
      admin: null,
    })
  })

  it('较晚返回的恢复失败不会覆盖刚完成的登录', () => {
    const authenticated = authReducer(
      undefined,
      sessionReceived({
        accessToken: 'new-login-token',
        accessExpiresAt: '2026-10-01T10:10:00Z',
        admin: { id: 1, username: 'admin' },
      }),
    )
    const staleRestoreFailure = restoreSession.rejected(
      new Error('expired'),
      'request-id',
    )

    expect(authReducer(authenticated, staleRestoreFailure)).toMatchObject({
      status: 'authenticated',
      accessToken: 'new-login-token',
    })
  })
})
