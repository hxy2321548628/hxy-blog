import { useEffect, useState } from 'react'

type ApiState = 'checking' | 'online' | 'offline'

function App() {
  const [apiState, setApiState] = useState<ApiState>('checking')

  useEffect(() => {
    const controller = new AbortController()

    fetch('/api/health', { signal: controller.signal })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Health check failed: ${response.status}`)
        }
        setApiState('online')
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === 'AbortError') {
          return
        }
        setApiState('offline')
      })

    return () => controller.abort()
  }, [])

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
        <span aria-hidden="true" />
        {statusText}
      </p>
    </main>
  )
}

export default App
