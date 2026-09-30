import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import './styles.css'

// index.html 提供唯一挂载点；缺失时立即失败比渲染空白页更容易定位构建问题。
const root = document.getElementById('root')

if (!root) {
  throw new Error('Root element not found')
}

// StrictMode 仅在开发阶段额外检查副作用；生产构建不会重复渲染组件。
createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
