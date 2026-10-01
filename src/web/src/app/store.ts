import { configureStore } from '@reduxjs/toolkit'
import authReducer from '../features/auth/authSlice'
import mediaReducer from '../features/media/mediaSlice'
import { postApi } from '../features/posts/postApi'

// RTK Query 缓存文章数据；auth 与 media 只保存当前页面生命周期内的短期状态。
export const store = configureStore({
  reducer: {
    auth: authReducer,
    media: mediaReducer,
    [postApi.reducerPath]: postApi.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(postApi.middleware),
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
