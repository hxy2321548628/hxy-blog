import { lazy, Suspense } from 'react'
import { Route, Routes } from 'react-router-dom'
import SessionBootstrap from './features/auth/SessionBootstrap'
import PostListPage from './features/posts/PostListPage'

const PostDetailPage = lazy(() => import('./features/posts/PostDetailPage'))
const LoginPage = lazy(() => import('./features/auth/LoginPage'))
const AdminPage = lazy(() => import('./features/auth/AdminPage'))

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
  return (
    <>
      <SessionBootstrap />
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
        <Route
          path="/admin"
          element={
            <Suspense fallback={<PageLoading />}>
              <AdminPage />
            </Suspense>
          }
        />
        <Route path="*" element={<PostListPage />} />
      </Routes>
    </>
  )
}

export default App
