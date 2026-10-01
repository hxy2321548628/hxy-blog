import { createAsyncThunk, createSlice, type PayloadAction } from '@reduxjs/toolkit'
import type { ApiError } from '../../app/httpClient'
import {
  isApiError,
  loginSession,
  logoutSession,
  refreshSession,
  type AdminIdentity,
  type AuthSession,
  type LoginCredentials,
} from './authClient'

export interface AuthState {
  status: 'checking' | 'authenticated' | 'anonymous'
  accessToken: string | null
  accessExpiresAt: string | null
  admin: AdminIdentity | null
  restoreAttempted: boolean
  loginPending: boolean
  loginError: string | null
}

interface AuthRootState {
  auth: AuthState
}

const initialState: AuthState = {
  status: 'checking',
  accessToken: null,
  accessExpiresAt: null,
  admin: null,
  restoreAttempted: false,
  loginPending: false,
  loginError: null,
}

function rejection(error: unknown): ApiError {
  return isApiError(error)
    ? error
    : { status: 0, code: 'UNKNOWN_ERROR', message: '请求失败，请稍后重试' }
}

export const restoreSession = createAsyncThunk<
  AuthSession,
  void,
  { state: AuthRootState; rejectValue: ApiError }
>(
  'auth/restoreSession',
  async (_, { rejectWithValue }) => {
    try {
      return await refreshSession()
    } catch (error: unknown) {
      return rejectWithValue(rejection(error))
    }
  },
  {
    // React StrictMode 会重放副作用；同步置位后拒绝第二次恢复，保护一次性刷新令牌。
    condition: (_, { getState }) => !getState().auth.restoreAttempted,
  },
)

export const login = createAsyncThunk<
  AuthSession,
  LoginCredentials,
  { rejectValue: ApiError }
>('auth/login', async (credentials, { rejectWithValue }) => {
  try {
    return await loginSession(credentials)
  } catch (error: unknown) {
    return rejectWithValue(rejection(error))
  }
})

export const logout = createAsyncThunk<void, void, { rejectValue: ApiError }>(
  'auth/logout',
  async (_, { rejectWithValue }) => {
    try {
      await logoutSession()
    } catch (error: unknown) {
      return rejectWithValue(rejection(error))
    }
  },
)

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    sessionReceived(state, action: PayloadAction<AuthSession>) {
      state.status = 'authenticated'
      state.accessToken = action.payload.accessToken
      state.accessExpiresAt = action.payload.accessExpiresAt
      state.admin = action.payload.admin
      state.loginError = null
    },
    sessionCleared(state) {
      state.status = 'anonymous'
      state.accessToken = null
      state.accessExpiresAt = null
      state.admin = null
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(restoreSession.pending, (state) => {
        state.restoreAttempted = true
      })
      .addCase(restoreSession.fulfilled, (state, action) => {
        authSlice.caseReducers.sessionReceived(state, action)
      })
      .addCase(restoreSession.rejected, (state) => {
        // 慢响应不得覆盖在此期间刚完成的手动登录。
        if (state.status !== 'authenticated') {
          authSlice.caseReducers.sessionCleared(state)
        }
      })
      .addCase(login.pending, (state) => {
        state.loginPending = true
        state.loginError = null
      })
      .addCase(login.fulfilled, (state, action) => {
        state.loginPending = false
        authSlice.caseReducers.sessionReceived(state, action)
      })
      .addCase(login.rejected, (state, action) => {
        state.loginPending = false
        state.loginError = action.payload?.message ?? '登录失败，请稍后重试'
      })
      .addCase(logout.fulfilled, (state) => {
        authSlice.caseReducers.sessionCleared(state)
      })
      .addCase(logout.rejected, (state) => {
        // 即使服务端暂时不可用，也先清除内存中的访问令牌，避免本机继续显示已登录。
        authSlice.caseReducers.sessionCleared(state)
      })
  },
})

export const { sessionCleared, sessionReceived } = authSlice.actions
export default authSlice.reducer
