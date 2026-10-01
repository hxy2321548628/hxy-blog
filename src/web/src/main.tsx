import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Provider } from 'react-redux'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import App from './App'
import { store } from './app/store'
import './styles.css'

// index.html 提供唯一挂载点；缺失时立即失败比渲染空白页更容易定位构建问题。
const root = document.getElementById('root')

if (!root) {
  throw new Error('Root element not found')
}

// 数据路由保留现有页面结构，并让上传中的离开确认可以阻止误导航。
const router = createBrowserRouter([{ path: '*', element: <App /> }])

// StrictMode 仅在开发阶段额外检查副作用；生产构建不会重复渲染组件。
createRoot(root).render(
  <StrictMode>
    <Provider store={store}>
      <RouterProvider router={router} />
    </Provider>
  </StrictMode>,
)
