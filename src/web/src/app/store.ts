import { configureStore } from '@reduxjs/toolkit'
import { postApi } from '../features/posts/postApi'

// RTK Query 是服务器状态的唯一缓存层，列表数据不再复制到普通 slice。
export const store = configureStore({
  reducer: {
    [postApi.reducerPath]: postApi.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(postApi.middleware),
})
