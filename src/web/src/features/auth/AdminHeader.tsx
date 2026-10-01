import { Link } from 'react-router-dom'
import { useAppDispatch, useAppSelector } from '../../app/hooks'
import { logout } from './authSlice'

interface AdminHeaderProps {
  title: string
  backTo?: string
}

function AdminHeader({ title, backTo }: AdminHeaderProps) {
  const dispatch = useAppDispatch()
  const username = useAppSelector((state) => state.auth.admin?.username)

  return (
    <header className="admin-header">
      <div>
        {backTo ? (
          <Link className="admin-header__back" to={backTo}>
            ← 返回内容管理
          </Link>
        ) : (
          <p className="eyebrow">ADMIN · {username}</p>
        )}
        <h1>{title}</h1>
      </div>
      <button
        className="button-secondary"
        type="button"
        onClick={() => void dispatch(logout())}
      >
        退出登录
      </button>
    </header>
  )
}

export default AdminHeader
