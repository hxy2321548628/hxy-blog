import { lazy, Suspense } from 'react'
import { Route, Routes } from 'react-router-dom'
import PostListPage from './features/posts/PostListPage'

const PostDetailPage = lazy(() => import('./features/posts/PostDetailPage'))

function App() {
  return (
    <Routes>
      <Route path="/" element={<PostListPage />} />
      <Route
        path="/posts/:slug"
        element={
          <Suspense
            fallback={
              <main className="article-shell">
                <p className="request-state" role="status">
                  正在加载页面…
                </p>
              </main>
            }
          >
            <PostDetailPage />
          </Suspense>
        }
      />
      <Route path="*" element={<PostListPage />} />
    </Routes>
  )
}

export default App
