import { Navigate } from 'react-router-dom'
import { useAppDispatch, useAppSelector } from '../../app/hooks'
import { logout } from './authSlice'

function AdminPage() {
  const dispatch = useAppDispatch()
  const { status, admin } = useAppSelector((state) => state.auth)

  if (status === 'checking') {
    return (
      <main className="auth-shell">
        <p className="request-state" role="status">
          正在确认登录状态…
        </p>
      </main>
    )
  }
  if (status === 'anonymous') {
    return <Navigate to="/admin/login" replace />
  }

  return (
    <main className="admin-shell">
      <header className="admin-header">
        <div>
          <p className="eyebrow">ADMIN</p>
          <h1>内容管理</h1>
        </div>
        <button
          className="button-secondary"
          type="button"
          onClick={() => void dispatch(logout())}
        >
          退出登录
        </button>
      </header>

      <section className="admin-welcome" aria-labelledby="welcome-title">
        <h2 id="welcome-title">欢迎，{admin?.username}</h2>
        <p>登录链路已就绪。文章编辑与发布功能将在下一步加入这里。</p>
      </section>
    </main>
  )
}

export default AdminPage
