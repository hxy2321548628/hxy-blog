import { useEffect, useState } from 'react'

type ApiState = 'checking' | 'online' | 'offline'

function App() {
	// 联合类型限制状态只能取三个已知值，渲染分支不会出现任意字符串。
  const [apiState, setApiState] = useState<ApiState>('checking')

  useEffect(() => {
	// 组件卸载时中止尚未完成的请求，避免请求完成后更新已卸载组件。
    const controller = new AbortController()

	// 使用相对路径让开发代理和生产 Nginx 都能把请求转发到同一个 Go API。
    fetch('/api/health', { signal: controller.signal })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Health check failed: ${response.status}`)
        }
        setApiState('online')
      })
      .catch((error: unknown) => {
		// AbortError 是主动清理，不代表 API 离线；其他网络/协议错误统一显示离线。
        if (error instanceof DOMException && error.name === 'AbortError') {
          return
        }
        setApiState('offline')
      })

    return () => controller.abort()
  }, [])

  // 以状态为键查表，避免 JSX 中嵌套多个条件表达式。
  const statusText = {
    checking: '正在检查 API…',
    online: 'API 已连接',
    offline: 'API 未连接',
  }[apiState]

  return (
    <main className="shell">
      <p className="eyebrow">HXY BLOG</p>
      <h1>一个正在生长的个人博客</h1>
      <p className="summary">工程骨架已就绪，下一步将从第一篇可发布的文章开始。</p>
      <p className={`status status--${apiState}`} role="status">
        {/* 圆点只表达视觉状态，aria-hidden 避免读屏重复朗读无意义元素。 */}
        <span aria-hidden="true" />
        {statusText}
      </p>
    </main>
  )
}

export default App
