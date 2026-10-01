import { Navigate, Outlet } from 'react-router-dom'
import { useAppSelector } from '../../app/hooks'

function AdminGuard() {
  const status = useAppSelector((state) => state.auth.status)

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
  return <Outlet />
}

export default AdminGuard
