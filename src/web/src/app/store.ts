import { configureStore } from '@reduxjs/toolkit'
import authReducer from '../features/auth/authSlice'
import { postApi } from '../features/posts/postApi'

// RTK Query 缓存文章数据；auth 只保存当前页面生命周期内的短期访问令牌。
export const store = configureStore({
  reducer: {
    auth: authReducer,
    [postApi.reducerPath]: postApi.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(postApi.middleware),
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
