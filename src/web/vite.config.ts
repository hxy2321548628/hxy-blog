import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// defineConfig 提供类型推导；配置同时服务开发服务器和生产构建。
export default defineConfig({
  // React 插件负责 JSX 转换和开发时的 Fast Refresh。
  plugins: [react()],
  server: {
    proxy: {
      // 仅本地开发使用：浏览器仍请求同源 /api，Vite 再转发到 Go 的 8080 端口。
      // 生产环境的同一路径由 Nginx 处理，因此业务代码不需要环境分支。
      '/api': 'http://localhost:8080',
    },
  },
})
