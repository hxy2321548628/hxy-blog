import { lazy, Suspense } from 'react'
import { Route, Routes, useLocation } from 'react-router-dom'
import BackgroundMusic from './components/BackgroundMusic'
import AdminGuard from './features/auth/AdminGuard'
import SessionBootstrap from './features/auth/SessionBootstrap'
import PostListPage from './features/posts/PostListPage'

const PostDetailPage = lazy(() => import('./features/posts/PostDetailPage'))
const LoginPage = lazy(() => import('./features/auth/LoginPage'))
const AdminPage = lazy(() => import('./features/auth/AdminPage'))
const AdminEditorPage = lazy(
  () => import('./features/posts/AdminEditorPage'),
)

function PageLoading() {
  return (
    <main className="article-shell">
      <p className="request-state" role="status">
        正在加载页面…
      </p>
    </main>
  )
}

function App() {
  const { pathname } = useLocation()
  const isPublicRoute = !pathname.startsWith('/admin')

  return (
    // 公开页使用独立的阅读配色，管理端沿用全局令牌；音乐控件随公开路由挂载。
    <div className={isPublicRoute ? 'public-app' : undefined}>
      <SessionBootstrap />
      {isPublicRoute ? <BackgroundMusic /> : null}
      <Routes>
        <Route path="/" element={<PostListPage />} />
        <Route
          path="/posts/:slug"
          element={
            <Suspense fallback={<PageLoading />}>
              <PostDetailPage />
            </Suspense>
          }
        />
        <Route
          path="/admin/login"
          element={
            <Suspense fallback={<PageLoading />}>
              <LoginPage />
            </Suspense>
          }
        />
        <Route element={<AdminGuard />}>
          <Route
            path="/admin"
            element={
              <Suspense fallback={<PageLoading />}>
                <AdminPage />
              </Suspense>
            }
          />
          <Route
            path="/admin/posts/:id"
            element={
              <Suspense fallback={<PageLoading />}>
                <AdminEditorPage />
              </Suspense>
            }
          />
        </Route>
        <Route path="*" element={<PostListPage />} />
      </Routes>
    </div>
  )
}

export default App
